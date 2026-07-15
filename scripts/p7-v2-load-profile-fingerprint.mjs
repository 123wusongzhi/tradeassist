import crypto from 'node:crypto';
import path from 'node:path';

export const LOAD_PROFILE_FINGERPRINT_VERSION = 2;

const DURATION_UNITS = new Map([
  ['ms', 1],
  ['s', 1_000],
  ['m', 60_000],
  ['h', 3_600_000],
]);

export class InvalidLoadProfileError extends Error {
  constructor(message, details = []) {
    super(message);
    this.name = 'InvalidLoadProfileError';
    this.details = details;
  }
}

function sha256(value) {
  return crypto.createHash('sha256').update(value, 'utf8').digest('hex');
}

export function stableObject(value) {
  if (Array.isArray(value)) return value.map(stableObject);
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.keys(value).sort().map((key) => [key, stableObject(value[key])]));
  }
  return value;
}

export function normalizeDuration(value, field = 'duration') {
  if (typeof value === 'number' && Number.isInteger(value) && value >= 0) return value;
  if (typeof value !== 'string') throw new InvalidLoadProfileError(`${field} must be a non-negative duration`);
  const match = value.trim().match(/^(\d+)(ms|s|m|h)?$/i);
  if (!match) throw new InvalidLoadProfileError(`${field} has an invalid duration: ${value}`);
  const amount = Number(match[1]);
  const unit = (match[2] || 'ms').toLowerCase();
  if (!Number.isSafeInteger(amount) || !DURATION_UNITS.has(unit)) throw new InvalidLoadProfileError(`${field} has an invalid duration: ${value}`);
  return amount * DURATION_UNITS.get(unit);
}

function normalizeInteger(value, field) {
  const normalized = typeof value === 'string' && /^\d+$/.test(value.trim()) ? Number(value) : value;
  if (!Number.isInteger(normalized) || normalized < 0) throw new InvalidLoadProfileError(`${field} must be a non-negative integer`);
  return normalized;
}

function normalizeWeight(value, field) {
  const normalized = typeof value === 'string' && /^\d+(\.\d+)?$/.test(value.trim()) ? Number(value) : value;
  if (!Number.isFinite(normalized) || normalized < 0) throw new InvalidLoadProfileError(`${field} must be a non-negative number`);
  return normalized;
}

export function repoRelativePosixPath(value, repositoryRoot = '') {
  if (!value || typeof value !== 'string') throw new InvalidLoadProfileError('load script path is required');
  const normalized = value.replaceAll('\\', '/');
  const root = repositoryRoot.replaceAll('\\', '/').replace(/\/+$/, '');
  const candidates = [
    root && normalized.startsWith(`${root}/`) ? normalized.slice(root.length + 1) : '',
    normalized.match(/(?:^|\/)(tests\/load\/.+)$/)?.[1] || '',
    normalized.match(/(?:^|\/)(scripts\/.+)$/)?.[1] || '',
  ].filter(Boolean);
  const relative = candidates[0] || normalized.replace(/^[A-Za-z]:\//, '').replace(/^\/mnt\/[a-z]\//i, '');
  if (path.posix.isAbsolute(relative) || relative.startsWith('../') || relative.includes('/../')) {
    throw new InvalidLoadProfileError(`load script path is not repository-relative: ${value}`);
  }
  return relative;
}

function requiredString(value, field) {
  if (typeof value !== 'string' || !value.trim()) throw new InvalidLoadProfileError(`${field} is required`);
  return value.trim();
}

function normalizeStages(profile) {
  const explicit = Array.isArray(profile.stages) ? profile.stages : [
    { name: 'warmup', duration: profile.warmup, targetVUs: profile.targetVUs },
    { name: 'ramp', duration: profile.ramp, targetVUs: profile.targetVUs },
    { name: 'steady', duration: profile.steady, targetVUs: profile.targetVUs },
    { name: 'rampdown', duration: profile.rampdown, targetVUs: 0 },
  ];
  if (!explicit.length) throw new InvalidLoadProfileError('at least one load stage is required');
  return explicit.map((stage, index) => ({
    name: requiredString(stage.name || ['warmup', 'ramp', 'steady', 'rampdown'][index], `stages[${index}].name`),
    durationMs: normalizeDuration(stage.durationMs ?? stage.duration, `stages[${index}].duration`),
    targetVUs: normalizeInteger(stage.targetVUs ?? stage.target ?? profile.targetVUs, `stages[${index}].targetVUs`),
  }));
}

function normalizeScenarios(scenarios = []) {
  if (!Array.isArray(scenarios)) throw new InvalidLoadProfileError('scenarios must be an array');
  return scenarios.map((scenario, index) => ({
    name: requiredString(scenario.name, `scenarios[${index}].name`),
    executor: requiredString(scenario.executor, `scenarios[${index}].executor`),
    startTimeMs: normalizeDuration(scenario.startTimeMs ?? scenario.startTime ?? 0, `scenarios[${index}].startTime`),
    gracefulStopMs: normalizeDuration(scenario.gracefulStopMs ?? scenario.gracefulStop ?? 0, `scenarios[${index}].gracefulStop`),
    ...(scenario.weight === undefined ? {} : { weight: normalizeWeight(scenario.weight, `scenarios[${index}].weight`) }),
    ...(scenario.stages === undefined ? {} : { stages: normalizeStages({ stages: scenario.stages, targetVUs: scenario.vus ?? 0 }) }),
  }));
}

function normalizeWeightedSet(items, label, keys) {
  if (!Array.isArray(items)) throw new InvalidLoadProfileError(`${label} must be an array`);
  return items.map((item, index) => {
    const normalized = Object.fromEntries(keys.map((key) => [key, requiredString(item[key], `${label}[${index}].${key}`)]));
    return { ...normalized, weight: normalizeWeight(item.weight, `${label}[${index}].weight`) };
  }).sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b)));
}

function normalizeLibraries(libraries = [], repositoryRoot) {
  if (!Array.isArray(libraries)) throw new InvalidLoadProfileError('loadLibraries must be an array');
  return libraries.map((library, index) => ({
    relativePath: repoRelativePosixPath(library.relativePath || library.path, repositoryRoot),
    sha256: requiredString(library.sha256, `loadLibraries[${index}].sha256`),
  })).sort((a, b) => a.relativePath.localeCompare(b.relativePath));
}

export function canonicalizeLoadProfile(input, { repositoryRoot = '', loadScript = null, loadLibraries = [] } = {}) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) throw new InvalidLoadProfileError('load profile must be an object');
  const supported = new Set(['kind', 'targetVUs', 'configuredVUs', 'warmup', 'ramp', 'steady', 'rampdown', 'stages', 'scenarios', 'requestMix', 'credentialMix', 'loadScript', 'loadLibraries', 'databaseName', 'runId', 'createdAt', 'startedAt', 'finishedAt']);
  const unknownFields = Object.keys(input).filter((key) => !supported.has(key)).sort();
  const configuredVUs = normalizeInteger(input.configuredVUs ?? input.targetVUs, 'configuredVUs');
  if (configuredVUs <= 0) throw new InvalidLoadProfileError('configuredVUs must be greater than zero');
  const script = input.loadScript || loadScript;
  if (!script?.sha256) throw new InvalidLoadProfileError('loadScript.sha256 is required');
  const canonical = {
    schemaVersion: LOAD_PROFILE_FINGERPRINT_VERSION,
    loadEngine: { name: 'k6' },
    load: { configuredVUs, stages: normalizeStages(input) },
    scenarios: normalizeScenarios(input.scenarios || []),
    requestMix: normalizeWeightedSet(input.requestMix || [], 'requestMix', ['routeId', 'method']),
    credentialMix: normalizeWeightedSet(input.credentialMix || [], 'credentialMix', ['role']),
    loadScript: {
      relativePath: repoRelativePosixPath(script.relativePath || script.path, repositoryRoot),
      sha256: requiredString(script.sha256, 'loadScript.sha256'),
    },
    loadLibraries: normalizeLibraries(input.loadLibraries ?? loadLibraries, repositoryRoot),
  };
  return { canonicalProfile: stableObject(canonical), unknownFields };
}

export function calculateLoadProfileFingerprint(input, options = {}) {
  const { canonicalProfile, unknownFields } = canonicalizeLoadProfile(input, options);
  const canonicalJson = JSON.stringify(canonicalProfile);
  return {
    fingerprintVersion: LOAD_PROFILE_FINGERPRINT_VERSION,
    canonicalProfile,
    canonicalJson,
    canonicalJsonSha256: sha256(canonicalJson),
    loadProfileFingerprint: sha256(canonicalJson),
    unknownFields,
  };
}

function walkDiff(baseline, current, currentPath = '$', differences = []) {
  if (JSON.stringify(baseline) === JSON.stringify(current)) return differences;
  if (Array.isArray(baseline) || Array.isArray(current) || typeof baseline !== 'object' || typeof current !== 'object' || baseline === null || current === null) {
    differences.push({ jsonPath: currentPath, baselineNormalizedValue: baseline, currentNormalizedValue: current, classification: 'semantic_load_difference', semanticImpact: true });
    return differences;
  }
  for (const key of [...new Set([...Object.keys(baseline), ...Object.keys(current)])].sort()) {
    walkDiff(baseline[key], current[key], `${currentPath}.${key}`, differences);
  }
  return differences;
}

export function diffCanonicalLoadProfiles(baseline, current) {
  const baselineResult = baseline?.canonicalProfile ? baseline : calculateLoadProfileFingerprint(baseline);
  const currentResult = current?.canonicalProfile ? current : calculateLoadProfileFingerprint(current);
  return walkDiff(baselineResult.canonicalProfile, currentResult.canonicalProfile);
}

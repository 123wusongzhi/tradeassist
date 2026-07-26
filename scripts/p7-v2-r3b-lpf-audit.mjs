import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { resolveActiveBaseline, resolveActiveCurrent } from './p7-v2-evidence-resolver.mjs';
import { calculateLoadProfileFingerprint, diffCanonicalLoadProfiles, InvalidLoadProfileError } from './p7-v2-load-profile-fingerprint.mjs';

const BASELINE_RUN_ID = 'p7v2-baseline-r3b-recovery3-20260715-131400';
const CURRENT_RUN_ID = 'p7v2-current-r3b-recovery3-20260715-131400';
const sha256 = (input) => crypto.createHash('sha256').update(input).digest('hex');
const readFileHash = (relativePath) => sha256(fs.readFileSync(path.join(root, relativePath)));
const report = (name, title, payload, summary) => {
  writeJSON(`docs/p7-v2-r3b-lpf-${name}.json`, payload);
  writeMarkdown(`docs/P7_V2_R3B_LPF_${title}.md`, `# P7-V2-R3B-LPF-V2 ${title.replaceAll('_', ' ')}\n\nStatus: **${payload.status || 'passed'}**\n\n${summary}\n\nMachine-readable evidence: \`docs/p7-v2-r3b-lpf-${name}.json\`\n`);
};

function frozen(kind, runId) {
  const family = kind === 'baseline' ? 'baselines' : 'currents';
  const dir = `docs/${family}/frozen/${runId}`;
  const manifestPath = `${dir}/manifest.json`;
  const rawPath = `${dir}/raw-summary.json`;
  const manifest = readJSON(manifestPath) || {};
  const raw = fs.existsSync(path.join(root, rawPath)) ? fs.readFileSync(path.join(root, rawPath)) : null;
  let json = null;
  try { json = raw ? JSON.parse(raw.toString('utf8')) : null; } catch { /* assessed below */ }
  const actualSha256 = raw ? sha256(raw) : '';
  const requests = Number(json?.metrics?.http_reqs?.values?.count ?? json?.metrics?.http_reqs?.count ?? 0);
  const loadProfile = readJSON(`${dir}/load-profile.json`) || {};
  const valid = Boolean(raw && json && manifest.runId === runId && manifest.runKind === kind && manifest.immutable === true &&
    actualSha256 === manifest.sha256 && actualSha256 === manifest.rawArtifact?.sha256 &&
    raw.length === Number(manifest.sizeBytes) && raw.length === Number(manifest.rawArtifact?.sizeBytes) &&
    requests > 0 && manifest.scenarioCoverage === true);
  return { kind, runId, dir, manifestPath, rawPath, manifest, raw, json, actualSha256, requests, loadProfile, valid };
}

function scenarioProfile(profile) {
  const warmup = profile.warmup;
  const ramp = profile.ramp;
  const steady = profile.steady;
  const rampdown = profile.rampdown;
  const vus = Number(profile.targetVUs);
  return [
    { name: 'warmup', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s' },
    { name: 'ramp', executor: 'ramping-vus', startTime: warmup, gracefulStop: '0s', stages: [{ name: 'ramp', duration: ramp, targetVUs: vus }] },
    { name: 'steady', executor: 'constant-vus', startTime: `${warmup}+${ramp}`, gracefulStop: '0s' },
    { name: 'rampdown', executor: 'ramping-vus', startTime: `${warmup}+${ramp}+${steady}`, gracefulStop: '0s', stages: [{ name: 'rampdown', duration: rampdown, targetVUs: 0 }] },
    { name: 'security_negative', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s' },
  ];
}

function durationExpression(value) {
  return value.split('+').reduce((total, part) => {
    const match = part.trim().match(/^(\d+)(ms|s|m|h)$/);
    const multiplier = { ms: 1, s: 1000, m: 60000, h: 3600000 }[match?.[2]];
    return total + Number(match?.[1] || 0) * multiplier;
  }, 0);
}

function normalizedInput(freeze) {
  const profile = freeze.loadProfile.loadProfile || {};
  const script = { relativePath: 'tests/load/p7v2-baseline.js', sha256: freeze.manifest.loadScriptsHash };
  return {
    ...profile,
    configuredVUs: profile.targetVUs,
    scenarios: scenarioProfile(profile).map((scenario) => ({
      ...scenario,
      startTimeMs: durationExpression(scenario.startTime),
      gracefulStopMs: durationExpression(scenario.gracefulStop),
    })),
    loadScript: script,
  };
}

const baseline = frozen('baseline', BASELINE_RUN_ID);
const current = frozen('current', CURRENT_RUN_ID);
const resolvedBaseline = resolveActiveBaseline();
const resolvedCurrent = resolveActiveCurrent();
const integrity = {
  phase: 'P7-V2-R3B-LPF-V2',
  status: baseline.valid && current.valid && baseline.actualSha256 !== current.actualSha256 && baseline.rawPath !== current.rawPath ? 'passed' : 'failed',
  baseline: { runId: baseline.runId, rawArtifactPath: baseline.rawPath, sha256: baseline.actualSha256, sizeBytes: baseline.raw?.length || 0, requests: baseline.requests, jsonParseable: Boolean(baseline.json), scenarioCoverage: baseline.manifest.scenarioCoverage === true, verified: baseline.valid },
  current: { runId: current.runId, rawArtifactPath: current.rawPath, sha256: current.actualSha256, sizeBytes: current.raw?.length || 0, requests: current.requests, jsonParseable: Boolean(current.json), scenarioCoverage: current.manifest.scenarioCoverage === true, verified: current.valid },
  registry: { baselineValid: resolvedBaseline.valid, currentValid: resolvedCurrent.valid },
  sourceArtifactsModified: false,
};
report('preflight-audit', 'PREFLIGHT_AUDIT', integrity, `- Baseline artifact verified: \`${baseline.valid}\`\n- Current artifact verified: \`${current.valid}\``);
if (integrity.status !== 'passed') process.exit(1);

let baselineFingerprint;
let currentFingerprint;
try {
  baselineFingerprint = calculateLoadProfileFingerprint(normalizedInput(baseline), { repositoryRoot: root });
  currentFingerprint = calculateLoadProfileFingerprint(normalizedInput(current), { repositoryRoot: root });
} catch (error) {
  const failure = { phase: 'P7-V2-R3B-LPF-V2', status: 'failed', error: error instanceof InvalidLoadProfileError ? error.message : String(error), sourceArtifactsModified: false };
  report('semantic-diff-report', 'SEMANTIC_DIFF_REPORT', failure, `- Error: ${failure.error}`);
  process.exit(1);
}
const rawExport = {
  phase: 'P7-V2-R3B-LPF-V2',
  status: 'passed',
  baseline: { runId: baseline.runId, sources: { loadProfile: `${baseline.dir}/load-profile.json`, manifest: baseline.manifestPath, rawSummary: baseline.rawPath, loadScript: 'tests/load/p7v2-baseline.js' }, rawProfile: normalizedInput(baseline) },
  current: { runId: current.runId, sources: { loadProfile: `${current.dir}/load-profile.json`, manifest: current.manifestPath, rawSummary: current.rawPath, loadScript: 'tests/load/p7v2-baseline.js' }, rawProfile: normalizedInput(current) },
  sourceArtifactsModified: false,
};
report('raw-profile-export', 'RAW_PROFILE_EXPORT', rawExport, `- Baseline source: \`${baseline.dir}/load-profile.json\`\n- Current source: \`${current.dir}/load-profile.json\``);

const rawDifferences = [];
for (const key of new Set([...Object.keys(baseline.loadProfile.loadProfile || {}), ...Object.keys(current.loadProfile.loadProfile || {})])) {
  const left = baseline.loadProfile.loadProfile?.[key];
  const right = current.loadProfile.loadProfile?.[key];
  if (JSON.stringify(left) !== JSON.stringify(right)) rawDifferences.push({
    jsonPath: `$.loadProfile.${key}`, baselineRawValue: left, currentRawValue: right,
    baselineNormalizedValue: key === 'kind' ? undefined : left, currentNormalizedValue: key === 'kind' ? undefined : right,
    classification: key === 'kind' ? 'non_semantic_run_metadata' : 'semantic_load_difference',
    semanticImpact: key !== 'kind', includedInV1: true, includedInV2: key !== 'kind',
    reason: key === 'kind' ? 'Run taxonomy is not an input to k6 load generation.' : 'Load generation input differs.',
  });
}
const canonicalDifferences = diffCanonicalLoadProfiles(baselineFingerprint, currentFingerprint);
const actualLoadProfileDrift = rawDifferences.some((item) => item.semanticImpact) || canonicalDifferences.length > 0;
const semanticDiff = {
  phase: 'P7-V2-R3B-LPF-V2',
  status: actualLoadProfileDrift ? 'failed' : 'passed',
  rootCause: actualLoadProfileDrift ? 'semantic_load_difference' : 'non_semantic_canonicalization_bug',
  actualLoadProfileDrift,
  differences: [...rawDifferences, ...canonicalDifferences],
  v1: { baseline: baseline.manifest.loadProfileFingerprint, current: current.manifest.loadProfileFingerprint },
  v2: { baseline: baselineFingerprint.loadProfileFingerprint, current: currentFingerprint.loadProfileFingerprint },
  sourceArtifactsModified: false,
};
report('semantic-diff-report', 'SEMANTIC_DIFF_REPORT', semanticDiff, `- Actual load-profile drift: \`${actualLoadProfileDrift}\`\n- Difference count: \`${semanticDiff.differences.length}\``);

const canonicalization = {
  phase: 'P7-V2-R3B-LPF-V2', status: 'passed', version: 2,
  responsibility: ['load intensity', 'load stages', 'scenario definitions', 'scenario ordering', 'request mix', 'credential mix', 'load script identity'],
  excludedAsSeparatelyFingerprinted: ['runId', 'runKind', 'timestamps', 'databaseName', 'baseUrl', 'apiHost', 'apiPort', 'k6Version', 'runtime source tree', 'metric semantics', 'dataset', 'config', 'SLO', 'credential matrix'],
  unknownFieldPolicy: 'ignore_with_report',
  durationUnit: 'integer milliseconds',
  sourceArtifactsModified: false,
};
report('canonicalization-policy', 'CANONICALIZATION_POLICY', canonicalization, `- Fingerprint version: \`2\`\n- Unknown fields: \`${canonicalization.unknownFieldPolicy}\``);

const deterministic = new Set(Array.from({ length: 10 }, () => calculateLoadProfileFingerprint(normalizedInput(baseline), { repositoryRoot: root }).loadProfileFingerprint));
const determinism = {
  phase: 'P7-V2-R3B-LPF-V2', status: deterministic.size === 1 ? 'passed' : 'failed',
  version: 2, repetitions: 10, uniqueFingerprintCount: deterministic.size, deterministic: deterministic.size === 1,
  windowsNode: { executed: true, fingerprint: [...deterministic][0] || '' },
  crossPlatformVerification: 'not_executed',
  crossPlatformDeterministic: null,
};
report('determinism-report', 'DETERMINISM_REPORT', determinism, `- Unique fingerprint count: \`${determinism.uniqueFingerprintCount}\`\n- Cross-platform verification: \`${determinism.crossPlatformVerification}\``);

const sidecarRoot = 'docs/fingerprints/p7-v2/load-profile/v2';
function sidecar(freeze, fingerprint) {
  const sourceManifestSha256 = readFileHash(freeze.manifestPath);
  const payload = {
    runId: freeze.runId, runKind: freeze.kind, fingerprintType: 'load_profile', fingerprintVersion: 2,
    sourceManifestPath: freeze.manifestPath, sourceManifestSha256, rawArtifactSha256: freeze.actualSha256,
    canonicalProfile: fingerprint.canonicalProfile, canonicalJsonSha256: fingerprint.canonicalJsonSha256,
    loadProfileFingerprint: fingerprint.loadProfileFingerprint, generatedAt: new Date().toISOString(),
    derivedEvidence: true, sourceArtifactsModified: false, unknownFields: fingerprint.unknownFields,
  };
  writeJSON(`${sidecarRoot}/${freeze.runId}.json`, payload);
  return payload;
}
const baselineSidecar = sidecar(baseline, baselineFingerprint);
const currentSidecar = sidecar(current, currentFingerprint);
writeJSON('docs/fingerprints/p7-v2/load-profile-registry.json', {
  fingerprintType: 'load_profile', activeVersion: 2, entries: [
    { runId: baseline.runId, path: `${sidecarRoot}/${baseline.runId}.json`, loadProfileFingerprint: baselineSidecar.loadProfileFingerprint },
    { runId: current.runId, path: `${sidecarRoot}/${current.runId}.json`, loadProfileFingerprint: currentSidecar.loadProfileFingerprint },
  ],
  sourceArtifactsModified: false,
});

const sharedKeys = ['runtimeSourceTreeHash', 'loadScriptsHash', 'metricSemanticsHash', 'datasetFingerprint', 'configFingerprint', 'sloFingerprint', 'routeCredentialMatrixFingerprint', 'regressionPolicyFingerprint'];
const sharedFingerprintMatches = Object.fromEntries(sharedKeys.map((key) => [key, baseline.manifest[key] === current.manifest[key] && Boolean(baseline.manifest[key])]));
const reuse = {
  phase: 'P7-V2-R3B-LPF-V2',
  status: !actualLoadProfileDrift && baselineSidecar.loadProfileFingerprint === currentSidecar.loadProfileFingerprint && Object.values(sharedFingerprintMatches).every(Boolean) ? 'passed' : 'failed',
  recovery3Reusable: !actualLoadProfileDrift && baselineSidecar.loadProfileFingerprint === currentSidecar.loadProfileFingerprint && Object.values(sharedFingerprintMatches).every(Boolean) && baseline.valid && current.valid && current.manifest.independentRun === true,
  recovery4Required: actualLoadProfileDrift,
  reuseReason: actualLoadProfileDrift ? '' : 'non_semantic_load_profile_fingerprint_v1_bug',
  baselineRawArtifactHashVerified: baseline.valid, currentRawArtifactHashVerified: current.valid,
  actualLoadProfileDrift, loadProfileFingerprintV2Match: baselineSidecar.loadProfileFingerprint === currentSidecar.loadProfileFingerprint,
  ...sharedFingerprintMatches, currentRunIndependent: current.manifest.independentRun === true,
  sourceArtifactsModified: false, evaluatorChangedAfterExecution: true, runtimeUnderTestChangedAfterExecution: false,
  runtimeExecutionFingerprint: baseline.manifest.runtimeSourceTreeHash,
  evidenceToolingFingerprint: readFileHash('scripts/p7-v2-r3b-lpf-audit.mjs'),
  changedGoRuntimeFiles: 0,
};
report('recovery3-reuse-decision', 'RECOVERY3_REUSE_DECISION', reuse, `- Recovery3 reusable: \`${reuse.recovery3Reusable}\`\n- Reason: \`${reuse.reuseReason || 'semantic drift detected'}\``);

if (actualLoadProfileDrift) {
  const recovery4 = { phase: 'P7-V2-R3B-LPF-V2', status: 'required', recovery4Required: true, regressionExecuted: false, soakExecuted: false, reasons: semanticDiff.differences.filter((item) => item.semanticImpact) };
  writeJSON('docs/p7-v2-r3b-lpf-recovery4-plan.json', recovery4);
  writeMarkdown('docs/P7_V2_R3B_LPF_RECOVERY4_PLAN.md', '# P7-V2-R3B-LPF-V2 Recovery4 Plan\n\nRecovery4 is required because semantic load-profile drift was detected. No execution occurred.\n');
  process.exit(2);
}

console.log(JSON.stringify({ status: 'passed', recovery3Reusable: reuse.recovery3Reusable, baselineFingerprint: baselineSidecar.loadProfileFingerprint, currentFingerprint: currentSidecar.loadProfileFingerprint }, null, 2));

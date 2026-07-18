import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { calculateLoadProfileFingerprint } from './p7-v2-load-profile-fingerprint.mjs';
import { readJSON, resolveP7V2PortConfig, root, run } from './p7-v2-lib.mjs';

export const RUNTIME_FREEZE_SCOPE_VERSION = 3;
export const CONFIG_FINGERPRINT_VERSION = 2;
export const CANONICAL_SCHEMA_VERSION = 3;
export const LOAD_PROFILE_FINGERPRINT_VERSION = 3;
export const FORMAL_DATASET_PROFILE = 'medium';
export const FORMAL_DATASET_EXPECTED_ROWS = 1_900_150;

const GENERATED_PREFIXES = ['docs/', 'artifacts/', 'logs/', 'tmp/', 'data/'];
const IGNORED_PARTS = new Set(['.git', 'node_modules', 'dist', 'artifacts', 'docs', 'data', 'logs', 'tmp']);
const RUNTIME_ROOTS = ['backend', 'tests/load', 'scripts', 'package.json', 'pnpm-lock.yaml', '.env.example', '.env.docker.example', 'docker-compose.yml', 'docker-compose.full.yml'];
const FORMAL_TEMPLATE_FILES = new Set(['package.json', 'pnpm-lock.yaml', '.env.example', '.env.docker.example', 'docker-compose.yml', 'docker-compose.full.yml']);
const IMMUTABLE_INPUT_DOCS = new Set([
  'docs/p7-v2-r3b-formal-binary-provenance-manifest.json',
  'docs/p7-v2-r3b-formal-input-sequence-manifest.json',
  'docs/p7-v2-r3b-lpc-r3-preflight-audit.json',
  'docs/p7-v2-r3b-lpc-r3-determinism-report.json',
  'docs/p7-v2-r3b-lpc-r3-consumer-compatibility.json',
  'docs/p7-v2-r3b-formal-binary-provenance-final-gate.json',
]);
const GENERATED_EVIDENCE_PATHS = new Set([
  'docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json',
  'docs/P7_V2_R3B_FAST_CLOSE_R3_RUNTIME_FREEZE.md',
  'docs/p7-v2-r3b-runtime-freeze-revalidation.json',
  'docs/p7-v2-r3b-preflight-audit.json',
  'docs/P7_V2_R3B_PREFLIGHT_AUDIT.md',
  'docs/p7-v2-r3b-precommit-runtime-freeze-closeout.json',
  'docs/P7_V2_R3B_PRECOMMIT_RUNTIME_FREEZE_CLOSEOUT.md',
  'docs/p7-v2-r3b-clean-head-runtime-freeze-final-gate.json',
  'docs/P7_V2_R3B_CLEAN_HEAD_RUNTIME_FREEZE_FINAL_GATE.md',
]);
const IMMUTABLE_DIFF_PATHS = [
  'backend',
  'tests/load',
  ':(glob)scripts/p7-v2-*.mjs',
  ':(glob)tests/gates/p7-v2/**/*.mjs',
  ...IMMUTABLE_INPUT_DOCS,
  'package.json',
  'pnpm-lock.yaml',
  '.env.example',
  '.env.docker.example',
  'docker-compose.yml',
  'docker-compose.full.yml',
];

export function sha256Bytes(value) {
  return crypto.createHash('sha256').update(value).digest('hex');
}

export function stableJson(value) {
  if (Array.isArray(value)) return `[${value.map((entry) => stableJson(entry)).join(',')}]`;
  if (value && typeof value === 'object') {
    return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${stableJson(value[key])}`).join(',')}}`;
  }
  return JSON.stringify(value);
}

export function sha256Json(value) {
  return sha256Bytes(stableJson(value));
}

export function repoRelativePath(filePath, repositoryRoot = root) {
  const rel = path.relative(repositoryRoot, path.resolve(repositoryRoot, filePath));
  return rel.replaceAll('\\', '/');
}

export function isRuntimeSourcePath(relPath) {
  const rel = relPath.replaceAll('\\', '/');
  return (
    (rel.startsWith('backend/') && (rel.endsWith('.go') || rel === 'backend/go.mod' || rel === 'backend/go.sum')) ||
    rel.startsWith('tests/load/') ||
    /^scripts\/p7-v2-.*\.mjs$/.test(rel) ||
    FORMAL_TEMPLATE_FILES.has(rel)
  );
}

export function isEvidenceToolingPath(relPath) {
  const rel = relPath.replaceAll('\\', '/');
  return /^scripts\/p7-v2-.*\.mjs$/.test(rel) || /^tests\/gates\/p7-v2\/.*\.mjs$/.test(rel) || FORMAL_TEMPLATE_FILES.has(rel);
}

export function isImmutableRuntimeInputPath(relPath) {
  const rel = relPath.replaceAll('\\', '/');
  return isRuntimeSourcePath(rel) || isEvidenceToolingPath(rel) || IMMUTABLE_INPUT_DOCS.has(rel);
}

export function isGeneratedEvidencePath(relPath) {
  const rel = relPath.replaceAll('\\', '/');
  return GENERATED_EVIDENCE_PATHS.has(rel) ||
    rel.startsWith('artifacts/') ||
    rel.startsWith('logs/') ||
    rel.startsWith('tmp/') ||
    rel.startsWith('data/') ||
    (/^docs\/(baselines|currents|fingerprints|runs|regressions)\//.test(rel) && !IMMUTABLE_INPUT_DOCS.has(rel));
}

export function classifyFreezePath(relPath) {
  const rel = relPath.replaceAll('\\', '/');
  if (isImmutableRuntimeInputPath(rel)) {
    return { classification: 'immutable_execution_input', reason: 'formal runtime, tooling, plan, binary, or input binding is included in clean-head scope' };
  }
  if (isGeneratedEvidencePath(rel)) {
    return { classification: 'generated_evidence_output', reason: 'runtime evidence and artifacts are generated outputs' };
  }
  if (rel === 'docs/PROGRESS.md' || rel === 'PROGRESS.md') {
    return { classification: 'generated_evidence_output', reason: 'progress report is generated closure evidence' };
  }
  if (rel === 'docs/p7-v2-r3b-run-manifest.json' || /\/registry\.json$/.test(rel) || /docs\/(baselines|currents|fingerprints)\//.test(rel)) {
    return { classification: 'mutable_execution_state', reason: 'manifest lifecycle fields and registry pointers mutate during formal execution' };
  }
  if (GENERATED_PREFIXES.some((prefix) => rel.startsWith(prefix))) {
    return { classification: 'generated_evidence_output', reason: 'runtime evidence and artifacts are generated outputs' };
  }
  return { classification: 'generated_evidence_output', reason: 'path is outside formal runtime freeze scope' };
}

function walkRepository(abs, rel, output, predicate) {
  if (!fs.existsSync(abs)) return;
  const stat = fs.statSync(abs);
  if (stat.isFile()) {
    const posix = rel.replaceAll('\\', '/');
    if (predicate(posix)) output.push({ path: posix, sha256: sha256Bytes(fs.readFileSync(abs)) });
    return;
  }
  for (const child of fs.readdirSync(abs).sort()) {
    if (IGNORED_PARTS.has(child)) continue;
    walkRepository(path.join(abs, child), path.posix.join(rel.replaceAll('\\', '/'), child), output, predicate);
  }
}

function manifestFromFileMap(fileMap, predicate) {
  return Object.entries(fileMap)
    .map(([filePath, body]) => ({ path: filePath.replaceAll('\\', '/'), body }))
    .filter((entry) => predicate(entry.path))
    .map((entry) => ({ path: entry.path, sha256: sha256Bytes(Buffer.isBuffer(entry.body) ? entry.body : String(entry.body)) }))
    .sort((a, b) => a.path.localeCompare(b.path));
}

function buildScopeManifest({ repositoryRoot = root, fileMap = null, predicate, scopeName }) {
  const includedFiles = fileMap
    ? manifestFromFileMap(fileMap, predicate)
    : RUNTIME_ROOTS.flatMap((rel) => {
        const files = [];
        walkRepository(path.join(repositoryRoot, rel), rel, files, predicate);
        return files;
      }).sort((a, b) => a.path.localeCompare(b.path));
  const payload = { scopeVersion: RUNTIME_FREEZE_SCOPE_VERSION, scopeName, includedFiles };
  return {
    ...payload,
    excludedPaths: ['docs/**', 'artifacts/**', 'logs/**', 'tmp/**', 'data/**', 'node_modules/**', 'dist/**'],
    manifestSha256: sha256Json(payload),
  };
}

export function buildRuntimeFreezeSourceManifest(options = {}) {
  return buildScopeManifest({ ...options, predicate: isRuntimeSourcePath, scopeName: 'runtime_source' });
}

export function buildEvidenceToolingManifest(options = {}) {
  return buildScopeManifest({ ...options, predicate: isEvidenceToolingPath, scopeName: 'evidence_tooling' });
}

export function buildScopedHash(fileMap, predicate) {
  return buildScopeManifest({ fileMap, predicate, scopeName: 'fixture' }).manifestSha256;
}

export function immutableTrackedDiffHash() {
  const diff = run('git', ['diff', '--binary', 'HEAD', '--', ...IMMUTABLE_DIFF_PATHS], { maxBuffer: 50 * 1024 * 1024 });
  const staged = run('git', ['diff', '--cached', '--name-only', '--', ...IMMUTABLE_DIFF_PATHS]);
  const unstaged = run('git', ['diff', '--name-only', '--', ...IMMUTABLE_DIFF_PATHS]);
  const untrackedAll = run('git', ['ls-files', '--others', '--exclude-standard', '--', ...IMMUTABLE_DIFF_PATHS]);
  const all = run('git', ['status', '--porcelain=v1', '--untracked-files=all']);
  const immutableStatus = run('git', ['status', '--porcelain=v1', '--untracked-files=all', '--', ...IMMUTABLE_DIFF_PATHS]);
  const split = (text) => (text || '').split(/\r?\n/).map((line) => line.trim()).filter(Boolean).sort();
  const summary = summarizeImmutableChangeSet({
    stagedPaths: split(staged.stdout),
    unstagedPaths: split(unstaged.stdout),
    untrackedPaths: split(untrackedAll.stdout),
  });
  return {
    algorithm: 'sha256',
    scopeVersion: RUNTIME_FREEZE_SCOPE_VERSION,
    scope: 'immutable_execution_inputs',
    hash: summary.immutableWorkingTreeClean ? null : sha256Bytes(diff.stdout || JSON.stringify(summary)),
    clean: summary.immutableWorkingTreeClean,
    ...summary,
    immutableScopeDirty: Boolean((immutableStatus.stdout || '').trim()),
    allRepositoryDirty: Boolean((all.stdout || '').trim()),
    commandStatus: diff.status,
    pathspecs: IMMUTABLE_DIFF_PATHS,
  };
}

export function summarizeImmutableChangeSet({ stagedPaths = [], unstagedPaths = [], untrackedPaths = [] } = {}) {
  const sort = (paths) => [...new Set(paths.map((entry) => String(entry).replaceAll('\\', '/')).filter(Boolean))].sort();
  const staged = sort(stagedPaths);
  const unstaged = sort(unstagedPaths);
  const untracked = sort(untrackedPaths);
  const immutableWorkingTreeClean = staged.length === 0 && unstaged.length === 0 && untracked.length === 0;
  return {
    immutableWorkingTreeClean,
    immutableTrackedDiffPresent: !immutableWorkingTreeClean,
    stagedImmutableChangeCount: staged.length,
    unstagedImmutableChangeCount: unstaged.length,
    untrackedImmutableChangeCount: untracked.length,
    stagedImmutablePaths: staged,
    unstagedImmutablePaths: unstaged,
    untrackedImmutablePaths: untracked,
  };
}

export function generatedEvidenceDiffAudit() {
  const status = run('git', ['status', '--porcelain=v1', '--untracked-files=all']);
  const rows = (status.stdout || '').split(/\r?\n/).filter(Boolean).map((row) => ({
    status: row.slice(0, 2),
    path: row.slice(3).replaceAll('\\', '/'),
  }));
  const unexpected = rows.filter((row) => !isGeneratedEvidencePath(row.path));
  return {
    workingTreeGloballyClean: rows.length === 0,
    generatedEvidenceExcluded: unexpected.length === 0,
    generatedEvidenceChangeCount: rows.length - unexpected.length,
    unexpectedChangeCount: unexpected.length,
    unexpectedPaths: unexpected.map((row) => row.path).sort(),
  };
}

export const CONFIG_FIELD_CLASSIFICATION = {
  'network.host': 'semantic',
  'network.port': 'semantic',
  'network.baseUrl': 'semantic',
  'environment.appEnv': 'semantic',
  'environment.performanceTestMode': 'semantic',
  'environment.externalProviderMode': 'semantic',
  'environment.douyinWriteEnabled': 'semantic',
  'environment.autoListingEnabled': 'semantic',
  'load.canonicalSchemaVersion': 'semantic',
  'load.loadProfileFingerprintVersion': 'semantic',
  'load.loadProfileFingerprint': 'semantic',
  'dataset.profile': 'semantic',
  'dataset.expectedRows': 'semantic',
  'dataset.datasetGeneratorHash': 'semantic',
  'policy.sloFingerprint': 'semantic',
  'policy.regressionPolicyFingerprint': 'semantic',
  'policy.routeCredentialMatrixFingerprint': 'semantic',
};

export function buildFormalLoadProfile() {
  return {
    configuredVUs: 10,
    stages: [
      { name: 'warmup', duration: '5m', targetVUs: 10 },
      { name: 'ramp', duration: '3m', targetVUs: 10 },
      { name: 'steady', duration: '10m', targetVUs: 10 },
      { name: 'rampdown', duration: '2m', targetVUs: 0 },
    ],
    scenarios: [
      { name: 'warmup', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s' },
      { name: 'ramp', executor: 'ramping-vus', startTime: '5m', gracefulStop: '0s' },
      { name: 'steady', executor: 'constant-vus', startTime: '8m', gracefulStop: '0s' },
      { name: 'rampdown', executor: 'ramping-vus', startTime: '18m', gracefulStop: '0s' },
      { name: 'security_negative', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s', weight: 1 },
    ],
    requestMix: [
      ['product_list', 20],
      ['order_list', 20],
      ['inventory_list', 15],
      ['task_list', 10],
      ['webhook_event_list', 8],
      ['operation_log_list', 7],
      ['webhook_ingestion', 5],
      ['provider_mock_flow', 5],
      ['auth_security', 2],
    ].map(([routeId, weight]) => ({ routeId, method: routeId === 'webhook_ingestion' ? 'POST' : 'GET', weight })),
    credentialMix: [
      { role: 'system_admin', weight: 1 },
      { role: 'tenant_admin', weight: 1 },
      { role: 'operator', weight: 1 },
      { role: 'readonly', weight: 1 },
    ],
    loadScript: {
      path: 'tests/load/p7v2-baseline.js',
      sha256: fs.existsSync(path.join(root, 'tests/load/p7v2-baseline.js')) ? sha256Bytes(fs.readFileSync(path.join(root, 'tests/load/p7v2-baseline.js'))) : '',
    },
  };
}

export function calculateFormalLoadProfileFingerprint(repositoryRoot = root) {
  return calculateLoadProfileFingerprint(buildFormalLoadProfile(), { repositoryRoot });
}

export function buildFormalConfigPayload({
  env = {},
  network = null,
  loadProfileFingerprint = '',
  datasetGeneratorHash = '',
  sloFingerprint = '',
  regressionPolicyFingerprint = '',
  routeCredentialMatrixFingerprint = '',
  datasetProfile = FORMAL_DATASET_PROFILE,
  expectedRows = FORMAL_DATASET_EXPECTED_ROWS,
} = {}) {
  const portConfig = network || resolveP7V2PortConfig({ ...process.env, ...env });
  return {
    version: CONFIG_FINGERPRINT_VERSION,
    network: {
      host: portConfig.host || '127.0.0.1',
      port: Number(portConfig.port || 18080),
      baseUrl: portConfig.baseUrl || `http://${portConfig.host || '127.0.0.1'}:${Number(portConfig.port || 18080)}`,
    },
    environment: {
      appEnv: String(env.APP_ENV ?? process.env.APP_ENV ?? 'performance'),
      performanceTestMode: String(env.PERFORMANCE_TEST_MODE ?? process.env.PERFORMANCE_TEST_MODE ?? 'true') === 'true',
      externalProviderMode: String(env.EXTERNAL_PROVIDER_MODE ?? process.env.EXTERNAL_PROVIDER_MODE ?? 'mock'),
      douyinWriteEnabled: String(env.DOUYIN_WRITE_ENABLED ?? process.env.DOUYIN_WRITE_ENABLED ?? 'false') === 'true',
      autoListingEnabled: String(env.AUTO_LISTING_ENABLED ?? process.env.AUTO_LISTING_ENABLED ?? 'false') === 'true',
    },
    load: {
      canonicalSchemaVersion: CANONICAL_SCHEMA_VERSION,
      loadProfileFingerprintVersion: LOAD_PROFILE_FINGERPRINT_VERSION,
      loadProfileFingerprint,
    },
    dataset: {
      profile: datasetProfile,
      expectedRows,
      datasetGeneratorHash,
    },
    policy: {
      sloFingerprint,
      regressionPolicyFingerprint,
      routeCredentialMatrixFingerprint,
    },
  };
}

export function buildFormalConfigFingerprint(options = {}) {
  const payload = buildFormalConfigPayload(options);
  return {
    version: CONFIG_FINGERPRINT_VERSION,
    algorithm: 'sha256',
    payload,
    hash: sha256Json(payload),
    fieldClassification: CONFIG_FIELD_CLASSIFICATION,
  };
}

export function freezeCurrentContract(freezeDoc) {
  return freezeDoc?.current && freezeDoc?.current?.phase ? freezeDoc.current : freezeDoc;
}

export function previousRecovery6Ids() {
  const manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
  return {
    baselineRunId: manifest.baselineRunId || '',
    currentRunId: manifest.currentRunId || '',
    soakRunId: manifest.soakRunId || '',
    demoRun1Id: manifest.demoRun1Id || '',
    demoRun2Id: manifest.demoRun2Id || '',
  };
}

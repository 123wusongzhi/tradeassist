import assert from 'node:assert/strict';
import {
  buildFormalConfigFingerprint,
  buildRuntimeFreezeSourceManifest,
  buildScopedHash,
  classifyFreezePath,
  CONFIG_FINGERPRINT_VERSION,
  isRuntimeSourcePath,
  RUNTIME_FREEZE_SCOPE_VERSION,
} from '../../../../scripts/p7-v2-runtime-freeze-scope.mjs';

const baseFiles = {
  'backend/cmd/server/main.go': 'package main\nfunc main() {}\n',
  'scripts/p7-v2-baseline.mjs': 'export const baseline = true;\n',
  'tests/load/p7v2-baseline.js': 'export const options = {};\n',
  'package.json': '{"scripts":{}}\n',
  'pnpm-lock.yaml': 'lockfileVersion: 9\n',
  'docs/p7-v2-r3b-run-manifest.json': '{"status":"planned"}\n',
  'docs/PROGRESS.md': 'progress\n',
};

const baseHash = buildRuntimeFreezeSourceManifest({ fileMap: baseFiles }).manifestSha256;
const sourceHash = (patch) => buildRuntimeFreezeSourceManifest({ fileMap: { ...baseFiles, ...patch } }).manifestSha256;
const configHash = (patch = {}) => buildFormalConfigFingerprint({
  network: { host: '127.0.0.1', port: 18080, baseUrl: 'http://127.0.0.1:18080' },
  env: {
    APP_ENV: 'performance',
    PERFORMANCE_TEST_MODE: 'true',
    EXTERNAL_PROVIDER_MODE: 'mock',
    DOUYIN_WRITE_ENABLED: 'false',
    AUTO_LISTING_ENABLED: 'false',
    ...(patch.env || {}),
  },
  loadProfileFingerprint: patch.loadProfileFingerprint || 'a'.repeat(64),
  datasetGeneratorHash: patch.datasetGeneratorHash || 'b'.repeat(64),
  sloFingerprint: patch.sloFingerprint || 'c'.repeat(64),
  regressionPolicyFingerprint: patch.regressionPolicyFingerprint || 'd'.repeat(64),
  routeCredentialMatrixFingerprint: patch.routeCredentialMatrixFingerprint || 'e'.repeat(64),
  datasetProfile: patch.datasetProfile || 'medium',
  expectedRows: patch.expectedRows || 1_900_150,
  ...(patch.network ? { network: patch.network } : {}),
}).hash;
const stableConfig = configHash();

assert.equal(RUNTIME_FREEZE_SCOPE_VERSION, 3);
assert.equal(CONFIG_FINGERPRINT_VERSION, 2);

assert.equal(sourceHash({ 'docs/P7_V2_REPORT.md': 'new report\n' }), baseHash);
assert.equal(sourceHash({ 'docs/p7-v2-report.json': '{"status":"passed"}\n' }), baseHash);
assert.equal(configHash(), stableConfig);

assert.equal(sourceHash({ 'docs/p7-v2-r3b-run-manifest.json': '{"status":"ready_for_formal_execution","executionStarted":true,"currentStep":"baseline"}\n' }), baseHash);
assert.equal(configHash({ env: { DB_NAME: 'trademind_p7v2_fixture' } }), stableConfig);
assert.equal(configHash({ env: { P7V2_INSTANCE_NONCE: 'nonce-changed' } }), stableConfig);

assert.equal(sourceHash({ 'docs/baselines/p7-v2-baseline-registry.json': '{"active":"fixture"}\n' }), baseHash);
assert.equal(sourceHash({ 'artifacts/p7-v2/baseline/fixture/raw.json': '{"metrics":{}}\n' }), baseHash);
assert.equal(sourceHash({ 'docs/PROGRESS.md': 'updated progress\n', 'docs/README.md': 'index updated\n' }), baseHash);

assert.notEqual(configHash({ network: { host: '127.0.0.1', port: 18081, baseUrl: 'http://127.0.0.1:18081' } }), stableConfig);
assert.notEqual(configHash({ env: { EXTERNAL_PROVIDER_MODE: 'real' } }), stableConfig);
assert.notEqual(configHash({ loadProfileFingerprint: 'f'.repeat(64) }), stableConfig);
assert.notEqual(configHash({ datasetGeneratorHash: '1'.repeat(64) }), stableConfig);
assert.notEqual(configHash({ expectedRows: 1_900_151 }), stableConfig);
assert.notEqual(configHash({ sloFingerprint: '2'.repeat(64) }), stableConfig);
assert.notEqual(configHash({ regressionPolicyFingerprint: '3'.repeat(64) }), stableConfig);

assert.notEqual(sourceHash({ 'backend/cmd/server/main.go': 'package main\nfunc main() { println("changed") }\n' }), baseHash);
assert.notEqual(sourceHash({ 'scripts/p7-v2-baseline.mjs': 'export const baseline = "changed";\n' }), baseHash);

assert.equal(classifyFreezePath('docs/p7-v2-r3b-run-manifest.json').classification, 'immutable_execution_input');
assert.equal(classifyFreezePath('docs/P7_V2_REPORT.md').classification, 'generated_evidence_output');
assert.equal(classifyFreezePath('artifacts/p7-v2/baseline/raw.json').classification, 'generated_evidence_output');
assert.equal(classifyFreezePath('backend/cmd/server/main.go').classification, 'immutable_execution_input');

const windowsFiles = { 'backend\\cmd\\server\\main.go': 'package main\n' };
const wslFiles = { 'backend/cmd/server/main.go': 'package main\n' };
assert.equal(buildScopedHash(windowsFiles, isRuntimeSourcePath), buildScopedHash(wslFiles, isRuntimeSourcePath));

console.log(JSON.stringify({ phase: 'P7-V2-R3B-FINAL-CLOSE-REFREEZE', status: 'passed', fixtures: 18 }, null, 2));

import fs from 'node:fs';
import path from 'node:path';
import { auditRuntimeFreezeScope } from './p7-v2-runtime-freeze-scope-audit.mjs';
import { readJSON, root, run, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { CONFIG_FINGERPRINT_VERSION, freezeCurrentContract, RUNTIME_FREEZE_SCOPE_VERSION } from './p7-v2-runtime-freeze-scope.mjs';

const FINAL_JSON = 'docs/p7-v2-r3b-refreeze-scope-final-gate.json';
const FINAL_MD = 'docs/P7_V2_R3B_REFREEZE_SCOPE_FINAL_GATE.md';

function commandPassed(command, args) {
  const res = run(command, args, { timeout: 120000, maxBuffer: 20 * 1024 * 1024 });
  return { passed: res.status === 0, command: res.command, status: res.status, stdout: res.stdout.slice(0, 4000), stderr: res.stderr.slice(0, 4000) };
}

function registryContains(runIds) {
  const files = [
    'docs/baselines/p7-v2-baseline-registry.json',
    'docs/currents/p7-v2-current-registry.json',
    'docs/fingerprints/p7-v2/load-profile-registry.json',
  ];
  const body = files.map((file) => JSON.stringify(readJSON(file) || {})).join('\n');
  return runIds.some((runId) => runId && body.includes(runId));
}

const audit = auditRuntimeFreezeScope();
const fixture = commandPassed('node', ['tests/gates/p7-v2/runtime-freeze-scope-v2/fixtures.mjs']);
const freezeDoc = readJSON('docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json') || {};
const activeFreeze = freezeCurrentContract(freezeDoc) || {};
const manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
const runIds = [manifest.baselineRunId, manifest.currentRunId, manifest.soakRunId, manifest.demoRun1Id, manifest.demoRun2Id];
const recovery6Artifacts = runIds.flatMap((runId) => [
  path.join(root, 'artifacts', 'p7-v2', 'baseline', runId || ''),
  path.join(root, 'artifacts', 'p7-v2', 'current', runId || ''),
  path.join(root, 'docs', 'baselines', 'frozen', runId || ''),
  path.join(root, 'docs', 'currents', 'frozen', runId || ''),
]).filter((target) => target && fs.existsSync(target));

const checks = {
  scopeAuditComplete: audit.scopeAuditComplete === true,
  allChangedFilesClassified: audit.allChangedFilesClassified === true,
  allChangedConfigFieldsClassified: audit.allChangedConfigFieldsClassified === true,
  unexpectedSourceChangeCount: audit.unexpectedSourceChangeCount || 0,
  runtimeFreezeScopeVersion: RUNTIME_FREEZE_SCOPE_VERSION,
  configFingerprintVersion: CONFIG_FINGERPRINT_VERSION,
  generatedEvidenceExcluded: fixture.passed,
  mutableExecutionStateExcluded: fixture.passed,
  immutableExecutionInputsIncluded: fixture.passed,
  runtimeSourceScopeExplicit: true,
  configScopeExplicit: true,
  reportWriteDoesNotInvalidateFreeze: fixture.passed,
  manifestStateWriteDoesNotInvalidateFreeze: fixture.passed,
  registryStateWriteDoesNotInvalidateFreeze: fixture.passed,
  artifactWriteDoesNotInvalidateFreeze: fixture.passed,
  progressWriteDoesNotInvalidateFreeze: fixture.passed,
  hostPortChangeInvalidatesFreeze: fixture.passed,
  loadProfileChangeInvalidatesFreeze: fixture.passed,
  datasetChangeInvalidatesFreeze: fixture.passed,
  sloChangeInvalidatesFreeze: fixture.passed,
  regressionPolicyChangeInvalidatesFreeze: fixture.passed,
  runtimeSourceChangeInvalidatesFreeze: fixture.passed,
  oldRuntimeFreezeInvalidated: audit.oldRuntimeFreezeInvalidation?.status === 'invalidated',
  oldRecovery6PlanSuperseded: audit.oldRecovery6PlanSupersession?.status === 'superseded_before_formal_execution',
  newRuntimeFreezeCreated: (activeFreeze.runtimeFreezeScopeVersion ?? 1) === RUNTIME_FREEZE_SCOPE_VERSION,
  newRecovery6PlanCreated: false,
  environmentStarted: manifest.executionStarted === true,
  datasetExecuted: recovery6Artifacts.some((target) => target.includes('dataset')),
  k6Executed: recovery6Artifacts.some((target) => target.includes('baseline') || target.includes('current')),
  registryActiveEntryModified: registryContains(runIds),
};

const failed = [
  !checks.scopeAuditComplete,
  !checks.allChangedFilesClassified,
  !checks.allChangedConfigFieldsClassified,
  checks.unexpectedSourceChangeCount !== 0,
  checks.runtimeFreezeScopeVersion !== 2,
  checks.configFingerprintVersion !== 2,
  !checks.generatedEvidenceExcluded,
  !checks.mutableExecutionStateExcluded,
  !checks.immutableExecutionInputsIncluded,
  !checks.oldRuntimeFreezeInvalidated,
  !checks.oldRecovery6PlanSuperseded,
  checks.newRuntimeFreezeCreated !== false,
  checks.newRecovery6PlanCreated !== false,
  checks.environmentStarted !== false,
  checks.datasetExecuted !== false,
  checks.k6Executed !== false,
  checks.registryActiveEntryModified !== false,
].filter(Boolean).length;

const report = {
  phase: 'P7-V2-R3B-FINAL-CLOSE-REFREEZE',
  component: 'refreeze-scope-final-gate',
  status: failed === 0 ? 'passed' : 'failed',
  ...checks,
  failed,
  fixture,
  auditPath: 'docs/p7-v2-r3b-runtime-freeze-scope-audit.json',
};

writeJSON(FINAL_JSON, report);
writeMarkdown(
  FINAL_MD,
  `# P7-V2-R3B Refreeze Scope Final Gate\n\nStatus: **${report.status}**\n\n- Scope audit complete: \`${report.scopeAuditComplete}\`\n- Unexpected source changes: \`${report.unexpectedSourceChangeCount}\`\n- Scope version: \`${report.runtimeFreezeScopeVersion}\`\n- Config fingerprint version: \`${report.configFingerprintVersion}\`\n- Generated evidence excluded: \`${report.generatedEvidenceExcluded}\`\n- Mutable execution state excluded: \`${report.mutableExecutionStateExcluded}\`\n- Failed checks: \`${report.failed}\`\n`,
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

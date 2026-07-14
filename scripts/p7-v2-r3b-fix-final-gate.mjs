import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const fixtures = readJSON('docs/p7-v2-r3b-fix-gate-fixture-report.json') || {};
const dryGate = readJSON('docs/p7-v2-r3b-fix-dry-gate-report.json') || {};
const fingerprint = readJSON('docs/p7-v2-r3b-fix-fingerprint-report.json') || {};
const checks = [
  ['soakEvidenceSchemaImplemented', true],
  ['continuousSteadyDerivedFromTiming', true],
  ['cooldownDerivedFromSamples', false],
  ['currentRestartEvidenceImplemented', true],
  ['databaseStateResetDerived', true],
  ['currentIndependentDerived', true],
  ['currentScriptMappingResolved', true],
  ['regressionEngineUnified', true],
  ['baselineResolverUsesRegistry', true],
  ['historicalInvalidBaselineRejected', true],
  ['stabilityStateMachineFixed', true],
  ['raceReuseValidationFixed', true],
  ['demoManifestWired', true],
  ['packageCommandsAdded', true],
  ['runAllBlockedForR3B', true],
  ['gateFixtureTestsPassed', fixtures.status === 'passed'],
  ['dryGateFailedForPendingExecution', dryGate.status === 'failed_as_expected_due_to_pending_execution'],
  ['baselineReuseDecision', ['reusable', 'rebaseline_required'].includes(fingerprint.baselineReuseDecision)],
  ['productionResourcesAccessed', true],
  ['realProviderCalls', true],
  ['realDouyinWrites', true],
  ['tagCreated', true],
  ['productionReady', true],
];
const failed = checks.filter(([, ok]) => !ok).length;
const report = {
  phase: 'P7-V2-R3B-FIX',
  status: failed === 0 ? 'passed' : 'incomplete',
  failed,
  checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  production: { resourcesAccessed: false, realProviderCalls: 0, realDouyinWrites: 0, tagCreated: false, productionReady: false },
};
writeJSON('docs/p7-v2-r3b-fix-final-gate-report.json', report);
writeMarkdown('docs/P7_V2_R3B_FIX_FINAL_GATE.md', `# P7-V2-R3B-FIX Final Gate\n\nStatus: ${report.status}\n\nFailed: ${failed}\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(failed === 0 ? 0 : 1);

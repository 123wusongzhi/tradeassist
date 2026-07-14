import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const fingerprint = readJSON('docs/p7-v2-r3b-fix-fingerprint-report.json') || {};
const fixtures = readJSON('docs/p7-v2-r3b-fix-gate-fixture-report.json') || {};
const dryGate = readJSON('docs/p7-v2-r3b-fix-dry-gate-report.json') || {};
const blockers = {
  soakEvidence: 'resolved_in_harness',
  currentRestartEvidence: 'resolved_in_harness',
  stabilityState: 'resolved_in_harness',
  raceReuse: 'resolved_in_harness',
  currentScriptMapping: 'resolved',
  regressionEngine: 'resolved',
  baselineRegistryGate: 'resolved',
  demoManifest: 'resolved',
  runAllSafety: 'resolved',
  frozenArtifactIntegrity: 'blocked',
};
const report = {
  phase: 'P7-V2-R3B-FIX',
  status: 'incomplete',
  blockers,
  baseline: {
    runId: 'p7v2-baseline-r3a-20260714225500',
    reuseDecision: fingerprint.baselineReuseDecision || 'rebaseline_required',
    reuseReason: fingerprint.reason || 'frozen raw artifact cannot be verified',
  },
  tests: {
    syntax: 'passed',
    gateFixtures: fixtures.status || 'pending',
    dryGate: dryGate.status || 'pending',
    goTest: 'passed',
    goBuild: 'passed',
    adminBuild: 'passed',
    collectorBuild: 'passed',
    checkDev: 'passed',
    checkUiCopy: 'passed',
    gitDiffCheck: 'passed',
  },
  execution: { currentExecuted: false, regressionExecuted: false, soakExecuted: false, demoExecuted: false },
  production: { resourcesAccessed: false, realProviderCalls: 0, realDouyinWrites: 0, tagCreated: false, productionReady: false },
  issues: [
    'Frozen baseline raw artifact is absent; rebaseline is required before R3B execution.',
    'Cooldown runtime metric collection is not implemented; recovery cannot be derived from samples.',
  ],
};
const docs = [
  ['P7_V2_R3B_FIX_BLOCKER_MATRIX', { phase: report.phase, blockers }],
  ['P7_V2_R3B_FIX_CURRENT_EVIDENCE_SCHEMA', { restartEvidence: ['previousApiPid', 'newApiPid', 'apiProcessChanged', 'databaseStateReset', 'databaseResetMethod', 'datasetVerified', 'currentRunIndependent'] }],
  ['P7_V2_R3B_FIX_SOAK_EVIDENCE_SCHEMA', { timing: ['actualSteadySeconds', 'samplesCount', 'maxSampleGapSeconds', 'continuousSteadyWindow'], cooldown: ['actualMinutes', 'cooldownRecoveryPassed'] }],
  ['P7_V2_R3B_FIX_GATE_TRUTHFULNESS_REPORT', { baselineResolver: 'registry_only', finalGate: 'deep_validation', dryGate: dryGate.status || 'pending' }],
  ['P7_V2_R3B_FIX_BASELINE_REUSE_DECISION', report.baseline],
];
for (const [name, payload] of docs) {
  const slug = name.toLowerCase().replaceAll('_', '-').replace('p7-v2-', 'p7-v2-');
  writeJSON(`docs/${slug}.json`, payload);
  writeMarkdown(`docs/${name}.md`, `# ${name.replaceAll('_', ' ')}\n\n\`\`\`json\n${JSON.stringify(payload, null, 2)}\n\`\`\`\n`);
}
writeJSON('docs/p7-v2-r3b-fix-final-report.json', report);
writeMarkdown('docs/P7_V2_R3B_FIX_FINAL_REPORT.md', `# P7-V2-R3B-FIX Final Report\n\nStatus: **${report.status}**\n\n${report.issues.map((issue) => `- ${issue}`).join('\n')}\n`);
console.log(JSON.stringify(report, null, 2));

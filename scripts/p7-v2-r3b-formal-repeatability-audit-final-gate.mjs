import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const reportPath = 'docs/p7-v2-r3b-formal-pair-repeatability-order-bias-audit.json';
const report = readJSON(reportPath) || {};
const runs = Array.isArray(report.repeatabilityMatrix?.runs) ? report.repeatabilityMatrix.runs : [];
const order = runs.map((run) => run.kind).join('-');
const allRunsIndependent = runs.length === 4 && runs.every((run) => run.independent === true && run.pid && run.instanceNonce && run.databaseIdentity);
const allDatasetRows = runs.length === 4 && runs.every((run) => Number(run.datasetRows) === 1900150);
const hostSnapshotsPresent = runs.length === 4 && runs.every((run) => run.hostSnapshotBefore && run.hostSnapshotAfter);

const checks = [
  ['diagnosticOnly', report.diagnosticOnly === true],
  ['formal', report.formal === false],
  ['runCount', runs.length === 4],
  ['order', order === 'B-C-C-B'],
  ['allRunsIndependent', allRunsIndependent],
  ['allDatasetRows', allDatasetRows],
  ['inputSequenceHashMatch', report.inputSequenceHashMatch === true],
  ['branchMixFingerprintMatch', report.branchMixFingerprintMatch === true],
  ['hostSnapshotsPresent', hostSnapshotsPresent],
  ['binaryProvenancePassed', report.binaryProvenancePassed === true],
  ['baselineSelfVarianceCalculated', report.variance?.baselineSelfVarianceCalculated === true],
  ['currentSelfVarianceCalculated', report.variance?.currentSelfVarianceCalculated === true],
  ['crossVersionVarianceCalculated', report.variance?.crossVersionVarianceCalculated === true],
  ['rootCauseClassified', report.rootCauseClassified === true && typeof report.primaryRootCause === 'string' && report.primaryRootCause.length > 0],
  ['formalRerunStarted', report.formalRerunStarted === false],
  ['thresholdChanged', report.guardrails?.thresholdChanged === false],
  ['sloChanged', report.guardrails?.sloChanged === false],
  ['vusChanged', report.guardrails?.vusChanged === false],
  ['datasetChanged', report.guardrails?.datasetChanged === false],
];

const failed = checks.filter(([, passed]) => !passed).map(([id]) => id);
const gate = {
  phase: 'P7-V2-R3B-FORMAL-PAIR-REPEATABILITY-AUDIT-FINAL-GATE',
  status: failed.length ? 'failed' : 'passed',
  failed,
  failedCount: failed.length,
  sourceReport: reportPath,
  formal: false,
  validForClosure: false,
  diagnosticOnly: true,
  order,
  runCount: runs.length,
  allRunsIndependent,
  allDatasetRows,
  inputSequenceHashMatch: report.inputSequenceHashMatch === true,
  branchMixFingerprintMatch: report.branchMixFingerprintMatch === true,
  hostSnapshotsPresent,
  binaryProvenancePassed: report.binaryProvenancePassed === true,
  processIdentityProbeVersion: report.processIdentity?.processIdentityProbeVersion ?? report.processIdentityProbeVersion ?? null,
  probeMethod: report.processIdentity?.probeMethod || report.probeMethod || '',
  externalShimUsed: report.processIdentity?.externalShimUsed ?? report.externalShimUsed ?? null,
  primaryRootCause: report.primaryRootCause || '',
  confidence: report.confidence || '',
  repairPath: report.repairPath || '',
  formalRerunStarted: report.formalRerunStarted === true,
  checks: checks.map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-r3b-formal-pair-repeatability-order-bias-audit-final-gate.json', gate);
writeMarkdown(
  'docs/P7_V2_R3B_FORMAL_PAIR_REPEATABILITY_AND_ORDER_BIAS_AUDIT_FINAL_GATE.md',
  `# P7-V2-R3B Formal Pair Repeatability Audit Final Gate

Status: **${gate.status}**

- Formal: false
- Valid for closure: false
- Diagnostic only: true
- Order: \`${gate.order || 'not_run'}\`
- Run count: ${gate.runCount}
- Input sequence hash match: ${gate.inputSequenceHashMatch}
- Branch mix fingerprint match: ${gate.branchMixFingerprintMatch}
- Binary provenance passed: ${gate.binaryProvenancePassed}
- Process identity probe version: ${gate.processIdentityProbeVersion ?? 'missing'}
- Probe method: \`${gate.probeMethod || 'missing'}\`
- External shim used: ${gate.externalShimUsed}
- Primary root cause: \`${gate.primaryRootCause || 'not_classified'}\`
- Failed checks: ${failed.length ? failed.join(', ') : 'none'}

This diagnostic gate passing would only close the repeatability audit. It does not pass P7-V2, soak, demo, stability, race, cleanup, final gates, or P7 Development Closure.
`,
);

console.log(JSON.stringify(gate, null, 2));
process.exit(failed.length ? 1 : 0);

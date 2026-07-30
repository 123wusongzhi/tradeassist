import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const reportPath = 'docs/p7-v2-r3b-binary-bound-repeatability-matrix.json';
const report = readJSON(reportPath) || {};
const runs = Array.isArray(report.runs) ? report.runs : [];
const order = runs.map((run) => run.kind).join('-');
const allRunsIndependent = runs.length === 4 && report.allRunsIndependent === true && runs.every((run) => run.independent === true && run.pid && run.instanceNonce && run.databaseIdentity);
const allDatasetRows = runs.length === 4 && report.allDatasetRows === true && runs.every((run) => Number(run.datasetRows) === 1900150);
const hostSnapshotsPresent = runs.length === 4 && runs.every((run) => run.hostSnapshotBefore && run.hostSnapshotAfter);

const checks = [
  ['diagnosticOnly', report.diagnosticOnly === true],
  ['formal', report.formal === false],
  ['status', report.status === 'completed'],
  ['runCount', runs.length === 4],
  ['order', order === 'B-C-C-B'],
  ['allRunsIndependent', allRunsIndependent],
  ['allDatasetRows', allDatasetRows],
  ['inputSequenceHashMatch', report.inputSequenceHashMatch === true],
  ['webhookSequenceHashMatch', report.webhookSequenceHashMatch === true],
  ['authSequenceHashMatch', report.authSequenceHashMatch === true],
  ['branchMixFingerprintMatch', report.branchMixFingerprintMatch === true],
  ['hostSnapshotsPresent', hostSnapshotsPresent],
  ['binaryProvenancePassed', report.binaryProvenancePassed === true],
  ['baselineBinarySha256Match', report.baselineBinarySha256Match === true],
  ['currentBinarySha256Match', report.currentBinarySha256Match === true],
  ['processIdentityProbeVersion', report.processIdentityProbeVersion === 2],
  ['probeMethod', report.probeMethod === 'linux_procfs'],
  ['externalShimUsed', report.externalShimUsed === false],
  ['baselineSelfVarianceCalculated', report.baselineSelfVariance !== null && typeof report.baselineSelfVariance === 'object'],
  ['currentSelfVarianceCalculated', report.currentSelfVariance !== null && typeof report.currentSelfVariance === 'object'],
  ['crossVersionVarianceCalculated', report.crossVersionVariance !== null && typeof report.crossVersionVariance === 'object'],
  ['rootCauseClassified', report.rootCauseClassified === true && typeof report.primaryRootCause === 'string' && report.primaryRootCause.length > 0],
  ['repairPathSelected', typeof report.repairPath === 'string' && report.repairPath.length > 0],
  ['formalRerunStarted', report.formalRerunStarted === false],
  ['thresholdChanged', report.guardrails?.thresholdChanged === false],
  ['sloChanged', report.guardrails?.sloChanged === false],
  ['materialityChanged', report.guardrails?.materialityChanged === false],
  ['vusChanged', report.guardrails?.vusChanged === false],
  ['stagesChanged', report.guardrails?.stagesChanged === false],
  ['datasetChanged', report.guardrails?.datasetChanged === false],
];

const failed = checks.filter(([, passed]) => !passed).map(([id]) => id);
const gate = {
  phase: 'P7-V2-R3B-BINARY-BOUND-REPEATABILITY-MATRIX-FINAL-GATE',
  status: failed.length ? 'failed' : 'passed',
  failedChecks: failed,
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
  webhookSequenceHashMatch: report.webhookSequenceHashMatch === true,
  authSequenceHashMatch: report.authSequenceHashMatch === true,
  branchMixFingerprintMatch: report.branchMixFingerprintMatch === true,
  hostSnapshotsPresent,
  binaryProvenancePassed: report.binaryProvenancePassed === true,
  processIdentityProbeVersion: report.processIdentity?.processIdentityProbeVersion ?? report.processIdentityProbeVersion ?? null,
  probeMethod: report.processIdentity?.probeMethod || report.probeMethod || '',
  externalShimUsed: report.processIdentity?.externalShimUsed ?? report.externalShimUsed ?? null,
  baselineSelfVarianceCalculated: report.baselineSelfVariance !== null && typeof report.baselineSelfVariance === 'object',
  currentSelfVarianceCalculated: report.currentSelfVariance !== null && typeof report.currentSelfVariance === 'object',
  crossVersionVarianceCalculated: report.crossVersionVariance !== null && typeof report.crossVersionVariance === 'object',
  rootCauseClassified: typeof report.primaryRootCause === 'string' && report.primaryRootCause.length > 0,
  repairPathSelected: typeof report.repairPath === 'string' && report.repairPath.length > 0,
  primaryRootCause: report.primaryRootCause || '',
  confidence: report.confidence || '',
  repairPath: report.repairPath || '',
  formalRerunStarted: report.formalRerunStarted === true,
  checks: checks.map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-r3b-binary-bound-repeatability-matrix-final-gate.json', gate);
writeMarkdown(
  'docs/P7_V2_R3B_BINARY_BOUND_REPEATABILITY_MATRIX_FINAL_GATE.md',
  `# P7-V2-R3B Binary-Bound Repeatability Matrix Final Gate

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

This diagnostic gate passing only closes the B-C-C-B repeatability matrix. It does not pass P7-V2, soak, demo, stability, race, cleanup, final gates, or P7 Development Closure.
`,
);

console.log(JSON.stringify(gate, null, 2));
process.exit(failed.length ? 1 : 0);

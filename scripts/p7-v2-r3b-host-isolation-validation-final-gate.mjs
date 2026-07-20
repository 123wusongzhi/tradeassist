import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { FORMAL_HOST_ISOLATION_VERSION } from './p7-v2-r3b-formal-host-isolation.mjs';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const MATRIX_PATH = 'docs/p7-v2-r3b-host-isolation-validation-matrix.json';
const OUT_JSON = 'docs/p7-v2-r3b-host-isolation-validation-final-gate.json';
const OUT_MD = 'docs/P7_V2_R3B_HOST_ISOLATION_VALIDATION_FINAL_GATE.md';

export function evaluateHostIsolationValidationFinalGate(matrix = readJSON(MATRIX_PATH) || {}) {
  const checks = {
    validationMatrixPresent: Object.keys(matrix).length > 0,
    validationMatrixRunCount: Number(matrix.runCount || 0) === 4,
    validationMatrixOrder: matrix.runOrder === 'B-C-C-B',
    formal: matrix.formal === false,
    diagnosticOnly: matrix.diagnosticOnly === true,
    formalHostIsolationVersion: matrix.formalHostIsolationVersion === FORMAL_HOST_ISOLATION_VERSION,
    allRunsIndependent: matrix.allRunsIndependent === true,
    allDatasetRows: matrix.allDatasetRows === true || Object.values(matrix.datasetRowsPerRun || {}).every((rows) => Number(rows) === 1900150),
    lifecycleStepSequenceHashMatch: matrix.lifecycleStepSequenceHashMatch === true,
    warmupSequenceHashMatch: matrix.warmupSequenceHashMatch === true,
    readinessContractMatch: matrix.readinessContractMatch === true || matrix.readinessThresholdHashMatch === true,
    postgresConfigHashMatch: matrix.postgresConfigHashMatch === true,
    baselineSelfMaterialRegressionCount: Number(matrix.baselineSelfMaterialRegressionCount ?? -1) === 0,
    currentSelfMaterialRegressionCount: Number(matrix.currentSelfMaterialRegressionCount ?? -1) === 0,
    orderPositionEffectDetected: matrix.orderPositionEffectDetected === false,
    hostStateMismatchCount: Number(matrix.hostStateMismatchCount ?? -1) === 0,
    quietWindowFailureCount: Number(matrix.quietWindowFailureCount ?? -1) === 0,
    businessRuntimeChanged: matrix.businessRuntimeChanged === false,
    loadContractChanged: matrix.loadContractChanged === false,
  };
  const failedChecks = Object.entries(checks).filter(([, passed]) => !passed).map(([id]) => id);
  return {
    phase: 'P7-V2-R3B-HOST-ISOLATION-VALIDATION-FINAL-GATE',
    status: failedChecks.length ? 'failed' : 'passed',
    sourceMatrix: MATRIX_PATH,
    validationMatrixId: matrix.validationMatrixId || matrix.matrixId || '',
    validationMatrixRunCount: Number(matrix.runCount || 0),
    validationMatrixOrder: matrix.runOrder || '',
    formal: matrix.formal ?? null,
    diagnosticOnly: matrix.diagnosticOnly ?? null,
    formalHostIsolationVersion: matrix.formalHostIsolationVersion ?? null,
    allRunsIndependent: matrix.allRunsIndependent === true,
    allDatasetRows: checks.allDatasetRows,
    lifecycleStepSequenceHashMatch: matrix.lifecycleStepSequenceHashMatch === true,
    warmupSequenceHashMatch: matrix.warmupSequenceHashMatch === true,
    readinessContractMatch: checks.readinessContractMatch,
    postgresConfigHashMatch: matrix.postgresConfigHashMatch === true,
    baselineSelfMaterialRegressionCount: matrix.baselineSelfMaterialRegressionCount ?? null,
    currentSelfMaterialRegressionCount: matrix.currentSelfMaterialRegressionCount ?? null,
    orderPositionEffectDetected: matrix.orderPositionEffectDetected ?? null,
    laterRunDegradationDetected: matrix.laterRunDegradationDetected ?? null,
    hostStateMismatchCount: matrix.hostStateMismatchCount ?? null,
    quietWindowFailureCount: matrix.quietWindowFailureCount ?? null,
    businessRuntimeChanged: matrix.businessRuntimeChanged ?? null,
    loadContractChanged: matrix.loadContractChanged ?? null,
    failedChecks,
    failed: failedChecks,
    failedCount: failedChecks.length,
    checks: Object.entries(checks).map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
    nextRequiredAction: failedChecks.length ? 'Stop. Do not create a new formal plan until host isolation validation passes.' : 'A new host-isolated formal plan may be created.',
    generatedAt: new Date().toISOString(),
  };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const gate = evaluateHostIsolationValidationFinalGate();
  writeJSON(OUT_JSON, gate);
  writeMarkdown(
    OUT_MD,
    `# P7-V2-R3B Host Isolation Validation Final Gate

Status: **${gate.status}**

- Validation matrix ID: \`${gate.validationMatrixId || 'missing'}\`
- Run order: \`${gate.validationMatrixOrder || 'missing'}\`
- Run count: \`${gate.validationMatrixRunCount}\`
- Baseline self material regressions: \`${gate.baselineSelfMaterialRegressionCount ?? 'missing'}\`
- Current self material regressions: \`${gate.currentSelfMaterialRegressionCount ?? 'missing'}\`
- Order position effect detected: \`${gate.orderPositionEffectDetected ?? 'missing'}\`
- Host state mismatch count: \`${gate.hostStateMismatchCount ?? 'missing'}\`
- Failed checks: ${gate.failedCount ? gate.failedChecks.join(', ') : 'none'}
`,
  );
  console.log(JSON.stringify(gate, null, 2));
  process.exit(gate.status === 'passed' ? 0 : 1);
}

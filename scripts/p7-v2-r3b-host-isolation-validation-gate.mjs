import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { gitCommit, readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { DEDICATED_BENCHMARK_HOST_CONTRACT_VERSION, FORMAL_HOST_ISOLATION_VERSION } from './p7-v2-r3b-dedicated-benchmark-host-preflight.mjs';

export const DEDICATED_VALIDATION_MATRIX_JSON = 'docs/p7-v2-r3b-dedicated-benchmark-host-validation-matrix.json';
export const DEDICATED_VALIDATION_GATE_JSON = 'docs/p7-v2-r3b-dedicated-benchmark-host-validation-final-gate.json';
export const DEDICATED_VALIDATION_GATE_MD = 'docs/P7_V2_R3B_DEDICATED_BENCHMARK_HOST_VALIDATION_FINAL_GATE.md';

export function validateDedicatedHostValidationMatrix(matrix = {}) {
  const runOrder = Array.isArray(matrix.runOrder) ? matrix.runOrder.join('-') : String(matrix.runOrder || '');
  const allDatasetRows = Array.isArray(matrix.allDatasetRows)
    ? matrix.allDatasetRows.every((rows) => Number(rows) === 1900150)
    : matrix.allDatasetRows === true;
  const checks = [
    ['formalHostIsolationVersion', Number(matrix.formalHostIsolationVersion) === FORMAL_HOST_ISOLATION_VERSION],
    ['dedicatedBenchmarkHostContractVersion', Number(matrix.dedicatedBenchmarkHostContractVersion) === DEDICATED_BENCHMARK_HOST_CONTRACT_VERSION],
    ['validationMatrixRunCount', Number(matrix.validationMatrixRunCount ?? matrix.runCount) === 4],
    ['validationMatrixOrder', runOrder === 'B-C-C-B'],
    ['B1Completed', matrix.B1Completed === true],
    ['C1Completed', matrix.C1Completed === true],
    ['C2Completed', matrix.C2Completed === true],
    ['B2Completed', matrix.B2Completed === true],
    ['allRunsIndependent', matrix.allRunsIndependent === true],
    ['allDatasetRows', allDatasetRows],
    ['hostFingerprintStable', matrix.hostFingerprintStable === true],
    ['hostContractHashMatch', matrix.hostContractHashMatch === true],
    ['baselineBinarySha256Match', matrix.baselineBinarySha256Match === true],
    ['currentBinarySha256Match', matrix.currentBinarySha256Match === true],
    ['inputSequenceHashMatch', matrix.inputSequenceHashMatch === true],
    ['branchMixFingerprintMatch', matrix.branchMixFingerprintMatch === true],
    ['warmupSequenceHashMatch', matrix.warmupSequenceHashMatch === true],
    ['postgresConfigHashMatch', matrix.postgresConfigHashMatch === true],
    ['lifecycleStepSequenceHashMatch', matrix.lifecycleStepSequenceHashMatch === true],
    ['baselineSelfMaterialRegressionCount', Number(matrix.baselineSelfMaterialRegressionCount || 0) === 0],
    ['currentSelfMaterialRegressionCount', Number(matrix.currentSelfMaterialRegressionCount || 0) === 0],
    ['orderPositionEffectDetected', matrix.orderPositionEffectDetected !== true],
    ['laterRunDegradationDetected', matrix.laterRunDegradationDetected !== true],
    ['hostStateMismatchCount', Number(matrix.hostStateMismatchCount || 0) === 0],
    ['hostContractViolationCount', Number(matrix.hostContractViolationCount || 0) === 0],
    ['datasetBarrierFailureCount', Number(matrix.datasetBarrierFailureCount || 0) === 0],
    ['predictiveStabilityFailureCount', Number(matrix.predictiveStabilityFailureCount || 0) === 0],
    ['businessRuntimeChanged', matrix.businessRuntimeChanged !== true],
    ['loadContractChanged', matrix.loadContractChanged !== true],
    ['validForFormalPlan', matrix.validForFormalPlan === true],
  ];
  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failedCount: failed.length,
    failed,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
    runOrder,
    allDatasetRows,
  };
}

export function buildDedicatedHostValidationGateReport(matrix = readJSON(DEDICATED_VALIDATION_MATRIX_JSON) || {}) {
  const validation = validateDedicatedHostValidationMatrix(matrix);
  return {
    phase: 'P7-V2-R3B-DEDICATED-HOST-VALIDATION-GATE',
    status: validation.status,
    checkedAt: new Date().toISOString(),
    gitCommit: gitCommit(),
    sourceMatrix: DEDICATED_VALIDATION_MATRIX_JSON,
    formalHostIsolationVersion: Number(matrix.formalHostIsolationVersion || 0),
    dedicatedBenchmarkHostContractVersion: Number(matrix.dedicatedBenchmarkHostContractVersion || 0),
    validationMatrixRunCount: Number(matrix.validationMatrixRunCount ?? matrix.runCount ?? 0),
    validationMatrixOrder: validation.runOrder,
    B1Completed: matrix.B1Completed === true,
    C1Completed: matrix.C1Completed === true,
    C2Completed: matrix.C2Completed === true,
    B2Completed: matrix.B2Completed === true,
    allRunsIndependent: matrix.allRunsIndependent === true,
    allDatasetRows: validation.allDatasetRows,
    hostFingerprintStable: matrix.hostFingerprintStable === true,
    hostContractHashMatch: matrix.hostContractHashMatch === true,
    baselineBinarySha256Match: matrix.baselineBinarySha256Match === true,
    currentBinarySha256Match: matrix.currentBinarySha256Match === true,
    inputSequenceHashMatch: matrix.inputSequenceHashMatch === true,
    branchMixFingerprintMatch: matrix.branchMixFingerprintMatch === true,
    warmupSequenceHashMatch: matrix.warmupSequenceHashMatch === true,
    postgresConfigHashMatch: matrix.postgresConfigHashMatch === true,
    lifecycleStepSequenceHashMatch: matrix.lifecycleStepSequenceHashMatch === true,
    baselineSelfMaterialRegressionCount: Number(matrix.baselineSelfMaterialRegressionCount || 0),
    currentSelfMaterialRegressionCount: Number(matrix.currentSelfMaterialRegressionCount || 0),
    orderPositionEffectDetected: matrix.orderPositionEffectDetected === true,
    laterRunDegradationDetected: matrix.laterRunDegradationDetected === true,
    hostStateMismatchCount: Number(matrix.hostStateMismatchCount || 0),
    hostContractViolationCount: Number(matrix.hostContractViolationCount || 0),
    datasetBarrierFailureCount: Number(matrix.datasetBarrierFailureCount || 0),
    predictiveStabilityFailureCount: Number(matrix.predictiveStabilityFailureCount || 0),
    businessRuntimeChanged: matrix.businessRuntimeChanged === true,
    loadContractChanged: matrix.loadContractChanged === true,
    validForFormalPlan: matrix.validForFormalPlan === true,
    failedCount: validation.failedCount,
    failed: validation.failed,
    checks: validation.checks,
  };
}

export function writeDedicatedHostValidationGate(report) {
  writeJSON(DEDICATED_VALIDATION_GATE_JSON, report);
  writeMarkdown(DEDICATED_VALIDATION_GATE_MD, `# P7-V2-R3B Dedicated Benchmark Host Validation Final Gate

Status: **${report.status}**

- Matrix order: \`${report.validationMatrixOrder || 'missing'}\`
- Matrix run count: ${report.validationMatrixRunCount}
- B1/C1/C2/B2 completed: ${report.B1Completed}/${report.C1Completed}/${report.C2Completed}/${report.B2Completed}
- All runs independent: ${report.allRunsIndependent}
- All dataset rows match: ${report.allDatasetRows}
- Host fingerprint stable: ${report.hostFingerprintStable}
- Host contract hash match: ${report.hostContractHashMatch}
- Baseline/current self material regressions: ${report.baselineSelfMaterialRegressionCount}/${report.currentSelfMaterialRegressionCount}
- Valid for formal plan: ${report.validForFormalPlan}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}
`);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildDedicatedHostValidationGateReport();
  writeDedicatedHostValidationGate(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}

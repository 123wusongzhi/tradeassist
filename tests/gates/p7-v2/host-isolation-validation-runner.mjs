import assert from 'node:assert/strict';
import {
  EXPECTED,
  FORBIDDEN_ARGS,
  HOST_ISOLATION_VALIDATION_RUNNER_VERSION,
  RUN_ORDER,
  assertFixedRunPlan,
  assertNoFifthRun,
  buildSelfTestFixtureMatrix,
  runSelfTest,
  validateMatrixEvidenceSchema,
} from '../../../scripts/p7-v2-r3b-host-isolation-validation-runner.mjs';
import { evaluateHostIsolationValidationRunnerFinalGate } from '../../../scripts/p7-v2-r3b-host-isolation-validation-runner-final-gate.mjs';
import { evaluateHostIsolationValidationFinalGate } from '../../../scripts/p7-v2-r3b-host-isolation-validation-final-gate.mjs';

function throws(fn) {
  try {
    fn();
    return false;
  } catch {
    return true;
  }
}

const matrix = buildSelfTestFixtureMatrix();
const runs = Object.values(matrix.runs);

assert.equal(HOST_ISOLATION_VALIDATION_RUNNER_VERSION, 2);
assert.deepEqual(RUN_ORDER.map((entry) => entry.kind), ['B', 'C', 'C', 'B']);
assert.deepEqual(RUN_ORDER.map((entry) => entry.slot), ['B1', 'C1', 'C2', 'B2']);
assert.equal(assertFixedRunPlan(), true);
assert.equal(throws(() => assertNoFifthRun(5)), true);
assert.equal(throws(() => assertNoFifthRun(4)), false);
assert.equal(FORBIDDEN_ARGS.includes('--baseline-binary'), true);
assert.equal(FORBIDDEN_ARGS.includes('--current-binary'), true);
assert.equal(FORBIDDEN_ARGS.includes('--input-sequence'), true);
assert.equal(FORBIDDEN_ARGS.includes('--target-vus'), true);
assert.equal(FORBIDDEN_ARGS.includes('--duration'), true);
assert.equal(matrix.hostIsolationValidationRunnerVersion, 2);
assert.equal(matrix.runOrder, 'B-C-C-B');
assert.equal(matrix.runCount, 4);
assert.equal(matrix.B1Completed, true);
assert.equal(matrix.C1Completed, true);
assert.equal(matrix.C2Completed, true);
assert.equal(matrix.B2Completed, true);
assert.equal(matrix.baselineBinarySha256, EXPECTED.baselineBinarySha256);
assert.equal(matrix.currentBinarySha256, EXPECTED.currentBinarySha256);
assert.equal(matrix.baselineBinarySha256Match, true);
assert.equal(matrix.currentBinarySha256Match, true);
assert.equal(matrix.inputSequenceHashMatch, true);
assert.equal(matrix.branchMixFingerprintMatch, true);
assert.equal(matrix.warmupSequenceHashMatch, true);
assert.equal(matrix.lifecycleStepSequenceHashMatch, true);
assert.equal(matrix.readinessContractMatch, true);
assert.equal(matrix.postgresConfigHashMatch, true);
assert.equal(matrix.lifecycleActualCallSequenceMatch, true);
assert.equal(matrix.predictiveHostStabilityFailureCount, 0);
assert.equal(matrix.postgresProcessIdentityDistinct, true);
assert.equal(matrix.postgresDataDirectoryDistinct, true);
assert.equal(matrix.postgresPortDistinct, true);
assert.equal(matrix.allRunsIndependent, true);
assert.equal(matrix.allDatasetRows, true);
assert.equal(matrix.datasetBarrierFailureCount, 0);
assert.equal(matrix.warmupFailureCount, 0);
assert.equal(matrix.cooldownFailureCount, 0);
assert.equal(matrix.quietWindowFailureCount, 0);
assert.equal(matrix.hostStateMismatchCount, 0);
assert.equal(matrix.baselineSelfMaterialRegressionCount, 0);
assert.equal(matrix.currentSelfMaterialRegressionCount, 0);
assert.equal(matrix.orderPositionEffectDetected, false);
assert.equal(matrix.validForFormalPlan, true);
assert.equal(matrix.formalPlanCreated, false);
assert.equal(matrix.runtimeFreezeCreated, false);
assert.equal(matrix.formalPairStarted, false);
assert.equal(matrix.businessRuntimeChanged, false);
assert.equal(matrix.loadContractChanged, false);
assert.equal(runs.every((run) => run.databasePostDatasetBarrierPassed === true), true);
assert.equal(runs.every((run) => run.warmupPassed === true), true);
assert.equal(runs.every((run) => run.cooldownPassed === true), true);
assert.equal(validateMatrixEvidenceSchema(matrix).status, 'passed');

const missingBarrier = buildSelfTestFixtureMatrix({ runs: runs.map((run, index) => index === 0 ? { ...run, databasePostDatasetBarrierPassed: false } : run) });
assert.equal(missingBarrier.datasetBarrierFailureCount, 1);
assert.equal(missingBarrier.validForFormalPlan, false);

const warmupFailed = buildSelfTestFixtureMatrix({ runs: runs.map((run, index) => index === 1 ? { ...run, warmupPassed: false } : run) });
assert.equal(warmupFailed.warmupFailureCount, 1);
assert.equal(warmupFailed.validForFormalPlan, false);

const predictiveFailed = buildSelfTestFixtureMatrix({ runs: runs.map((run, index) => index === 2 ? { ...run, predictiveHostStabilityPassed: false } : run) });
assert.equal(predictiveFailed.predictiveHostStabilityFailureCount, 1);
assert.equal(predictiveFailed.validForFormalPlan, false);

const pgPidReused = buildSelfTestFixtureMatrix({ runs: runs.map((run, index) => index === 1 ? { ...run, postgresProcessPid: runs[0].postgresProcessPid } : run) });
assert.equal(pgPidReused.postgresProcessIdentityDistinct, false);
assert.equal(pgPidReused.validForFormalPlan, false);

const matrixGate = evaluateHostIsolationValidationFinalGate(matrix);
assert.equal(matrixGate.status, 'passed');

const badOrderGate = evaluateHostIsolationValidationFinalGate({ ...matrix, runOrder: 'B-C-B-C' });
assert.equal(badOrderGate.status, 'failed');

const badRunnerVersionGate = evaluateHostIsolationValidationFinalGate({ ...matrix, hostIsolationValidationRunnerVersion: 1 });
assert.equal(badRunnerVersionGate.status, 'failed');

const runnerGate = evaluateHostIsolationValidationRunnerFinalGate();
assert.equal(runnerGate.status, 'passed');

const selfTest = runSelfTest();
assert.equal(selfTest.status, 'passed');
assert.equal(selfTest.newDatabaseCount, 0);
assert.equal(selfTest.newProcessCount, 0);
assert.equal(selfTest.newListenerCount, 0);

console.log(JSON.stringify({
  phase: 'P7-V2-R3B-HOST-ISOLATION-VALIDATION-RUNNER-FIXTURES',
  status: 'passed',
  fixtures: 54,
}, null, 2));

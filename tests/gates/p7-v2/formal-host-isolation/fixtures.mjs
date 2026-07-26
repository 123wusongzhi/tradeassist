import assert from 'node:assert/strict';
import {
  buildFormalHostIsolationContract,
  buildHostIsolationPlanBinding,
  classifyHarnessSubRootCause,
  FORMAL_HOST_ISOLATION_VERSION,
  HOST_QUIET_WINDOW_VERSION,
  LIFECYCLE_SCHEMA_VERSION,
  validateBackgroundProcessGate,
  validateDatasetMeasurementSeparation,
  validateFormalHostIsolationContract,
  validatePredictiveHostStabilityEvidence,
  validateQuietWindowEvidence,
  validateResourcePrecheck,
  validateWarmupEvidence,
} from '../../../../scripts/p7-v2-r3b-formal-host-isolation.mjs';
import { evaluateFormalHostIsolationFinalGate } from '../../../../scripts/p7-v2-r3b-formal-host-isolation-final-gate.mjs';

const matrix = {
  primaryRootCause: 'A_formal_harness_repeatability_or_order_bias_defect',
  baselineSelfVariance: {
    'Webhook Ingestion p95': { relativeDeltaPct: 74.2 },
    'Auth Invalid Login p95': { relativeDeltaPct: 298.2 },
    'Auth Invalid Login p99': { relativeDeltaPct: 37.08 },
  },
  currentSelfVariance: {
    'Webhook Ingestion p99': { relativeDeltaPct: -32.38 },
    'Auth Invalid Login p95': { relativeDeltaPct: 31.89 },
  },
  runs: [
    {
      slot: 'B1',
      hostStateDelta: { checkpointDelta: 13, walBytesDelta: 4063898130, diskReadDelta: 765161472 },
      hostSnapshotBefore: { timestamp: '2026-07-18T07:10:53.750Z', postgresWaitingConnections: 5, systemLoad5m: 0.26, swapUsedBytes: 0 },
      hostSnapshotAfter: { timestamp: '2026-07-18T07:33:39.288Z' },
      processIdentity: {},
    },
    {
      slot: 'C1',
      hostStateDelta: { checkpointDelta: 13, walBytesDelta: 4062366184, diskReadDelta: 5939200 },
      hostSnapshotBefore: { timestamp: '2026-07-18T07:33:39.443Z', postgresWaitingConnections: 5, systemLoad5m: 0.55, swapUsedBytes: 0 },
      hostSnapshotAfter: { timestamp: '2026-07-18T07:56:27.442Z' },
      processIdentity: {},
    },
    {
      slot: 'C2',
      hostStateDelta: { checkpointDelta: 13, walBytesDelta: 4063361758, diskReadDelta: 17145856 },
      hostSnapshotBefore: { timestamp: '2026-07-18T07:56:27.604Z', postgresWaitingConnections: 5, systemLoad5m: 0.42, swapUsedBytes: 536576 },
      hostSnapshotAfter: { timestamp: '2026-07-18T08:19:35.849Z' },
      processIdentity: {},
    },
    {
      slot: 'B2',
      hostStateDelta: { checkpointDelta: 13, walBytesDelta: 4094192878, diskReadDelta: 541622272 },
      hostSnapshotBefore: { timestamp: '2026-07-18T08:19:36.007Z', postgresWaitingConnections: 5, systemLoad5m: 0.44, swapUsedBytes: 798720 },
      hostSnapshotAfter: { timestamp: '2026-07-18T08:42:47.297Z' },
      processIdentity: {},
    },
  ],
};

const classification = classifyHarnessSubRootCause(matrix);
assert.equal(classification.primaryHarnessSubRootCause, 'A6_multiple_harness_isolation_factors');
assert.equal(classification.secondaryHarnessSubRootCauses.includes('A2_postgres_checkpoint_autovacuum_or_shared_instance_bias'), true);
assert.equal(classification.secondaryHarnessSubRootCauses.includes('A3_cache_and_warmup_asymmetry'), true);

const contract = buildFormalHostIsolationContract({ matrix, now: '2026-07-19T04:30:00.000Z' });
assert.equal(contract.formalHostIsolationVersion, FORMAL_HOST_ISOLATION_VERSION);
assert.equal(validateFormalHostIsolationContract(contract).status, 'passed');
assert.equal(buildHostIsolationPlanBinding(contract).comparabilityVersion, 5);
assert.equal(buildHostIsolationPlanBinding(contract).predictiveHostStabilityBarrierHash, contract.predictiveHostStabilityBarrierHash);

const gate = evaluateFormalHostIsolationFinalGate({
  contract,
  repair: {
    primaryRootCause: 'A_formal_harness_repeatability_or_order_bias_defect',
  },
});
assert.equal(gate.status, 'passed');
assert.equal(gate.runtimeFreezeBindingEnabled, true);

assert.equal(validateDatasetMeasurementSeparation({
  datasetBuildCompleted: true,
  actualRows: 1900150,
  datasetWriterExited: true,
  datasetWriteConnectionCount: 0,
  immediateLoadAfterDataset: true,
}).status, 'failed');
assert.equal(validateDatasetMeasurementSeparation({
  datasetBuildCompleted: true,
  actualRows: 1900150,
  datasetWriterExited: true,
  datasetWriteConnectionCount: 0,
  immediateLoadAfterDataset: false,
}).status, 'passed');

const warmup = {
  formalWarmupVersion: 1,
  warmupErrorCount: 0,
  warmupTimeoutCount: 0,
  warmupSequenceHash: contract.warmupManifest.warmupSequenceHash,
  warmupBranchCountsMatch: true,
  warmupIncludedInFormalMetrics: false,
};
assert.equal(validateWarmupEvidence({ ...warmup, warmupIncludedInFormalMetrics: true }, contract).status, 'failed');
assert.equal(validateWarmupEvidence(warmup, contract).status, 'passed');

assert.equal(validateQuietWindowEvidence({
  hostQuietWindowVersion: HOST_QUIET_WINDOW_VERSION,
  readinessThresholdHash: contract.hostQuietWindowContract.readinessThresholdHash,
  requiredConsecutiveSamples: contract.hostQuietWindowContract.requiredConsecutiveSamples,
  hostQuietWindowPassed: false,
}, contract).status, 'failed');
assert.equal(validateQuietWindowEvidence({
  hostQuietWindowVersion: HOST_QUIET_WINDOW_VERSION,
  readinessThresholdHash: contract.hostQuietWindowContract.readinessThresholdHash,
  requiredConsecutiveSamples: contract.hostQuietWindowContract.requiredConsecutiveSamples,
  hostQuietWindowPassed: true,
}, contract).status, 'passed');

assert.equal(validatePredictiveHostStabilityEvidence({
  predictiveHostStabilityBarrierVersion: 1,
  predictiveReadinessThresholdHash: contract.predictiveHostStabilityBarrier.predictiveReadinessThresholdHash,
  observedWindows: contract.predictiveHostStabilityBarrier.requiredObservationWindows,
  predictiveHostStabilityPassed: false,
  failureReason: 'checkpoint_active',
}, contract).status, 'failed');
assert.equal(validatePredictiveHostStabilityEvidence({
  predictiveHostStabilityBarrierVersion: 1,
  predictiveReadinessThresholdHash: contract.predictiveHostStabilityBarrier.predictiveReadinessThresholdHash,
  observedWindows: contract.predictiveHostStabilityBarrier.requiredObservationWindows,
  predictiveHostStabilityPassed: true,
}, contract).status, 'passed');

const zeroPrecheck = {
  listener18080Count: 0,
  unknownP7ProcessCount: 0,
  activeP7DatabaseConnectionCount: 0,
  activePriorRunConnectionCount: 0,
  unexpectedGoProcessCount: 0,
  unexpectedK6ProcessCount: 0,
  unexpectedNodeHarnessProcessCount: 0,
};
assert.equal(validateResourcePrecheck(zeroPrecheck).measurementStartBlocked, false);
assert.equal(validateResourcePrecheck({ ...zeroPrecheck, listener18080Count: 1 }).measurementStartBlocked, true);

assert.equal(validateBackgroundProcessGate({
  activeGoBuildCount: 0,
  activePnpmInstallCount: 0,
  activeGitCompressionCount: 0,
  activeDatabaseDumpCount: 0,
  activeDiagnosticRunnerCount: 0,
}).status, 'passed');
assert.equal(validateBackgroundProcessGate({
  activeGoBuildCount: 1,
  activePnpmInstallCount: 0,
  activeGitCompressionCount: 0,
  activeDatabaseDumpCount: 0,
  activeDiagnosticRunnerCount: 0,
}).status, 'failed');

assert.equal(contract.lifecycleContract.steps.length, 15);
assert.equal(contract.lifecycleSchemaVersion, LIFECYCLE_SCHEMA_VERSION);

console.log(JSON.stringify({ phase: 'P7-V2-R3B-FORMAL-HOST-ISOLATION-FIXTURES', status: 'passed', fixtures: 22 }, null, 2));

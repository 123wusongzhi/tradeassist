import assert from 'node:assert/strict';
import {
  DEDICATED_BENCHMARK_HOST_CONTRACT,
  DEDICATED_BENCHMARK_HOST_CONTRACT_VERSION,
  FORMAL_HOST_ISOLATION_VERSION,
  buildHostFingerprint,
  dedicatedBenchmarkHostContractHash,
  validateDedicatedBenchmarkHostFacts,
} from '../../../scripts/p7-v2-r3b-dedicated-benchmark-host-preflight.mjs';
import { buildDedicatedBenchmarkHostFinalGateReport } from '../../../scripts/p7-v2-r3b-dedicated-benchmark-host-final-gate.mjs';
import { validateDedicatedHostValidationMatrix } from '../../../scripts/p7-v2-r3b-host-isolation-validation-gate.mjs';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';

const contractHash = dedicatedBenchmarkHostContractHash();

function validFacts(overrides = {}) {
  const facts = {
    formalHostIsolationVersion: FORMAL_HOST_ISOLATION_VERSION,
    dedicatedBenchmarkHostContractVersion: DEDICATED_BENCHMARK_HOST_CONTRACT_VERSION,
    hostContractHash: contractHash,
    hostContractStartHash: contractHash,
    hostContractEndHash: contractHash,
    hostname: 'bench-01',
    machineIdHash: 'a'.repeat(64),
    architecture: 'x64',
    kernelVersion: '6.8.0',
    cpuModel: 'Benchmark CPU',
    logicalCpuCount: 16,
    nodePlatform: 'linux',
    nodeArch: 'x64',
    GOOS: 'linux',
    GOARCH: 'amd64',
    repositoryOnNativeLinuxFilesystem: true,
    postgresDataOnNativeLinuxFilesystem: true,
    rawEvidenceOnNativeLinuxFilesystem: true,
    filesystemType: 'ext4',
    benchmarkFilesystemMount: '/opt/trademind-benchmark',
    freeDiskBytes: 120 * 1024 * 1024 * 1024,
    memoryTotalBytes: 64 * 1024 * 1024 * 1024,
    swapTotalBytes: 0,
    swapUsedBytes: 0,
    cpuQuotaStable: true,
    dockerDaemonReachable: true,
    gccPath: '/usr/bin/gcc',
    k6Version: 'k6 v0.57.0 (commit/abcdef, go1.24.0, linux/amd64)',
    postgresVersion: 'postgres (PostgreSQL) 16.3',
    repositoryGitDirty: false,
    unknownHeavyProcessCount: 0,
    concurrentBuildProcessCount: 0,
    unknownDatabaseWorkloadCount: 0,
    schedulerSamples: [
      { loadAverage1m: 2, logicalCpuCount: 16, cpuStealPct: 0, ioWaitPct: 0.2 },
      { loadAverage1m: 2.5, logicalCpuCount: 16, cpuStealPct: 0, ioWaitPct: 0.3 },
    ],
    timeSyncActive: true,
    thermalThrottleDetected: false,
    businessRuntimeChanged: false,
    loadContractChanged: false,
    formalPlanCreated: false,
    runtimeFreezeCreated: false,
    formalPairStarted: false,
    contract: DEDICATED_BENCHMARK_HOST_CONTRACT,
    ...overrides,
  };
  return { ...facts, hostFingerprint: buildHostFingerprint(facts) };
}

function assertFailed(id, overrides) {
  const result = validateDedicatedBenchmarkHostFacts(validFacts(overrides));
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateDedicatedBenchmarkHostFacts(validFacts()).status, 'passed');
assertFailed('nativeLinuxFilesystem', { repositoryOnNativeLinuxFilesystem: false, benchmarkFilesystemMount: '/mnt/d/project' });
assertFailed('unknownHeavyProcessCount', { unknownHeavyProcessCount: 1 });
assertFailed('exclusiveBenchmarkWindow', { concurrentBuildProcessCount: 1 });
assertFailed('dockerDaemonReachable', { dockerDaemonReachable: false });
assertFailed('gccPath', { gccPath: '' });
assertFailed('k6VersionMatch', { k6Version: 'k6 v0.56.0' });
assertFailed('freeDiskHeadroomPassed', { freeDiskBytes: 1024 });
assertFailed('schedulerContentionDetected', { schedulerSamples: [{ loadAverage1m: 20, logicalCpuCount: 4, cpuStealPct: 2, ioWaitPct: 0 }] });
assertFailed('activeSwapDuringWindow', { swapUsedBytes: 4096 });
assertFailed('unknownDatabaseWorkloadCount', { unknownDatabaseWorkloadCount: 1 });
assertFailed('hostContractImmutable', { hostContractEndHash: 'b'.repeat(64) });

const goodGate = buildDedicatedBenchmarkHostFinalGateReport({ ...validFacts(), status: 'passed' });
assert.equal(goodGate.status, 'passed');
assert.equal(
  buildDedicatedBenchmarkHostFinalGateReport({ ...validFacts(), status: 'passed', formalPlanCreated: true }).status,
  'failed',
  'formal manifest creation must fail the host gate',
);
assert.equal(
  buildDedicatedBenchmarkHostFinalGateReport({ ...validFacts(), status: 'passed', runtimeFreezeCreated: true }).status,
  'failed',
  'runtime freeze creation must fail the host gate',
);

const validMatrix = {
  formalHostIsolationVersion: FORMAL_HOST_ISOLATION_VERSION,
  dedicatedBenchmarkHostContractVersion: DEDICATED_BENCHMARK_HOST_CONTRACT_VERSION,
  validationMatrixRunCount: 4,
  runOrder: 'B-C-C-B',
  B1Completed: true,
  C1Completed: true,
  C2Completed: true,
  B2Completed: true,
  allRunsIndependent: true,
  allDatasetRows: [1900150, 1900150, 1900150, 1900150],
  hostFingerprintStable: true,
  hostContractHashMatch: true,
  baselineBinarySha256Match: true,
  currentBinarySha256Match: true,
  inputSequenceHashMatch: true,
  branchMixFingerprintMatch: true,
  warmupSequenceHashMatch: true,
  postgresConfigHashMatch: true,
  lifecycleStepSequenceHashMatch: true,
  baselineSelfMaterialRegressionCount: 0,
  currentSelfMaterialRegressionCount: 0,
  orderPositionEffectDetected: false,
  laterRunDegradationDetected: false,
  hostStateMismatchCount: 0,
  hostContractViolationCount: 0,
  datasetBarrierFailureCount: 0,
  predictiveStabilityFailureCount: 0,
  businessRuntimeChanged: false,
  loadContractChanged: false,
  validForFormalPlan: true,
};
assert.equal(validateDedicatedHostValidationMatrix(validMatrix).status, 'passed');
assert.equal(
  validateDedicatedHostValidationMatrix({ ...validMatrix, hostFingerprintStable: false }).status,
  'failed',
  'host fingerprint drift must fail validation',
);

const report = { phase: 'P7-V2-R3B-DEDICATED-BENCHMARK-HOST-CONTRACT', status: 'passed', fixtures: 15 };
writeJSON('docs/p7-v2-r3b-dedicated-benchmark-host-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));

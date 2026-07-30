import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { gitCommit, readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import {
  DEDICATED_BENCHMARK_HOST_CONTRACT_VERSION,
  DEDICATED_HOST_PREFLIGHT_JSON,
  FORMAL_HOST_ISOLATION_VERSION,
  dedicatedBenchmarkHostContractHash,
  validateDedicatedBenchmarkHostFacts,
} from './p7-v2-r3b-dedicated-benchmark-host-preflight.mjs';

export const DEDICATED_HOST_GATE_JSON = 'docs/p7-v2-r3b-dedicated-benchmark-host-final-gate.json';
export const DEDICATED_HOST_GATE_MD = 'docs/P7_V2_R3B_DEDICATED_BENCHMARK_HOST_FINAL_GATE.md';

export function buildDedicatedBenchmarkHostFinalGateReport(preflight = readJSON(DEDICATED_HOST_PREFLIGHT_JSON) || {}) {
  const validation = validateDedicatedBenchmarkHostFacts(preflight);
  const expectedHash = dedicatedBenchmarkHostContractHash(preflight.contract);
  const checks = [
    ['preflightStatusPassed', preflight.status === 'passed'],
    ['formalHostIsolationVersion', Number(preflight.formalHostIsolationVersion) === FORMAL_HOST_ISOLATION_VERSION],
    ['dedicatedBenchmarkHostContractVersion', Number(preflight.dedicatedBenchmarkHostContractVersion) === DEDICATED_BENCHMARK_HOST_CONTRACT_VERSION],
    ['nativeLinuxFilesystem', validation.nativeLinuxFilesystem === true],
    ['exclusiveBenchmarkWindow', validation.exclusiveBenchmarkWindow === true],
    ['toolchainAttestationPassed', validation.k6VersionMatch === true && preflight.dockerDaemonReachable === true && Boolean(preflight.gccPath)],
    ['dockerDaemonReachable', preflight.dockerDaemonReachable === true],
    ['k6VersionMatch', validation.k6VersionMatch === true],
    ['schedulerContentionDetected', validation.schedulerContentionDetected === false],
    ['backgroundIoContentionDetected', validation.backgroundIoContentionDetected === false],
    ['activeSwapDuringWindow', validation.activeSwapDuringWindow === false],
    ['unknownHeavyProcessCount', Number(preflight.unknownHeavyProcessCount || 0) === 0],
    ['unknownDatabaseWorkloadCount', Number(preflight.unknownDatabaseWorkloadCount || 0) === 0],
    ['hostContractImmutable', validation.hostContractImmutable === true && preflight.hostContractHash === expectedHash],
    ['hostFingerprintPresent', /^[a-f0-9]{64}$/.test(preflight.hostFingerprint || '')],
    ['businessRuntimeChanged', preflight.businessRuntimeChanged === false],
    ['loadContractChanged', preflight.loadContractChanged === false],
    ['formalPlanCreated', preflight.formalPlanCreated === false],
    ['runtimeFreezeCreated', preflight.runtimeFreezeCreated === false],
    ['formalPairStarted', preflight.formalPairStarted === false],
  ];
  const failed = [
    ...validation.failed.map((id) => `preflight:${id}`),
    ...checks.filter(([, ok]) => !ok).map(([id]) => id),
  ];
  const uniqueFailed = [...new Set(failed)];
  return {
    phase: 'P7-V2-R3B-DEDICATED-BENCHMARK-HOST-CONTRACT',
    status: uniqueFailed.length ? 'failed' : 'passed',
    failedCount: uniqueFailed.length,
    failed: uniqueFailed,
    checkedAt: new Date().toISOString(),
    gitCommit: gitCommit(),
    sourcePreflight: DEDICATED_HOST_PREFLIGHT_JSON,
    formalHostIsolationVersion: Number(preflight.formalHostIsolationVersion || 0),
    dedicatedBenchmarkHostContractVersion: Number(preflight.dedicatedBenchmarkHostContractVersion || 0),
    nativeLinuxFilesystem: validation.nativeLinuxFilesystem,
    exclusiveBenchmarkWindow: validation.exclusiveBenchmarkWindow,
    toolchainAttestationPassed: validation.k6VersionMatch === true && preflight.dockerDaemonReachable === true && Boolean(preflight.gccPath),
    dockerDaemonReachable: preflight.dockerDaemonReachable === true,
    k6VersionMatch: validation.k6VersionMatch,
    schedulerContentionDetected: validation.schedulerContentionDetected,
    backgroundIoContentionDetected: validation.backgroundIoContentionDetected,
    activeSwapDuringWindow: validation.activeSwapDuringWindow,
    unknownHeavyProcessCount: Number(preflight.unknownHeavyProcessCount || 0),
    unknownDatabaseWorkloadCount: Number(preflight.unknownDatabaseWorkloadCount || 0),
    hostContractImmutable: validation.hostContractImmutable === true && preflight.hostContractHash === expectedHash,
    hostFingerprintPresent: /^[a-f0-9]{64}$/.test(preflight.hostFingerprint || ''),
    hostFingerprint: preflight.hostFingerprint || '',
    hostContractHash: preflight.hostContractHash || '',
    businessRuntimeChanged: preflight.businessRuntimeChanged === true,
    loadContractChanged: preflight.loadContractChanged === true,
    formalPlanCreated: preflight.formalPlanCreated === true,
    runtimeFreezeCreated: preflight.runtimeFreezeCreated === true,
    formalPairStarted: preflight.formalPairStarted === true,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function writeDedicatedBenchmarkHostFinalGate(report) {
  writeJSON(DEDICATED_HOST_GATE_JSON, report);
  writeMarkdown(DEDICATED_HOST_GATE_MD, `# P7-V2-R3B Dedicated Benchmark Host Final Gate

Status: **${report.status}**

- Formal host isolation version: ${report.formalHostIsolationVersion}
- Dedicated benchmark host contract version: ${report.dedicatedBenchmarkHostContractVersion}
- Native Linux filesystem: ${report.nativeLinuxFilesystem}
- Exclusive benchmark window: ${report.exclusiveBenchmarkWindow}
- Toolchain attestation passed: ${report.toolchainAttestationPassed}
- Host fingerprint present: ${report.hostFingerprintPresent}
- Host contract immutable: ${report.hostContractImmutable}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This gate closes only the dedicated-host tooling contract. It does not make P7-V2 production ready and does not authorize a formal run.
`);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildDedicatedBenchmarkHostFinalGateReport();
  writeDedicatedBenchmarkHostFinalGate(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}

import { fileURLToPath } from 'node:url';
import path from 'node:path';
import {
  buildHostIsolationPlanBinding,
  FORMAL_HOST_ISOLATION_VERSION,
  HOST_ISOLATION_CONTRACT_PATH,
  HOST_ISOLATION_REPORT_PATH,
  validateFormalHostIsolationContract,
} from './p7-v2-r3b-formal-host-isolation.mjs';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const OUT_JSON = 'docs/p7-v2-r3b-formal-host-isolation-final-gate.json';
const OUT_MD = 'docs/P7_V2_R3B_FORMAL_HOST_ISOLATION_FINAL_GATE.md';

export function evaluateFormalHostIsolationFinalGate({
  contract = readJSON(HOST_ISOLATION_CONTRACT_PATH) || {},
  repair = readJSON(HOST_ISOLATION_REPORT_PATH) || {},
} = {}) {
  const contractValidation = validateFormalHostIsolationContract(contract);
  const binding = buildHostIsolationPlanBinding(contract);
  const checks = {
    formalHostIsolationVersion: contract.formalHostIsolationVersion === FORMAL_HOST_ISOLATION_VERSION,
    primaryHarnessSubRootCausePresent: typeof contract.primaryHarnessSubRootCause === 'string' && contract.primaryHarnessSubRootCause.length > 0,
    repairMatchesEvidence: contract.repairMatchesEvidence === true && repair.primaryRootCause === 'A_formal_harness_repeatability_or_order_bias_defect',
    baselineCurrentLifecycleSymmetric: contract.lifecycleContract?.symmetricForRoles?.includes('baseline') && contract.lifecycleContract?.symmetricForRoles?.includes('current'),
    lifecycleStepSequenceHashMatch: contractValidation.lifecycleStepSequenceHashMatch === true,
    datasetMeasurementSeparated: contract.databasePostDatasetBarrier?.databasePostDatasetBarrierVersion === 1 && contract.databasePostDatasetBarrier?.fixedSleepSubstituteAllowed === false,
    warmupEnabled: contract.warmupManifest?.formalWarmupVersion === 1,
    warmupSequenceBound: /^[a-f0-9]{64}$/.test(contract.warmupManifest?.warmupSequenceHash || ''),
    warmupExcludedFromMetrics: contract.warmupManifest?.excludedFromFormalMetrics === true,
    cooldownEnabled: contract.cooldownContract?.formalCooldownVersion === 1,
    hostQuietWindowEnabled: contract.hostQuietWindowContract?.hostQuietWindowVersion === 1,
    hostQuietWindowDeterministic: contract.hostQuietWindowContract?.deterministic === true,
    predictiveHostStabilityBarrierEnabled: contract.predictiveHostStabilityBarrier?.predictiveHostStabilityBarrierVersion === 1,
    predictiveHostStabilityBarrierBound: /^[a-f0-9]{64}$/.test(contract.predictiveHostStabilityBarrierHash || '') && /^[a-f0-9]{64}$/.test(contract.predictiveHostStabilityBarrier?.predictiveReadinessThresholdHash || ''),
    postgresIsolationContractPresent: contract.postgresIsolationContract?.postgresIsolationMode === 'dedicated_ephemeral_postgres_instance_per_run',
    backgroundProcessGateEnabled: contract.backgroundProcessGate?.measurementStartBlockedOnAnyNonzero === true,
    evidenceIoIsolationEnabled: contract.evidenceWriterContract?.evidenceWriterMode === 'async_or_outside_measurement_window',
    comparabilityV5BindingEnabled: contract.comparabilityVersion === 5,
    runtimeFreezeBindingEnabled: contract.runtimeFreezeBindingFields?.every((field) => field === 'formalHostIsolationVersion' || /^[a-f0-9]{64}$/.test(contract[field] || '')),
    businessRuntimeChanged: contract.businessRuntimeChanged === false,
    thresholdChanged: contract.thresholdChanged === false,
    sloChanged: contract.sloChanged === false,
    materialityChanged: contract.materialityChanged === false,
    vusChanged: contract.vusChanged === false,
    stagesChanged: contract.stagesChanged === false,
    durationChanged: contract.durationChanged === false,
    datasetChanged: contract.datasetChanged === false,
    inputSequenceChanged: contract.inputSequenceChanged === false,
  };
  const failedChecks = [
    ...Object.entries(checks).filter(([, passed]) => !passed).map(([id]) => id),
    ...contractValidation.issues,
  ];
  return {
    phase: 'P7-V2-R3B-FORMAL-HOST-ISOLATION-FINAL-GATE',
    status: failedChecks.length ? 'failed' : 'passed',
    ...binding,
    primaryHarnessSubRootCause: contract.primaryHarnessSubRootCause || '',
    secondaryHarnessSubRootCauses: contract.secondaryHarnessSubRootCauses || [],
    confidence: contract.confidence || '',
    ...checks,
    formalHostIsolationVersion: contract.formalHostIsolationVersion ?? null,
    businessRuntimeChanged: contract.businessRuntimeChanged === true,
    thresholdChanged: contract.thresholdChanged === true,
    sloChanged: contract.sloChanged === true,
    materialityChanged: contract.materialityChanged === true,
    vusChanged: contract.vusChanged === true,
    stagesChanged: contract.stagesChanged === true,
    durationChanged: contract.durationChanged === true,
    datasetChanged: contract.datasetChanged === true,
    inputSequenceChanged: contract.inputSequenceChanged === true,
    failedChecks,
    failed: failedChecks,
    failedCount: failedChecks.length,
    checks: Object.entries(checks).map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
    sourceContract: HOST_ISOLATION_CONTRACT_PATH,
    sourceRepair: HOST_ISOLATION_REPORT_PATH,
    generatedAt: new Date().toISOString(),
  };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const gate = evaluateFormalHostIsolationFinalGate();
  writeJSON(OUT_JSON, gate);
  writeMarkdown(
    OUT_MD,
    `# P7-V2-R3B Formal Host Isolation Final Gate

Status: **${gate.status}**

- Formal host isolation version: \`${gate.formalHostIsolationVersion ?? 'missing'}\`
- Lifecycle sequence match: \`${gate.lifecycleStepSequenceHashMatch}\`
- Dataset/measurement separated: \`${gate.datasetMeasurementSeparated}\`
- Warmup sequence bound: \`${gate.warmupSequenceBound}\`
- Host quiet window enabled: \`${gate.hostQuietWindowEnabled}\`
- Predictive host stability barrier enabled: \`${gate.predictiveHostStabilityBarrierEnabled}\`
- PostgreSQL isolation contract present: \`${gate.postgresIsolationContractPresent}\`
- Runtime freeze binding enabled: \`${gate.runtimeFreezeBindingEnabled}\`
- Failed checks: ${gate.failedCount ? gate.failedChecks.join(', ') : 'none'}

This gate only validates the repaired harness contract. The post-repair B-C-C-B validation matrix remains a separate diagnostic-only gate.
`,
  );
  console.log(JSON.stringify(gate, null, 2));
  process.exit(gate.status === 'passed' ? 0 : 1);
}

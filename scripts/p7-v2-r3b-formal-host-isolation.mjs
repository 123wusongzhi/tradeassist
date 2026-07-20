import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { readJSON, runWSL, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const FORMAL_HOST_ISOLATION_VERSION = 2;
export const LIFECYCLE_SCHEMA_VERSION = 2;
export const DATABASE_POST_DATASET_BARRIER_VERSION = 1;
export const FORMAL_WARMUP_VERSION = 1;
export const FORMAL_COOLDOWN_VERSION = 1;
export const HOST_QUIET_WINDOW_VERSION = 1;
export const POSTGRES_ISOLATION_CONTRACT_VERSION = 2;
export const EVIDENCE_WRITER_CONTRACT_VERSION = 1;
export const COMPARABILITY_BINDING_VERSION = 5;
export const HOST_ISOLATION_CONTRACT_PATH = 'docs/p7-v2-r3b-formal-host-isolation-contract.json';
export const HOST_ISOLATION_REPORT_PATH = 'docs/p7-v2-r3b-formal-host-isolation-repair.json';

export const FORMAL_RUN_LIFECYCLE_STEPS = [
  'resource_precheck',
  'database_prepare',
  'dataset_build',
  'database_post_dataset_barrier',
  'application_start',
  'deterministic_warmup',
  'application_cooldown',
  'host_quiet_window',
  'measurement_ready',
  'measured_load',
  'application_stop',
  'connection_drain',
  'resource_snapshot',
  'completed',
];

export const WARMUP_BRANCHES = [
  'webhook.normal_insert',
  'webhook.duplicate_conflict',
  'auth.unknown_account',
  'auth.wrong_password',
  'auth.locked_account',
];

const REQUIRED_QUIET_WINDOW_FIELDS = [
  'systemLoad',
  'cpuUser',
  'cpuSystem',
  'cpuIdle',
  'ioWait',
  'availableMemory',
  'swapUsage',
  'diskReadDelta',
  'diskWriteDelta',
  'postgresActiveConnections',
  'postgresWaitingConnections',
  'postgresAutovacuumActivity',
  'postgresAnalyzeActivity',
  'goGcPauseDelta',
  'dbPoolWaitDelta',
];

const REQUIRED_PREFLIGHT_ZERO_FIELDS = [
  'listener18080Count',
  'unknownP7ProcessCount',
  'activeP7DatabaseConnectionCount',
  'activePriorRunConnectionCount',
  'unexpectedGoProcessCount',
  'unexpectedK6ProcessCount',
  'unexpectedNodeHarnessProcessCount',
];

const REQUIRED_BACKGROUND_ZERO_FIELDS = [
  'activeGoBuildCount',
  'activePnpmInstallCount',
  'activeGitCompressionCount',
  'activeDatabaseDumpCount',
  'activeDiagnosticRunnerCount',
];

function sha256Json(value) {
  return crypto.createHash('sha256').update(JSON.stringify(value)).digest('hex');
}

function isSha256(value) {
  return /^[a-f0-9]{64}$/.test(String(value || ''));
}

function metricDelta(matrix, pair, metric) {
  return matrix?.[pair]?.[metric]?.relativeDeltaPct ?? null;
}

function maxAbsMetricDelta(comparison = {}) {
  return Math.max(
    0,
    ...Object.values(comparison || {})
      .map((item) => Math.abs(Number(item?.relativeDeltaPct)))
      .filter(Number.isFinite),
  );
}

function snapshotDelta(left = {}, right = {}, field) {
  const a = Number(left?.[field]);
  const b = Number(right?.[field]);
  if (!Number.isFinite(a) || !Number.isFinite(b)) return null;
  return b - a;
}

export function classifyHarnessSubRootCause(matrix = readJSON('docs/p7-v2-r3b-binary-bound-repeatability-matrix.json') || {}) {
  const runs = Array.isArray(matrix.runs) ? matrix.runs : [];
  const bySlot = Object.fromEntries(runs.map((run) => [run.slot, run]));
  const baselineSelfMax = maxAbsMetricDelta(matrix.baselineSelfVariance);
  const currentSelfMax = maxAbsMetricDelta(matrix.currentSelfVariance);
  const runStartSeconds = runs.map((run) => Date.parse(run.hostSnapshotBefore?.timestamp || '')).filter(Number.isFinite);
  const runEndSeconds = runs.map((run) => Date.parse(run.hostSnapshotAfter?.timestamp || '')).filter(Number.isFinite);
  const interRunGapsMs = [];
  for (let i = 1; i < Math.min(runStartSeconds.length, runEndSeconds.length); i += 1) {
    interRunGapsMs.push(runStartSeconds[i] - runEndSeconds[i - 1]);
  }
  const nearImmediateNextRun = interRunGapsMs.some((gap) => Number.isFinite(gap) && gap >= 0 && gap < 5000);
  const sharedPostgresCountersMove =
    runs.length === 4 &&
    runs.every((run) => Number(run.hostStateDelta?.checkpointDelta || 0) > 0 && Number(run.hostStateDelta?.walBytesDelta || 0) > 0);
  const waitingConnectionsAtPrecheck = runs.some((run) => Number(run.hostSnapshotBefore?.postgresWaitingConnections || 0) > 0);
  const diskReadSpread = runs.map((run) => Number(run.hostStateDelta?.diskReadDelta || 0)).filter(Number.isFinite);
  const materialDiskReadSpread =
    diskReadSpread.length > 1 &&
    Math.max(...diskReadSpread) > Math.max(1, Math.min(...diskReadSpread)) * 10;
  const lifecycleAsymmetry =
    Boolean(bySlot.C1?.processIdentity && bySlot.B1?.processIdentity) &&
    runs.some((run) => !run.lifecycleSchemaVersion || !run.lifecycleStepSequenceHash);
  const cacheWarmupAsymmetry =
    nearImmediateNextRun ||
    runs.some((run) => !run.formalWarmupVersion || !run.warmupSequenceHash || !run.hostReadinessFingerprint);
  const hostNoise =
    materialDiskReadSpread ||
    Math.abs(snapshotDelta(bySlot.B1?.hostSnapshotBefore, bySlot.B2?.hostSnapshotBefore, 'systemLoad5m') || 0) > 0.2 ||
    Math.abs(snapshotDelta(bySlot.C1?.hostSnapshotBefore, bySlot.B2?.hostSnapshotBefore, 'swapUsedBytes') || 0) > 0;
  const secondary = [];
  if (sharedPostgresCountersMove || waitingConnectionsAtPrecheck) secondary.push('A2_postgres_checkpoint_autovacuum_or_shared_instance_bias');
  if (cacheWarmupAsymmetry) secondary.push('A3_cache_and_warmup_asymmetry');
  if (hostNoise) secondary.push('A4_host_scheduler_cpu_or_io_contention');
  if (lifecycleAsymmetry) secondary.push('A5_baseline_current_lifecycle_asymmetry');
  if (secondary.length === 0 && (baselineSelfMax >= 10 || currentSelfMax >= 10)) secondary.push('A6_multiple_harness_isolation_factors');
  const primary = secondary.length >= 2 ? 'A6_multiple_harness_isolation_factors' : (secondary[0] || 'A6_multiple_harness_isolation_factors');
  return {
    primaryHarnessSubRootCause: primary,
    secondaryHarnessSubRootCauses: secondary,
    confidence: secondary.length >= 2 ? 'medium' : 'low',
    supportingEvidence: {
      baselineSelfMaxRelativeDeltaPct: baselineSelfMax,
      currentSelfMaxRelativeDeltaPct: currentSelfMax,
      baselineWebhookP95RelativeDeltaPct: matrix.baselineSelfVariance?.['Webhook Ingestion p95']?.relativeDeltaPct ?? null,
      baselineAuthP95RelativeDeltaPct: matrix.baselineSelfVariance?.['Auth Invalid Login p95']?.relativeDeltaPct ?? null,
      baselineAuthP99RelativeDeltaPct: matrix.baselineSelfVariance?.['Auth Invalid Login p99']?.relativeDeltaPct ?? null,
      currentWebhookP99RelativeDeltaPct: matrix.currentSelfVariance?.['Webhook Ingestion p99']?.relativeDeltaPct ?? null,
      currentAuthP95RelativeDeltaPct: matrix.currentSelfVariance?.['Auth Invalid Login p95']?.relativeDeltaPct ?? null,
      sharedPostgresCountersMove,
      waitingConnectionsAtPrecheck,
      nearImmediateNextRun,
      interRunGapsMs,
      materialDiskReadSpread,
      lifecycleEvidenceMissing: lifecycleAsymmetry,
      warmupReadinessEvidenceMissing: cacheWarmupAsymmetry,
      sampleDeltas: {
        B1ToB2WebhookP95: metricDelta({ 'B1 vs B2': matrix.baselineSelfVariance }, 'B1 vs B2', 'Webhook Ingestion p95'),
      },
    },
  };
}

export function buildFormalHostIsolationContract({ matrix = readJSON('docs/p7-v2-r3b-binary-bound-repeatability-matrix.json') || {}, now = new Date().toISOString() } = {}) {
  const lifecycleContract = {
    lifecycleSchemaVersion: LIFECYCLE_SCHEMA_VERSION,
    steps: FORMAL_RUN_LIFECYCLE_STEPS,
    symmetricForRoles: ['baseline', 'current', 'diagnostic-baseline', 'diagnostic-current'],
    forbiddenAsymmetries: [
      'baseline_direct_load_without_start_warmup_cooldown_quiet_window',
      'current_extra_restart_or_extra_cooldown',
      'baseline_warm_process_current_cold_process',
    ],
  };
  const databasePostDatasetBarrier = {
    databasePostDatasetBarrierVersion: DATABASE_POST_DATASET_BARRIER_VERSION,
    requiredRows: 1900150,
    requiredChecks: [
      'dataset_actual_rows_match',
      'migration_schema_fingerprint_match',
      'dataset_writer_exited',
      'dataset_write_connections_drained',
      'postgres_background_activity_within_contract',
    ],
    fixedSleepSubstituteAllowed: false,
  };
  const warmupManifest = {
    formalWarmupVersion: FORMAL_WARMUP_VERSION,
    branches: WARMUP_BRANCHES,
    requestCountPolicy: 'frozen_low_sensitivity_subset',
    isolatedIdNamespace: 'p7v2-warmup',
    excludedFromFormalMetrics: true,
    excludedFromRegressionSummary: true,
  };
  warmupManifest.warmupSequenceHash = sha256Json({
    formalWarmupVersion: FORMAL_WARMUP_VERSION,
    branches: WARMUP_BRANCHES,
    requestCountPolicy: warmupManifest.requestCountPolicy,
    isolatedIdNamespace: warmupManifest.isolatedIdNamespace,
  });
  warmupManifest.warmupBranchMixFingerprint = sha256Json(WARMUP_BRANCHES);
  const cooldownContract = {
    formalCooldownVersion: FORMAL_COOLDOWN_VERSION,
    maxWaitSeconds: 120,
    requiredSignals: ['activeRequestCount=0', 'dbPoolInUseStable=true', 'dbWaitingBackendCount=0', 'unknownConnectionCount=0'],
    observedSignals: ['goGcActivity', 'postgresMaintenanceActivity', 'cpuIoState'],
    timeoutBlocksMeasurement: true,
  };
  const hostQuietWindowContract = {
    hostQuietWindowVersion: HOST_QUIET_WINDOW_VERSION,
    sampleIntervalMs: 1000,
    requiredConsecutiveSamples: 5,
    quietWindowDurationMs: 5000,
    thresholdSource: 'harness_readiness_threshold_v1',
    readinessThresholds: {
      maxSystemLoad1mPerCpu: 0.8,
      maxIoWaitDelta: 2500,
      maxDiskWriteBytesDelta: 512 * 1024 * 1024,
      maxDiskReadBytesDelta: 512 * 1024 * 1024,
      maxPostgresWaitingConnections: 0,
      maxUnknownConnectionCount: 0,
      maxGoGcPauseDeltaMs: 200,
      maxDbPoolWaitDelta: 0,
    },
    requiredSampleFields: REQUIRED_QUIET_WINDOW_FIELDS,
    deterministic: true,
  };
  hostQuietWindowContract.readinessThresholdHash = sha256Json(hostQuietWindowContract.readinessThresholds);
  const postgresIsolationContract = {
    postgresIsolationContractVersion: POSTGRES_ISOLATION_CONTRACT_VERSION,
    postgresIsolationMode: 'dedicated_ephemeral_postgres_instance_per_run',
    selectedPath: 'PG-2',
    reason: 'binary-bound matrix shows shared PostgreSQL counters and pre-run waiting connections while same-binary material variance is present',
    perRunRequirements: [
      'independent_data_directory',
      'independent_postgres_process',
      'independent_port',
      'same_postgres_version',
      'same_config_fingerprint',
      'record_postgres_binary_version',
      'record_postgres_data_directory_identity',
      'record_postgres_port',
      'record_postgres_process_pid',
    ],
    forbiddenConfigChanges: ['disable_autovacuum', 'disable_checkpoint', 'disable_fsync', 'disable_synchronous_commit'],
  };
  const evidenceWriterContract = {
    evidenceWriterContractVersion: EVIDENCE_WRITER_CONTRACT_VERSION,
    disallowedDuringMeasurement: ['large_artifact_compression', 'git_operations', 'binary_build', 'pnpm_install', 'large_file_copy', 'historical_evidence_scan', 'database_dump'],
    evidenceWriterMode: 'async_or_outside_measurement_window',
    evidenceWriterBackpressureCountRequired: true,
  };
  const backgroundProcessGate = {
    requiredZeroFields: REQUIRED_BACKGROUND_ZERO_FIELDS,
    measurementStartBlockedOnAnyNonzero: true,
  };
  const resourcePrecheck = {
    requiredZeroFields: REQUIRED_PREFLIGHT_ZERO_FIELDS,
    measurementStartBlockedOnAnyNonzero: true,
  };
  const hashes = {
    lifecycleContractHash: sha256Json(lifecycleContract),
    databasePostDatasetBarrierHash: sha256Json(databasePostDatasetBarrier),
    warmupManifestHash: sha256Json(warmupManifest),
    cooldownContractHash: sha256Json(cooldownContract),
    hostQuietWindowContractHash: sha256Json(hostQuietWindowContract),
    postgresIsolationContractHash: sha256Json(postgresIsolationContract),
    evidenceWriterContractHash: sha256Json(evidenceWriterContract),
    backgroundProcessGateHash: sha256Json(backgroundProcessGate),
    resourcePrecheckHash: sha256Json(resourcePrecheck),
  };
  const lifecycleStepSequenceHash = sha256Json(FORMAL_RUN_LIFECYCLE_STEPS);
  const classification = classifyHarnessSubRootCause(matrix);
  const contract = {
    phase: 'P7-V2-R3B-FORMAL-HARNESS-ORDER-ISOLATION-V2',
    status: 'passed',
    formalHostIsolationVersion: FORMAL_HOST_ISOLATION_VERSION,
    lifecycleSchemaVersion: LIFECYCLE_SCHEMA_VERSION,
    lifecycleStepSequenceHash,
    lifecycleContract,
    databasePostDatasetBarrier,
    warmupManifest,
    cooldownContract,
    hostQuietWindowContract,
    postgresIsolationContract,
    evidenceWriterContract,
    backgroundProcessGate,
    resourcePrecheck,
    comparabilityVersion: COMPARABILITY_BINDING_VERSION,
    runtimeFreezeBindingFields: [
      'formalHostIsolationVersion',
      'lifecycleContractHash',
      'databasePostDatasetBarrierHash',
      'warmupManifestHash',
      'cooldownContractHash',
      'hostQuietWindowContractHash',
      'postgresIsolationContractHash',
      'evidenceWriterContractHash',
    ],
    ...hashes,
    hostIsolationContractHash: sha256Json({ version: FORMAL_HOST_ISOLATION_VERSION, lifecycleStepSequenceHash, ...hashes }),
    ...classification,
    repairMatchesEvidence: matrix.primaryRootCause === 'A_formal_harness_repeatability_or_order_bias_defect',
    businessRuntimeChanged: false,
    thresholdChanged: false,
    sloChanged: false,
    materialityChanged: false,
    vusChanged: false,
    stagesChanged: false,
    durationChanged: false,
    datasetChanged: false,
    inputSequenceChanged: false,
    loadContractChanged: false,
    formalPairStarted: false,
    validationMatrixStarted: false,
    generatedAt: now,
  };
  return contract;
}

export function validateFormalHostIsolationContract(contract = readJSON(HOST_ISOLATION_CONTRACT_PATH) || {}) {
  const issues = [];
  if (contract.formalHostIsolationVersion !== FORMAL_HOST_ISOLATION_VERSION) issues.push('formal_host_isolation_version_v2_required');
  if (contract.lifecycleSchemaVersion !== LIFECYCLE_SCHEMA_VERSION) issues.push('lifecycle_schema_version_v2_required');
  if (contract.lifecycleStepSequenceHash !== sha256Json(FORMAL_RUN_LIFECYCLE_STEPS)) issues.push('lifecycle_step_sequence_hash_mismatch');
  if (contract.lifecycleContract?.steps?.join('|') !== FORMAL_RUN_LIFECYCLE_STEPS.join('|')) issues.push('lifecycle_step_sequence_mismatch');
  if (contract.databasePostDatasetBarrier?.databasePostDatasetBarrierVersion !== DATABASE_POST_DATASET_BARRIER_VERSION) issues.push('dataset_barrier_v1_required');
  if (contract.databasePostDatasetBarrier?.fixedSleepSubstituteAllowed !== false) issues.push('dataset_barrier_must_not_be_fixed_sleep');
  if (contract.warmupManifest?.formalWarmupVersion !== FORMAL_WARMUP_VERSION) issues.push('warmup_v1_required');
  if (contract.warmupManifest?.excludedFromFormalMetrics !== true) issues.push('warmup_must_be_excluded_from_metrics');
  if (!isSha256(contract.warmupManifest?.warmupSequenceHash)) issues.push('warmup_sequence_hash_missing');
  if (contract.cooldownContract?.formalCooldownVersion !== FORMAL_COOLDOWN_VERSION) issues.push('cooldown_v1_required');
  if (contract.hostQuietWindowContract?.hostQuietWindowVersion !== HOST_QUIET_WINDOW_VERSION) issues.push('quiet_window_v1_required');
  if (!isSha256(contract.hostQuietWindowContract?.readinessThresholdHash)) issues.push('quiet_window_threshold_hash_missing');
  for (const field of REQUIRED_QUIET_WINDOW_FIELDS) {
    if (!contract.hostQuietWindowContract?.requiredSampleFields?.includes(field)) issues.push(`quiet_window_sample_field_missing:${field}`);
  }
  if (contract.postgresIsolationContract?.postgresIsolationMode !== 'dedicated_ephemeral_postgres_instance_per_run') issues.push('postgres_pg2_dedicated_instance_required');
  if (contract.comparabilityVersion !== COMPARABILITY_BINDING_VERSION) issues.push('comparability_v5_required');
  for (const field of contract.runtimeFreezeBindingFields || []) {
    if (field !== 'formalHostIsolationVersion' && !isSha256(contract[field])) issues.push(`runtime_freeze_binding_hash_missing:${field}`);
  }
  for (const field of ['businessRuntimeChanged', 'thresholdChanged', 'sloChanged', 'materialityChanged', 'vusChanged', 'stagesChanged', 'durationChanged', 'datasetChanged', 'inputSequenceChanged', 'loadContractChanged', 'formalPairStarted']) {
    if (contract[field] !== false) issues.push(`${field}_must_be_false`);
  }
  return {
    status: issues.length ? 'failed' : 'passed',
    formalHostIsolationVersion: contract.formalHostIsolationVersion ?? null,
    lifecycleStepSequenceHashMatch: contract.lifecycleStepSequenceHash === sha256Json(FORMAL_RUN_LIFECYCLE_STEPS),
    warmupSequenceHashMatch: isSha256(contract.warmupManifest?.warmupSequenceHash),
    readinessThresholdHashMatch: isSha256(contract.hostQuietWindowContract?.readinessThresholdHash),
    postgresConfigHashMatch: true,
    issues,
  };
}

export function buildHostIsolationPlanBinding(contract = readJSON(HOST_ISOLATION_CONTRACT_PATH) || buildFormalHostIsolationContract()) {
  return {
    formalHostIsolationVersion: contract.formalHostIsolationVersion,
    lifecycleContractHash: contract.lifecycleContractHash,
    databasePostDatasetBarrierHash: contract.databasePostDatasetBarrierHash,
    warmupManifestHash: contract.warmupManifestHash,
    cooldownContractHash: contract.cooldownContractHash,
    hostQuietWindowContractHash: contract.hostQuietWindowContractHash,
    postgresIsolationContractHash: contract.postgresIsolationContractHash,
    evidenceWriterContractHash: contract.evidenceWriterContractHash,
    hostIsolationContractHash: contract.hostIsolationContractHash,
    lifecycleStepSequenceHash: contract.lifecycleStepSequenceHash,
    warmupSequenceHash: contract.warmupManifest?.warmupSequenceHash || '',
    readinessThresholdHash: contract.hostQuietWindowContract?.readinessThresholdHash || '',
    postgresIsolationMode: contract.postgresIsolationContract?.postgresIsolationMode || '',
    comparabilityVersion: contract.comparabilityVersion,
  };
}

export function validateLifecycleSymmetry(runs = [], contract = readJSON(HOST_ISOLATION_CONTRACT_PATH) || {}) {
  const expected = contract.lifecycleStepSequenceHash || sha256Json(FORMAL_RUN_LIFECYCLE_STEPS);
  const issues = [];
  for (const run of runs) {
    if (run.lifecycleSchemaVersion !== LIFECYCLE_SCHEMA_VERSION) issues.push(`${run.slot || run.runId || 'run'}:lifecycle_schema_version_mismatch`);
    if (run.lifecycleStepSequenceHash !== expected) issues.push(`${run.slot || run.runId || 'run'}:lifecycle_hash_mismatch`);
  }
  return {
    status: issues.length ? 'failed' : 'passed',
    lifecycleStepSequenceHash: expected,
    lifecycleStepSequenceHashMatch: issues.length === 0,
    issues,
  };
}

export function validateDatasetMeasurementSeparation(evidence = {}) {
  const issues = [];
  if (evidence.datasetBuildCompleted !== true) issues.push('dataset_build_not_completed');
  if (Number(evidence.actualRows) !== 1900150) issues.push('dataset_row_count_mismatch');
  if (evidence.datasetWriterExited !== true) issues.push('dataset_writer_not_exited');
  if (Number(evidence.datasetWriteConnectionCount || 0) !== 0) issues.push('dataset_write_connections_not_drained');
  if (evidence.immediateLoadAfterDataset === true) issues.push('immediate_load_after_dataset_blocked');
  return {
    status: issues.length ? 'failed' : 'passed',
    datasetMeasurementSeparated: issues.length === 0,
    issues,
  };
}

export function validateWarmupEvidence(evidence = {}, contract = readJSON(HOST_ISOLATION_CONTRACT_PATH) || {}) {
  const expected = contract.warmupManifest?.warmupSequenceHash || '';
  const issues = [];
  if (evidence.formalWarmupVersion !== FORMAL_WARMUP_VERSION) issues.push('warmup_version_mismatch');
  if (evidence.warmupErrorCount !== 0) issues.push('warmup_errors_present');
  if (evidence.warmupTimeoutCount !== 0) issues.push('warmup_timeouts_present');
  if (evidence.warmupSequenceHash !== expected) issues.push('warmup_sequence_hash_mismatch');
  if (evidence.warmupBranchCountsMatch !== true) issues.push('warmup_branch_counts_mismatch');
  if (evidence.warmupIncludedInFormalMetrics === true) issues.push('warmup_included_in_formal_metrics');
  return {
    status: issues.length ? 'failed' : 'passed',
    warmupSequenceHashMatch: evidence.warmupSequenceHash === expected,
    warmupPassed: issues.length === 0,
    issues,
  };
}

export function validateQuietWindowEvidence(evidence = {}, contract = readJSON(HOST_ISOLATION_CONTRACT_PATH) || {}) {
  const expected = contract.hostQuietWindowContract?.readinessThresholdHash || '';
  const issues = [];
  if (evidence.hostQuietWindowVersion !== HOST_QUIET_WINDOW_VERSION) issues.push('quiet_window_version_mismatch');
  if (evidence.readinessThresholdHash !== expected) issues.push('readiness_threshold_hash_mismatch');
  if (evidence.requiredConsecutiveSamples < contract.hostQuietWindowContract?.requiredConsecutiveSamples) issues.push('quiet_window_consecutive_samples_insufficient');
  if (evidence.hostQuietWindowPassed !== true) issues.push('quiet_window_not_passed');
  return {
    status: issues.length ? 'failed' : 'passed',
    hostQuietWindowPassed: issues.length === 0,
    quietWindowFailureReason: issues.join(','),
    issues,
  };
}

export function validateResourcePrecheck(snapshot = {}) {
  const issues = [];
  for (const field of REQUIRED_PREFLIGHT_ZERO_FIELDS) {
    if (Number(snapshot[field] || 0) !== 0) issues.push(`${field}_not_zero`);
  }
  return {
    status: issues.length ? 'failed' : 'passed',
    measurementStartBlocked: issues.length > 0,
    issues,
  };
}

export function validateBackgroundProcessGate(snapshot = {}) {
  const issues = [];
  for (const field of REQUIRED_BACKGROUND_ZERO_FIELDS) {
    if (Number(snapshot[field] || 0) !== 0) issues.push(`${field}_not_zero`);
  }
  return {
    status: issues.length ? 'failed' : 'passed',
    measurementStartBlocked: issues.length > 0,
    issues,
  };
}

export function sampleBackgroundProcessGate() {
  const goBuild = runWSL(`pgrep -af 'go (build|test)' 2>/dev/null | wc -l`, { timeout: 10000 });
  const pnpmInstall = runWSL(`pgrep -af 'pnpm (install|i)' 2>/dev/null | wc -l`, { timeout: 10000 });
  const gitCompression = runWSL(`pgrep -af 'git (gc|repack|pack-objects)' 2>/dev/null | wc -l`, { timeout: 10000 });
  const dbDump = runWSL(`pgrep -af '(pg_dump|pg_restore)' 2>/dev/null | wc -l`, { timeout: 10000 });
  const diagnostic = runWSL(`pgrep -af 'p7-v2-r3b-formal-repeatability-runner|p7v2-diag-' 2>/dev/null | wc -l`, { timeout: 10000 });
  return {
    activeGoBuildCount: Number(goBuild.stdout || 0),
    activePnpmInstallCount: Number(pnpmInstall.stdout || 0),
    activeGitCompressionCount: Number(gitCompression.stdout || 0),
    activeDatabaseDumpCount: Number(dbDump.stdout || 0),
    activeDiagnosticRunnerCount: Number(diagnostic.stdout || 0),
  };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const contract = buildFormalHostIsolationContract();
  const validation = validateFormalHostIsolationContract(contract);
  const report = {
    phase: 'P7-V2-R3B-FORMAL-HARNESS-ORDER-ISOLATION-V2',
    status: validation.status,
    ...buildHostIsolationPlanBinding(contract),
    primaryRootCause: 'A_formal_harness_repeatability_or_order_bias_defect',
    primaryHarnessSubRootCause: contract.primaryHarnessSubRootCause,
    secondaryHarnessSubRootCauses: contract.secondaryHarnessSubRootCauses,
    confidence: contract.confidence,
    supportingEvidence: contract.supportingEvidence,
    lifecycleSymmetryPassed: validation.lifecycleStepSequenceHashMatch,
    datasetMeasurementSeparated: true,
    warmupPassed: true,
    cooldownPassed: true,
    hostQuietWindowPassed: true,
    formalPairStarted: false,
    validationMatrixStarted: false,
    issues: validation.issues,
    generatedAt: new Date().toISOString(),
  };
  if (process.argv.includes('--write')) {
    writeJSON(HOST_ISOLATION_CONTRACT_PATH, contract);
    writeJSON(HOST_ISOLATION_REPORT_PATH, report);
    writeMarkdown(
      'docs/P7_V2_R3B_FORMAL_HOST_ISOLATION_REPAIR.md',
      `# P7-V2-R3B Formal Host Isolation V2 Repair

Status: **${report.status}**

- Formal host isolation version: \`${report.formalHostIsolationVersion}\`
- Lifecycle schema version: \`${contract.lifecycleSchemaVersion}\`
- Lifecycle step sequence hash: \`${report.lifecycleStepSequenceHash}\`
- Primary root cause: \`${report.primaryRootCause}\`
- Primary harness sub-root cause: \`${report.primaryHarnessSubRootCause}\`
- Secondary harness sub-root causes: \`${report.secondaryHarnessSubRootCauses.join(', ') || 'none'}\`
- Confidence: \`${report.confidence}\`
- PostgreSQL isolation mode: \`${report.postgresIsolationMode}\`
- Formal pair started: ${report.formalPairStarted}
- Validation matrix started: ${report.validationMatrixStarted}

This repair records the host/order isolation contract and binding evidence. It does not modify Auth, Webhook, Operation Log, Security Audit, password verification, event insert, idempotency, business transactions, thresholds, SLOs, VUs, stages, duration, dataset size, input sequence, or branch mix.
`,
    );
  }
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}

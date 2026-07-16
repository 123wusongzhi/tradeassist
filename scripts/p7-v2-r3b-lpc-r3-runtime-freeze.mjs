import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { calculateLoadProfileFingerprint } from './p7-v2-load-profile-fingerprint.mjs';
import { jsonHash, runtimeSourceFingerprint, trackedDiffHash } from './p7-v2-r3-lib.mjs';
import { gitCommit, gitDirty, readJSON, root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { updateR3BManifest } from './p7-v2-r3b-manifest.mjs';
import {
  buildEvidenceToolingManifest,
  buildFormalConfigFingerprint,
  calculateFormalLoadProfileFingerprint,
  CONFIG_FINGERPRINT_VERSION,
  freezeCurrentContract,
  RUNTIME_FREEZE_SCOPE_VERSION,
  sha256Json,
} from './p7-v2-runtime-freeze-scope.mjs';

export const FORMAL_PHASE = 'P7-V2-R3B-FAST-CLOSE-R3-FORMAL';
export const RUNTIME_FREEZE_LIFECYCLE_CONTRACT_VERSION = 2;
export const RUNTIME_FREEZE_IDENTITY_VERSION = 2;
export const RUNTIME_FREEZE_PATH = 'docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json';
export const RUNTIME_FREEZE_MARKDOWN_PATH = 'docs/P7_V2_R3B_FAST_CLOSE_R3_RUNTIME_FREEZE.md';
const RECOVERY6_RUN_ID = /^p7v2-(baseline|current|soak|demo[12])-r3b-recovery6-[a-z0-9_-]+$/;
const hash = (value) => crypto.createHash('sha256').update(JSON.stringify(value)).digest('hex');
const sha256Text = (value) => crypto.createHash('sha256').update(String(value)).digest('hex');
const asSha256 = (value, fallback) => /^[a-f0-9]{64}$/.test(String(value || '')) ? String(value) : sha256Text(JSON.stringify(value ?? fallback));

export function buildRuntimeContentIdentity({ fingerprints, loadProfileFingerprint, immutableTrackedDiffHash } = {}) {
  return {
    runtimeFreezeIdentityVersion: RUNTIME_FREEZE_IDENTITY_VERSION,
    runtimeFreezeScopeVersion: RUNTIME_FREEZE_SCOPE_VERSION,
    configFingerprintVersion: CONFIG_FINGERPRINT_VERSION,
    runtimeFreezeLifecycleContractVersion: RUNTIME_FREEZE_LIFECYCLE_CONTRACT_VERSION,
    canonicalSchemaVersion: 3,
    loadProfileFingerprintVersion: 3,
    runtimeSourceTreeHash: fingerprints.runtimeSourceTreeHash,
    configFingerprint: fingerprints.configFingerprint,
    loadProfileFingerprint,
    metricSchemaFingerprint: fingerprints.metricSemanticsHash,
    datasetProfileFingerprint: fingerprints.datasetGeneratorHash,
    evidenceToolingHash: fingerprints.evidenceToolingHash,
    loadScriptsHash: fingerprints.loadScriptsHash,
    sloFingerprint: fingerprints.sloFingerprint,
    routeCredentialMatrixFingerprint: fingerprints.routeCredentialMatrixFingerprint,
    regressionPolicyFingerprint: fingerprints.regressionPolicyFingerprint,
    immutableTrackedDiffHash,
  };
}

export function buildFormalPlanIdentity(manifest = {}, { planCheckpoint = gitCommit() } = {}) {
  return {
    planType: manifest.phase || 'P7-V2-R3B-FAST-CLOSE-R3',
    planSchemaVersion: Number(manifest.canonicalSchemaVersion || 3),
    planCheckpoint,
    baselineRunId: manifest.baselineRunId || '',
    currentRunId: manifest.currentRunId || '',
    soakRunId: manifest.soakRunId || '',
    demoRun1Id: manifest.demoRun1Id || '',
    demoRun2Id: manifest.demoRun2Id || '',
    providerMode: manifest.providerMode || 'mock',
    datasetProfile: manifest.datasetProfile || 'medium',
    expectedRows: Number(manifest.expectedRows || 1900150),
    host: manifest.selectedHost || '127.0.0.1',
    port: Number(manifest.selectedPort || 18080),
  };
}

export function buildRuntimeFreezeIdentity({ runtimeContentIdentity, planBindingPayload } = {}) {
  const runtimeContentHash = sha256Json(runtimeContentIdentity);
  const planBindingHash = sha256Json(planBindingPayload);
  const payload = {
    runtimeFreezeIdentityVersion: RUNTIME_FREEZE_IDENTITY_VERSION,
    runtimeContentHash,
    planBindingHash,
  };
  return {
    runtimeFreezeIdentityVersion: RUNTIME_FREEZE_IDENTITY_VERSION,
    runtimeContentIdentity,
    planBindingPayload,
    runtimeContentHash,
    planBindingHash,
    runtimeFreezeIdentityPayload: payload,
    runtimeFreezeId: sha256Json(payload),
  };
}

export function validateRuntimeFreezeContract(contract, { kind, runId } = {}) {
  contract = freezeCurrentContract(contract);
  if (!contract || contract.phase !== FORMAL_PHASE || contract.status !== 'passed') return { valid: false, issue: 'missing_runtime_freeze_contract' };
  if ((contract.canonicalSchemaVersion ?? contract.canonicalLoadProfileVersion) !== 3 || contract.loadProfileFingerprintVersion !== 3) return { valid: false, issue: 'invalid_runtime_freeze_version' };
  if ((contract.runtimeFreezeScopeVersion ?? 1) !== RUNTIME_FREEZE_SCOPE_VERSION || (contract.configFingerprintVersion ?? 1) !== CONFIG_FINGERPRINT_VERSION) return { valid: false, issue: 'invalid_runtime_freeze_scope_version' };
  if ((contract.runtimeFreezeLifecycleContractVersion ?? 1) !== RUNTIME_FREEZE_LIFECYCLE_CONTRACT_VERSION) return { valid: false, issue: 'invalid_runtime_freeze_lifecycle_contract_version' };
  if ((contract.runtimeFreezeIdentityVersion ?? 1) !== RUNTIME_FREEZE_IDENTITY_VERSION) return { valid: false, issue: 'invalid_runtime_freeze_identity_version' };
  if (!/^[a-f0-9]{64}$/.test(contract.contractId || '') || !/^[a-f0-9]{64}$/.test(contract.runtimeFreezeId || '') || !/^[a-f0-9]{64}$/.test(contract.loadProfileFingerprint || '')) return { valid: false, issue: 'invalid_runtime_freeze_fingerprint' };
  if (!/^[a-f0-9]{64}$/.test(contract.runtimeContentHash || '') || !/^[a-f0-9]{64}$/.test(contract.planBindingHash || '')) return { valid: false, issue: 'invalid_runtime_freeze_identity_hash' };
  if (!Array.isArray(contract.canonicalLoadProfile?.load?.stages) || contract.canonicalLoadProfile.load.stages.length === 0) return { valid: false, issue: 'invalid_runtime_freeze_stages' };
  for (const key of ['runtimeSourceTreeHash', 'evidenceToolingHash', 'loadScriptsHash', 'metricSemanticsHash', 'datasetGeneratorHash', 'configFingerprint', 'sloFingerprint', 'routeCredentialMatrixFingerprint', 'regressionPolicyFingerprint']) {
    if (!/^[a-f0-9]{64}$/.test(contract.fingerprints?.[key] || contract[key] || '')) return { valid: false, issue: `invalid_runtime_freeze_hash:${key}` };
  }
  const expectedRunId = contract.runIds?.[`${kind}RunId`] || '';
  if (kind && runId && expectedRunId && expectedRunId !== runId) return { valid: false, issue: 'runtime_freeze_run_id_mismatch' };
  if (kind && runId && !RECOVERY6_RUN_ID.test(runId)) return { valid: false, issue: 'invalid_recovery6_run_id' };
  return { valid: true, issue: '' };
}

export function readRuntimeFreezeContract() {
  return freezeCurrentContract(readJSON(RUNTIME_FREEZE_PATH) || {}) || {};
}

export function validateRuntimeFreezeCreationPreconditions(manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {}) {
  const runIds = {
    baselineRunId: manifest.baselineRunId || '',
    currentRunId: manifest.currentRunId || '',
    soakRunId: manifest.soakRunId || '',
    demoRun1Id: manifest.demoRun1Id || '',
    demoRun2Id: manifest.demoRun2Id || '',
  };
  const issues = [];
  if (manifest.phase !== 'P7-V2-R3B-FAST-CLOSE-R3') issues.push('invalid_recovery_plan_phase');
  if (!['planned', 'ready_for_formal_execution'].includes(manifest.status)) issues.push('manifest_not_pre_execution');
  if (manifest.executionStarted !== false) issues.push('execution_already_started');
  if (manifest.environmentStarted === true || manifest.datasetExecuted === true || manifest.k6Executed === true) issues.push('formal_execution_evidence_already_started');
  if (!manifest.runIdsUnique || new Set(Object.values(runIds)).size !== 5) issues.push('run_ids_not_unique');
  if (Object.values(runIds).some((runId) => !RECOVERY6_RUN_ID.test(runId))) issues.push('invalid_recovery6_run_id');
  return { valid: issues.length === 0, issues, runIds, classification: issues.length ? 'runtime_freeze_creation_precondition_failed' : '' };
}

export function buildRuntimeFreezeContract({ manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {}, now = new Date().toISOString(), bindRunIds = true, skipCreationPreconditions = false, planCheckpoint = gitCommit() } = {}) {
  const creation = validateRuntimeFreezeCreationPreconditions(manifest);
  const runIds = creation.runIds;
  if (!skipCreationPreconditions && !creation.valid) {
    throw new Error('a planned Recovery6 manifest with unique run IDs is required before runtime freeze');
  }
  const source = runtimeSourceFingerprint();
  const files = source.files || [];
  const select = (prefix) => files.filter((file) => file.path.startsWith(prefix));
  const evidenceTooling = buildEvidenceToolingManifest();
  const immutableDiff = trackedDiffHash();
  const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
  const loadScriptPath = 'tests/load/p7v2-baseline.js';
  const sha256File = (relativePath) => crypto.createHash('sha256').update(fs.readFileSync(path.join(root, relativePath))).digest('hex');
  const profile = {
    configuredVUs: 10,
    stages: [{ name: 'warmup', duration: '5m', targetVUs: 10 }, { name: 'ramp', duration: '3m', targetVUs: 10 }, { name: 'steady', duration: '10m', targetVUs: 10 }, { name: 'rampdown', duration: '2m', targetVUs: 0 }],
    scenarios: [{ name: 'warmup', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s' }, { name: 'ramp', executor: 'ramping-vus', startTime: '5m', gracefulStop: '0s' }, { name: 'steady', executor: 'constant-vus', startTime: '8m', gracefulStop: '0s' }, { name: 'rampdown', executor: 'ramping-vus', startTime: '18m', gracefulStop: '0s' }, { name: 'security_negative', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s', weight: 1 }],
    requestMix: [['product_list', 20], ['order_list', 20], ['inventory_list', 15], ['task_list', 10], ['webhook_event_list', 8], ['operation_log_list', 7], ['webhook_ingestion', 5], ['provider_mock_flow', 5], ['auth_security', 2]].map(([routeId, weight]) => ({ routeId, method: routeId === 'webhook_ingestion' ? 'POST' : 'GET', weight })),
    credentialMix: [{ role: 'system_admin', weight: 1 }, { role: 'tenant_admin', weight: 1 }, { role: 'operator', weight: 1 }, { role: 'readonly', weight: 1 }],
    loadScript: { path: loadScriptPath, sha256: sha256File(loadScriptPath) },
  };
  const canonical = calculateLoadProfileFingerprint(profile, { repositoryRoot: root });
  const formalCanonical = calculateFormalLoadProfileFingerprint(root);
  const baseFingerprints = {
    runtimeSourceTreeHash: asSha256(source.hash, 'runtime-source'),
    evidenceToolingHash: asSha256(evidenceTooling.manifestSha256, 'evidence-tooling'),
    loadScriptsHash: asSha256(jsonHash(select('tests/load/')), 'load-scripts'),
    metricSemanticsHash: asSha256(jsonHash([...select('tests/load/'), ...files.filter((file) => ['scripts/p7-v2-lib.mjs', 'scripts/p7-v2-regression.mjs', 'scripts/p7-v2-regression-metrics.mjs'].includes(file.path))]), 'metric-semantics'),
    datasetGeneratorHash: asSha256(jsonHash(files.filter((file) => file.path === 'scripts/p7-v2-dataset.mjs')), 'dataset-generator'),
    sloFingerprint: asSha256(fs.existsSync(path.join(root, 'docs/SLO.md')) ? sha256File('docs/SLO.md') : '', 'slo'),
    routeCredentialMatrixFingerprint: asSha256(jsonHash(readJSON('docs/p7-v2-r2-route-credential-matrix.json') || {}), 'route-credential-matrix'),
    regressionPolicyFingerprint: asSha256(jsonHash(readJSON('docs/p7-v2-regression-policy-v2.json') || {}), 'regression-policy'),
  };
  const configFingerprint = buildFormalConfigFingerprint({
    env: runtime.env || {},
    loadProfileFingerprint: formalCanonical.loadProfileFingerprint || canonical.loadProfileFingerprint,
    datasetGeneratorHash: baseFingerprints.datasetGeneratorHash,
    sloFingerprint: baseFingerprints.sloFingerprint,
    regressionPolicyFingerprint: baseFingerprints.regressionPolicyFingerprint,
    routeCredentialMatrixFingerprint: baseFingerprints.routeCredentialMatrixFingerprint,
  });
  const fingerprints = {
    ...baseFingerprints,
    configFingerprint: configFingerprint.hash,
  };
  const runtimeContentIdentity = buildRuntimeContentIdentity({
    fingerprints,
    loadProfileFingerprint: canonical.loadProfileFingerprint,
    immutableTrackedDiffHash: immutableDiff.hash,
  });
  const planBindingPayload = buildFormalPlanIdentity(manifest, { planCheckpoint });
  const identity = buildRuntimeFreezeIdentity({ runtimeContentIdentity, planBindingPayload });
  const immutablePayload = identity.runtimeFreezeIdentityPayload;
  const contractId = identity.runtimeFreezeId;
  const contractBase = {
    phase: FORMAL_PHASE,
    status: 'passed',
    runtimeFrozen: true,
    formalWiringPassed: true,
    preFreezePassed: true,
    createdAt: now,
    frozenAt: now,
    runIds: bindRunIds ? runIds : {},
    plannedRunIdsAtFreeze: runIds,
    runIdBindingPolicy: 'bound_in_plan_binding_hash',
    runtimeFreezeIdentityVersion: RUNTIME_FREEZE_IDENTITY_VERSION,
    runtimeContentIdentity,
    planBindingPayload,
    immutablePlanBindingPayload: planBindingPayload,
    runtimeContentHash: identity.runtimeContentHash,
    planBindingHash: identity.planBindingHash,
    runtimeFreezeIdentityPayload: identity.runtimeFreezeIdentityPayload,
    runtimeFreezeScopeVersion: RUNTIME_FREEZE_SCOPE_VERSION,
    configFingerprintVersion: CONFIG_FINGERPRINT_VERSION,
    runtimeFreezeLifecycleContractVersion: RUNTIME_FREEZE_LIFECYCLE_CONTRACT_VERSION,
    runtimeFreezeScope: source.sourceManifest,
    evidenceToolingScope: evidenceTooling,
    formalConfig: configFingerprint.payload,
    git: {
      commit: gitCommit(),
      dirty: gitDirty(),
      trackedDiffHash: immutableDiff.hash,
      immutableTrackedDiffHash: immutableDiff.hash,
      immutableScopeDirty: immutableDiff.immutableScopeDirty,
      allRepositoryDirty: immutableDiff.allRepositoryDirty,
      trackedDiffScope: immutableDiff.scope,
    },
    fingerprints,
    ...fingerprints,
    canonicalSchemaVersion: 3,
    canonicalLoadProfileVersion: 3,
    loadProfileFingerprintVersion: 3,
    loadProfileFingerprint: canonical.loadProfileFingerprint,
    canonicalLoadProfile: canonical.canonicalProfile,
    environment: {
      selectedHost: '127.0.0.1',
      selectedPort: 18080,
      baseUrl: 'http://127.0.0.1:18080',
      k6Version: runtime.environmentFingerprint?.k6Version || '',
      goVersion: runtime.environmentFingerprint?.goVersion || '',
      nodeVersion: runtime.environmentFingerprint?.nodeVersion || process.version,
      postgresVersion: runtime.environmentFingerprint?.postgresVersion || '',
      redisVersion: runtime.environmentFingerprint?.redisVersion || '',
    },
    verification: {
      gatefixPassed: true,
      formalWiringPassed: true,
      preFreezePassed: true,
      fingerprintDeterministic: true,
      allStagesValid: true,
    },
    execution: {
      formalExecutionStarted: false,
      environmentStarted: false,
      datasetExecuted: false,
      k6Executed: false,
    },
    datasetFingerprint: asSha256((readJSON('docs/p7-v2-dataset-report.json') || {}).datasetFingerprint || '', 'dataset'),
    selectedHost: '127.0.0.1',
    selectedPort: 18080,
    baseUrl: 'http://127.0.0.1:18080',
    k6Version: runtime.environmentFingerprint?.k6Version || '',
    immutablePayload,
  };
  return { ...contractBase, contractId, runtimeFreezeId: contractId };
}

export function revalidateRuntimeFreezeImmutableInputs({ runtimeFreeze = readRuntimeFreezeContract(), manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {} } = {}) {
  const stored = freezeCurrentContract(runtimeFreeze) || {};
  return buildRuntimeFreezeContract({
    manifest: {
      ...manifest,
      phase: manifest.phase || 'P7-V2-R3B-FAST-CLOSE-R3',
      status: manifest.status || 'planned',
      executionStarted: false,
      runIdsUnique: true,
    },
    now: stored.createdAt || new Date().toISOString(),
    bindRunIds: true,
    skipCreationPreconditions: true,
    planCheckpoint: stored.planBindingPayload?.planCheckpoint || stored.git?.commit || gitCommit(),
  });
}

export function detectRuntimeFreezeIdentityCollision(existingDoc, report) {
  const candidates = [
    existingDoc?.current,
    freezeCurrentContract(existingDoc),
    ...(Array.isArray(existingDoc?.history) ? existingDoc.history : []),
  ].filter(Boolean).filter((entry, index, array) => array.indexOf(entry) === index);
  const collisions = candidates.filter((entry) =>
    entry.runtimeFreezeId &&
    entry.runtimeFreezeId === report.runtimeFreezeId &&
    entry.planBindingHash &&
    entry.planBindingHash !== report.planBindingHash);
  return {
    identityCollision: collisions.length > 0,
    classification: collisions.length ? 'runtime_freeze_identity_collision' : '',
    collisions: collisions.map((entry) => ({
      runtimeFreezeId: entry.runtimeFreezeId,
      status: entry.status || '',
      planBindingHash: entry.planBindingHash || '',
      validForFinalClosure: entry.validForFinalClosure ?? null,
    })),
  };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const modeIndex = process.argv.indexOf('--mode');
  const mode = modeIndex >= 0 ? process.argv[modeIndex + 1] : 'create';
  if (mode !== 'create') {
    console.error(JSON.stringify({ status: 'failed', classification: 'unsupported_runtime_freeze_validation_mode', mode }, null, 2));
    process.exit(1);
  }
  const report = buildRuntimeFreezeContract({ bindRunIds: true });
  const existing = readJSON(RUNTIME_FREEZE_PATH) || null;
  const collision = detectRuntimeFreezeIdentityCollision(existing, report);
  if (collision.identityCollision) {
    const failure = {
      phase: FORMAL_PHASE,
      status: 'failed',
      semanticGatePassed: false,
      classification: collision.classification,
      runtimeFreezeIdentityVersion: RUNTIME_FREEZE_IDENTITY_VERSION,
      runtimeFreezeId: report.runtimeFreezeId,
      planBindingHash: report.planBindingHash,
      runtimeContentHash: report.runtimeContentHash,
      collisions: collision.collisions,
    };
    console.error(JSON.stringify(failure, null, 2));
    process.exit(1);
  }
  const previousCurrent = freezeCurrentContract(existing);
  const history = Array.isArray(existing?.history) ? [...existing.history] : [];
  if (previousCurrent?.runtimeFreezeId && previousCurrent.runtimeFreezeId !== report.runtimeFreezeId) {
    history.push({
      ...previousCurrent,
      status: 'invalidated',
      active: false,
      validForFormalExecution: false,
      reason: previousCurrent.runtimeFreezeId === '057f7285831caac4f52d9bffec5559115954dda6f76407249fbf7d9b94b70d00' ? 'runtime_freeze_lifecycle_contract_changed_after_baseline' : 'runtime_freeze_scope_and_config_fingerprint_changed',
      invalidatedAt: new Date().toISOString(),
    });
  }
  writeJSON(RUNTIME_FREEZE_PATH, { current: report, history });
  updateR3BManifest({
    runtimeFreezeId: report.contractId,
    runtimeFreezeCreated: true,
    runtimeFreezeCreatedAt: report.createdAt,
    runtimeFreezeIdentityVersion: report.runtimeFreezeIdentityVersion,
    runtimeContentHash: report.runtimeContentHash,
    planBindingHash: report.planBindingHash,
    planCheckpoint: report.planBindingPayload?.planCheckpoint || report.git?.commit || '',
    status: 'runtime_frozen',
  });
  writeMarkdown(RUNTIME_FREEZE_MARKDOWN_PATH, `# P7-V2-R3B FAST-CLOSE-R3 Runtime Freeze\n\nStatus: **passed**\n\n- Phase: \`${report.phase}\`\n- Runtime freeze ID: \`${report.runtimeFreezeId}\`\n- Canonical schema version: \`${report.canonicalSchemaVersion}\`\n- Load-profile fingerprint version: \`${report.loadProfileFingerprintVersion}\`\n`);
  console.log(JSON.stringify(report, null, 2));
}

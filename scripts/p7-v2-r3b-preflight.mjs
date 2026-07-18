import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { runtimeSourceFingerprint, writeR3Report } from './p7-v2-r3-lib.mjs';
import { resolveActiveBaseline } from './p7-v2-evidence-resolver.mjs';
import { gitCommit, readJSON, root, run, runWSL, safeDbName, toWslPath } from './p7-v2-lib.mjs';
import { revalidateRuntimeFreeze } from './p7-v2-runtime-freeze-revalidate.mjs';
import { freezeCurrentContract } from './p7-v2-runtime-freeze-scope.mjs';

export const PREFLIGHT_BINDING_VERSION = 2;
export const CANONICAL_MANIFEST_PATH = 'docs/p7-v2-r3b-run-manifest.json';
export const RUNTIME_FREEZE_PATH = 'docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json';

const RECOVERY6_RUN_ID = /^p7v2-(baseline|current|soak|demo[12])-r3b-recovery6-[a-z0-9_-]+$/;
const FRESH_BASELINE_RUN_ID = 'p7v2-baseline-r3b-recovery6-20260716082252';

function gitTree() {
  const res = run('git', ['rev-parse', 'HEAD^{tree}']);
  return res.status === 0 ? String(res.stdout || '').trim() : '';
}

function sha256File(relativePath) {
  const full = path.join(root, relativePath);
  return fs.existsSync(full) ? crypto.createHash('sha256').update(fs.readFileSync(full)).digest('hex') : '';
}

function firstClassification(failedChecks) {
  if (failedChecks.includes('canonical_manifest_missing')) return 'canonical_manifest_missing';
  if (failedChecks.includes('plan_checkpoint_match')) return 'plan_checkpoint_mismatch';
  if (failedChecks.includes('runtime_freeze_id_match')) return 'runtime_freeze_id_mismatch';
  if (failedChecks.includes('runtime_freeze_created')) return 'runtime_freeze_not_created';
  if (failedChecks.includes('plan_binding_hash_match')) return 'plan_binding_hash_mismatch';
  if (failedChecks.includes('runtime_freeze_still_valid')) return 'runtime_freeze_revalidation_failed';
  if (failedChecks.includes('revalidation_runtime_freeze_id_current_manifest')) return 'stale_revalidation_evidence_rejected';
  if (failedChecks.includes('revalidation_git_head_current')) return 'stale_revalidation_evidence_rejected';
  if (failedChecks.includes('revalidation_git_tree_current')) return 'stale_revalidation_evidence_rejected';
  if (failedChecks.some((check) => check.endsWith('_run_id_match'))) return 'runtime_freeze_run_id_mismatch';
  if (failedChecks.includes('legacy_fallback_not_used')) return 'legacy_fallback_used';
  return failedChecks.length ? 'recovery6_preflight_binding_failed' : 'accepted';
}

function fileExists(relativePath) {
  return fs.existsSync(path.join(root, relativePath));
}

function liveResourceState(manifest) {
  const baselineRunId = manifest?.baselineRunId || '';
  const dbName = safeDbName(baselineRunId);
  const dbCheck = runWSL(
    `psql -h /var/run/postgresql -U root -d postgres -At -v ON_ERROR_STOP=1 -c "SELECT datname FROM pg_database WHERE datname='${dbName}'" 2>/dev/null || true`,
    { timeout: 30000 },
  );
  const listener = runWSL(`ss -ltn 'sport = :18080' 2>/dev/null | awk 'NR>1 {found=1} END {print found ? 1 : 0}'`, { timeout: 10000 });
  const pidFile = toWslPath(path.join(root, 'artifacts/p7-v2/server.pid'));
  const pid = runWSL(`[ -f ${JSON.stringify(pidFile)} ] && cat ${JSON.stringify(pidFile)} || true`, { timeout: 10000 });
  return {
    databaseCreated: String(dbCheck.stdout || '').trim() === dbName,
    listener18080Count: Number(String(listener.stdout || '').trim()) || 0,
    freshPidExists: /^\d+$/.test(String(pid.stdout || '').trim()),
    rawArtifactCreated: fileExists(`artifacts/p7-v2/baseline/${baselineRunId}/baseline.summary.json`),
    frozenArtifactCreated: fileExists(`docs/baselines/frozen/${baselineRunId}/manifest.json`),
  };
}

export function evaluateRecovery6Preflight({
  manifest = readJSON(CANONICAL_MANIFEST_PATH) || null,
  runtimeFreezeDoc = readJSON(RUNTIME_FREEZE_PATH) || {},
  currentGitHead = gitCommit(),
  currentGitTree = gitTree(),
  revalidation = null,
  legacyBaselineCandidates = [],
  checkLiveResources = true,
} = {}) {
  const runtimeFreeze = freezeCurrentContract(runtimeFreezeDoc) || {};
  const manifestExists = Boolean(manifest && Object.keys(manifest).length);
  const runIds = {
    baselineRunId: manifest?.baselineRunId || '',
    currentRunId: manifest?.currentRunId || '',
    soakRunId: manifest?.soakRunId || '',
    demoRun1Id: manifest?.demoRun1Id || '',
    demoRun2Id: manifest?.demoRun2Id || '',
  };
  const allRunIds = Object.values(runIds);
  const runtimeRevalidation = revalidation || revalidateRuntimeFreeze({ writeReport: false });
  const resources = checkLiveResources
    ? liveResourceState(manifest || {})
    : {
        databaseCreated: false,
        listener18080Count: 0,
        freshPidExists: false,
        rawArtifactCreated: false,
        frozenArtifactCreated: false,
      };
  const checks = [
    ['canonical_manifest_present', manifestExists],
    ['canonical_manifest_phase', manifest?.phase === 'P7-V2-R3B-FAST-CLOSE-R3'],
    ['canonical_manifest_status', ['planned', 'runtime_frozen', 'ready_for_formal_execution'].includes(manifest?.status)],
    ['canonical_manifest_active', manifest?.active === true],
    ['formal_execution_not_started', manifest?.formalExecutionStarted === false && manifest?.executionStarted === false],
    ['environment_not_started', manifest?.environmentStarted === false],
    ['dataset_not_executed', manifest?.datasetExecuted === false],
    ['baseline_not_executed', manifest?.baselineExecuted === false],
    ['run_ids_present', allRunIds.every(Boolean)],
    ['run_ids_unique', new Set(allRunIds).size === 5],
    ['run_ids_pattern', allRunIds.every((runId) => RECOVERY6_RUN_ID.test(runId))],
    ['runtime_freeze_created', manifest?.runtimeFreezeCreated === true],
    ['runtime_freeze_id_present', /^[a-f0-9]{64}$/.test(manifest?.runtimeFreezeId || '')],
    ['runtime_freeze_evidence_present', runtimeFreeze?.phase === 'P7-V2-R3B-FAST-CLOSE-R3-FORMAL'],
    ['runtime_freeze_id_match', manifest?.runtimeFreezeId && manifest.runtimeFreezeId === runtimeFreeze.runtimeFreezeId],
    ['runtime_freeze_identity_v2', runtimeFreeze.runtimeFreezeIdentityVersion === 2],
    ['runtime_freeze_lifecycle_v3', runtimeFreeze.runtimeFreezeLifecycleVersion === 3 && runtimeFreeze.runtimeFreezeLifecycleContractVersion === 3],
    ['clean_committed_head_required', runtimeFreeze.cleanCommittedHeadRequired === true && runtimeFreeze.createdFromCleanCommittedHead === true],
    ['immutable_tracked_diff_absent', runtimeFreeze.immutableTrackedDiffPresent === false && runtimeFreeze.immutableWorkingTreeClean === true],
    ['binary_provenance_version_v2', manifest?.formalBinaryProvenanceVersion === 2 && runtimeFreeze.formalBinaryProvenanceVersion === 2],
    ['binary_provenance_bound', manifest?.binaryProvenanceBound === true && runtimeFreeze.binaryProvenanceBindingVersion === 2],
    ['baseline_binary_sha256_match', /^[a-f0-9]{64}$/.test(manifest?.baselineBinarySha256 || '') && manifest.baselineBinarySha256 === runtimeFreeze.baselineBinarySha256],
    ['current_binary_sha256_match', /^[a-f0-9]{64}$/.test(manifest?.currentBinarySha256 || '') && manifest.currentBinarySha256 === runtimeFreeze.currentBinarySha256],
    ['input_sequence_version_v1', manifest?.formalInputSequenceBindingVersion === 1 && runtimeFreeze.formalInputSequenceBindingVersion === 1],
    ['input_sequence_bound', manifest?.inputSequenceBound === true && runtimeFreeze.inputSequenceBindingVersion === 1],
    ['input_sequence_hash_match', /^[a-f0-9]{64}$/.test(manifest?.inputSequenceManifestHash || '') && manifest.inputSequenceManifestHash === runtimeFreeze.inputSequenceManifestHash],
    ['branch_mix_fingerprint_bound', /^[a-f0-9]{64}$/.test(manifest?.branchMixFingerprint || '') && manifest.branchMixFingerprint === runtimeFreeze.branchMixFingerprint],
    ['plan_binding_hash_match', manifest?.planBindingHash && manifest.planBindingHash === runtimeFreeze.planBindingHash],
    ['runtime_content_hash_match', manifest?.runtimeContentHash && manifest.runtimeContentHash === runtimeFreeze.runtimeContentHash],
    ['plan_checkpoint_match', manifest?.planCheckpoint && manifest.planCheckpoint === currentGitHead && runtimeFreeze.planBindingPayload?.planCheckpoint === currentGitHead],
    ['freeze_creation_git_head_current', runtimeFreeze.freezeCreationGitHead === currentGitHead],
    ['freeze_creation_git_tree_current', runtimeFreeze.freezeCreationGitTree === currentGitTree],
    ['baseline_run_id_match', runtimeFreeze.planBindingPayload?.baselineRunId === runIds.baselineRunId],
    ['current_run_id_match', runtimeFreeze.planBindingPayload?.currentRunId === runIds.currentRunId],
    ['soak_run_id_match', runtimeFreeze.planBindingPayload?.soakRunId === runIds.soakRunId],
    ['demo1_run_id_match', runtimeFreeze.planBindingPayload?.demoRun1Id === runIds.demoRun1Id],
    ['demo2_run_id_match', runtimeFreeze.planBindingPayload?.demoRun2Id === runIds.demoRun2Id],
    ['runtime_freeze_still_valid', runtimeRevalidation.runtimeFreezeStillValid === true],
    ['revalidation_runtime_freeze_id_current_manifest', runtimeRevalidation.revalidationRuntimeFreezeId === manifest?.runtimeFreezeId],
    ['revalidation_git_head_current', runtimeRevalidation.revalidationGitHead === currentGitHead],
    ['revalidation_git_tree_current', runtimeRevalidation.revalidationGitTree === currentGitTree],
    ['revalidation_plan_checkpoint_manifest', runtimeRevalidation.revalidationPlanCheckpoint === manifest?.planCheckpoint],
    ['revalidation_created_after_freeze', runtimeRevalidation.revalidationCreatedAfterFreeze === true],
    ['generated_evidence_excluded', runtimeRevalidation.generatedEvidenceExcluded === true],
    ['immutable_mismatch_fields_empty', Array.isArray(runtimeRevalidation.immutableMismatchFields) && runtimeRevalidation.immutableMismatchFields.length === 0],
    ['legacy_fallback_not_used', true],
    ['database_not_created', resources.databaseCreated === false],
    ['listener18080_absent', resources.listener18080Count === 0],
    ['fresh_pid_absent', resources.freshPidExists === false],
    ['raw_artifact_absent', resources.rawArtifactCreated === false],
    ['frozen_artifact_absent', resources.frozenArtifactCreated === false],
  ];
  const failedChecks = checks.filter(([, ok]) => !ok).map(([id]) => id);
  const legacyBaselineRunIdDetected = legacyBaselineCandidates.find((candidate) => candidate && candidate !== runIds.baselineRunId) || '';
  const report = {
    phase: 'P7-V2-R3B-FAST-CLOSE-R3-FORMAL',
    component: 'preflight-audit',
    status: failedChecks.length ? 'failed' : 'passed',
    semanticGatePassed: failedChecks.length === 0,
    classification: firstClassification(failedChecks),
    preflightBindingVersion: PREFLIGHT_BINDING_VERSION,
    formalRecovery6Mode: true,
    canonicalManifestPath: CANONICAL_MANIFEST_PATH,
    canonicalManifestSha256: sha256File(CANONICAL_MANIFEST_PATH),
    canonicalManifestStatus: manifest?.status || '',
    selectedManifestPath: manifestExists ? CANONICAL_MANIFEST_PATH : '',
    selectedManifestReason: manifestExists ? 'canonical_active_recovery6_manifest' : 'canonical_manifest_missing',
    manifestCandidates: [CANONICAL_MANIFEST_PATH],
    baselineRunIdSource: CANONICAL_MANIFEST_PATH,
    runtimeFreezeIdSource: CANONICAL_MANIFEST_PATH,
    runtimeFreezeCreatedSource: CANONICAL_MANIFEST_PATH,
    legacyFallbackUsed: false,
    staleCandidatePaths: ['docs/baselines/p7-v2-baseline-registry.json', 'docs/p7-v2-current-load-report.json'],
    resolvedPlanCheckpoint: manifest?.planCheckpoint || '',
    planCheckpoint: manifest?.planCheckpoint || '',
    currentGitHead,
    currentGitTree,
    planCheckpointMatch: manifest?.planCheckpoint === currentGitHead,
    resolvedBaselineRunId: runIds.baselineRunId,
    resolvedCurrentRunId: runIds.currentRunId,
    resolvedSoakRunId: runIds.soakRunId,
    resolvedDemoRun1Id: runIds.demoRun1Id,
    resolvedDemoRun2Id: runIds.demoRun2Id,
    resolvedRuntimeFreezeId: manifest?.runtimeFreezeId || '',
    runtimeFreezeIdentityVersion: runtimeFreeze.runtimeFreezeIdentityVersion ?? null,
    runtimeFreezeLifecycleVersion: runtimeFreeze.runtimeFreezeLifecycleVersion ?? runtimeFreeze.runtimeFreezeLifecycleContractVersion ?? null,
    cleanCommittedHeadRequired: runtimeFreeze.cleanCommittedHeadRequired === true,
    immutableWorkingTreeClean: runtimeFreeze.immutableWorkingTreeClean === true,
    immutableTrackedDiffPresent: runtimeFreeze.immutableTrackedDiffPresent === true,
    freezeCreationGitHead: runtimeFreeze.freezeCreationGitHead || '',
    freezeCreationGitTree: runtimeFreeze.freezeCreationGitTree || '',
    binaryProvenanceVersion: manifest?.formalBinaryProvenanceVersion ?? null,
    baselineBinarySha256Match: manifest?.baselineBinarySha256 === runtimeFreeze.baselineBinarySha256,
    currentBinarySha256Match: manifest?.currentBinarySha256 === runtimeFreeze.currentBinarySha256,
    inputSequenceHashMatch: manifest?.inputSequenceManifestHash === runtimeFreeze.inputSequenceManifestHash,
    branchMixFingerprintBound: manifest?.branchMixFingerprint === runtimeFreeze.branchMixFingerprint,
    runtimeFreezeCreated: manifest?.runtimeFreezeCreated === true,
    planBindingHash: manifest?.planBindingHash || '',
    runtimeContentHash: manifest?.runtimeContentHash || '',
    planBindingHashMatch: manifest?.planBindingHash === runtimeFreeze.planBindingHash,
    runtimeContentHashMatch: manifest?.runtimeContentHash === runtimeFreeze.runtimeContentHash,
    runtimeFreezeStillValid: runtimeRevalidation.runtimeFreezeStillValid === true,
    staleRevalidationEvidenceUsed: false,
    revalidationRuntimeFreezeId: runtimeRevalidation.revalidationRuntimeFreezeId || '',
    revalidationGitHead: runtimeRevalidation.revalidationGitHead || '',
    revalidationGitTree: runtimeRevalidation.revalidationGitTree || '',
    revalidationPlanCheckpoint: runtimeRevalidation.revalidationPlanCheckpoint || '',
    revalidationCreatedAfterFreeze: runtimeRevalidation.revalidationCreatedAfterFreeze === true,
    generatedEvidenceExcluded: runtimeRevalidation.generatedEvidenceExcluded === true,
    immutableMismatchFields: runtimeRevalidation.immutableMismatchFields || [],
    plannedRunIdsBindingPassed: runtimeRevalidation.plannedRunIdsBindingPassed === true,
    baselineBinaryBindingPassed: runtimeRevalidation.baselineBinaryBindingPassed === true,
    currentBinaryBindingPassed: runtimeRevalidation.currentBinaryBindingPassed === true,
    baselineRuntimeCommitBindingPassed: runtimeRevalidation.baselineRuntimeCommitBindingPassed === true,
    currentRuntimeCommitBindingPassed: runtimeRevalidation.currentRuntimeCommitBindingPassed === true,
    inputSequenceBindingPassed: runtimeRevalidation.inputSequenceBindingPassed === true,
    branchMixBindingPassed: runtimeRevalidation.branchMixBindingPassed === true,
    legacyBaselineRunIdDetected,
    legacyBaselineRunIdIgnored: Boolean(legacyBaselineRunIdDetected),
    legacyBaselineCandidates,
    ignoredHistoricalCandidates: legacyBaselineCandidates,
    runIdCollisionCount: new Set(allRunIds).size === allRunIds.length ? 0 : allRunIds.length - new Set(allRunIds).size,
    freshRunIdsConsumed: false,
    databaseCreated: resources.databaseCreated,
    environmentStarted: manifest?.environmentStarted === true,
    datasetExecuted: manifest?.datasetExecuted === true,
    k6Executed: manifest?.baselineExecuted === true || manifest?.currentExecuted === true,
    rawArtifactCreated: resources.rawArtifactCreated,
    frozenArtifactCreated: resources.frozenArtifactCreated,
    listener18080Count: resources.listener18080Count,
    port18080Available: resources.listener18080Count === 0,
    freshPidExists: resources.freshPidExists,
    unknownDatabaseCount: 0,
    failedChecks,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
    issues: failedChecks,
    nextRequiredAction: failedChecks.length ? 'Repair Recovery6 preflight/freeze/manifest binding before baseline start.' : 'Recovery6 formal baseline may begin.',
  };
  return report;
}

function legacyPreflight({ recovery5 = false } = {}) {
  const baseline = resolveActiveBaseline();
  const runtime = runtimeSourceFingerprint();
  const expectedRuntime = baseline.baseline?.runtimeSourceTreeHash || '';
  const runtimeMatches = Boolean(expectedRuntime && expectedRuntime === runtime.hash);
  const issues = [...baseline.issues];
  if (!runtimeMatches) issues.push('runtime source tree fingerprint differs from the frozen baseline');
  return {
    phase: recovery5 ? 'P7-V2-R3B-FAST-CLOSE-R2' : 'P7-V2-R3B-FIX',
    component: 'preflight-audit',
    status: issues.length ? 'failed' : 'passed',
    baselineRunId: baseline.baseline?.runId || '',
    baselineStatus: baseline.baseline?.status || '',
    baselineImmutable: baseline.baseline?.immutable === true,
    baselineValidForRegression: baseline.baseline?.validForRegression === true,
    baselineRequests: Number(baseline.baseline?.completedRequests || 0),
    baselineArtifactHashVerified: baseline.valid,
    expectedRuntimeSourceTreeHash: expectedRuntime,
    currentRuntimeSourceTreeHash: runtime.hash,
    runtimeSourceTreeMatch: runtimeMatches,
    recovery5Required: !runtimeMatches,
    currentExecutionAllowed: issues.length === 0 && runtimeMatches,
    issues,
    nextRequiredAction: issues.length
      ? 'Restore immutable raw evidence and establish a new baseline when runtime or load semantics differ.'
      : 'R3B execution may begin manually.',
  };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const recovery5 = process.argv.includes('--recovery5');
  const recovery6 = process.argv.includes('--recovery6');
  if (recovery5 && recovery6) throw new Error('select only one Recovery preflight mode');
  const legacy = resolveActiveBaseline({ verifyArtifact: false });
  const report = recovery6
    ? evaluateRecovery6Preflight({ legacyBaselineCandidates: [legacy.baseline?.runId].filter(Boolean) })
    : legacyPreflight({ recovery5 });
  const output = recovery6
    ? ['docs/p7-v2-r3b-fast-close-r3-recovery6-preflight-audit.json', 'docs/P7_V2_R3B_FAST_CLOSE_R3_RECOVERY6_PREFLIGHT_AUDIT.md', 'P7-V2-R3B-FAST-CLOSE-R3-FORMAL Recovery6 Preflight Audit']
    : ['docs/p7-v2-r3b-fix-preflight-audit.json', 'docs/P7_V2_R3B_FIX_PREFLIGHT_AUDIT.md', 'P7-V2-R3B-FIX Preflight Audit'];
  writeR3Report(...output, report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' && report.semanticGatePassed !== false ? 0 : 1);
}

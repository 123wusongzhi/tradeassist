import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { gitCommit, readJSON, root, run, writeJSON } from './p7-v2-lib.mjs';
import { resolveBinaryForRunId, sha256File, verifyBinaryReceipt } from './p7-v2-formal-binary-provenance-lib.mjs';

export const FORMAL_INVOCATION_CONTRACT_VERSION = 2;
export const CANONICAL_FORMAL_MANIFEST_PATH = 'docs/p7-v2-r3b-run-manifest.json';
export const FORMAL_INVOCATION_EVIDENCE_DIR = 'docs/formal-invocations/p7-v2-r3b';

const SHA256_RE = /^[a-f0-9]{64}$/;
const RECOVERY6_RUN_ID = /^p7v2-(baseline|current|soak|demo[12])-r3b-recovery6-[a-z0-9_-]+$/;

function sha256Text(value) {
  return crypto.createHash('sha256').update(String(value)).digest('hex');
}

export function manifestSha256(manifestPath = CANONICAL_FORMAL_MANIFEST_PATH) {
  const full = path.join(root, manifestPath);
  return fs.existsSync(full) ? crypto.createHash('sha256').update(fs.readFileSync(full)).digest('hex') : '';
}

export function readCanonicalFormalManifest() {
  return readJSON(CANONICAL_FORMAL_MANIFEST_PATH) || {};
}

export function parseNamedArg(args, name) {
  const prefix = `${name}=`;
  const inline = args.find((arg) => arg === name || arg.startsWith(prefix));
  if (!inline) return { present: false, value: '', valueMissing: false };
  if (inline === name) {
    const index = args.indexOf(name);
    const next = args[index + 1];
    const valueMissing = next == null || String(next).trim() === '' || String(next).startsWith('--');
    return { present: true, value: valueMissing ? '' : String(next).trim(), valueMissing };
  }
  const value = String(inline.slice(prefix.length)).trim();
  return { present: true, value, valueMissing: value === '' };
}

export function formalRoleForRunId(runId) {
  const value = String(runId || '');
  if (value.includes('-baseline-')) return 'baseline';
  if (value.includes('-current-')) return 'current';
  if (value.includes('-soak-')) return 'soak';
  if (value.includes('-demo1-')) return 'demo1';
  if (value.includes('-demo2-')) return 'demo2';
  return '';
}

export function runIdForRole(manifest, role) {
  return {
    baseline: manifest.baselineRunId,
    current: manifest.currentRunId,
    soak: manifest.soakRunId,
    demo1: manifest.demoRun1Id,
    demo2: manifest.demoRun2Id,
  }[role] || '';
}

export function manifestRunIds(manifest = readCanonicalFormalManifest()) {
  return ['baseline', 'current', 'soak', 'demo1', 'demo2'].map((role) => runIdForRole(manifest, role)).filter(Boolean);
}

export function isManifestRunId(runId, manifest = readCanonicalFormalManifest()) {
  return manifestRunIds(manifest).includes(String(runId || '').trim());
}

export function validateEnvStartArgs(args, { env = process.env, manifest = readCanonicalFormalManifest() } = {}) {
  const runIdArg = parseNamedArg(args, '--run-id');
  const formal = args.includes('--formal');
  const issues = [];
  if (runIdArg.present && runIdArg.valueMissing) {
    issues.push('run_id_argument_value_missing');
  }
  const envRunId = String(env.P7_V2_RUN_ID || '').trim();
  const resolvedRunId = runIdArg.present ? runIdArg.value : envRunId;
  const role = formalRoleForRunId(resolvedRunId);
  const manifestRunId = runIdForRole(manifest, role);
  const runIdIsManifestBound = Boolean(resolvedRunId && manifestRunId && resolvedRunId === manifestRunId);
  if (formal) {
    if (!resolvedRunId) issues.push('formal_run_id_required');
    if (!RECOVERY6_RUN_ID.test(resolvedRunId)) issues.push('formal_run_id_must_be_recovery6');
    if (!runIdIsManifestBound) issues.push('formal_run_id_must_match_canonical_manifest');
    if (manifest.formalInvocationContractVersion !== FORMAL_INVOCATION_CONTRACT_VERSION) issues.push('formal_invocation_contract_v2_required');
    if (manifest.preflightBindingVersion !== 3) issues.push('preflight_binding_v3_required');
    if (manifest.active !== true || manifest.validForExecution !== true) issues.push('canonical_manifest_not_active_for_formal_execution');
    if (!['planned', 'runtime_frozen', 'ready_for_formal_execution'].includes(manifest.status)) issues.push('canonical_manifest_status_not_pre_execution');
  } else if (resolvedRunId && isManifestRunId(resolvedRunId, manifest)) {
    issues.push('manifest_formal_run_id_requires_formal_mode');
  }
  return {
    status: issues.length ? 'failed' : 'passed',
    valid: issues.length === 0,
    formal,
    runIdArgumentPresent: runIdArg.present,
    runIdArgumentValueMissing: runIdArg.valueMissing,
    resolvedRunId,
    role,
    runIdSource: runIdArg.present ? 'argv' : envRunId ? 'env' : 'implicit_dev',
    runIdIsManifestBound,
    issues,
  };
}

export function resolveFormalBinaryBinding({ runId, role = formalRoleForRunId(runId), manifest = readCanonicalFormalManifest() } = {}) {
  const issues = [];
  if (!role || !['baseline', 'current'].includes(role)) return { status: 'not_applicable', role, binding: null, issues };
  const binding = resolveBinaryForRunId(runId, manifest);
  if (!binding) issues.push('formal_binary_binding_missing');
  const binaryPath = binding?.binaryPath || '';
  const absBinary = binaryPath ? path.join(root, binaryPath) : '';
  if (!absBinary || !fs.existsSync(absBinary)) issues.push('formal_frozen_binary_missing');
  const actualSha256 = absBinary && fs.existsSync(absBinary) ? sha256File(absBinary) : '';
  if (binding?.binarySha256 && actualSha256 && binding.binarySha256 !== actualSha256) issues.push('formal_frozen_binary_sha256_mismatch');
  if (binding?.receiptPath) {
    const receipt = verifyBinaryReceipt(path.join(root, binding.receiptPath), {
      role,
      runtimeCommit: binding.runtimeCommit,
    });
    if (!receipt.valid) issues.push(...receipt.issues.map((issue) => `formal_binary_receipt_${issue}`));
  }
  return {
    status: issues.length ? 'failed' : 'passed',
    role,
    binding,
    expectedBinaryPath: binaryPath,
    expectedBinarySha256: binding?.binarySha256 || '',
    actualBinarySha256: actualSha256,
    binaryExists: Boolean(absBinary && fs.existsSync(absBinary)),
    binarySha256Match: Boolean(binding?.binarySha256 && actualSha256 === binding.binarySha256),
    issues,
  };
}

export function validateFormalInvocationGate({ stage, role = '', manifest = readCanonicalFormalManifest(), requirePreflight = true, requireRuntimeFreeze = true } = {}) {
  const currentGitHead = gitCommit();
  const preflight = readJSON('docs/p7-v2-r3b-fast-close-r3-recovery6-preflight-audit.json') || {};
  const revalidation = readJSON('docs/p7-v2-r3b-runtime-freeze-revalidation.json') || {};
  const resolvedRunId = runIdForRole(manifest, role);
  const binary = role ? resolveFormalBinaryBinding({ runId: resolvedRunId, role, manifest }) : { status: 'not_applicable', issues: [] };
  const checks = [
    ['formal_invocation_contract_v2', manifest.formalInvocationContractVersion === FORMAL_INVOCATION_CONTRACT_VERSION],
    ['preflight_binding_v3', manifest.preflightBindingVersion === 3],
    ['manifest_status_pre_execution', ['planned', 'runtime_frozen', 'ready_for_formal_execution'].includes(manifest.status)],
    ['manifest_active', manifest.active === true && manifest.validForExecution === true],
    ['plan_checkpoint_current_head', manifest.planCheckpoint === currentGitHead],
    ['resolved_run_id_present', role ? Boolean(resolvedRunId) : true],
    ['resolved_run_id_matches_role', role ? formalRoleForRunId(resolvedRunId) === role : true],
    ['formal_execution_state_allows_stage', manifest.formalExecutionStarted !== true || ['cleanup-check', 'cleanup-execute', 'final-gates'].includes(stage)],
    ['runtime_freeze_created', requireRuntimeFreeze ? manifest.runtimeFreezeCreated === true && SHA256_RE.test(manifest.runtimeFreezeId || '') : true],
    ['runtime_freeze_still_valid', requireRuntimeFreeze ? revalidation.runtimeFreezeStillValid === true : true],
    ['revalidation_evidence_fresh', requireRuntimeFreeze ? revalidation.revalidationRuntimeFreezeId === manifest.runtimeFreezeId && revalidation.revalidationGitHead === currentGitHead : true],
    ['preflight_passed', requirePreflight ? preflight.status === 'passed' && preflight.semanticGatePassed === true && preflight.preflightBindingVersion === 3 : true],
    ['binary_exists', binary.status === 'not_applicable' || binary.binaryExists === true],
    ['binary_sha256_match', binary.status === 'not_applicable' || binary.binarySha256Match === true],
  ];
  const failedChecks = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failedChecks.length ? 'failed' : 'passed',
    semanticGatePassed: failedChecks.length === 0,
    formalInvocationContractVersion: FORMAL_INVOCATION_CONTRACT_VERSION,
    stage,
    role,
    resolvedRunId,
    currentGitHead,
    manifestPath: CANONICAL_FORMAL_MANIFEST_PATH,
    manifestSha256: manifestSha256(),
    runtimeFreezeId: manifest.runtimeFreezeId || '',
    planCheckpoint: manifest.planCheckpoint || '',
    preflightPassed: preflight.status === 'passed' && preflight.semanticGatePassed === true,
    binary,
    failedChecks,
    issues: failedChecks,
  };
}

export function writeFormalInvocationEvidence(evidence) {
  const id = evidence.formalInvocationId || sha256Text(`${evidence.stage}:${Date.now()}:${Math.random()}`).slice(0, 16);
  const rel = `${FORMAL_INVOCATION_EVIDENCE_DIR}/${id}.json`;
  writeJSON(rel, { ...evidence, formalInvocationId: id });
  writeJSON('docs/p7-v2-r3b-latest-formal-invocation.json', { ...evidence, formalInvocationId: id, evidencePath: rel });
  return rel;
}

export function spawnFormalCommand(command, argv, opts = {}) {
  return spawnSync(command, argv, {
    cwd: opts.cwd || root,
    env: { ...process.env, ...(opts.env || {}) },
    stdio: opts.stdio || 'inherit',
    encoding: opts.encoding || 'utf8',
    timeout: opts.timeout ?? 2 * 60 * 60 * 1000,
    shell: false,
  });
}

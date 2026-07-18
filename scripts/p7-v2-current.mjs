import { spawnSync } from 'node:child_process';
import { freezeRawArtifact } from './p7-v2-artifact-freeze.mjs';
import { readJSON, valueOf, writeJSON } from './p7-v2-lib.mjs';
import { resolveActiveBaseline } from './p7-v2-evidence-resolver.mjs';
import { updateR3BManifest } from './p7-v2-r3b-manifest.mjs';
import { readRuntimeFreezeContract, validateRuntimeFreezeContract } from './p7-v2-r3b-lpc-r3-runtime-freeze.mjs';
import { revalidateRuntimeFreeze } from './p7-v2-runtime-freeze-revalidate.mjs';
import { validateFormalExecutionLifecycle } from './p7-v2-r3b-lifecycle.mjs';

const args = process.argv.slice(2);
const formal = args.includes('--formal');
const runId = valueOf(args, '--run-id') || `p7v2-current-${new Date().toISOString().replace(/[:.]/g, '-')}`;
if (!/^p7v2-current-r3b-recovery6-[a-z0-9_-]+$/.test(runId)) {
  throw new Error('P7-V2-R3B-FAST-CLOSE-R3-FORMAL requires a unique Recovery6 current run ID');
}
const runtimeFreeze = readRuntimeFreezeContract();
const runtimeFreezeValidation = validateRuntimeFreezeContract(runtimeFreeze, { kind: 'current', runId });
if (!runtimeFreezeValidation.valid) throw new Error(`valid Recovery6 runtime freeze contract is required: ${runtimeFreezeValidation.issue}`);
const activeBaseline = resolveActiveBaseline();
if (!activeBaseline.valid) throw new Error(`active frozen baseline is invalid: ${activeBaseline.issues.join('; ')}`);
const baselineRunId = activeBaseline.baseline.runId;
if (runId === baselineRunId) throw new Error('Current run ID must differ from the active baseline run ID');
function loadArgsWithoutControllerOnlyFlags(values) {
  const filtered = [];
  for (let i = 0; i < values.length; i += 1) {
    const value = values[i];
    if (value === '--run-id') {
      i += 1;
      continue;
    }
    if (value.startsWith('--run-id=')) continue;
    if (value === '--formal') continue;
    filtered.push(value);
  }
  return filtered;
}
const restartRunId = formal ? runId : `${runId}-restart`;
const restartArgs = ['scripts/p7-v2-restart-environment.mjs', '--run-id', restartRunId];
if (formal) restartArgs.push('--formal');
const restart = spawnSync(process.execPath, restartArgs, {
  stdio: 'inherit',
});
if (restart.status !== 0) process.exit(restart.status ?? 1);
const res = spawnSync(process.execPath, ['scripts/p7-v2-load.mjs', '--kind', 'current', '--run-id', runId, ...loadArgsWithoutControllerOnlyFlags(args)], {
  stdio: 'inherit',
});
const current = readJSON('docs/p7-v2-current-load-report.json');
const restartEvidence = readJSON('docs/p7-v2-environment-restart-report.json') || {};
if (current?.runId === runId) {
  current.restartEvidence = restartEvidence;
  current.currentRunIndependent =
    current.runId !== baselineRunId &&
    restartEvidence.currentRunIndependent === true &&
    restartEvidence.database?.stateReset === true &&
    restartEvidence.database?.datasetVerified === true &&
    restartEvidence.api?.portOwnerVerified === true &&
    restartEvidence.api?.serverBinaryVerified === true &&
    restartEvidence.api?.instanceNonceVerified === true &&
    (restartEvidence.api?.freshProcessVerified === true || restartEvidence.api?.processChanged === true) &&
    restartEvidence.worker?.status === 'passed' &&
    restartEvidence.redis?.stateResetVerified === true &&
    restartEvidence.mockProvider?.freshStateVerified === true;
  current.independentRun = current.currentRunIndependent;
  current.baselineRunId = baselineRunId;
  current.runtimeFreezeId = runtimeFreeze.contractId;
  current.runtimeFreezeContractHash = runtimeFreeze.contractId;
  current.runtimeFreezeRunId = runId;
  current.status = current.status === 'passed' && current.independentRun ? 'passed' : 'failed';
  writeJSON('docs/p7-v2-current-load-report.json', current);
  writeJSON(`docs/runs/p7-v2-current-${runId}.json`, current);
}
if (res.status !== 0) process.exit(res.status ?? 1);
try {
  const frozen = freezeRawArtifact({
    kind: 'current',
    runId,
    reportPath: 'docs/p7-v2-current-load-report.json',
  });
  writeJSON(`docs/currents/p7-v2-current-${runId}.json`, {
    ...current,
    immutable: true,
    rawArtifactPath: frozen.originalPath,
    rawArtifactSha256: frozen.sha256,
    rawArtifactSizeBytes: frozen.sizeBytes,
    rawArtifactHashVerified: true,
    scenarioCoverage: frozen.scenarioCoverage,
    frozenArtifactPath: frozen.frozenPath,
    frozenAt: frozen.createdAt,
  });
  const registryPath = 'docs/currents/p7-v2-current-registry.json';
  const registry = readJSON(registryPath) || { activeRegressionCurrent: '', entries: [] };
  if ((registry.entries || []).some((entry) => entry.runId === runId)) {
    throw new Error(`Current registry already contains run ID: ${runId}`);
  }
  const entry = {
    runId,
    status: 'passed',
    independentRun: true,
    immutable: true,
    validForRegression: true,
    rawArtifactPath: frozen.frozenPath,
    rawArtifactSha256: frozen.sha256,
    rawArtifactSizeBytes: frozen.sizeBytes,
    baselineRunId,
    runtimeSourceTreeHash: current.runtimeSourceTreeHash || '',
    formalBinaryProvenanceVersion: current.formalBinaryProvenanceVersion || null,
    serverBinarySha256: current.serverBinarySha256 || '',
    expectedBinarySha256: current.expectedBinarySha256 || '',
    processExecutableSha256: current.processExecutableSha256 || '',
    processExecutableSha256Match: current.processExecutableSha256Match ?? null,
    formalInputSequenceBindingVersion: current.formalInputSequenceBindingVersion || null,
    inputSequenceManifestHash: current.inputSequenceManifestHash || '',
    requestSequenceHash: current.requestSequenceHash || '',
    webhookSequenceHash: current.webhookSequenceHash || '',
    authSequenceHash: current.authSequenceHash || '',
    webhookDuplicateSequenceHash: current.webhookDuplicateSequenceHash || '',
    webhookBranchMixFingerprint: current.webhookBranchMixFingerprint || '',
    authBranchMixFingerprint: current.authBranchMixFingerprint || '',
    branchMixFingerprint: current.branchMixFingerprint || '',
    loadScriptsHash: current.loadScriptsHash || current.loadScriptHash || '',
    metricSemanticsHash: current.metricSemanticsHash || '',
    datasetFingerprint: current.datasetFingerprint || '',
    configFingerprint: current.configFingerprint || '',
    loadProfileFingerprint: current.loadProfileFingerprint || '',
    sloFingerprint: current.sloFingerprint || '',
    routeCredentialMatrixFingerprint: current.routeCredentialMatrixFingerprint || '',
    regressionPolicyFingerprint: current.regressionPolicyFingerprint || '',
    selectedHost: current.selectedHost || '',
    selectedPort: current.selectedPort || 0,
    baseUrl: current.baseUrl || '',
    frozenAt: frozen.createdAt,
  };
  writeJSON(registryPath, { ...registry, activeRegressionCurrent: runId, entries: [...(registry.entries || []), entry] });
  updateR3BManifest({ currentRunId: runId, status: 'current_frozen' });
  const freezeRevalidation = revalidateRuntimeFreeze({ writeReport: true, mode: 'revalidate' });
  if (freezeRevalidation.status !== 'passed') throw new Error(`runtime freeze immutable revalidation failed after current freeze: ${freezeRevalidation.rebuildError || 'immutable mismatch'}`);
  const lifecycle = validateFormalExecutionLifecycle({ previousState: 'current_completed', nextState: 'current_frozen' });
  writeJSON('docs/p7-v2-r3b-runtime-freeze-lifecycle-validation.json', lifecycle);
  if (lifecycle.status !== 'passed') throw new Error(`current lifecycle validation failed: ${lifecycle.issues.map((issue) => issue.issue).join('; ')}`);
  console.log(JSON.stringify({ phase: 'P7-V2-R3B-FAST-CLOSE-R3-FORMAL', kind: 'current', runId, freeze: 'passed', sha256: frozen.sha256 }, null, 2));
} catch (error) {
  console.error(JSON.stringify({ phase: 'P7-V2-R3B-REBASELINE', kind: 'current', runId, freeze: 'failed', error: error.message }, null, 2));
  process.exit(1);
}

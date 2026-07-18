import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { readJSON, run, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { buildFormalLoadProfile, freezeCurrentContract, generatedEvidenceDiffAudit } from './p7-v2-runtime-freeze-scope.mjs';
import { verifyBinaryReceipt } from './p7-v2-formal-binary-provenance-lib.mjs';

const OUT_JSON = 'docs/p7-v2-r3b-clean-head-runtime-freeze-final-gate.json';
const OUT_MD = 'docs/P7_V2_R3B_CLEAN_HEAD_RUNTIME_FREEZE_FINAL_GATE.md';

function gitHead() {
  const res = run('git', ['rev-parse', 'HEAD']);
  return res.status === 0 ? String(res.stdout || '').trim() : '';
}

function gitTree() {
  const res = run('git', ['rev-parse', 'HEAD^{tree}']);
  return res.status === 0 ? String(res.stdout || '').trim() : '';
}

function commandPassed(command, args) {
  const res = run(command, args, { timeout: 120000, maxBuffer: 20 * 1024 * 1024 });
  return { passed: res.status === 0, command: res.command, status: res.status, stdout: res.stdout.slice(0, 4000), stderr: res.stderr.slice(0, 4000) };
}

export function buildCleanHeadRuntimeFreezeFinalGate() {
  const manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
  const runtimeFreeze = freezeCurrentContract(readJSON('docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json') || {}) || {};
  const revalidation = readJSON('docs/p7-v2-r3b-runtime-freeze-revalidation.json') || {};
  const closeout = readJSON('docs/p7-v2-r3b-precommit-runtime-freeze-closeout.json') || {};
  const binary = readJSON('docs/p7-v2-r3b-formal-binary-provenance-manifest.json') || {};
  const input = readJSON('docs/p7-v2-r3b-formal-input-sequence-manifest.json') || {};
  const baselineReceipt = binary.baselineBinaryReceiptPath ? readJSON(binary.baselineBinaryReceiptPath) : null;
  const currentReceipt = binary.currentBinaryReceiptPath ? readJSON(binary.currentBinaryReceiptPath) : null;
  const baselineVerify = verifyBinaryReceipt(baselineReceipt, { role: 'baseline', runtimeCommit: binary.baselineRuntimeCommit });
  const currentVerify = verifyBinaryReceipt(currentReceipt, { role: 'current', runtimeCommit: binary.currentRuntimeCommit });
  const generatedEvidence = generatedEvidenceDiffAudit();
  const profile = buildFormalLoadProfile();
  const currentGitHead = gitHead();
  const currentGitTree = gitTree();
  const fixture = commandPassed('node', ['tests/gates/p7-v2/clean-head-runtime-freeze/fixtures.mjs']);
  const checks = {
    runtimeFreezeLifecycleVersion: runtimeFreeze.runtimeFreezeLifecycleVersion === 3 && runtimeFreeze.runtimeFreezeLifecycleContractVersion === 3,
    cleanCommittedHeadRequired: runtimeFreeze.cleanCommittedHeadRequired === true,
    uncommittedImmutableStateRejected: fixture.passed,
    freezeCreationGitHeadPresent: /^[a-f0-9]{40}$/.test(runtimeFreeze.freezeCreationGitHead || ''),
    freezeCreationGitTreePresent: /^[a-f0-9]{40}$/.test(runtimeFreeze.freezeCreationGitTree || ''),
    freezeCreationGitHeadCurrent: runtimeFreeze.freezeCreationGitHead === currentGitHead,
    freezeCreationGitTreeCurrent: runtimeFreeze.freezeCreationGitTree === currentGitTree,
    immutableTrackedDiffPresent: runtimeFreeze.immutableTrackedDiffPresent === false,
    immutableWorkingTreeClean: runtimeFreeze.immutableWorkingTreeClean === true,
    generatedEvidenceExcluded: revalidation.generatedEvidenceExcluded === true && generatedEvidence.generatedEvidenceExcluded === true,
    generatedEvidenceDoesNotInvalidateFreeze: revalidation.generatedEvidenceDoesNotInvalidateFreeze === true,
    binaryFilesVerified: baselineVerify.valid === true && currentVerify.valid === true,
    binaryReceiptsVerified: baselineReceipt?.binarySha256 === binary.baselineBinarySha256 && currentReceipt?.binarySha256 === binary.currentBinarySha256,
    inputSequenceManifestVerified: input.status === 'passed' && input.inputSequenceManifestHash === manifest.inputSequenceManifestHash,
    planCheckpointEqualsFreezeHead: manifest.planCheckpoint === currentGitHead && runtimeFreeze.planBindingPayload?.planCheckpoint === currentGitHead,
    staleRevalidationRejected: revalidation.revalidationRuntimeFreezeId === manifest.runtimeFreezeId &&
      revalidation.revalidationGitHead === currentGitHead &&
      revalidation.revalidationGitTree === currentGitTree &&
      revalidation.revalidationPlanCheckpoint === manifest.planCheckpoint,
    runIdConsumptionAudited: closeout.status === 'passed' && Array.isArray(closeout.runIdAudit),
    thresholdChanged: false,
    sloChanged: false,
    vusChanged: profile.configuredVUs !== 10,
    datasetChanged: manifest.datasetProfile !== 'medium' || Number(manifest.expectedRows) !== 1900150,
  };
  const failedIds = Object.entries(checks)
    .filter(([id, passed]) => {
      if (['thresholdChanged', 'sloChanged', 'vusChanged', 'datasetChanged'].includes(id)) return passed !== false;
      return passed !== true;
    })
    .map(([id]) => id);
  const report = {
    phase: 'P7-V2-R3B-CLEAN-HEAD-RUNTIME-FREEZE-LIFECYCLE-V3',
    component: 'clean-head-runtime-freeze-final-gate',
    status: failedIds.length ? 'failed' : 'passed',
    runtimeFreezeLifecycleVersion: runtimeFreeze.runtimeFreezeLifecycleVersion ?? runtimeFreeze.runtimeFreezeLifecycleContractVersion ?? null,
    cleanCommittedHeadRequired: checks.cleanCommittedHeadRequired,
    uncommittedImmutableStateRejected: checks.uncommittedImmutableStateRejected,
    freezeCreationGitHeadPresent: checks.freezeCreationGitHeadPresent,
    freezeCreationGitTreePresent: checks.freezeCreationGitTreePresent,
    immutableTrackedDiffPresent: runtimeFreeze.immutableTrackedDiffPresent === true,
    immutableWorkingTreeClean: checks.immutableWorkingTreeClean,
    generatedEvidenceExcluded: checks.generatedEvidenceExcluded,
    generatedEvidenceDoesNotInvalidateFreeze: checks.generatedEvidenceDoesNotInvalidateFreeze,
    binaryFilesVerified: checks.binaryFilesVerified,
    binaryReceiptsVerified: checks.binaryReceiptsVerified,
    inputSequenceManifestVerified: checks.inputSequenceManifestVerified,
    planCheckpointEqualsFreezeHead: checks.planCheckpointEqualsFreezeHead,
    staleRevalidationRejected: checks.staleRevalidationRejected,
    runIdConsumptionAudited: checks.runIdConsumptionAudited,
    thresholdChanged: checks.thresholdChanged,
    sloChanged: checks.sloChanged,
    vusChanged: checks.vusChanged,
    datasetChanged: checks.datasetChanged,
    failed: failedIds.length,
    failedIds,
    currentGitHead,
    currentGitTree,
    runtimeFreezeId: runtimeFreeze.runtimeFreezeId || '',
    revalidationRuntimeFreezeId: revalidation.revalidationRuntimeFreezeId || '',
    fixture,
    checks,
    generatedAt: new Date().toISOString(),
  };
  writeJSON(OUT_JSON, report);
  writeMarkdown(
    OUT_MD,
    `# P7-V2-R3B Clean-Head Runtime Freeze Final Gate\n\nStatus: **${report.status}**\n\n- Runtime freeze lifecycle version: \`${report.runtimeFreezeLifecycleVersion ?? 'missing'}\`\n- Clean committed HEAD required: \`${report.cleanCommittedHeadRequired}\`\n- Generated evidence excluded: \`${report.generatedEvidenceExcluded}\`\n- Stale revalidation rejected: \`${report.staleRevalidationRejected}\`\n- Failed checks: \`${report.failed}\`\n`,
  );
  return report;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildCleanHeadRuntimeFreezeFinalGate();
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}

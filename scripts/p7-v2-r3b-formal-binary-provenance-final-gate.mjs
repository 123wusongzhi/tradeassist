import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { gitCommit, readJSON, root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { FORMAL_BINARY_PROVENANCE_VERSION, verifyBinaryReceipt } from './p7-v2-formal-binary-provenance-lib.mjs';
import { FORMAL_INPUT_SEQUENCE_BINDING_VERSION } from './p7-v2-formal-input-sequence.mjs';

const SECRET_RE = /(^|_)(SECRET|PASSWORD|TOKEN|COOKIE|DATABASE_URL|JWT|PROVIDER_KEY|DB_PASS)$/i;

function hasSha(value) {
  return /^[a-f0-9]{64}$/.test(String(value || ''));
}

function fileText(rel) {
  try {
    return fs.readFileSync(path.join(root, rel), 'utf8');
  } catch {
    return '';
  }
}

function countSecrets(value) {
  const text = JSON.stringify(value || {});
  return SECRET_RE.test(text) ? 1 : 0;
}

export function buildFormalBinaryProvenanceFinalGate() {
  const closeout = readJSON('docs/p7-v2-r3b-failed-formal-pair-binary-provenance-closeout.json') || {};
  const binary = readJSON('docs/p7-v2-r3b-formal-binary-provenance-manifest.json') || {};
  const input = readJSON('docs/p7-v2-r3b-formal-input-sequence-manifest.json') || {};
  const runtimeFreezeDoc = readJSON('docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json') || {};
  const runtimeFreeze = runtimeFreezeDoc.current || runtimeFreezeDoc;
  const baselineReceipt = binary.baselineBinaryReceiptPath ? readJSON(binary.baselineBinaryReceiptPath) : binary.binaryProvenance?.baseline;
  const currentReceipt = binary.currentBinaryReceiptPath ? readJSON(binary.currentBinaryReceiptPath) : binary.binaryProvenance?.current;
  const baselineVerify = verifyBinaryReceipt(baselineReceipt, { role: 'baseline', runtimeCommit: binary.baselineRuntimeCommit });
  const currentVerify = verifyBinaryReceipt(currentReceipt, { role: 'current', runtimeCommit: binary.currentRuntimeCommit });
  const libSource = fileText('scripts/p7-v2-lib.mjs');
  const checks = [
    ['historicalPairCloseoutCompleted', closeout.status === 'passed' && closeout.validForPerformanceComparison === false && closeout.binaryProvenancePassed === false],
    ['formalBinaryProvenanceVersion', binary.formalBinaryProvenanceVersion === FORMAL_BINARY_PROVENANCE_VERSION],
    ['formalInputSequenceBindingVersion', input.formalInputSequenceBindingVersion === FORMAL_INPUT_SEQUENCE_BINDING_VERSION],
    ['canonicalBaselineSelected', Boolean(binary.baselineRuntimeCommit)],
    ['canonicalCurrentSelected', Boolean(binary.currentRuntimeCommit)],
    ['baselineRuntimeCommitPresent', /^[a-f0-9]{40}$/.test(binary.baselineRuntimeCommit || '')],
    ['currentRuntimeCommitPresent', /^[a-f0-9]{40}$/.test(binary.currentRuntimeCommit || '')],
    ['baselineSourceTreeHashPresent', hasSha(baselineReceipt?.sourceTreeHash)],
    ['currentSourceTreeHashPresent', hasSha(currentReceipt?.sourceTreeHash)],
    ['baselineBinarySha256Present', hasSha(binary.baselineBinarySha256)],
    ['currentBinarySha256Present', hasSha(binary.currentBinarySha256) && binary.currentBinarySha256 !== binary.baselineBinarySha256],
    ['baselineBinaryVerified', baselineVerify.valid],
    ['currentBinaryVerified', currentVerify.valid],
    ['baselineMigrationSetHashPresent', hasSha(baselineReceipt?.migrationSetHash)],
    ['currentMigrationSetHashPresent', hasSha(currentReceipt?.migrationSetHash)],
    ['implicitBuildDisabled', /formalBinaryBinding/.test(libSource) && /manifest-bound formal binary/.test(libSource)],
    ['processExecutableVerificationEnabled', libSource.includes('processExecutableSha256') && libSource.includes('/proc/${pid}/exe')],
    ['inputSequenceManifestPresent', hasSha(input.inputSequenceManifestHash)],
    ['webhookSequenceHashPresent', hasSha(input.webhookSequenceHash)],
    ['authSequenceHashPresent', hasSha(input.authSequenceHash)],
    ['branchMixFingerprintPresent', hasSha(input.branchMixFingerprint)],
    ['runtimeFreezeBinaryBindingPresent', !runtimeFreeze.runtimeFreezeId || (runtimeFreeze.binaryProvenanceBindingVersion === 2 && hasSha(runtimeFreeze.baselineBinarySha256) && hasSha(runtimeFreeze.currentBinarySha256))],
    ['runtimeFreezeInputBindingPresent', !runtimeFreeze.runtimeFreezeId || (runtimeFreeze.inputSequenceBindingVersion === 1 && hasSha(runtimeFreeze.inputSequenceManifestHash))],
  ];
  const failed = checks.filter(([, passed]) => !passed).map(([id]) => id);
  const secretsInEvidenceCount = countSecrets(binary) + countSecrets(input) + countSecrets(closeout);
  if (secretsInEvidenceCount > 0) failed.push('secretsInEvidenceCount');
  return {
    phase: 'P7-V2-R3B-FORMAL-BINARY-PROVENANCE-V2-INPUT-SEQUENCE-BINDING-FINAL-GATE',
    status: failed.length ? 'failed' : 'passed',
    controlToolingCommit: gitCommit(),
    formalBinaryProvenanceVersion: binary.formalBinaryProvenanceVersion || null,
    formalInputSequenceBindingVersion: input.formalInputSequenceBindingVersion || null,
    baselineRuntimeCommit: binary.baselineRuntimeCommit || '',
    currentRuntimeCommit: binary.currentRuntimeCommit || '',
    baselineSourceTreeHash: baselineReceipt?.sourceTreeHash || '',
    currentSourceTreeHash: currentReceipt?.sourceTreeHash || '',
    baselineBinarySha256: binary.baselineBinarySha256 || '',
    currentBinarySha256: binary.currentBinarySha256 || '',
    baselineBinaryVerified: baselineVerify.valid,
    currentBinaryVerified: currentVerify.valid,
    baselineMigrationSetHash: baselineReceipt?.migrationSetHash || '',
    currentMigrationSetHash: currentReceipt?.migrationSetHash || '',
    implicitBuildDisabled: !failed.includes('implicitBuildDisabled'),
    processExecutableVerificationEnabled: !failed.includes('processExecutableVerificationEnabled'),
    inputSequenceManifestHash: input.inputSequenceManifestHash || '',
    webhookSequenceHash: input.webhookSequenceHash || '',
    authSequenceHash: input.authSequenceHash || '',
    branchMixFingerprint: input.branchMixFingerprint || '',
    secretsInEvidenceCount,
    failed,
    failedCount: failed.length,
    checks: checks.map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
    historicalPairCloseoutCompleted: closeout.status === 'passed',
    formalRerun: 'notStarted',
    productionReady: false,
    generatedAt: new Date().toISOString(),
  };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const gate = buildFormalBinaryProvenanceFinalGate();
  writeJSON('docs/p7-v2-r3b-formal-binary-provenance-final-gate.json', gate);
  writeMarkdown(
    'docs/P7_V2_R3B_FORMAL_BINARY_PROVENANCE_FINAL_GATE.md',
    `# P7-V2-R3B Binary Provenance and Input Sequence Final Gate\n\nStatus: **${gate.status}**\n\n- Binary provenance version: \`${gate.formalBinaryProvenanceVersion ?? 'missing'}\`\n- Input sequence binding version: \`${gate.formalInputSequenceBindingVersion ?? 'missing'}\`\n- Baseline binary verified: ${gate.baselineBinaryVerified}\n- Current binary verified: ${gate.currentBinaryVerified}\n- Failed checks: ${gate.failed.length ? gate.failed.join(', ') : 'none'}\n\nThis gate does not start formal execution, soak, demo, stability, race, cleanup, push, tag, release, or production acceptance.\n`,
  );
  console.log(JSON.stringify(gate, null, 2));
  process.exit(gate.status === 'passed' ? 0 : 1);
}

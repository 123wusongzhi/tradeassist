import crypto from 'node:crypto';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { readJSON, writeJSON } from './p7-v2-lib.mjs';
import { stableJson, sha256Json } from './p7-v2-formal-binary-provenance-lib.mjs';

export const FORMAL_INPUT_SEQUENCE_BINDING_VERSION = 1;
export const INPUT_SEQUENCE_MANIFEST_PATH = 'docs/p7-v2-r3b-formal-input-sequence-manifest.json';

const DEFAULT_LOAD_SEED = 'p7-v2-r3b-recovery6-load-seed-v1';
const DEFAULT_SCENARIO_SEED = 'p7-v2-r3b-recovery6-scenario-seed-v1';

function valueOf(args, name) {
  const idx = args.indexOf(name);
  if (idx >= 0 && args[idx + 1]) return args[idx + 1];
  const prefix = `${name}=`;
  const hit = args.find((arg) => arg.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : '';
}

function sha256Text(value) {
  return crypto.createHash('sha256').update(String(value)).digest('hex');
}

function makeTemplateHash(type, branch) {
  return sha256Text(stableJson({ type, branch, schema: 'low-sensitive-template-v1' }));
}

function buildRequests({
  normalInsertTargetCount = 250,
  duplicateConflictTargetCount = 125,
  unknownAccountTargetCount = 120,
  wrongPasswordTargetCount = 120,
  lockedAccountTargetCount = 60,
} = {}) {
  const rows = [];
  let seq = 1;
  const add = (module, branch, count) => {
    for (let i = 0; i < count; i += 1) {
      rows.push({
        seq: seq++,
        module,
        requestType: module === 'webhook' ? 'WEBHOOK_INGESTION' : 'AUTH_INVALID_LOGIN',
        branch,
        deterministicId: `${module}-${branch}-${String(i + 1).padStart(6, '0')}`,
        inputTemplateHash: makeTemplateHash(module, branch),
      });
    }
  };
  add('webhook', 'normal_insert', normalInsertTargetCount);
  add('webhook', 'duplicate_conflict', duplicateConflictTargetCount);
  add('auth', 'unknown_account', unknownAccountTargetCount);
  add('auth', 'wrong_password', wrongPasswordTargetCount);
  add('auth', 'locked_account', lockedAccountTargetCount);
  return rows;
}

function fingerprintFor(rows, predicate) {
  return sha256Json(rows.filter(predicate).map((row) => ({
    seq: row.seq,
    module: row.module,
    requestType: row.requestType,
    branch: row.branch,
    deterministicId: row.deterministicId,
    inputTemplateHash: row.inputTemplateHash,
  })));
}

export function buildFormalInputSequenceManifest(options = {}) {
  const loadSeed = options.loadSeed || DEFAULT_LOAD_SEED;
  const scenarioSeed = options.scenarioSeed || DEFAULT_SCENARIO_SEED;
  const targets = {
    normalInsertTargetCount: Number(options.normalInsertTargetCount ?? 250),
    duplicateConflictTargetCount: Number(options.duplicateConflictTargetCount ?? 125),
    unknownAccountTargetCount: Number(options.unknownAccountTargetCount ?? 120),
    wrongPasswordTargetCount: Number(options.wrongPasswordTargetCount ?? 120),
    lockedAccountTargetCount: Number(options.lockedAccountTargetCount ?? 60),
  };
  const requestSequence = buildRequests(targets);
  const webhookRows = requestSequence.filter((row) => row.module === 'webhook');
  const authRows = requestSequence.filter((row) => row.module === 'auth');
  const webhookDuplicateRows = requestSequence.filter((row) => row.module === 'webhook' && row.branch === 'duplicate_conflict');
  const webhookBranchMix = {
    normalInsertTargetCount: targets.normalInsertTargetCount,
    duplicateConflictTargetCount: targets.duplicateConflictTargetCount,
    normalInsertActualCount: webhookRows.filter((row) => row.branch === 'normal_insert').length,
    duplicateConflictActualCount: webhookDuplicateRows.length,
  };
  const authBranchMix = {
    unknownAccountTargetCount: targets.unknownAccountTargetCount,
    wrongPasswordTargetCount: targets.wrongPasswordTargetCount,
    lockedAccountTargetCount: targets.lockedAccountTargetCount,
    unknownAccountActualCount: authRows.filter((row) => row.branch === 'unknown_account').length,
    wrongPasswordActualCount: authRows.filter((row) => row.branch === 'wrong_password').length,
    lockedAccountActualCount: authRows.filter((row) => row.branch === 'locked_account').length,
  };
  const payload = {
    formalInputSequenceBindingVersion: FORMAL_INPUT_SEQUENCE_BINDING_VERSION,
    loadSeed,
    scenarioSeed,
    requestSequence,
  };
  const requestSequenceHash = sha256Json(payload);
  const webhookSequenceHash = fingerprintFor(requestSequence, (row) => row.module === 'webhook');
  const authSequenceHash = fingerprintFor(requestSequence, (row) => row.module === 'auth');
  const webhookDuplicateSequenceHash = fingerprintFor(requestSequence, (row) => row.module === 'webhook' && row.branch === 'duplicate_conflict');
  const webhookBranchMixFingerprint = sha256Json(webhookBranchMix);
  const authBranchMixFingerprint = sha256Json(authBranchMix);
  return {
    phase: 'P7-V2-R3B-FORMAL-INPUT-SEQUENCE-BINDING-V1',
    status: 'passed',
    formalInputSequenceBindingVersion: FORMAL_INPUT_SEQUENCE_BINDING_VERSION,
    loadSeed,
    scenarioSeed,
    requestSequenceHash,
    webhookSequenceHash,
    authSequenceHash,
    webhookDuplicateSequenceHash,
    webhookBranchMixFingerprint,
    authBranchMixFingerprint,
    branchMixFingerprint: sha256Json({ webhookBranchMixFingerprint, authBranchMixFingerprint }),
    inputSequenceManifestHash: sha256Json({
      formalInputSequenceBindingVersion: FORMAL_INPUT_SEQUENCE_BINDING_VERSION,
      loadSeed,
      scenarioSeed,
      requestSequenceHash,
      webhookSequenceHash,
      authSequenceHash,
      webhookDuplicateSequenceHash,
      webhookBranchMixFingerprint,
      authBranchMixFingerprint,
    }),
    requestCount: requestSequence.length,
    requestSequence,
    webhookBranchMix,
    authBranchMix,
    lowSensitivity: true,
    excludedFields: ['realEmail', 'realPassword', 'realUserId', 'webhookSecret', 'realOrderNumber'],
    generatedAt: new Date().toISOString(),
  };
}

export function compareFormalInputSequences(baseline = {}, current = {}) {
  const keys = [
    'formalInputSequenceBindingVersion',
    'requestSequenceHash',
    'webhookSequenceHash',
    'authSequenceHash',
    'webhookDuplicateSequenceHash',
    'webhookBranchMixFingerprint',
    'authBranchMixFingerprint',
    'branchMixFingerprint',
  ];
  const mismatches = keys.filter((key) => !baseline[key] || baseline[key] !== current[key]);
  const semanticGatePassed =
    baseline.webhookBranchMix?.normalInsertTargetCount === baseline.webhookBranchMix?.normalInsertActualCount &&
    baseline.webhookBranchMix?.duplicateConflictTargetCount === baseline.webhookBranchMix?.duplicateConflictActualCount &&
    baseline.authBranchMix?.unknownAccountTargetCount === baseline.authBranchMix?.unknownAccountActualCount &&
    baseline.authBranchMix?.wrongPasswordTargetCount === baseline.authBranchMix?.wrongPasswordActualCount &&
    baseline.authBranchMix?.lockedAccountTargetCount === baseline.authBranchMix?.lockedAccountActualCount;
  return {
    status: mismatches.length || !semanticGatePassed ? 'not_comparable' : 'passed',
    notComparable: mismatches.length > 0,
    semanticGatePassed,
    mismatches,
  };
}

export function writeFormalInputSequenceManifest(options = {}) {
  const manifest = buildFormalInputSequenceManifest(options);
  writeJSON(options.outputPath || INPUT_SEQUENCE_MANIFEST_PATH, manifest);
  return manifest;
}

export function runInputSequenceCli(argv = process.argv.slice(2)) {
  argv = argv.filter((arg) => arg !== '--');
  const command = argv[0] || 'generate';
  if (command === 'generate') {
    return writeFormalInputSequenceManifest({
      loadSeed: valueOf(argv, '--load-seed') || undefined,
      scenarioSeed: valueOf(argv, '--scenario-seed') || undefined,
      outputPath: valueOf(argv, '--output') || INPUT_SEQUENCE_MANIFEST_PATH,
    });
  }
  if (command === 'verify') {
    const baseline = readJSON(valueOf(argv, '--baseline') || INPUT_SEQUENCE_MANIFEST_PATH) || {};
    const current = readJSON(valueOf(argv, '--current') || INPUT_SEQUENCE_MANIFEST_PATH) || {};
    return {
      phase: 'P7-V2-R3B-FORMAL-INPUT-SEQUENCE-BINDING-V1',
      ...compareFormalInputSequences(baseline, current),
      formalInputSequenceBindingVersion: baseline.formalInputSequenceBindingVersion || null,
    };
  }
  return {
    phase: 'P7-V2-R3B-FORMAL-INPUT-SEQUENCE-BINDING-V1',
    status: 'failed',
    classification: 'unsupported_input_sequence_command',
    supportedCommands: ['generate', 'verify'],
  };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = runInputSequenceCli();
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}

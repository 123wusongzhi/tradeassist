import { fileURLToPath } from 'node:url';
import path from 'node:path';
import {
  CANONICAL_FORMAL_MANIFEST_PATH,
  FORMAL_INVOCATION_CONTRACT_VERSION,
  manifestSha256,
  parseNamedArg,
  readCanonicalFormalManifest,
  runIdForRole,
  spawnFormalCommand,
  validateFormalInvocationGate,
  writeFormalInvocationEvidence,
} from './p7-v2-formal-invocation-lib.mjs';

const STAGES = new Set([
  'preflight',
  'baseline-env-start',
  'baseline-dataset',
  'baseline-load',
  'current-load',
  'comparability',
  'regression',
  'soak',
  'demo-preflight',
  'demo1',
  'demo2',
  'stability',
  'race',
  'cleanup-check',
  'cleanup-execute',
  'final-gates',
]);

function pnpmCommand() {
  return process.platform === 'win32' ? 'pnpm.cmd' : 'pnpm';
}

function commandForStage(stage, manifest) {
  const baselineRunId = runIdForRole(manifest, 'baseline');
  const currentRunId = runIdForRole(manifest, 'current');
  const soakRunId = runIdForRole(manifest, 'soak');
  const demoRun1Id = runIdForRole(manifest, 'demo1');
  const demoRun2Id = runIdForRole(manifest, 'demo2');
  const pnpm = pnpmCommand();
  const byStage = {
    preflight: { role: '', requirePreflight: false, requireRuntimeFreeze: true, command: pnpm, argv: ['p7-v2:r3b:preflight', '--', '--recovery6'] },
    'baseline-env-start': { role: 'baseline', command: pnpm, argv: ['p7-v2:env:start', '--', '--formal', '--run-id', baselineRunId] },
    'baseline-dataset': { role: 'baseline', command: pnpm, argv: ['p7-v2:dataset', '--', '--run-id', baselineRunId, '--execute'] },
    'baseline-load': { role: 'baseline', command: pnpm, argv: ['p7-v2:baseline', '--', '--run-id', baselineRunId] },
    'current-load': { role: 'current', command: pnpm, argv: ['p7-v2:r3b:current', '--', '--formal', '--run-id', currentRunId] },
    comparability: { role: '', command: pnpm, argv: ['p7-v2:r3b:comparability'] },
    regression: { role: '', command: pnpm, argv: ['p7-v2:r3b:regression'] },
    soak: { role: 'soak', command: pnpm, argv: ['p7-v2:r3b:soak', '--', '--run-id', soakRunId] },
    'demo-preflight': { role: '', command: pnpm, argv: ['p7-v2:r3b:demo-preflight'] },
    demo1: { role: 'demo1', command: pnpm, argv: ['p7-v2:r3b:demo', '--', '--run', '1', '--run-id', demoRun1Id] },
    demo2: { role: 'demo2', command: pnpm, argv: ['p7-v2:r3b:demo', '--', '--run', '2', '--run-id', demoRun2Id] },
    stability: { role: '', command: pnpm, argv: ['p7-v2:r3b:stability'] },
    race: { role: '', command: pnpm, argv: ['p7-v2:r3b:race'] },
    'cleanup-check': { role: '', command: pnpm, argv: ['p7-v2:r3b:cleanup', '--', '--check'] },
    'cleanup-execute': { role: '', command: pnpm, argv: ['p7-v2:r3b:cleanup'] },
  };
  if (stage === 'final-gates') {
    return {
      role: '',
      command: process.execPath,
      argv: ['scripts/p7-v2-r3b-formal-invocation-preflight-v3-final-gate.mjs'],
    };
  }
  return byStage[stage] || null;
}

function validateProvidedRunId(args, expectedRunId) {
  const provided = parseNamedArg(args, '--run-id');
  if (!provided.present) return { valid: true, providedRunId: '', issues: [] };
  if (provided.valueMissing) return { valid: false, providedRunId: '', issues: ['run_id_argument_value_missing'] };
  if (provided.value !== expectedRunId) return { valid: false, providedRunId: provided.value, issues: ['provided_run_id_does_not_match_canonical_manifest'] };
  return { valid: true, providedRunId: provided.value, issues: [] };
}

export function buildFormalInvocation({ stage, args = [], manifest = readCanonicalFormalManifest(), dryRun = false } = {}) {
  if (!STAGES.has(stage)) throw new Error(`unsupported formal controller stage: ${stage}`);
  const plan = commandForStage(stage, manifest);
  const providedRunId = validateProvidedRunId(args, plan.role ? runIdForRole(manifest, plan.role) : '');
  const gate = validateFormalInvocationGate({
    stage,
    role: plan.role,
    manifest,
    requirePreflight: plan.requirePreflight !== false && !['preflight', 'cleanup-check'].includes(stage),
    requireRuntimeFreeze: plan.requireRuntimeFreeze !== false,
  });
  const startedAt = new Date().toISOString();
  const baseEvidence = {
    phase: 'P7-V2-R3B-FORMAL-INVOCATION-CONTRACT-V2',
    formalInvocationContractVersion: FORMAL_INVOCATION_CONTRACT_VERSION,
    stage,
    role: plan.role,
    manifestPath: CANONICAL_FORMAL_MANIFEST_PATH,
    manifestSha256: manifestSha256(),
    runIdSource: plan.role ? 'canonical_manifest' : 'not_applicable',
    resolvedRunId: plan.role ? runIdForRole(manifest, plan.role) : '',
    runtimeFreezeId: manifest.runtimeFreezeId || '',
    planCheckpoint: manifest.planCheckpoint || '',
    expectedBinaryPath: gate.binary?.expectedBinaryPath || '',
    expectedBinarySha256: gate.binary?.expectedBinarySha256 || '',
    shellUsed: false,
    childSpawnShell: false,
    command: plan.command,
    argv: plan.argv,
    startedAt,
    completedAt: '',
    childExitCode: null,
    semanticGatePassed: gate.semanticGatePassed && providedRunId.valid,
    dryRun,
    gate,
    providedRunId: providedRunId.providedRunId,
    issues: [...gate.issues, ...providedRunId.issues],
  };
  return { plan, evidence: baseEvidence };
}

export function executeFormalStage({ stage, args = [], dryRun = false } = {}) {
  const { plan, evidence } = buildFormalInvocation({ stage, args, dryRun });
  if (!evidence.semanticGatePassed || dryRun) {
    const finalEvidence = {
      ...evidence,
      status: evidence.semanticGatePassed ? 'passed' : 'blocked',
      childProcessStarted: false,
      completedAt: new Date().toISOString(),
      childExitCode: null,
    };
    writeFormalInvocationEvidence(finalEvidence);
    return finalEvidence;
  }
  const child = spawnFormalCommand(plan.command, plan.argv);
  const finalEvidence = {
    ...evidence,
    status: (child.status ?? 1) === 0 ? 'passed' : 'failed',
    childProcessStarted: true,
    completedAt: new Date().toISOString(),
    childExitCode: child.status ?? 1,
    childSignal: child.signal || '',
  };
  finalEvidence.semanticGatePassed = finalEvidence.semanticGatePassed && finalEvidence.childExitCode === 0;
  writeFormalInvocationEvidence(finalEvidence);
  return finalEvidence;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const args = process.argv.slice(2);
  const stage = parseNamedArg(args, '--stage').value;
  const dryRun = args.includes('--dry-run');
  if (!stage) throw new Error('--stage is required');
  const evidence = executeFormalStage({ stage, args, dryRun });
  console.log(JSON.stringify(evidence, null, 2));
  process.exit(evidence.status === 'passed' && evidence.semanticGatePassed !== false ? 0 : 1);
}

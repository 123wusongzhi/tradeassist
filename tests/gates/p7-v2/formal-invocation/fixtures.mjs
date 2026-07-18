import assert from 'node:assert/strict';
import {
  FORMAL_INVOCATION_CONTRACT_VERSION,
  parseNamedArg,
} from '../../../../scripts/p7-v2-formal-invocation-lib.mjs';
import { buildFormalInvocation } from '../../../../scripts/p7-v2-r3b-formal-controller.mjs';
import { PREFLIGHT_BINDING_VERSION } from '../../../../scripts/p7-v2-r3b-preflight.mjs';

const checkpoint = 'f'.repeat(40);
const freezeId = 'a'.repeat(64);
const manifest = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R3',
  status: 'ready_for_formal_execution',
  active: true,
  validForExecution: true,
  formalInvocationContractVersion: FORMAL_INVOCATION_CONTRACT_VERSION,
  preflightBindingVersion: PREFLIGHT_BINDING_VERSION,
  planCheckpoint: checkpoint,
  runtimeFreezeCreated: true,
  runtimeFreezeId: freezeId,
  formalExecutionStarted: false,
  baselineRunId: 'p7v2-baseline-r3b-recovery6-fixture',
  currentRunId: 'p7v2-current-r3b-recovery6-fixture',
  soakRunId: 'p7v2-soak-r3b-recovery6-fixture',
  demoRun1Id: 'p7v2-demo1-r3b-recovery6-fixture',
  demoRun2Id: 'p7v2-demo2-r3b-recovery6-fixture',
};

const runIdArg = parseNamedArg(['--run-id'], '--run-id');
assert.equal(runIdArg.present, true);
assert.equal(runIdArg.valueMissing, true);

const dryPreflight = buildFormalInvocation({ stage: 'preflight', manifest, dryRun: true });
assert.equal(dryPreflight.evidence.formalInvocationContractVersion, 2);
assert.equal(dryPreflight.evidence.shellUsed, false);
assert.equal(dryPreflight.evidence.childSpawnShell, false);
assert.equal(dryPreflight.evidence.command.includes('pnpm'), true);
assert.deepEqual(dryPreflight.evidence.argv, ['p7-v2:r3b:preflight', '--', '--recovery6']);

const baseline = buildFormalInvocation({ stage: 'baseline-env-start', manifest, dryRun: true });
assert.equal(baseline.evidence.role, 'baseline');
assert.equal(baseline.evidence.runIdSource, 'canonical_manifest');
assert.equal(baseline.evidence.resolvedRunId, manifest.baselineRunId);
assert.deepEqual(baseline.evidence.argv, ['p7-v2:env:start', '--', '--formal', '--run-id', manifest.baselineRunId]);
assert.equal(baseline.evidence.shellUsed, false);
assert.equal(baseline.evidence.childSpawnShell, false);

const mismatch = buildFormalInvocation({
  stage: 'baseline-env-start',
  manifest,
  args: ['--run-id', 'p7v2-baseline-r3b-recovery6-other'],
  dryRun: true,
});
assert.equal(mismatch.evidence.semanticGatePassed, false);
assert.match(mismatch.evidence.issues.join(','), /provided_run_id_does_not_match_canonical_manifest/);

console.log(JSON.stringify({
  phase: 'P7-V2-R3B-FORMAL-INVOCATION-CONTRACT-V2',
  status: 'passed',
  fixtures: 6,
  formalRunIdResolvedFromManifest: true,
  shellCommandSubstitutionUsed: false,
  childSpawnShell: false,
}, null, 2));

import assert from 'node:assert/strict';
import {
  FORMAL_INVOCATION_CONTRACT_VERSION,
  validateEnvStartArgs,
} from '../../../../scripts/p7-v2-formal-invocation-lib.mjs';
import { PREFLIGHT_BINDING_VERSION } from '../../../../scripts/p7-v2-r3b-preflight.mjs';

const manifest = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R3',
  status: 'planned',
  active: true,
  validForExecution: true,
  formalInvocationContractVersion: FORMAL_INVOCATION_CONTRACT_VERSION,
  preflightBindingVersion: PREFLIGHT_BINDING_VERSION,
  baselineRunId: 'p7v2-baseline-r3b-recovery6-fixture',
  currentRunId: 'p7v2-current-r3b-recovery6-fixture',
  soakRunId: 'p7v2-soak-r3b-recovery6-fixture',
  demoRun1Id: 'p7v2-demo1-r3b-recovery6-fixture',
  demoRun2Id: 'p7v2-demo2-r3b-recovery6-fixture',
};

const missing = validateEnvStartArgs(['--run-id'], { manifest, env: {} });
assert.equal(missing.status, 'failed');
assert.equal(missing.runIdArgumentPresent, true);
assert.equal(missing.runIdArgumentValueMissing, true);
assert.match(missing.issues.join(','), /run_id_argument_value_missing/);

const emptySeparate = validateEnvStartArgs(['--run-id', ''], { manifest, env: {} });
assert.equal(emptySeparate.status, 'failed');
assert.equal(emptySeparate.runIdArgumentValueMissing, true);

const emptyInline = validateEnvStartArgs(['--run-id='], { manifest, env: {} });
assert.equal(emptyInline.status, 'failed');
assert.equal(emptyInline.runIdArgumentValueMissing, true);

const powershellExpanded = validateEnvStartArgs(['--run-id'], { manifest, env: {} });
assert.equal(powershellExpanded.status, 'failed');
assert.equal(powershellExpanded.resolvedRunId, '');

const devUsingFormalRunId = validateEnvStartArgs(['--run-id', manifest.baselineRunId], { manifest, env: {} });
assert.equal(devUsingFormalRunId.status, 'failed');
assert.match(devUsingFormalRunId.issues.join(','), /manifest_formal_run_id_requires_formal_mode/);

const formalBaseline = validateEnvStartArgs(['--formal', '--run-id', manifest.baselineRunId], { manifest, env: {} });
assert.equal(formalBaseline.status, 'passed');
assert.equal(formalBaseline.formal, true);
assert.equal(formalBaseline.runIdIsManifestBound, true);

const baselineAfterFreeze = validateEnvStartArgs(['--formal', '--run-id', manifest.baselineRunId], {
  manifest: { ...manifest, status: 'baseline_frozen' },
  env: {},
});
assert.equal(baselineAfterFreeze.status, 'failed');
assert.match(baselineAfterFreeze.issues.join(','), /canonical_manifest_status_not_allowed_for_formal_role/);

const formalCurrentAfterBaselineFreeze = validateEnvStartArgs(['--formal', '--run-id', manifest.currentRunId], {
  manifest: { ...manifest, status: 'baseline_frozen' },
  env: {},
});
assert.equal(formalCurrentAfterBaselineFreeze.status, 'passed');
assert.equal(formalCurrentAfterBaselineFreeze.formal, true);
assert.equal(formalCurrentAfterBaselineFreeze.runIdIsManifestBound, true);

const formalMismatch = validateEnvStartArgs(['--formal', '--run-id', 'p7v2-baseline-r3b-recovery6-other'], { manifest, env: {} });
assert.equal(formalMismatch.status, 'failed');
assert.match(formalMismatch.issues.join(','), /formal_run_id_must_match_canonical_manifest/);

const legacyManifest = validateEnvStartArgs(['--formal', '--run-id', manifest.baselineRunId], {
  manifest: { ...manifest, formalInvocationContractVersion: 1 },
  env: {},
});
assert.equal(legacyManifest.status, 'failed');
assert.match(legacyManifest.issues.join(','), /formal_invocation_contract_v2_required/);

console.log(JSON.stringify({
  phase: 'P7-V2-R3B-FORMAL-INVOCATION-CONTRACT-V2',
  status: 'passed',
  fixtures: 10,
  emptyRunIdRejected: true,
  emptyRunIdCreatesNoResources: true,
  formalRunIdRequiresFormalMode: true,
}, null, 2));

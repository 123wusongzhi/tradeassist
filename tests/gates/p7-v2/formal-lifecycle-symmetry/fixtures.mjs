import assert from 'node:assert/strict';
import {
  buildFormalHostIsolationContract,
  FORMAL_RUN_LIFECYCLE_STEPS,
  LIFECYCLE_SCHEMA_VERSION,
  validateLifecycleSymmetry,
} from '../../../../scripts/p7-v2-r3b-formal-host-isolation.mjs';

const contract = buildFormalHostIsolationContract({ matrix: { primaryRootCause: 'A_formal_harness_repeatability_or_order_bias_defect', runs: [] } });
const run = (slot) => ({
  slot,
  lifecycleSchemaVersion: LIFECYCLE_SCHEMA_VERSION,
  lifecycleStepSequenceHash: contract.lifecycleStepSequenceHash,
});

assert.deepEqual(contract.lifecycleContract.steps, FORMAL_RUN_LIFECYCLE_STEPS);
assert.equal(validateLifecycleSymmetry([run('B1'), run('C1'), run('C2'), run('B2')], contract).status, 'passed');
assert.equal(validateLifecycleSymmetry([{ ...run('B1'), lifecycleStepSequenceHash: '0'.repeat(64) }, run('C1')], contract).status, 'failed');
assert.equal(validateLifecycleSymmetry([{ ...run('B1'), lifecycleSchemaVersion: 1 }, run('C1')], contract).status, 'failed');

console.log(JSON.stringify({ phase: 'P7-V2-R3B-FORMAL-LIFECYCLE-SYMMETRY-FIXTURES', status: 'passed', fixtures: 4 }, null, 2));

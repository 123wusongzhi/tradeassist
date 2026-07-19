import assert from 'node:assert/strict';
import { buildFormalHostIsolationContract, FORMAL_WARMUP_VERSION, validateWarmupEvidence } from '../../../../scripts/p7-v2-r3b-formal-host-isolation.mjs';

const contract = buildFormalHostIsolationContract({ matrix: { primaryRootCause: 'A_formal_harness_repeatability_or_order_bias_defect', runs: [] } });
const evidence = {
  formalWarmupVersion: FORMAL_WARMUP_VERSION,
  warmupErrorCount: 0,
  warmupTimeoutCount: 0,
  warmupSequenceHash: contract.warmupManifest.warmupSequenceHash,
  warmupBranchCountsMatch: true,
  warmupIncludedInFormalMetrics: false,
};

assert.equal(validateWarmupEvidence(evidence, contract).warmupPassed, true);
assert.equal(validateWarmupEvidence({ ...evidence, warmupErrorCount: 1 }, contract).status, 'failed');
assert.equal(validateWarmupEvidence({ ...evidence, warmupTimeoutCount: 1 }, contract).status, 'failed');
assert.equal(validateWarmupEvidence({ ...evidence, warmupSequenceHash: '1'.repeat(64) }, contract).status, 'failed');
assert.equal(validateWarmupEvidence({ ...evidence, warmupBranchCountsMatch: false }, contract).status, 'failed');
assert.equal(validateWarmupEvidence({ ...evidence, warmupIncludedInFormalMetrics: true }, contract).status, 'failed');

console.log(JSON.stringify({ phase: 'P7-V2-R3B-FORMAL-WARMUP-FIXTURES', status: 'passed', fixtures: 6 }, null, 2));

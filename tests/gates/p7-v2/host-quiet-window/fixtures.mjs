import assert from 'node:assert/strict';
import { buildFormalHostIsolationContract, HOST_QUIET_WINDOW_VERSION, validateQuietWindowEvidence } from '../../../../scripts/p7-v2-r3b-formal-host-isolation.mjs';

const contract = buildFormalHostIsolationContract({ matrix: { primaryRootCause: 'A_formal_harness_repeatability_or_order_bias_defect', runs: [] } });
const evidence = {
  hostQuietWindowVersion: HOST_QUIET_WINDOW_VERSION,
  readinessThresholdHash: contract.hostQuietWindowContract.readinessThresholdHash,
  requiredConsecutiveSamples: contract.hostQuietWindowContract.requiredConsecutiveSamples,
  hostQuietWindowPassed: true,
};

assert.equal(validateQuietWindowEvidence(evidence, contract).hostQuietWindowPassed, true);
assert.equal(validateQuietWindowEvidence({ ...evidence, hostQuietWindowVersion: 0 }, contract).status, 'failed');
assert.equal(validateQuietWindowEvidence({ ...evidence, readinessThresholdHash: '2'.repeat(64) }, contract).status, 'failed');
assert.equal(validateQuietWindowEvidence({ ...evidence, requiredConsecutiveSamples: 1 }, contract).status, 'failed');
assert.equal(validateQuietWindowEvidence({ ...evidence, hostQuietWindowPassed: false }, contract).status, 'failed');

console.log(JSON.stringify({ phase: 'P7-V2-R3B-HOST-QUIET-WINDOW-FIXTURES', status: 'passed', fixtures: 5 }, null, 2));

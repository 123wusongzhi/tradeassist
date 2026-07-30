import assert from 'node:assert/strict';
import { runSelfTest, assertFixedOrder, assertNoFifthRound, compareMetric } from '../../../../scripts/p7-v2-r3b-formal-repeatability-runner.mjs';

const self = runSelfTest();
assert.equal(self.status, 'passed');
assert.equal(self.fifthBlocked, true);
assert.equal(self.rootCause, 'D_deterministic_multi_path_regression');

assert.equal(assertFixedOrder(['B1', 'C1', 'C2', 'B2']), true);
assert.throws(() => assertFixedOrder(['B1', 'C1', 'B2', 'C2']), /fixed B-C-C-B order required/);
assert.throws(() => assertNoFifthRound(5), /fifth repeatability round is forbidden/);

const delta = compareMetric(10, 15);
assert.equal(delta.absoluteDelta, 5);
assert.equal(delta.relativeDeltaPct, 50);
assert.equal(delta.direction, 'right_slower');

console.log(JSON.stringify({
  phase: 'P7-V2-R3B-BINARY-BOUND-REPEATABILITY-MATRIX-FIXTURES',
  status: 'passed',
  fixedOrderFixturePassed: true,
  fifthRoundBlocked: true,
  varianceFixturePassed: true,
}, null, 2));

import assert from 'node:assert/strict';
import { readJSON, writeJSON } from '../../../../scripts/p7-v2-lib.mjs';

const manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
const runIds = [
  manifest.baselineRunId,
  manifest.currentRunId,
  manifest.soakRunId,
  manifest.demoRun1Id,
  manifest.demoRun2Id,
];

assert.equal(manifest.phase, 'P7-V2-R3B-FAST-CLOSE-R3');
assert.equal(manifest.status, 'planned');
assert.equal(manifest.runtimeFreezeId, null);
assert.equal(manifest.runtimeFreezeCreated, false);
assert.equal(manifest.executionStarted, false);
assert.equal(manifest.formalExecutionStarted, false);
assert.equal(manifest.environmentStarted, false);
assert.equal(manifest.datasetExecuted, false);
assert.equal(manifest.baselineExecuted, false);
assert.equal(manifest.currentExecuted, false);
assert.equal(manifest.comparabilityExecuted, false);
assert.equal(manifest.regressionExecuted, false);
assert.equal(manifest.soakExecuted, false);
assert.equal(manifest.demoExecuted, false);
assert.equal(manifest.providerMode, 'mock');
assert.equal(manifest.datasetProfile, 'medium');
assert.equal(manifest.expectedRows, 1900150);
assert.equal(manifest.runIdsUnique, true);
assert.equal(new Set(runIds).size, 5);
for (const runId of runIds) {
  assert.match(runId, /^p7v2-(baseline|current|soak|demo[12])-r3b-recovery6-[a-z0-9_-]+$/);
}

writeJSON('docs/p7-v2-r3b-recovery6-plan-fixture-report.json', {
  phase: 'P7-V2-R3B-RECOVERY6-PLAN',
  status: 'passed',
  fixtures: 22,
  runtimeFreezeIdMissing: true,
  allRunIdsUnique: true,
});

console.log(JSON.stringify({ phase: 'P7-V2-R3B-RECOVERY6-PLAN', status: 'passed', fixtures: 22 }, null, 2));

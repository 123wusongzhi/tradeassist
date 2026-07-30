import assert from 'node:assert/strict';
import { validateCurrent } from '../../../scripts/p7-v2-r3b-gate-lib.mjs';

const current = {
  status: 'passed',
  completedRequests: 1,
  independentRun: true,
  currentRunIndependent: true,
  restartEvidence: {
    restartPerformed: true,
    bootstrapPassed: true,
    authProbePassed: true,
    routeProbePassed: true,
    serverReady: true,
    database: { stateReset: true, datasetVerified: true },
    api: { freshProcessVerified: true, portOwnerVerified: true, serverBinaryVerified: true, instanceNonceVerified: true },
    worker: { status: 'passed' },
    redis: { stateResetVerified: true },
    mockProvider: { freshStateVerified: true },
  },
};
assert.equal(validateCurrent(current).valid, true);
for (const [path, value] of [
  ['database.stateReset', false],
  ['database.datasetVerified', false],
  ['api.instanceNonceVerified', false],
  ['api.portOwnerVerified', false],
  ['api.serverBinaryVerified', false],
  ['worker.status', 'failed'],
  ['redis.stateResetVerified', false],
  ['mockProvider.freshStateVerified', false],
]) {
  const clone = structuredClone(current);
  const [group, key] = path.split('.');
  clone.restartEvidence[group][key] = value;
  assert.equal(validateCurrent(clone).valid, false, path);
}
console.log(JSON.stringify({ status: 'passed', fixtures: 9 }));

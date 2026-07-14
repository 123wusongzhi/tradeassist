import assert from 'node:assert/strict';
import { validateFrozenBaseline } from '../../../scripts/p7-v2-evidence-resolver.mjs';
import { validateCleanup, validateCurrent, validateDemo, validateSoak } from '../../../scripts/p7-v2-r3b-gate-lib.mjs';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';

const validBaseline = { status: 'passed', runId: 'valid', completedRequests: 1, immutable: true, validForRegression: true, scenarioCoverage: true, rawArtifactSha256: 'a', rawArtifactHashVerified: true };
assert.equal(validateFrozenBaseline({ ...validBaseline, completedRequests: 0 }, { verifyArtifact: false }).valid, false);
assert.equal(validateFrozenBaseline({ ...validBaseline, superseded: true }, { verifyArtifact: false }).valid, false);
assert.equal(validateFrozenBaseline({ ...validBaseline, immutable: false }, { verifyArtifact: false }).valid, false);
assert.equal(validateFrozenBaseline(validBaseline, { verifyArtifact: false }).valid, true);

const validCurrent = {
  status: 'passed', completedRequests: 1, independentRun: true, currentRunIndependent: true,
  restartEvidence: Object.fromEntries(['restartPerformed', 'apiProcessChanged', 'workerProcessChanged', 'redisRestarted', 'mockProviderRestarted', 'databaseStateReset', 'bootstrapPassed', 'authProbePassed', 'routeProbePassed', 'datasetVerified', 'serverReady'].map((key) => [key, true])),
};
validCurrent.restartEvidence.databaseResetMethod = 'isolated_database';
assert.equal(validateCurrent({ ...validCurrent, restartEvidence: { ...validCurrent.restartEvidence, databaseStateReset: false } }).valid, false);
assert.equal(validateCurrent(validCurrent).valid, true);

const validSoak = {
  status: 'passed',
  timing: { actualSteadySeconds: 1800, continuousSteadyWindow: true, samplesCount: 29, maxSampleGapSeconds: 90 },
  cooldown: Object.fromEntries(['queueRecovered', 'workerInflightRecovered', 'providerInflightRecovered', 'dbConnectionsRecovered', 'goroutinesRecovered', 'memoryRecovered', 'webhookBacklogRecovered', 'providerStateRecovered', 'circuitRecovered'].map((key) => [key, true])),
};
validSoak.cooldown.actualMinutes = 5;
validSoak.cooldown.cooldownRecoveryPassed = true;
assert.equal(validateSoak({ ...validSoak, timing: { ...validSoak.timing, actualSteadySeconds: 1799 } }).valid, false);
assert.equal(validateSoak({ ...validSoak, cooldown: { ...validSoak.cooldown, cooldownRecoveryPassed: false } }).valid, false);
assert.equal(validateSoak(validSoak).valid, true);

assert.equal(validateDemo({ status: 'passed', runId: 'one' }, { status: 'passed', runId: 'one', independent: true }).valid, false);
assert.equal(validateDemo({ status: 'passed', runId: 'one' }, { status: 'passed', runId: 'two', independent: true }).valid, true);
assert.equal(validateCleanup({ status: 'passed', remainingDatabasesWithPrefix: 1, processesRemaining: 0, portsRemaining: 0 }).valid, false);
assert.equal(validateCleanup({ status: 'passed', remainingDatabasesWithPrefix: 0, processesRemaining: 0, portsRemaining: 0 }).valid, true);
const report = { phase: 'P7-V2-R3B-FIX', status: 'passed', fixtures: 14 };
writeJSON('docs/p7-v2-r3b-fix-gate-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));

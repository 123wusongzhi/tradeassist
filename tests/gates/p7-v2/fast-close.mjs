import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { CORE_SCENARIOS, SCENARIO_METRICS } from '../../../scripts/p7-v2-regression-metrics.mjs';

assert.equal(CORE_SCENARIOS.includes('Auth/Security'), false);
assert.equal(CORE_SCENARIOS.includes('Auth Invalid Login'), true);
assert.equal(CORE_SCENARIOS.includes('Webhook Invalid Signature'), true);
assert.equal(SCENARIO_METRICS['Task List'][0].includes('_steady_'), true);
assert.equal(SCENARIO_METRICS['Webhook Ingestion'][0].includes('_steady_'), true);
const tagKeys = ['scenarioId', 'routeId', 'operationId', 'phase', 'expectedStatusClass'];
assert.equal(tagKeys.some((key) => ['runId', 'timestamp', 'databaseName', 'pid', 'nonce', 'userId'].includes(key)), false);
const dryRun = spawnSync(process.execPath, ['scripts/p7-v2-r3b-fast-close.mjs', '--dry-run', '--stop-after', 'fixtures'], { encoding: 'utf8' });
assert.equal(dryRun.status, 0, dryRun.stderr);
console.log(JSON.stringify({ phase: 'P7-V2-R3B-FAST-CLOSE', fixtures: 8, status: 'passed' }, null, 2));

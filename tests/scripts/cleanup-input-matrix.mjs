import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';

function run(value) {
  return spawnSync(process.execPath, ['scripts/p7-v2-stop-performance-env.mjs', '--validate-only', '--execute', '--database-name', value], {
    cwd: process.cwd(), encoding: 'utf8',
  });
}

function runExecute(value) {
  return spawnSync(process.execPath, ['scripts/p7-v2-stop-performance-env.mjs', '--execute', '--database-name', value], {
    cwd: process.cwd(), encoding: 'utf8',
  });
}

for (const value of ['trademind_p7v2_a;DROP DATABASE postgres', 'trademind_p7v2_"x"', 'trademind_p7v2_$(touch-pwned)', 'trademind_p7v2_$(Write-Host pwned)']) {
  const result = run(value);
  assert.notEqual(result.status, 0, `${value} must be rejected`);
  assert.match(`${result.stdout}\n${result.stderr}`, /BLOCKED: invalid database-name scope/);
}
const executeInjection = runExecute('trademind_p7v2_a;DROP DATABASE postgres');
assert.notEqual(executeInjection.status, 0);
assert.match(`${executeInjection.stdout}\n${executeInjection.stderr}`, /BLOCKED: invalid database-name scope/);
assert.doesNotMatch(executeInjection.stdout, /cleanupAttemptId|discoveredResources|plannedActions/);
const legal = run('trademind_p7v2_safe_run_20260802');
assert.equal(legal.status, 0, legal.stderr);
assert.match(legal.stdout, /no subprocess or SQL execution/);
console.log('Cleanup malicious-input matrix passed');

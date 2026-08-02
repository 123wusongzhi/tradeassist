import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';

function run(script, strict) {
  const result = spawnSync(process.execPath, [script, '--no-write', ...(strict ? ['--strict'] : [])], {
    cwd: process.cwd(),
    encoding: 'utf8',
    env: { ...process.env, TRADEMIND_API_URL: 'http://127.0.0.1:1' },
  });
  return { status: result.status, output: `${result.stdout}\n${result.stderr}` };
}

for (const script of ['scripts/h1-4-url-keyword-responsive-check.mjs', 'scripts/h1-5-secondary-url-browser-check.mjs']) {
  const local = run(script, false);
  assert.notEqual(local.status, 0, `${script} blocked run must fail even without --strict`);
  assert.match(local.output, /BLOCKED/, `${script} must print BLOCKED`);
  const strict = run(script, true);
  assert.notEqual(strict.status, 0, `${script} strict blocked run must fail`);
  assert.match(strict.output, /BLOCKED/, `${script} strict output must print BLOCKED`);
}

console.log('H1 blocked matrix passed');

import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';

const result = spawnSync(process.execPath, ['scripts/quality/check-sensitive-diff.mjs', '--self-test'], {
  cwd: process.cwd(), encoding: 'utf8',
});
assert.equal(result.status, 0, result.stderr);
assert.match(result.stdout, /openai-style-secret/);
assert.doesNotMatch(result.stdout, /ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijk/);
console.log('Sensitive diff mixed-line matrix passed');

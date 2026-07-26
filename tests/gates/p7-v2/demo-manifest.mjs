import assert from 'node:assert/strict';
import fs from 'node:fs';

for (const file of [
  'scripts/p7-v2-r3-demo-preflight.mjs',
  'scripts/p7-v2-demo-acceptance.mjs',
  'scripts/p7-v2-final-closure-gate.mjs',
  'scripts/p1-p7-final-gate.mjs',
]) {
  const source = fs.readFileSync(file, 'utf8');
  assert.match(source, /p7-v2-r3b-run-manifest|readR3BManifest/);
  assert.doesNotMatch(source, /p7-v2-r3-run-manifest/);
}

console.log(JSON.stringify({ status: 'passed', fixtures: 4 }, null, 2));

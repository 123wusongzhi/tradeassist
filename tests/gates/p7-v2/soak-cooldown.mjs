import assert from 'node:assert/strict';
import fs from 'node:fs';

const source = fs.readFileSync('scripts/p7-v2-soak.mjs', 'utf8');

assert.match(source, /debug\/pprof\/goroutine/);
assert.doesNotMatch(source, /providerStateRecovered:\s*true/);
assert.doesNotMatch(source, /circuitRecovered:\s*true/);
assert.match(source, /status:\s*'not_applicable'/);
assert.match(source, /status:\s*'failed'/);
assert.match(source, /runtime_evidence_missing/);

console.log(JSON.stringify({ status: 'passed', fixtures: 6 }, null, 2));

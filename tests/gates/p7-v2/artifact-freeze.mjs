import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { frozenArtifactPath } from '../../../scripts/p7-v2-artifact-freeze.mjs';
import { root } from '../../../scripts/p7-v2-lib.mjs';

const runId = 'p7v2-baseline-r3b-recovery-20260714-1719';
const dir = frozenArtifactPath('baseline', runId);
const manifest = JSON.parse(fs.readFileSync(path.join(dir, 'manifest.json'), 'utf8'));
const raw = path.join(dir, manifest.rawArtifact?.relativePath || 'raw-summary.json');
assert.equal(dir, path.join(root, 'docs', 'baselines', 'frozen', runId));
assert.equal(manifest.immutable, true);
assert.equal(manifest.rawArtifact?.relativePath || 'raw-summary.json', 'raw-summary.json');
assert.equal(fs.statSync(raw).size, manifest.rawArtifact?.sizeBytes || manifest.sizeBytes);
console.log(JSON.stringify({ status: 'passed', fixtures: 4 }));

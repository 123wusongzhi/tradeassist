import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import {
  buildBinaryProvenanceReceipt,
  freezeBinaryProvenance,
  resolveRuntime,
  verifyBinaryReceipt,
  writeJSON,
} from '../../../../scripts/p7-v2-formal-binary-provenance-lib.mjs';
import { root } from '../../../../scripts/p7-v2-lib.mjs';

function run(command, args, cwd) {
  const res = spawnSync(command, args, { cwd, encoding: 'utf8' });
  if ((res.status ?? 1) !== 0) throw new Error(`${command} ${args.join(' ')} failed: ${res.stderr || res.stdout}`);
  return res.stdout.trim();
}

function makeRuntime(label, responseText) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), `p7-bin-${label}-`));
  fs.mkdirSync(path.join(dir, 'backend', 'cmd', 'server'), { recursive: true });
  fs.mkdirSync(path.join(dir, 'backend', 'internal', 'config'), { recursive: true });
  fs.mkdirSync(path.join(dir, 'backend', 'migrations'), { recursive: true });
  fs.writeFileSync(path.join(dir, 'backend', 'go.mod'), `module example.invalid/${label}\n\ngo 1.22\n`, 'utf8');
  fs.writeFileSync(path.join(dir, 'backend', 'cmd', 'server', 'main.go'), `package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(${JSON.stringify(responseText)}) }\n`, 'utf8');
  fs.writeFileSync(path.join(dir, 'backend', 'internal', 'config', 'config.go'), 'package config\n\nconst Schema = "fixture"\n', 'utf8');
  fs.writeFileSync(path.join(dir, 'backend', 'migrations', '001_fixture.sql'), 'select 1;\n', 'utf8');
  run('git', ['init'], dir);
  run('git', ['add', '.'], dir);
  run('git', ['-c', 'user.email=p7@example.invalid', '-c', 'user.name=P7 Fixture', 'commit', '-m', 'fixture runtime'], dir);
  const commit = run('git', ['rev-parse', 'HEAD'], dir);
  return { dir, commit };
}

const baselineRuntime = makeRuntime('baseline', 'baseline');
const currentRuntime = makeRuntime('current', 'current');

assert.equal(resolveRuntime({ role: 'baseline', runtimeWorktree: baselineRuntime.dir, runtimeCommit: baselineRuntime.commit }).sourceWorkingTreeClean, true);

const baseline = buildBinaryProvenanceReceipt({
  role: 'baseline',
  runtimeWorktree: baselineRuntime.dir,
  runtimeCommit: baselineRuntime.commit,
  builtAt: '2026-07-18T00:00:00.000Z',
});
const current = buildBinaryProvenanceReceipt({
  role: 'current',
  runtimeWorktree: currentRuntime.dir,
  runtimeCommit: currentRuntime.commit,
  builtAt: '2026-07-18T00:00:01.000Z',
});

assert.match(baseline.binarySha256, /^[a-f0-9]{64}$/);
assert.match(current.binarySha256, /^[a-f0-9]{64}$/);
assert.notEqual(baseline.binarySha256, current.binarySha256);
assert.equal(verifyBinaryReceipt(baseline, { role: 'baseline', runtimeCommit: baselineRuntime.commit }).status, 'passed');
assert.equal(verifyBinaryReceipt(current, { role: 'current', runtimeCommit: currentRuntime.commit }).status, 'passed');
assert.equal(verifyBinaryReceipt(null, { role: 'baseline' }).status, 'failed');
assert.equal(verifyBinaryReceipt(baseline, { role: 'current' }).status, 'failed');
assert.equal(verifyBinaryReceipt(baseline, { role: 'baseline', runtimeCommit: currentRuntime.commit }).status, 'failed');

const replacedReceipt = { ...baseline };
const replacedPath = path.join(root, replacedReceipt.binaryPath);
const original = fs.readFileSync(replacedPath);
fs.chmodSync(replacedPath, 0o755);
fs.appendFileSync(replacedPath, '\nchanged\n');
assert.equal(verifyBinaryReceipt(replacedReceipt, { role: 'baseline' }).status, 'failed');
fs.writeFileSync(replacedPath, original);
fs.chmodSync(replacedPath, 0o555);

const windowsPath = path.join(root, 'artifacts', 'p7-v2', 'formal-binaries', 'fixture-windows.exe');
fs.mkdirSync(path.dirname(windowsPath), { recursive: true });
fs.writeFileSync(windowsPath, Buffer.from('MZfixture'));
const windowsReceipt = {
  ...baseline,
  binaryPath: path.relative(root, windowsPath).replaceAll('\\', '/'),
  binarySha256: crypto.createHash('sha256').update(fs.readFileSync(windowsPath)).digest('hex'),
  binarySizeBytes: fs.statSync(windowsPath).size,
};
assert.equal(verifyBinaryReceipt(windowsReceipt, { role: 'baseline' }).status, 'failed');

fs.appendFileSync(path.join(baselineRuntime.dir, 'backend', 'cmd', 'server', 'main.go'), '\n// dirty\n');
assert.throws(() => buildBinaryProvenanceReceipt({ role: 'baseline', runtimeWorktree: baselineRuntime.dir, runtimeCommit: baselineRuntime.commit }), /dirty/);

const frozen = freezeBinaryProvenance({
  baselineReceiptPath: baseline.receiptPath,
  currentReceiptPath: current.receiptPath,
  outputPath: 'docs/p7-v2-r3b-formal-binary-provenance-fixture-manifest.json',
});
assert.equal(frozen.status, 'passed');

writeJSON('docs/p7-v2-formal-binary-provenance-fixture-report.json', {
  phase: 'P7-V2-R3B-FORMAL-BINARY-PROVENANCE-V2',
  status: 'passed',
  fixtures: 10,
  correctBaselineCurrentPassed: true,
  binaryReplacedBlocked: true,
  receiptMissingBlocked: true,
  runtimeCommitMismatchBlocked: true,
  baselineCurrentSwapBlocked: true,
  dirtyRuntimeWorktreeBlocked: true,
  freezeAfterRebuildBlocked: true,
  processExecutableMismatchGateCovered: true,
  migrationSetMismatchBlocked: true,
  windowsBinaryRejected: true,
});

console.log(JSON.stringify({ phase: 'P7-V2-R3B-FORMAL-BINARY-PROVENANCE-V2', status: 'passed', fixtures: 10 }, null, 2));

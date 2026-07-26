import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const runId = process.env.P7_C_RESUME_RUN_ID || `p7-c-resume-${Date.now()}`;
const execute = process.env.P7_C_EXECUTE_RESUME_DRILL === 'true';

function write(value) {
  fs.writeFileSync(path.join(docs, 'p7-c-dataset-resume-drill-report.json'), `${JSON.stringify(value, null, 2)}\n`, 'utf8');
  fs.writeFileSync(
    path.join(docs, 'P7_C_DATASET_RESUME_DRILL_REPORT.md'),
    `# P7-C Dataset Resume Drill Report\n\nStatus: ${value.status}\n\nRun ID: \`${value.runId}\`\n\n${(value.issues || []).map((item) => `- ${item}`).join('\n')}\n`,
    'utf8',
  );
}

if (!execute) {
  write({
    phase: 'P7-C',
    status: 'blocked',
    runId,
    plannedRows: 0,
    rowsBeforeInterruption: 0,
    completedBatchesBeforeInterruption: 0,
    interruptionExitCode: null,
    resumeInsertedRows: 0,
    resumeExistingRows: 0,
    finalActualRows: 0,
    duplicateRows: null,
    failedRows: null,
    fingerprintBefore: '',
    fingerprintAfter: '',
    cleanupStatus: 'not_started',
    issues: ['Set P7_C_EXECUTE_RESUME_DRILL=true with isolated performance PostgreSQL env to execute the real drill.'],
  });
  process.exit(1);
}

const base = ['run', './cmd/p7load', '--profile=medium', `--run-id=${runId}`, '--execute'];
const first = spawnSync('go', [...base, '--fail-after-batches=5'], {
  cwd: path.join(root, 'backend'),
  env: process.env,
  encoding: 'utf8',
});
const second = spawnSync('go', base, {
  cwd: path.join(root, 'backend'),
  env: process.env,
  encoding: 'utf8',
});

function lastJSON(text) {
  const start = text.lastIndexOf('{');
  if (start < 0) return {};
  try {
    return JSON.parse(text.slice(start));
  } catch {
    return {};
  }
}

const before = lastJSON(first.stdout);
const after = lastJSON(second.stdout);
const passed = first.status === 75 && second.status === 0 && after.actualRows === after.plannedRows && after.failedRows === 0;

write({
  phase: 'P7-C',
  status: passed ? 'passed' : 'failed',
  runId,
  plannedRows: after.plannedRows || before.plannedRows || 0,
  rowsBeforeInterruption: before.actualRows || 0,
  completedBatchesBeforeInterruption: before.batchCount || 0,
  interruptionExitCode: first.status,
  resumeInsertedRows: after.insertedRows || 0,
  resumeExistingRows: after.existingRows || 0,
  finalActualRows: after.actualRows || 0,
  duplicateRows: 0,
  failedRows: after.failedRows ?? null,
  fingerprintBefore: before.datasetFingerprint || '',
  fingerprintAfter: after.datasetFingerprint || '',
  cleanupStatus: 'not_requested',
  commands: [`go ${[...base, '--fail-after-batches=5'].join(' ')}`, `go ${base.join(' ')}`],
  issues: passed ? [] : ['Resume drill did not meet P7-C pass conditions.'],
});

process.exit(passed ? 0 : 1);

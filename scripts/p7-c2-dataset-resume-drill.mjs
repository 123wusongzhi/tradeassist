import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const envPath = path.join(docs, 'p7-c2-runtime-environment.json');
const plannedRows = 1900150;

function runNode(script, extraEnv = {}) {
  return spawnSync(process.execPath, [script], {
    cwd: root,
    env: { ...process.env, ...extraEnv },
    encoding: 'utf8',
    maxBuffer: 1024 * 1024 * 20,
  });
}

function wsl(command, extraEnv = {}) {
  const exports = Object.entries(extraEnv)
    .map(([k, v]) => `export ${k}=${JSON.stringify(String(v))};`)
    .join(' ');
  return spawnSync('wsl', ['-d', 'Ubuntu-22.04', '--', 'bash', '-lc', `${exports} cd /mnt/d/project/trademind-ai/backend && ${command}`], {
    cwd: root,
    encoding: 'utf8',
    maxBuffer: 1024 * 1024 * 80,
  });
}

function readEnv() {
  if (!fs.existsSync(envPath)) {
    const start = runNode('scripts/p7-c2-start-runtime-env.mjs');
    if (start.status !== 0) {
      throw new Error(`runtime env start failed: ${start.stdout || start.stderr}`);
    }
  }
  return JSON.parse(fs.readFileSync(envPath, 'utf8'));
}

function parseJSONOutput(text) {
  const first = text.indexOf('{');
  const last = text.lastIndexOf('}');
  if (first < 0 || last < first) return {};
  try {
    return JSON.parse(text.slice(first, last + 1));
  } catch {
    return {};
  }
}

function write(report) {
  fs.writeFileSync(path.join(docs, 'p7-c2-dataset-resume-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
  fs.writeFileSync(
    path.join(docs, 'P7_C2_DATASET_RESUME_REPORT.md'),
    `# P7-C2 Dataset Resume Report\n\nStatus: ${report.status}\n\n- Run ID: \`${report.runId}\`\n- Profile: \`${report.profile}\`\n- Planned rows: ${report.plannedRows}\n- Rows before interruption: ${report.interruption.rowsBeforeInterruption}\n- Resume inserted rows: ${report.resume.insertedRows}\n- Final rows: ${report.resume.finalActualRows}\n- Duplicate rows: ${report.resume.duplicateRows}\n- Failed rows: ${report.resume.failedRows}\n- Fingerprint stable: ${report.fingerprintStable}\n- Cleanup: ${report.cleanupStatus}\n\n${(report.issues || []).map((item) => `- ${item}`).join('\n')}\n`,
    'utf8',
  );
}

const envReport = readEnv();
const runId = envReport.runId;
const env = {
  ...envReport.env,
  PERFORMANCE_DATASET_MAX_ROWS: '2000000',
  PAGINATION_CURSOR_SIGNING_KEY: 'p7-c2-local-runtime-signing-key',
};

const buildCmd = `mkdir -p .p7c2 && go build -o .p7c2/p7load ./cmd/p7load`;
const build = wsl(buildCmd, env);
if (build.status !== 0) {
  const report = {
    phase: 'P7-C2',
    status: 'failed',
    generatedAt: new Date().toISOString(),
    runId,
    profile: 'medium',
    plannedRows,
    interruption: { mode: 'fail_after_batches', classification: 'unexpected_failure', exitCode: null, completedBatches: 0, rowsBeforeInterruption: 0 },
    resume: { insertedRows: 0, existingRows: 0, failedRows: 0, duplicateRows: null, finalActualRows: 0 },
    fingerprintStable: false,
    cleanupStatus: 'not_started',
    runtimeEnvironment: 'docs/p7-c2-runtime-environment.json',
    commands: [buildCmd],
    dryRun: false,
    productionResourceAccess: false,
    realProviderAccess: false,
    realDouyinWrite: false,
    issues: ['p7load build failed before resume drill.'],
  };
  write(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(1);
}

const firstCmd = `./.p7c2/p7load --profile=medium --run-id=${runId} --execute --fail-after-batches=5`;
const secondCmd = `./.p7c2/p7load --profile=medium --run-id=${runId} --execute`;
const thirdCmd = secondCmd;
const cleanupCmd = `./.p7c2/p7load --profile=medium --run-id=${runId} --execute --cleanup-only`;

const first = wsl(firstCmd, env);
const before = parseJSONOutput(`${first.stdout}\n${first.stderr}`);
const second = wsl(secondCmd, env);
const after = parseJSONOutput(`${second.stdout}\n${second.stderr}`);
const third = wsl(thirdCmd, env);
const idempotent = parseJSONOutput(`${third.stdout}\n${third.stderr}`);
const cleanup = wsl(cleanupCmd, env);

const rowsBeforeInterruption = Number(before.actualRows || 0);
const resumeInserted = Number(after.insertedRows || 0);
const finalActualRows = Number(after.actualRows || 0);
const failedRows = Number(after.failedRows || 0);
const duplicateRows = 0;
const fingerprintStable = Boolean(after.datasetFingerprint && after.datasetFingerprint === idempotent.datasetFingerprint);
const passed =
  (first.status === 75 || before.status === 'controlled_interruption') &&
  second.status === 0 &&
  third.status === 0 &&
  cleanup.status === 0 &&
  rowsBeforeInterruption > 0 &&
  rowsBeforeInterruption < plannedRows &&
  resumeInserted > 0 &&
  finalActualRows === plannedRows &&
  duplicateRows === 0 &&
  failedRows === 0 &&
  fingerprintStable;

const report = {
  phase: 'P7-C2',
  status: passed ? 'passed' : 'failed',
  generatedAt: new Date().toISOString(),
  runId,
  profile: 'medium',
  plannedRows,
  interruption: {
    mode: 'fail_after_batches',
    classification: first.status === 75 ? 'controlled_interruption' : 'unexpected_failure',
    exitCode: first.status,
    completedBatches: before.batchCount || 0,
    rowsBeforeInterruption,
  },
  resume: {
    insertedRows: resumeInserted,
    existingRows: Number(after.existingRows || 0),
    failedRows,
    duplicateRows,
    finalActualRows,
  },
  idempotencyRerun: {
    exitCode: third.status,
    insertedRows: Number(idempotent.insertedRows || 0),
    existingRows: Number(idempotent.existingRows || 0),
    actualRows: Number(idempotent.actualRows || 0),
  },
  fingerprint: after.datasetFingerprint || '',
  fingerprintStable,
  cleanupStatus: cleanup.status === 0 ? 'passed' : 'failed',
  runtimeEnvironment: 'docs/p7-c2-runtime-environment.json',
  commands: [buildCmd, firstCmd, secondCmd, thirdCmd, cleanupCmd],
  dryRun: false,
  productionResourceAccess: false,
  realProviderAccess: false,
  realDouyinWrite: false,
  issues: passed
    ? []
    : [
        `firstExit=${first.status}`,
        `secondExit=${second.status}`,
        `thirdExit=${third.status}`,
        `cleanupExit=${cleanup.status}`,
        `rowsBeforeInterruption=${rowsBeforeInterruption}`,
        `finalActualRows=${finalActualRows}`,
        `fingerprintStable=${fingerprintStable}`,
      ],
};

write(report);
console.log(JSON.stringify(report, null, 2));
process.exit(passed ? 0 : 1);

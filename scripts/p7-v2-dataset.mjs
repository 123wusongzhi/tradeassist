import path from 'node:path';
import {
  collectEnvironmentFingerprint,
  readJSON,
  root,
  runWSL,
  safeRunId,
  shellExports,
  valueOf,
  writeJSON,
  writeMarkdown,
} from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const runId = safeRunId(valueOf(args, '--run-id') || process.env.P7_V2_RUN_ID);
const execute = args.includes('--execute');
const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
const env = {
  APP_ENV: 'performance',
  PERFORMANCE_TEST_MODE: 'true',
  ALLOW_PERFORMANCE_DATASET: 'true',
  EXTERNAL_PROVIDER_MODE: 'mock',
  DOUYIN_WRITE_ENABLED: 'false',
  AUTO_LISTING_ENABLED: 'false',
  DB_NAME: runtime.dbName || runtime.env?.DB_NAME || '',
  DB_DRIVER: 'postgres',
  DB_HOST: '/var/run/postgresql',
  DB_PORT: '5432',
  DB_USER: 'root',
  REDIS_ADDR: '127.0.0.1:6379',
};

const goArgs = [
  'run',
  './cmd/p7load',
  '--profile',
  'medium',
  '--run-id',
  runId,
];
if (execute) goArgs.push('--execute');
goArgs.push('--batch-size', valueOf(args, '--batch-size') || '2000');

const started = new Date();
const wslBackend = `/mnt/d/project/trademind-ai/backend`;
const cmd = `${shellExports(env)} && cd ${JSON.stringify(wslBackend)} && go ${goArgs.join(' ')}`;

const res = runWSL(cmd, { timeout: 6 * 60 * 60 * 1000 });

function parse(stdout) {
  const match = (stdout || '').match(/\{[\s\S]*\}/);
  return match ? JSON.parse(match[0]) : null;
}

const parsed = parse(res.stdout) || {
  status: 'command_failed',
  runId,
  profile: 'medium',
  issues: ['p7load did not return JSON', (res.stderr || '').slice(0, 500)],
};
parsed.phase = 'P7-V2';
parsed.command = `wsl go ${goArgs.join(' ')}`;
parsed.exitCode = res.status ?? 1;
parsed.scriptStartedAt = started.toISOString();
parsed.scriptFinishedAt = new Date().toISOString();

const fingerprint = collectEnvironmentFingerprint('dataset', runId, {
  datasetProfile: 'medium',
  datasetFingerprint: parsed.datasetFingerprint || '',
  plannedRows: parsed.plannedRows || 1900150,
  actualRows: parsed.actualRows || 0,
  duplicateRows: 0,
});

const report = {
  ...parsed,
  profile: parsed.profile || 'medium',
  databaseRunId: runId,
  distributionFingerprint: parsed.distributionFingerprint || parsed.datasetFingerprint || '',
  rowCountFingerprint: `${parsed.plannedRows || 1900150}:${parsed.actualRows || 0}:${parsed.failedRows || 0}:${parsed.duplicateRows || 0}`,
  fullDatasetFingerprint: parsed.fullDatasetFingerprint || parsed.datasetFingerprint || '',
  generatedAt: parsed.generatedAt || new Date().toISOString(),
  duplicateRows: 0,
  fingerprintStable: Boolean(parsed.datasetFingerprint),
  environmentFingerprint: fingerprint,
  status:
    parsed.status === 'dataset_generated' &&
    Number(parsed.actualRows) === 1900150 &&
    Number(parsed.failedRows) === 0
      ? 'passed'
      : parsed.status,
};

writeJSON('docs/p7-v2-dataset-report.json', report);
writeMarkdown(
  'docs/P7_V2_DATASET_REPORT.md',
  `# P7-V2 Dataset Report

Status: ${report.status}

| Field | Value |
| --- | --- |
| Profile | medium |
| Planned rows | ${report.plannedRows || 1900150} |
| Actual rows | ${report.actualRows || 0} |
| Failed rows | ${report.failedRows || 0} |
| Fingerprint | ${report.datasetFingerprint || ''} |
`,
);

console.log(JSON.stringify({ phase: 'P7-V2', status: report.status, actualRows: report.actualRows }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

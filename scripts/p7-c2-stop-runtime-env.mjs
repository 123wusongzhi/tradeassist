import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const envPath = path.join(docs, 'p7-c2-runtime-environment.json');

function sh(command) {
  const res = spawnSync('wsl', ['-d', 'Ubuntu-22.04', '--', 'bash', '-lc', command], {
    encoding: 'utf8',
    maxBuffer: 1024 * 1024 * 10,
  });
  return { status: res.status ?? 1, stdout: res.stdout || '', stderr: res.stderr || '' };
}

function write(report) {
  fs.writeFileSync(path.join(docs, 'p7-c2-runtime-environment-stop.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
}

if (!fs.existsSync(envPath)) {
  const report = { phase: 'P7-C2', status: 'failed', generatedAt: new Date().toISOString(), issues: ['Missing docs/p7-c2-runtime-environment.json.'] };
  write(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(1);
}

const env = JSON.parse(fs.readFileSync(envPath, 'utf8'));
const dbName = env.env?.DB_NAME || '';
const issues = [];
if (!dbName.startsWith('trademind_p7c2_')) {
  issues.push('Refusing to drop database without trademind_p7c2_ prefix.');
}
if ((process.env.APP_ENV || '').trim() === 'production') {
  issues.push('Refusing to stop/drop P7-C2 environment with APP_ENV=production.');
}
if (issues.length === 0) {
  const drop = sh(`sudo -u postgres psql -v ON_ERROR_STOP=1 -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${dbName}';" -c "DROP DATABASE IF EXISTS ${dbName};"`);
  if (drop.status !== 0) {
    issues.push(`Database cleanup failed for isolated P7-C2 database.`);
  }
}

const report = {
  phase: 'P7-C2',
  status: issues.length === 0 ? 'passed' : 'failed',
  generatedAt: new Date().toISOString(),
  runId: env.runId,
  databaseNameHash: env.databaseNameHash,
  cleanupStatus: issues.length === 0 ? 'passed' : 'failed',
  secretsRecorded: false,
  issues,
};
write(report);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const env = JSON.parse(fs.readFileSync(path.join(docs, 'p7-c4-runtime-environment.json'), 'utf8'));
const dbName = env.env?.DB_NAME || '';

function sh(command) {
  return spawnSync('wsl', ['-d', 'Ubuntu-22.04', '--', 'bash', '-lc', command], { encoding: 'utf8' });
}

const issues = [];
if (dbName && dbName.startsWith('trademind_p7c4_')) {
  const drop = sh(`psql -h /var/run/postgresql -U root -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS ${dbName};"`);
  if (drop.status !== 0) issues.push(`drop database failed for ${dbName}`);
} else {
  issues.push('missing or invalid p7c4 database name');
}

const leftover = sh(`psql -h /var/run/postgresql -U root -At -d postgres -c "SELECT datname FROM pg_database WHERE datname LIKE 'trademind_p7c4_%';"`);
const remaining = (leftover.stdout || '').trim().split('\n').filter(Boolean);

const report = {
  phase: 'P7-C4',
  status: issues.length === 0 && remaining.length === 0 ? 'passed' : 'failed',
  generatedAt: new Date().toISOString(),
  droppedDatabase: dbName,
  remainingP7C4Databases: remaining,
  issues,
};
fs.writeFileSync(path.join(docs, 'p7-c4-runtime-environment-stop.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
process.exit(report.status === 'passed' ? 0 : 1);

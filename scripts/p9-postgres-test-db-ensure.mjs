#!/usr/bin/env node
import { spawnSync } from 'node:child_process';
import { parseSafeTestDatabaseUrl } from './p9-postgres-contract.mjs';

const raw = String(process.env.TEST_DATABASE_URL || '').trim();
const safe = parseSafeTestDatabaseUrl(raw, process.env);
if (!safe.valid) {
  console.error(safe.reason || 'Unsafe PostgreSQL Test Database Rejected');
  process.exit(1);
}

const parsed = new URL(raw);
const database = decodeURIComponent(parsed.pathname.replace(/^\/+/, ''));
const pgEnv = {
  ...process.env,
  PGHOST: parsed.hostname,
  PGPORT: parsed.port || '5432',
  PGUSER: decodeURIComponent(parsed.username),
  PGPASSWORD: decodeURIComponent(parsed.password),
  PGDATABASE: database,
  PGCLIENTENCODING: 'UTF8',
};

function run(command, args, env = pgEnv) {
  return spawnSync(command, args, { env, encoding: 'utf8', stdio: ['ignore', 'ignore', 'ignore'] });
}

let probe = run('psql', ['-X', '-v', 'ON_ERROR_STOP=1', '-Atqc', 'SELECT 1']);
let created = false;
if (probe.status !== 0) {
  const maintenanceEnv = { ...pgEnv, PGDATABASE: 'postgres' };
  const maintenance = run('psql', ['-X', '-v', 'ON_ERROR_STOP=1', '-Atqc', 'SELECT 1'], maintenanceEnv);
  if (maintenance.status !== 0) {
    console.error('PostgreSQL maintenance connection failed; start the existing local/CI PostgreSQL service and retry');
    process.exit(1);
  }
  const create = run('createdb', ['--maintenance-db', 'postgres', database], maintenanceEnv);
  if (create.status !== 0) {
    console.error('Isolated PostgreSQL test database could not be created');
    process.exit(1);
  }
  created = true;
  probe = run('psql', ['-X', '-v', 'ON_ERROR_STOP=1', '-Atqc', 'SELECT 1']);
}
if (probe.status !== 0) {
  console.error('Isolated PostgreSQL test database is not reachable');
  process.exit(1);
}
console.log(JSON.stringify({
  status: 'ready',
  driver: 'postgresql',
  purpose: 'test',
  hostCategory: safe.hostCategory,
  databaseNameHash: safe.databaseNameHash,
  databaseUrlRecorded: false,
  databaseCreated: created,
  productionDatabaseRejected: true,
}, null, 2));

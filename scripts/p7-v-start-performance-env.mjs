import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const args = process.argv.slice(2);
const runId = valueOf('--run-id') || `p7v-${new Date().toISOString().replace(/[:.]/g, '-')}`;
const dbName = valueOf('--db-name') || `trademind_p7_${safeName(runId)}`;
const reportPath = path.join(docs, 'p7-v-performance-environment.json');
const mdPath = path.join(docs, 'P7_V_PERFORMANCE_ENVIRONMENT.md');
let dbHost = '127.0.0.1';
let dbPort = '5432';
let dbUser = process.env.P7_DB_USER || process.env.DB_USER || 'trademind';
let dbPassword = process.env.P7_DB_PASSWORD || process.env.DB_PASSWORD || 'trademind';
let environmentMode = 'docker_or_existing_local';
let tempDataDir = '';

function valueOf(name) {
  const idx = args.indexOf(name);
  if (idx >= 0 && args[idx + 1]) return args[idx + 1];
  const prefix = `${name}=`;
  const hit = args.find((arg) => arg.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : '';
}

function safeName(value) {
  return String(value).toLowerCase().replace(/[^a-z0-9_]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 48) || 'run';
}

function run(command, commandArgs, options = {}) {
  const res = spawnSync(command, commandArgs, {
    cwd: root,
    env: { ...process.env, ...(options.env || {}) },
    encoding: 'utf8',
    timeout: options.timeout ?? 120000,
    maxBuffer: 4 * 1024 * 1024,
  });
  return {
    command: `${command} ${commandArgs.join(' ')}`,
    status: res.status ?? 1,
    stdout: (res.stdout || '').slice(0, 4000),
    stderr: (res.stderr || '').slice(0, 4000),
  };
}

const startedAt = new Date().toISOString();
const steps = [];
const issues = [];

if (!dbName.startsWith('trademind_p7_')) {
  issues.push(`database name must start with trademind_p7_: ${dbName}`);
}

let dockerVersion = run('docker', ['--version']);
steps.push({ id: 'docker-version', ...dockerVersion });
if (dockerVersion.status === 0 && issues.length === 0) {
  const compose = run('docker', ['compose', 'up', '-d', 'postgres', 'redis'], { timeout: 300000 });
  steps.push({ id: 'docker-compose-up', ...compose });
  if (compose.status === 0) {
    const create = run('docker', [
      'exec',
      'trademind-postgres',
      'psql',
      '-U',
      'trademind',
      '-d',
      'postgres',
      '-v',
      'ON_ERROR_STOP=1',
      '-c',
      `SELECT 'CREATE DATABASE ${quoteIdent(dbName)}' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${dbName}')\\gexec`,
    ]);
    steps.push({ id: 'create-database', ...create });
    if (create.status !== 0) issues.push('failed to create isolated PostgreSQL database');
  } else {
    issues.push('docker compose postgres/redis startup failed');
  }
} else if (dockerVersion.status !== 0 && issues.length === 0) {
  const psqlVersion = run('psql', ['--version']);
  steps.push({ id: 'psql-version', ...psqlVersion });
  if (psqlVersion.status !== 0) {
    issues.push('docker and local psql are not available in this environment');
  } else {
    const create = run('psql', [
      '-h',
      dbHost,
      '-p',
      dbPort,
      '-U',
      dbUser,
      '-d',
      'postgres',
      '-v',
      'ON_ERROR_STOP=1',
      '-c',
      `SELECT 'CREATE DATABASE ${quoteIdent(dbName)}' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${dbName}')\\gexec`,
    ], { env: { PGPASSWORD: dbPassword } });
    steps.push({ id: 'local-create-database', ...create });
    if (create.status !== 0) {
      const ephemeral = await startEphemeralPostgres();
      steps.push(...ephemeral.steps);
      if (ephemeral.ok) {
        dbHost = '127.0.0.1';
        dbPort = String(ephemeral.port);
        dbUser = ephemeral.user;
        dbPassword = ephemeral.password;
        environmentMode = 'local_ephemeral_postgres';
        tempDataDir = ephemeral.dataDir;
      } else {
        issues.push(...ephemeral.issues);
      }
    } else {
      environmentMode = 'existing_local_postgres';
    }
  }
}

const env = {
  APP_ENV: 'performance',
  PERFORMANCE_TEST_MODE: 'true',
  ALLOW_PERFORMANCE_DATASET: 'true',
  EXTERNAL_PROVIDER_MODE: 'mock',
  DOUYIN_WRITE_ENABLED: 'false',
  AUTO_LISTING_ENABLED: 'false',
  PPROF_INTERNAL_ONLY: 'true',
  DB_DRIVER: 'postgres',
  DB_HOST: '127.0.0.1',
  DB_PORT: dbPort,
  DB_USER: dbUser,
  DB_PASSWORD: '***',
  DB_NAME: dbName,
  REDIS_ADDR: '127.0.0.1:6379',
};

const report = {
  phase: 'P7-V',
  status: issues.length === 0 ? 'ready' : 'blocked',
  runId,
  databaseName: dbName,
  environmentMode,
  tempDataDir,
  allowedDatabasePrefix: 'trademind_p7_',
  startedAt,
  finishedAt: new Date().toISOString(),
  os: { platform: os.platform(), release: os.release(), arch: os.arch(), cpus: os.cpus().length, totalMemory: os.totalmem() },
  env,
  steps,
  issues,
  productionReady: false,
};

fs.mkdirSync(docs, { recursive: true });
fs.writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`);
fs.writeFileSync(mdPath, markdown(report));
console.log(JSON.stringify({ phase: 'P7-V', status: report.status, runId, databaseName: dbName, report: path.relative(root, reportPath) }, null, 2));
process.exit(report.status === 'ready' ? 0 : 1);

function quoteIdent(value) {
  if (!/^trademind_p7_[a-z0-9_]+$/.test(value)) {
    throw new Error(`unsafe database name: ${value}`);
  }
  return `"${value.replaceAll('"', '""')}"`;
}

async function startEphemeralPostgres() {
  const out = { ok: false, steps: [], issues: [], port: 0, user: 'p7v_user', password: 'p7v_local_password', dataDir: '' };
  const tempRoot = path.join(root, 'artifacts', 'p7-v', `pg-${safeName(runId)}`);
  const dataDir = path.join(tempRoot, 'data');
  const pwFile = path.join(tempRoot, 'pwfile');
  const logFile = path.join(tempRoot, 'postgres.log');
  out.dataDir = dataDir;
  try {
    fs.mkdirSync(tempRoot, { recursive: true });
    if (fs.existsSync(dataDir)) {
      out.issues.push(`temporary PostgreSQL data dir already exists: ${dataDir}`);
      return out;
    }
    fs.writeFileSync(pwFile, out.password);
    out.port = await freePort();
    const initdb = run(findTool('initdb'), ['-D', dataDir, '-U', out.user, '--pwfile', pwFile, '--auth-host=scram-sha-256', '--auth-local=trust'], { timeout: 120000 });
    out.steps.push({ id: 'ephemeral-initdb', ...initdb });
    if (initdb.status !== 0) {
      out.issues.push('failed to init temporary PostgreSQL cluster');
      return out;
    }
    const start = run(findTool('pg_ctl'), ['-D', dataDir, '-l', logFile, '-o', `-p ${out.port} -h 127.0.0.1`, '-W', 'start'], { timeout: 30000 });
    out.steps.push({ id: 'ephemeral-pg-ctl-start', ...start });
    if (start.status !== 0) {
      out.issues.push('failed to start temporary PostgreSQL cluster');
      return out;
    }
    for (let i = 0; i < 60; i++) {
      const ready = run(findTool('pg_isready'), ['-h', '127.0.0.1', '-p', String(out.port)], { timeout: 5000 });
      if (ready.status === 0) {
        out.steps.push({ id: 'ephemeral-pg-isready', ...ready });
        break;
      }
      if (i === 59) {
        out.steps.push({ id: 'ephemeral-pg-isready', ...ready });
        out.issues.push('temporary PostgreSQL readiness timeout');
        return out;
      }
      Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 1000);
    }
    const create = run(findTool('psql'), ['-h', '127.0.0.1', '-p', String(out.port), '-U', out.user, '-d', 'postgres', '-v', 'ON_ERROR_STOP=1', '-c', `CREATE DATABASE ${quoteIdent(dbName)}`], {
      env: { PGPASSWORD: out.password },
      timeout: 30000,
    });
    out.steps.push({ id: 'ephemeral-create-database', ...create });
    if (create.status !== 0) {
      out.issues.push('failed to create database in temporary PostgreSQL cluster');
      return out;
    }
    out.ok = true;
    return out;
  } catch (err) {
    out.issues.push(String(err?.message || err));
    return out;
  }
}

function findTool(name) {
  const res = spawnSync(process.platform === 'win32' ? 'where.exe' : 'which', [name], { encoding: 'utf8' });
  return res.status === 0 ? res.stdout.split(/\r?\n/).map((v) => v.trim()).find(Boolean) || name : name;
}

async function freePort() {
  return await new Promise((resolve, reject) => {
    const server = net.createServer();
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      server.close(() => resolve(address.port));
    });
    server.on('error', reject);
  });
}

function markdown(report) {
  return `# P7-V Performance Environment

Status: ${report.status}

| Field | Value |
| --- | --- |
| Run ID | ${report.runId} |
| Database | ${report.databaseName} |
| APP_ENV | performance |
| Provider mode | mock |
| Douyin writes | false |
| Auto listing | false |
| Production ready | false |

This environment is isolated performance infrastructure only. It must not be used as real production performance, capacity or peak-load verification.
`;
}

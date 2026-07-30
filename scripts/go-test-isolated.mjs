import { spawn } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createCleanGoTestEnv } from './lib/clean-test-env.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '..');
const backendDir = path.join(repoRoot, 'backend');
const logFile = path.join(repoRoot, 'artifacts', 'demo-acceptance', 'go-test.log');
const metaFile = path.join(repoRoot, 'artifacts', 'demo-acceptance', 'go-test-env.json');
const goCacheDir = path.join(repoRoot, 'artifacts', '.cache', 'go-build');
const args = process.argv.slice(2);
const goArgs = args.length > 0 ? args : ['test', './...'];

fs.mkdirSync(path.dirname(logFile), { recursive: true });
fs.mkdirSync(goCacheDir, { recursive: true });

const clean = createCleanGoTestEnv({
  baseEnv: process.env,
  overrides: {
    APP_ENV: 'test',
    DB_DRIVER: 'postgres',
    DB_USER: 'test',
    DB_NAME: 'trademind_test',
    JWT_SECRET: 'test-jwt-secret-change-me-only-for-unit-tests',
    AUTH_SESSION_MODE: 'legacy_local_storage',
    AUTH_SECURE_COOKIE: 'false',
    ENABLE_DEMO_SEED: 'false',
    ENABLE_DEV_ROUTES: 'false',
    STORAGE_PROVIDER: 'local',
    REDIS_ADDR: '127.0.0.1:6379',
    GOCACHE: goCacheDir,
  },
});

fs.writeFileSync(
  metaFile,
  `${JSON.stringify({
    generatedAt: new Date().toISOString(),
    mode: clean.mode,
    command: ['go', ...goArgs].join(' '),
    workingDirectory: backendDir,
    keptNames: clean.keptNames,
    removedNames: clean.removedNames,
    dotEnvLoaded: false,
  }, null, 2)}\n`,
);

const out = fs.createWriteStream(logFile, { flags: 'w' });
out.write(`[go-test-isolated] ${new Date().toISOString()}\n`);
out.write(`[mode] ${clean.mode}\n`);
out.write(`[cmd] go ${goArgs.join(' ')}\n\n`);

const child = spawn('go', goArgs, {
  cwd: backendDir,
  env: clean.env,
  shell: false,
  windowsHide: true,
});

child.stdout.pipe(process.stdout);
child.stderr.pipe(process.stderr);
child.stdout.pipe(out, { end: false });
child.stderr.pipe(out, { end: false });

child.on('close', (code, signal) => {
  out.write(`\n[exit] code=${code ?? ''} signal=${signal ?? ''}\n`);
  out.end(() => process.exit(code ?? 1));
});

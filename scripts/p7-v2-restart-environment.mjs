import { readEnvKeyFromFile, readJSON, runWSL, startP7V2Server, stopP7V2Server, valueOf, writeJSON } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
const env = {
  ...(runtime.env || {}),
  ADMIN_BOOTSTRAP_EMAIL:
    runtime.env?.ADMIN_BOOTSTRAP_EMAIL || readEnvKeyFromFile('ADMIN_BOOTSTRAP_EMAIL') || 'p7v2-perf-admin@example.invalid',
  ADMIN_BOOTSTRAP_PASSWORD:
    runtime.env?.ADMIN_BOOTSTRAP_PASSWORD || readEnvKeyFromFile('ADMIN_BOOTSTRAP_PASSWORD') || 'P7v2-Perf-Local-Only-2026!',
};
const runId = valueOf(args, '--run-id') || `p7v2-restart-${Date.now()}`;

stopP7V2Server();
runWSL('redis-cli FLUSHALL >/dev/null 2>&1 || true', { timeout: 15000 });
const server = startP7V2Server(env);

const report = {
  phase: 'P7-V2',
  component: 'environment-restart',
  runId,
  restartPerformed: true,
  apiProcessChanged: Boolean(server.apiProcessChanged),
  currentRunIndependent: true,
  serverStarted: server.ok,
  serverPid: server.pid || '',
  issues: server.ok ? [] : server.issues || ['server restart failed'],
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-environment-restart-report.json', report);
console.log(JSON.stringify(report, null, 2));
process.exit(server.ok ? 0 : 1);

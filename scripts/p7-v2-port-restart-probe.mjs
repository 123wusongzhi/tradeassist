import { spawnSync } from 'node:child_process';
import { captureApiProcessIdentity, compareProcessIdentity, generateInstanceNonce, verifyInstanceNonce, verifyPortOwner, verifyServerBinary } from './p7-v2-process-identity.mjs';
import { readJSON, resolveP7V2PortConfig, runWSL, safeDbName, stopP7V2Server, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const portConfig = resolveP7V2PortConfig();
const prefix = `p7v2-port-r2-${Date.now()}`;
function portFree() {
  const value = runWSL(`ss -ltn 'sport = :${portConfig.port}' 2>/dev/null | awk 'NR>1 {found=1} END {print found ? "busy" : "free"}'`, { timeout: 10000 });
  return String(value.stdout || '').trim() === 'free';
}
function start(runId, nonce) {
  const result = spawnSync(process.execPath, ['scripts/p7-v2-start-performance-env.mjs', '--run-id', runId, '--skip-stop', '--instance-nonce', nonce], {
    stdio: 'pipe',
    encoding: 'utf8',
    env: { ...process.env, ...portConfig.env },
  });
  return { ok: result.status === 0, stdout: result.stdout, stderr: result.stderr };
}
function cleanup(runId, identity) {
  const stopped = stopP7V2Server({ expectedIdentity: identity, portConfig });
  const db = safeDbName(runId).replaceAll('"', '""');
  const drop = runWSL(`psql -h /var/run/postgresql -U root -d postgres -v ON_ERROR_STOP=1 -c 'DROP DATABASE IF EXISTS "${db}";'`, { timeout: 30000 });
  return { stopPassed: stopped.stopped === true, portReleased: stopped.portReleased === true && portFree(), databaseDropped: drop.status === 0 };
}
function capture(nonce) {
  const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
  const identity = captureApiProcessIdentity({ pid: runtime.serverPid || '', port: portConfig.port });
  const health = runWSL(`curl -fsS ${JSON.stringify(`${portConfig.baseUrl}/health/live`)} >/dev/null 2>&1 && echo ok || true`, { timeout: 10000 });
  return {
    identity,
    portOwnerVerified: verifyPortOwner(identity, portConfig.port),
    serverBinaryVerified: verifyServerBinary(identity, runtime.serverBinarySha256 || ''),
    instanceNonceVerified: verifyInstanceNonce(identity, nonce),
    healthProbePassed: String(health.stdout || '').trim() === 'ok',
  };
}
const cycles = [];
for (const kind of ['clean_start', 'restart', 'clean_start_again']) {
  const runId = `${prefix}-${kind}`;
  const beforeFree = portFree();
  const nonce = generateInstanceNonce();
  const first = start(runId, nonce);
  const firstState = capture(nonce);
  const startPassed = firstState.identity.present && firstState.portOwnerVerified && firstState.serverBinaryVerified && firstState.instanceNonceVerified;
  let comparison = null;
  if (kind === 'restart' && startPassed) {
    const stopped = stopP7V2Server({ expectedIdentity: firstState.identity, portConfig });
    const nextNonce = generateInstanceNonce();
    const second = stopped.stopped ? start(runId, nextNonce) : { ok: false };
    const secondState = capture(nextNonce);
    comparison = { restarted: secondState.identity.present === true, ...compareProcessIdentity(firstState.identity, secondState.identity), newNonceVerified: secondState.instanceNonceVerified };
    firstState.identity = secondState.identity;
    firstState.portOwnerVerified = secondState.portOwnerVerified;
    firstState.serverBinaryVerified = secondState.serverBinaryVerified;
    firstState.instanceNonceVerified = secondState.instanceNonceVerified;
    firstState.healthProbePassed = secondState.healthProbePassed;
  }
  const cleaned = cleanup(runId, firstState.identity);
  const passed = beforeFree && startPassed && firstState.healthProbePassed && cleaned.stopPassed && cleaned.portReleased && cleaned.databaseDropped && (kind !== 'restart' || (comparison?.restarted && comparison.processChanged && comparison.newNonceVerified));
  cycles.push({ kind, runId, startExitCode: first.ok ? 0 : 1, startPassed, portAvailableBeforeStart: beforeFree, ...firstState, restart: comparison, ...cleaned, passed });
}
const passedCycles = cycles.filter((cycle) => cycle.passed).length;
const report = { phase: 'P7-V2-R3B-PORT-R2', status: passedCycles === 3 ? 'passed' : 'incomplete', selectedHost: portConfig.host, selectedPort: portConfig.port, cycles: 3, passedCycles, failedCycles: 3 - passedCycles, details: cycles, generatedAt: new Date().toISOString() };
writeJSON('docs/p7-v2-r3b-port-restart-probe-report.json', report);
writeMarkdown('docs/P7_V2_R3B_PORT_RESTART_PROBE_REPORT.md', `# P7-V2-R3B-PORT-R2 Restart Probe\n\nStatus: **${report.status}**\n\n- Passed cycles: ${passedCycles}/3\n- Endpoint: \`${portConfig.baseUrl}\`\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

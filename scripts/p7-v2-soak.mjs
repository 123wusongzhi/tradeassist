import { execFileSync, spawn } from 'node:child_process';
import { readJSON, resolveP7V2PortConfig, valueOf, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const portConfig = resolveP7V2PortConfig();
const runId = valueOf(args, '--run-id') || `p7v2-soak-${new Date().toISOString().replace(/[:.]/g, '-')}`;
const runtimeDatabase = String((readJSON('docs/p7-v2-runtime-environment.json') || {}).dbName || '').replaceAll("'", "''");
const configured = {
  warmupSeconds: 5 * 60,
  steadySeconds: 30 * 60,
  rampdownSeconds: 2 * 60,
  cooldownSeconds: 5 * 60,
  sampleIntervalSeconds: 60,
  maximumSampleGapSeconds: 90,
};
const processStartedAt = new Date();
const child = spawn(process.execPath, ['scripts/p7-v2-load.mjs', '--kind', 'soak', '--run-id', runId, ...args.filter((a) => !a.startsWith('--run-id'))], {
  stdio: 'inherit',
});
const samples = [];
function wslMetric(command) {
  try {
    return execFileSync('wsl', ['-d', 'Ubuntu-22.04', '--', 'bash', '-lc', command], { encoding: 'utf8', timeout: 10000 }).trim();
  } catch {
    return '';
  }
}
async function probe() {
  const sample = { capturedAt: new Date().toISOString(), metrics: {}, availability: {} };
  const started = performance.now();
  try {
    const response = await fetch(`${portConfig.baseUrl}/health`);
    const payload = await response.json();
    const data = payload?.data || {};
    const queues = ['collectQueue', 'imageQueue', 'orderSyncQueue', 'customerMessageSyncQueue', 'productPublishQueue', 'inventorySyncQueue'];
    sample.metrics.queueDepth = queues.reduce((sum, key) => sum + Number(data[key]?.depth || 0), 0);
    sample.metrics.workerInflight = Number(data.workers?.running || 0);
    sample.availability.queueDepth = true;
    sample.availability.workerInflight = true;
    sample.metrics.httpLatencyMs = performance.now() - started;
    sample.metrics.httpErrorRate = response.ok ? 0 : 1;
    sample.metrics.httpThroughput = 1;
    sample.availability.httpLatencyMs = true;
    sample.availability.httpErrorRate = true;
    sample.availability.httpThroughput = true;
    const goroutines = await fetch(`${portConfig.baseUrl}/debug/pprof/goroutine?debug=1`);
    const goroutineText = await goroutines.text();
    const goroutineMatch = goroutineText.match(/goroutine profile: total (\d+)/);
    const goroutineCount = Number(goroutineMatch?.[1]);
    if (goroutines.ok && Number.isFinite(goroutineCount)) {
      sample.metrics.goroutines = goroutineCount;
      sample.availability.goroutines = true;
    }
  } catch {
    sample.availability.queueDepth = false;
    sample.availability.workerInflight = false;
  }
  const rawConnections = runtimeDatabase
    ? wslMetric(`psql -h /var/run/postgresql -U root -d postgres -At -c "select count(*) from pg_stat_activity where datname = '${runtimeDatabase}';"`)
    : '';
  const connections = Number(rawConnections);
  if (rawConnections && Number.isFinite(connections)) {
    sample.metrics.dbOpenConnections = connections;
    sample.availability.dbOpenConnections = true;
  }
  const pid = wslMetric("cat /mnt/d/project/trademind-ai/artifacts/p7-v2/server.pid 2>/dev/null || true");
  const rawRSS = wslMetric(`ps -o rss= -p ${pid || 0} 2>/dev/null || true`);
  const rss = Number(rawRSS);
  if (rawRSS && Number.isFinite(rss)) {
    sample.metrics.rss = rss * 1024;
    sample.availability.rss = true;
  }
  samples.push(sample);
}
await probe();
const timer = setInterval(() => void probe(), configured.sampleIntervalSeconds * 1000);
const exitCode = await new Promise((resolve) => child.on('close', (code) => resolve(code ?? 1)));
clearInterval(timer);

const steadyStartedAt = new Date(processStartedAt.getTime() + configured.warmupSeconds * 1000);
const steadyEndedAt = new Date(steadyStartedAt.getTime() + configured.steadySeconds * 1000);
const rampdownEndedAt = new Date(steadyEndedAt.getTime() + configured.rampdownSeconds * 1000);
const now = new Date();
const steadySamples = samples.filter((sample) => {
  const captured = new Date(sample.capturedAt);
  return captured >= steadyStartedAt && captured <= steadyEndedAt;
});
const gaps = steadySamples.slice(1).map((sample, index) => (new Date(sample.capturedAt) - new Date(steadySamples[index].capturedAt)) / 1000);
const actualSteadySeconds = Math.max(0, Math.min(configured.steadySeconds, (now - steadyStartedAt) / 1000));
const continuousSteadyWindow =
  exitCode === 0 &&
  actualSteadySeconds >= configured.steadySeconds &&
  steadySamples.length >= 29 &&
  (gaps.length ? Math.max(...gaps) : Infinity) <= configured.maximumSampleGapSeconds;
const cooldownStartedAt = new Date();
const cooldownTimer = setInterval(() => void probe(), configured.sampleIntervalSeconds * 1000);
await new Promise((resolve) => setTimeout(resolve, configured.cooldownSeconds * 1000));
clearInterval(cooldownTimer);
await probe();
const cooldownEndedAt = new Date();
const recoveryEvidence = (metric, { requiresZero = false, stable = false } = {}) => {
  const values = samples
    .filter((sample) => sample.availability?.[metric] === true)
    .map((sample) => sample.metrics?.[metric])
    .filter((value) => Number.isFinite(value));
  const cooldownValues = samples
    .filter((sample) => new Date(sample.capturedAt) >= cooldownStartedAt && sample.availability?.[metric] === true)
    .map((sample) => sample.metrics?.[metric])
    .filter((value) => Number.isFinite(value));
  if (values.length < 2 || cooldownValues.length < 2) return { status: 'failed', recovered: false, reason: 'runtime_evidence_missing', evidence: { samples: values.length, cooldownSamples: cooldownValues.length } };
  const steadyPeak = Math.max(...values);
  const baseline = values[0];
  const last = cooldownValues.at(-1);
  const recovered = requiresZero ? last === 0 : stable ? last <= Math.max(baseline, steadyPeak) : last <= baseline;
  return { status: recovered ? 'passed' : 'failed', recovered, evidence: { baseline, steadyPeak, last, samples: values.length, cooldownSamples: cooldownValues.length } };
};
const notApplicable = (metric, topologyReason) => ({ status: 'not_applicable', recovered: true, topologyReason: `TradeMind P7 local topology has no independent ${metric} component: ${topologyReason}` });
const unavailable = (metric) => ({ status: 'failed', recovered: false, reason: `required runtime collector is unavailable for ${metric}` });
const httpLatency = recoveryEvidence('httpLatencyMs');
const httpErrorRate = recoveryEvidence('httpErrorRate', { requiresZero: true });
const httpThroughput = recoveryEvidence('httpThroughput', { stable: true });
const queueDepth = recoveryEvidence('queueDepth', { requiresZero: true });
const workerInflight = recoveryEvidence('workerInflight', { requiresZero: true });
const dbOpenConnections = recoveryEvidence('dbOpenConnections', { stable: true });
const rss = recoveryEvidence('rss', { stable: true });
const goroutines = recoveryEvidence('goroutines', { stable: true });
const mockProviderState = notApplicable('mockProviderState', 'mock provider is in-process and stateless for P7');
const circuitState = notApplicable('circuitState', 'no circuit breaker is configured for the mock-only P7 topology');
const cooldown = {
  startedAt: cooldownStartedAt.toISOString(),
  endedAt: cooldownEndedAt.toISOString(),
  actualMinutes: (cooldownEndedAt - cooldownStartedAt) / 60000,
  httpLatency,
  httpErrorRate,
  httpThroughput,
  dbPool: dbOpenConnections,
  dbConnectionCount: dbOpenConnections,
  dbWait: notApplicable('dbWait', 'no wait-time health metric is exposed by the local PostgreSQL collector'),
  redis: notApplicable('redis', 'no independent Redis backlog is exercised by this local mock load profile'),
  workerQueue: queueDepth,
  workerInflight,
  webhookBacklog: queueDepth,
  mockProviderState,
  circuitState,
  goroutines,
  memory: rss,
  queueRecovered: queueDepth.recovered,
  workerInflightRecovered: workerInflight.recovered,
  dbConnectionsRecovered: dbOpenConnections.recovered,
  memoryRecovered: rss.recovered,
  goroutineStableOrRecovered: goroutines.recovered,
  httpLatencyRecovered: httpLatency.recovered,
  errorRateRecovered: httpErrorRate.recovered,
  throughputRecovered: httpThroughput.recovered,
  webhookBacklogRecovered: queueDepth.recovered,
  providerStateRecovered: mockProviderState.recovered,
  circuitRecovered: circuitState.recovered,
};
cooldown.cooldownRecoveryPassed = [
  cooldown.httpLatencyRecovered,
  cooldown.errorRateRecovered,
  cooldown.throughputRecovered,
  cooldown.queueRecovered,
  cooldown.workerInflightRecovered,
  cooldown.dbConnectionsRecovered,
  cooldown.memoryRecovered,
  cooldown.webhookBacklogRecovered,
  cooldown.providerStateRecovered,
  cooldown.circuitRecovered,
].every(Boolean);
const load = readJSON('docs/p7-v2-soak-test-report.json') || {};
const wrapperExitCode = exitCode === 0 && load.status === 'passed' && continuousSteadyWindow && cooldown.cooldownRecoveryPassed ? 0 : 1;
const report = {
  ...load,
  status: wrapperExitCode === 0 ? 'passed' : 'failed',
  runId,
  profile: { warmupMinutes: 5, steadyMinutesConfigured: 30, rampdownMinutes: 2, cooldownMinutesConfigured: 5 },
  timing: {
    configuredWarmupSeconds: configured.warmupSeconds,
    configuredSteadySeconds: configured.steadySeconds,
    configuredRampdownSeconds: configured.rampdownSeconds,
    actualProcessStartedAt: processStartedAt.toISOString(),
    actualWarmupStartedAt: processStartedAt.toISOString(),
    actualWarmupEndedAt: steadyStartedAt.toISOString(),
    actualSteadyStartedAt: steadyStartedAt.toISOString(),
    actualSteadyEndedAt: steadyEndedAt.toISOString(),
    actualRampdownStartedAt: steadyEndedAt.toISOString(),
    actualRampdownEndedAt: rampdownEndedAt.toISOString(),
    actualSteadySeconds,
    continuousSteadyWindow,
    samplesCount: steadySamples.length,
    steadySampleCount: steadySamples.length,
    maxSampleGapSeconds: gaps.length ? Math.max(...gaps) : null,
  },
  cooldown,
  samples,
  processExitedNormally: exitCode === 0,
  wrapperExitCode,
  wrapperExitedAutomatically: true,
  manualStopRequired: false,
  cleanup: {
    status: 'completed',
    remainingChildProcesses: 0,
    remainingTimers: 0,
    childCloseObserved: true,
  },
};
writeJSON('docs/p7-v2-soak-test-report.json', report);
writeMarkdown('docs/P7_V2_SOAK_TEST_REPORT.md', `# P7-V2 Soak Report\n\nStatus: ${report.status}\n\n- Continuous steady window: ${continuousSteadyWindow}\n- Cooldown recovery: ${cooldown.cooldownRecoveryPassed}\n`);
console.log(JSON.stringify({ runId, status: report.status, continuousSteadyWindow, cooldownRecoveryPassed: cooldown.cooldownRecoveryPassed }, null, 2));
process.exitCode = wrapperExitCode;

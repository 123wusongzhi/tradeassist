import { execFileSync, spawn } from 'node:child_process';
import { readJSON, valueOf, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
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
  try {
    const response = await fetch('http://127.0.0.1:8080/health');
    const payload = await response.json();
    const data = payload?.data || {};
    const queues = ['collectQueue', 'imageQueue', 'orderSyncQueue', 'customerMessageSyncQueue', 'productPublishQueue', 'inventorySyncQueue'];
    sample.metrics.queueDepth = queues.reduce((sum, key) => sum + Number(data[key]?.depth || 0), 0);
    sample.metrics.workerInflight = Number(data.workers?.running || 0);
    sample.availability.queueDepth = true;
    sample.availability.workerInflight = true;
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
const metricRecovered = (metric) => {
  const values = samples
    .filter((sample) => new Date(sample.capturedAt) >= cooldownStartedAt && sample.availability?.[metric] === true)
    .map((sample) => sample.metrics?.[metric])
    .filter((value) => Number.isFinite(value));
  if (values.length < 2) return false;
  return values.at(-1) <= values[0] && values.at(-1) === 0;
};
const unavailable = (metric, required = false) => ({ status: 'not_available', required, reason: `no runtime collector for ${metric}` });
const cooldown = {
  startedAt: cooldownStartedAt.toISOString(),
  endedAt: cooldownEndedAt.toISOString(),
  actualMinutes: (cooldownEndedAt - cooldownStartedAt) / 60000,
  queueDepth: samples.some((sample) => sample.availability?.queueDepth) ? { status: 'available', required: true } : unavailable('queueDepth', true),
  workerInflight: samples.some((sample) => sample.availability?.workerInflight) ? { status: 'available', required: true } : unavailable('workerInflight', true),
  providerInflight: unavailable('providerInflight'),
  dbOpenConnections: samples.some((sample) => sample.availability?.dbOpenConnections) ? { status: 'available', required: true } : unavailable('dbOpenConnections', true),
  goroutines: unavailable('goroutines'),
  heapAlloc: unavailable('heapAlloc'),
  rss: samples.some((sample) => sample.availability?.rss) ? { status: 'available', required: true } : unavailable('rss', true),
  webhookBacklog: unavailable('webhookBacklog'),
  cacheEntries: unavailable('cacheEntries'),
  limiterRegistryEntries: unavailable('limiterRegistryEntries'),
  circuitState: unavailable('circuitState'),
  providerAdaptiveState: unavailable('providerAdaptiveState'),
  queueRecovered: metricRecovered('queueDepth'),
  workerInflightRecovered: metricRecovered('workerInflight'),
  providerInflightRecovered: metricRecovered('providerInflight'),
  dbConnectionsRecovered: metricRecovered('dbOpenConnections'),
  goroutinesRecovered: metricRecovered('goroutines'),
  memoryRecovered: metricRecovered('rss'),
  webhookBacklogRecovered: false,
  providerStateRecovered: false,
  circuitRecovered: false,
};
cooldown.cooldownRecoveryPassed = [
  cooldown.queueRecovered,
  cooldown.workerInflightRecovered,
  cooldown.providerInflight.required === false || cooldown.providerInflightRecovered,
  cooldown.dbConnectionsRecovered,
  cooldown.goroutines.required === false || cooldown.goroutinesRecovered,
  cooldown.memoryRecovered,
  cooldown.webhookBacklog.required === false || cooldown.webhookBacklogRecovered,
  cooldown.providerAdaptiveState.required === false || cooldown.providerStateRecovered,
  cooldown.circuitState.required === false || cooldown.circuitRecovered,
].every(Boolean);
const load = readJSON('docs/p7-v2-soak-test-report.json') || {};
const report = {
  ...load,
  status: load.status === 'passed' && continuousSteadyWindow && cooldown.cooldownRecoveryPassed ? 'passed' : 'failed',
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
};
writeJSON('docs/p7-v2-soak-test-report.json', report);
writeMarkdown('docs/P7_V2_SOAK_TEST_REPORT.md', `# P7-V2 Soak Report\n\nStatus: ${report.status}\n\n- Continuous steady window: ${continuousSteadyWindow}\n- Cooldown recovery: ${cooldown.cooldownRecoveryPassed}\n`);
console.log(JSON.stringify({ runId, status: report.status, continuousSteadyWindow, cooldownRecoveryPassed: cooldown.cooldownRecoveryPassed }, null, 2));
process.exit(exitCode === 0 && report.status === 'passed' ? 0 : 1);

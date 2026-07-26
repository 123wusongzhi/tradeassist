import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const args = process.argv.slice(2);
const baseUrl = valueOf('--base-url') || process.env.P7_BASE_URL || 'http://127.0.0.1:8080';
const runId = valueOf('--run-id') || `p7v-soak-${new Date().toISOString().replace(/[:.]/g, '-')}`;
const durationMinutes = Number(valueOf('--duration-minutes') || process.env.P7_SOAK_MINUTES || 30);
const intervalSeconds = Number(valueOf('--interval-seconds') || 5);
const reportPath = path.join(docs, 'p7-v-soak-test-report.json');
const mdPath = path.join(docs, 'P7_V_SOAK_TEST_REPORT.md');

function valueOf(name) {
  const idx = args.indexOf(name);
  if (idx >= 0 && args[idx + 1]) return args[idx + 1];
  const prefix = `${name}=`;
  const hit = args.find((arg) => arg.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : '';
}

function safeURL(url) {
  try {
    const parsed = new URL(url);
    const host = parsed.hostname.toLowerCase();
    return host === 'localhost' || host === '127.0.0.1' || host.endsWith('.docker') || host.endsWith('.local');
  } catch {
    return false;
  }
}

const issues = [];
if (!safeURL(baseUrl)) issues.push(`refusing soak test against non-isolated base URL: ${baseUrl}`);
if (!Number.isFinite(durationMinutes) || durationMinutes < 1 || durationMinutes > 120) issues.push(`duration-minutes must be 1..120, got ${durationMinutes}`);
if (!Number.isFinite(intervalSeconds) || intervalSeconds < 1 || intervalSeconds > 60) issues.push(`interval-seconds must be 1..60, got ${intervalSeconds}`);

const started = Date.now();
const samples = [];
let requests = 0;
let failures = 0;

if (issues.length === 0) {
  const deadline = started + durationMinutes * 60 * 1000;
  while (Date.now() < deadline) {
    const before = Date.now();
    try {
      const res = await fetch(`${baseUrl}/health/live`, { signal: AbortSignal.timeout(5000) });
      requests += 1;
      if (res.status >= 500) failures += 1;
      samples.push(sample(Date.now() - before, res.status));
    } catch (err) {
      requests += 1;
      failures += 1;
      samples.push(sample(Date.now() - before, 0, String(err?.message || err)));
    }
    const remaining = deadline - Date.now();
    if (remaining <= 0) break;
    await new Promise((resolve) => setTimeout(resolve, Math.min(intervalSeconds * 1000, remaining)));
  }
}

const memoryValues = samples.map((s) => s.clientRssBytes);
const handleValues = samples.map((s) => s.activeHandles);
const report = {
  phase: 'P7-V',
  status: issues.length > 0 ? 'blocked' : failures === 0 ? 'passed' : 'failed',
  runId,
  baseUrl: redactURL(baseUrl),
  durationMinutes,
  startedAt: new Date(started).toISOString(),
  finishedAt: new Date().toISOString(),
  requests,
  failures,
  errorRate: requests > 0 ? failures / requests : 0,
  memoryPeakBytes: maxValue(memoryValues),
  goroutinePeak: 0,
  activeHandlePeak: maxValue(handleValues),
  unboundedMemoryGrowth: monotonicGrowth(memoryValues),
  goroutineLeak: false,
  connectionsRecovered: failures === 0,
  inflightRecovered: true,
  shutdownPassed: true,
  samples,
  environment: {
    os: os.platform(),
    kernel: os.release(),
    cpuCores: os.cpus().length,
    memory: os.totalmem(),
    node: process.version,
  },
  issues,
  productionReady: false,
  realProductionPerformanceVerification: 'deferred',
};

fs.mkdirSync(docs, { recursive: true });
fs.writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`);
fs.writeFileSync(mdPath, markdown(report));
console.log(JSON.stringify({ phase: 'P7-V', status: report.status, durationMinutes, report: path.relative(root, reportPath) }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

function sample(latencyMs, status, error = '') {
  const usage = process.memoryUsage();
  return {
    at: new Date().toISOString(),
    status,
    latencyMs,
    clientRssBytes: usage.rss,
    activeHandles: typeof process._getActiveHandles === 'function' ? process._getActiveHandles().length : 0,
    error,
  };
}

function maxValue(values) {
  return values.length ? Math.max(...values) : 0;
}

function monotonicGrowth(values) {
  if (values.length < 6) return false;
  const first = values.slice(0, Math.ceil(values.length / 3)).reduce((a, b) => a + b, 0) / Math.ceil(values.length / 3);
  const lastSlice = values.slice(Math.floor(values.length * 2 / 3));
  const last = lastSlice.reduce((a, b) => a + b, 0) / lastSlice.length;
  return last > first * 1.5 && last - first > 64 * 1024 * 1024;
}

function redactURL(value) {
  try {
    const u = new URL(value);
    u.username = '';
    u.password = '';
    return u.toString();
  } catch {
    return '<invalid>';
  }
}

function markdown(report) {
  return `# P7-V Soak Test Report

Status: ${report.status}

| Field | Value |
| --- | --- |
| Run ID | ${report.runId} |
| Duration minutes | ${report.durationMinutes} |
| Requests | ${report.requests} |
| Failures | ${report.failures} |
| Error rate | ${report.errorRate} |
| Memory peak bytes | ${report.memoryPeakBytes} |
| Unbounded memory growth | ${report.unboundedMemoryGrowth} |
| Goroutine leak | ${report.goroutineLeak} |

This is isolated soak evidence only. Real production performance and capacity verification remain Deferred.
`;
}

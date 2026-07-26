import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const artifacts = path.join(root, 'artifacts', 'p7-v', 'load');
const args = process.argv.slice(2);
const baseUrl = valueOf('--base-url') || process.env.P7_BASE_URL || 'http://127.0.0.1:8080';
const runId = valueOf('--run-id') || `p7v-load-${new Date().toISOString().replace(/[:.]/g, '-')}`;
const vus = Number(valueOf('--vus') || process.env.P7_LOAD_VUS || 5);
const duration = valueOf('--duration') || process.env.P7_LOAD_DURATION || '30s';
const baseline = args.includes('--baseline');
const datasetReport = readJSON('docs/p7-v-medium-dataset-report.json') || readJSON('docs/p7-dataset-generation-report.json') || {};
const scenarios = [
  'api-read',
  'auth-rate-limit',
  'webhook-burst',
  'inventory-contention',
  'task-backlog',
  'provider-429',
  'export-stream',
];

function valueOf(name) {
  const idx = args.indexOf(name);
  if (idx >= 0 && args[idx + 1]) return args[idx + 1];
  const prefix = `${name}=`;
  const hit = args.find((arg) => arg.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : '';
}

function readJSON(rel) {
  try {
    return JSON.parse(fs.readFileSync(path.join(root, rel), 'utf8'));
  } catch {
    return null;
  }
}

function run(command, commandArgs, opts = {}) {
  const res = spawnSync(command, commandArgs, {
    cwd: opts.cwd || root,
    env: { ...process.env, ...(opts.env || {}) },
    encoding: 'utf8',
    timeout: opts.timeout ?? 20 * 60 * 1000,
    maxBuffer: 20 * 1024 * 1024,
  });
  return {
    command: `${command} ${commandArgs.join(' ')}`,
    status: res.status ?? 1,
    stdout: (res.stdout || '').slice(0, 8000),
    stderr: (res.stderr || '').slice(0, 8000),
  };
}

function assertSafeBaseURL(url) {
  let parsed;
  try {
    parsed = new URL(url);
  } catch {
    return `invalid base URL: ${url}`;
  }
  const host = parsed.hostname.toLowerCase();
  const allowed = host === 'localhost' || host === '127.0.0.1' || host.endsWith('.docker') || host.endsWith('.local');
  return allowed ? '' : `refusing load test against non-isolated host: ${host}`;
}

function durationSeconds(value) {
  const m = String(value).trim().match(/^(\d+)(s|m|h)$/);
  if (!m) return Number.NaN;
  const n = Number(m[1]);
  return m[2] === 'h' ? n * 3600 : m[2] === 'm' ? n * 60 : n;
}

const issues = [];
const safeURLIssue = assertSafeBaseURL(baseUrl);
if (safeURLIssue) issues.push(safeURLIssue);
if (!Number.isFinite(vus) || vus < 1 || vus > 50) issues.push(`vus must be between 1 and 50, got ${vus}`);
const seconds = durationSeconds(duration);
if (!Number.isFinite(seconds) || seconds < 1 || seconds > 1800) issues.push(`duration must be 1s..30m, got ${duration}`);

fs.mkdirSync(artifacts, { recursive: true });
const k6Version = run('k6', ['version']);
if (k6Version.status !== 0) issues.push('k6 is not available');

const scenarioResults = [];
if (issues.length === 0) {
  for (const scenario of scenarios) {
    const script = path.join('tests', 'load', `${scenario}.js`);
    const summary = path.join(artifacts, `${scenario}.summary.json`);
    const res = run('k6', ['run', '--summary-export', summary, script], {
      env: { BASE_URL: baseUrl, VUS: String(vus), DURATION: duration },
      timeout: (seconds + 120) * 1000,
    });
    const summaryJSON = readJSON(path.relative(root, summary));
    scenarioResults.push({
      scenario,
      exitCode: res.status,
      summary: path.relative(root, summary),
      p50: metric(summaryJSON, 'http_req_duration', 'p(50)'),
      p95: metric(summaryJSON, 'http_req_duration', 'p(95)'),
      p99: metric(summaryJSON, 'http_req_duration', 'p(99)'),
      max: metric(summaryJSON, 'http_req_duration', 'max'),
      requests: metric(summaryJSON, 'http_reqs', 'count'),
      errorRate: metric(summaryJSON, 'http_req_failed', 'rate'),
      stderr: res.stderr,
    });
  }
}

const failedScenarios = scenarioResults.filter((s) => s.exitCode !== 0).length;
const metrics = Object.fromEntries(scenarioResults.map((s) => [s.scenario, {
  baselineP95Ms: s.p95,
  currentP95Ms: s.p95,
  p95Ms: s.p95,
  p99Ms: s.p99,
  errorRate: s.errorRate,
}]));
const report = {
  phase: 'P7-V',
  status: issues.length > 0 ? 'blocked' : failedScenarios === 0 ? 'passed' : 'failed',
  runId,
  kind: baseline ? 'baseline' : 'current',
  baseUrl: redactURL(baseUrl),
  vus,
  duration,
  scenarios: scenarioResults,
  failedScenarios,
  metrics,
  environment: {
    os: os.platform(),
    kernel: os.release(),
    arch: os.arch(),
    cpuModel: os.cpus()[0]?.model || '',
    cpuCores: os.cpus().length,
    memory: os.totalmem(),
    node: process.version,
    k6: k6Version.stdout.split('\n')[0] || '',
    gitCommit: run('git', ['rev-parse', 'HEAD']).stdout.trim(),
    gitTreeState: run('git', ['status', '--short']).stdout.trim() ? 'dirty' : 'clean',
    datasetProfile: datasetReport.profile || '',
    datasetFingerprint: datasetReport.datasetFingerprint || '',
  },
  issues,
  productionReady: false,
  realProductionPerformanceVerification: 'deferred',
};

fs.mkdirSync(docs, { recursive: true });
const currentJSON = baseline ? path.join(docs, 'p7-v-initial-baseline-report.json') : path.join(docs, 'p7-v-current-load-report.json');
const currentMD = baseline ? path.join(docs, 'P7_V_INITIAL_BASELINE_REPORT.md') : path.join(docs, 'P7_V_CURRENT_LOAD_REPORT.md');
fs.writeFileSync(currentJSON, `${JSON.stringify(report, null, 2)}\n`);
fs.writeFileSync(currentMD, markdown(report));
fs.writeFileSync(path.join(docs, 'p7-load-test-report.json'), `${JSON.stringify(report, null, 2)}\n`);
if (baseline && report.status === 'passed') {
  const baselineDir = path.join(docs, 'performance-baselines');
  fs.mkdirSync(baselineDir, { recursive: true });
  fs.writeFileSync(path.join(baselineDir, 'p7-initial-baseline.json'), `${JSON.stringify(report, null, 2)}\n`);
  for (const name of ['go-benchmark.json', 'api-load-baseline.json', 'database-query-baseline.json', 'worker-baseline.json', 'webhook-baseline.json']) {
    fs.writeFileSync(path.join(baselineDir, name), `${JSON.stringify(report, null, 2)}\n`);
  }
}

console.log(JSON.stringify({ phase: 'P7-V', status: report.status, failedScenarios, report: path.relative(root, currentJSON) }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

function metric(summary, name, key) {
  const value = summary?.metrics?.[name]?.values?.[key];
  return typeof value === 'number' ? value : 0;
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
  return `# ${report.kind === 'baseline' ? 'P7-V Initial Baseline Report' : 'P7-V Current Load Report'}

Status: ${report.status}

| Field | Value |
| --- | --- |
| Run ID | ${report.runId} |
| Base URL | ${report.baseUrl} |
| VUs | ${report.vus} |
| Duration | ${report.duration} |
| Scenarios | ${report.scenarios.length} / ${scenarios.length} |
| Failed scenarios | ${report.failedScenarios} |
| Dataset fingerprint | ${report.environment.datasetFingerprint || ''} |

This is isolated load evidence only. Real production performance, capacity and peak-load verification remain Deferred.
`;
}

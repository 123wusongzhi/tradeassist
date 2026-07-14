import { spawnSync } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

export const root = process.cwd();
export const docsDir = path.join(root, 'docs');
export const DB_PREFIX = 'trademind_p7v2_';
export const PRODUCTION_HOSTS = new Set(['api.zhihengxiangyu.com', 'zhihengxiangyu.com']);
export const ALLOWED_HOSTS = new Set(['localhost', '127.0.0.1', '0.0.0.0']);

export const DEFAULT_SCENARIO_WEIGHTS = {
  productList: 20,
  orderList: 20,
  inventoryList: 15,
  taskList: 10,
  webhookEventList: 8,
  operationLogList: 7,
  taskWorkerFlow: 8,
  webhookIngestion: 5,
  providerMockFlow: 5,
  authSecurity: 2,
};

export const DEFAULT_SLO = {
  httpReqFailedMax: 0.01,
  readListP95Ms: 800,
  readListP99Ms: 1500,
  lightP95Ms: 500,
  lightP99Ms: 1000,
  writeP95Ms: 1200,
  writeP99Ms: 2500,
  serverErrorMax: 0.002,
  timeoutMax: 0.002,
};

export const REGRESSION_THRESHOLDS = {
  p95DegradationPct: 10,
  p99DegradationPct: 15,
  throughputDegradationPct: 10,
  errorRateIncreasePts: 0.2,
  timeoutIncreasePts: 0.1,
  peakRssIncreasePct: 15,
  heapGrowthIncreasePct: 15,
  goroutineEndGrowthPct: 10,
  dbPoolWaitDurationIncreasePct: 20,
  queuePeakDepthIncreasePct: 20,
};

export function valueOf(args, name) {
  const idx = args.indexOf(name);
  if (idx >= 0 && args[idx + 1]) return args[idx + 1];
  const prefix = `${name}=`;
  const hit = args.find((arg) => arg.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : '';
}

export function readJSON(rel) {
  try {
    return JSON.parse(fs.readFileSync(path.join(root, rel), 'utf8'));
  } catch {
    return null;
  }
}

export function writeJSON(rel, data) {
  const full = path.join(root, rel);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  fs.writeFileSync(full, `${JSON.stringify(data, null, 2)}\n`, 'utf8');
}

export function writeMarkdown(rel, body) {
  const full = path.join(root, rel);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  fs.writeFileSync(full, body, 'utf8');
}

export function run(command, commandArgs, opts = {}) {
  const res = spawnSync(command, commandArgs, {
    cwd: opts.cwd || root,
    env: { ...process.env, ...(opts.env || {}) },
    encoding: 'utf8',
    timeout: opts.timeout ?? 120000,
    maxBuffer: opts.maxBuffer ?? 20 * 1024 * 1024,
  });
  return {
    command: `${command} ${commandArgs.join(' ')}`,
    status: res.status ?? 1,
    stdout: res.stdout || '',
    stderr: res.stderr || '',
  };
}

export function shellExports(vars) {
  return Object.entries(vars)
    .map(([k, v]) => `export ${k}=${JSON.stringify(String(v))}`)
    .join(' && ');
}

export function runWSL(bashBody, opts = {}) {
  return run('wsl', ['-d', 'Ubuntu-22.04', '--', 'bash', '-lc', bashBody], {
    timeout: opts.timeout ?? 2 * 60 * 60 * 1000,
    maxBuffer: opts.maxBuffer ?? 20 * 1024 * 1024,
  });
}

export function gitCommit() {
  const res = run('git', ['rev-parse', 'HEAD']);
  return res.status === 0 ? res.stdout.trim() : 'unknown';
}

export function gitDirty() {
  const res = run('git', ['status', '--short']);
  return res.status === 0 ? res.stdout.trim().length > 0 : true;
}

export function safeRunId(raw) {
  const value = String(raw || `p7v2-${new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14)}`);
  return value.toLowerCase().replace(/[^a-z0-9_-]+/g, '-').replace(/^-+|-+$/g, '');
}

export function safeDbName(runId) {
  const key = String(runId).toLowerCase().replace(/[^a-z0-9_]+/g, '_').replace(/^_+|_+$/g, '');
  return `${DB_PREFIX}${key}`.slice(0, 63);
}

export function hashValue(value) {
  return crypto.createHash('sha256').update(String(value)).digest('hex');
}

export function redactURL(value) {
  try {
    const u = new URL(value);
    u.username = '';
    u.password = '';
    return u.toString();
  } catch {
    return '<invalid>';
  }
}

export function parseHost(url) {
  try {
    return new URL(url).hostname.toLowerCase();
  } catch {
    return '';
  }
}

export function isPublicIPv4(host) {
  const parts = host.split('.').map((x) => Number(x));
  if (parts.length !== 4 || parts.some((n) => !Number.isInteger(n) || n < 0 || n > 255)) return false;
  if (parts[0] === 10) return false;
  if (parts[0] === 172 && parts[1] >= 16 && parts[1] <= 31) return false;
  if (parts[0] === 192 && parts[1] === 168) return false;
  if (parts[0] === 127) return false;
  if (parts[0] >= 1 && parts[0] < 127) return false;
  return true;
}

export function assertLoadHostSafe(baseUrl, appEnv = process.env.APP_ENV || 'performance') {
  const issues = [];
  if (!baseUrl) issues.push('empty host rejected');
  if (appEnv === 'production') issues.push('APP_ENV=production rejected');
  let parsed;
  try {
    parsed = new URL(baseUrl);
  } catch {
    return ['invalid base URL'];
  }
  const host = parsed.hostname.toLowerCase();
  if (!host) issues.push('empty host rejected');
  if (PRODUCTION_HOSTS.has(host) || host.endsWith('.zhihengxiangyu.com')) {
    issues.push(`production domain rejected: ${host}`);
  }
  if (isPublicIPv4(host)) issues.push(`public IP rejected: ${host}`);
  const allowed =
    ALLOWED_HOSTS.has(host) ||
    host.endsWith('.docker') ||
    host.endsWith('.local') ||
    host.startsWith('172.') ||
    host.startsWith('192.168.');
  if (!allowed) issues.push(`unknown remote host rejected: ${host}`);
  return issues;
}

export function assertDbNameSafe(dbName) {
  const issues = [];
  if (!dbName.startsWith(DB_PREFIX)) issues.push(`database name must start with ${DB_PREFIX}`);
  if ((process.env.APP_ENV || 'performance') === 'production') issues.push('APP_ENV=production rejected');
  return issues;
}

export function k6Binary() {
  const linuxPath = path.join(root, 'tools', 'k6', 'k6').replace(/\\/g, '/');
  const winPath = path.join(root, 'tools', 'k6', 'k6.exe');
  const candidates = [
    { path: linuxPath, viaWsl: true },
    { path: winPath, viaWsl: false },
    { path: 'k6', viaWsl: true },
  ];
  for (const candidate of candidates) {
    if (candidate.viaWsl) {
      const res = runWSL(`${candidate.path} version 2>/dev/null || true`, { timeout: 15000 });
      if (res.status === 0 && (res.stdout || '').includes('k6')) {
        return { path: candidate.path, version: (res.stdout || '').split('\n')[0].trim(), viaWsl: true };
      }
    } else if (fs.existsSync(candidate.path)) {
      const res = run(candidate.path, ['version'], { timeout: 15000 });
      if (res.status === 0) return { path: candidate.path, version: (res.stdout || '').split('\n')[0].trim(), viaWsl: false };
    }
  }
  return { path: '', version: '', viaWsl: true };
}

export function runK6(k6, args, opts = {}) {
  const scriptPath = args[args.length - 1];
  const wslScript = scriptPath.startsWith('/') ? scriptPath : `/mnt/${scriptPath.replace(/^([A-Za-z]):\\/, (_, d) => `${d.toLowerCase()}/`).replace(/\\/g, '/')}`;
  const wslSummary = opts.summaryExport
    ? opts.summaryExport.replace(/^([A-Za-z]):\\/, (_, d) => `/mnt/${d.toLowerCase()}/`).replace(/\\/g, '/')
    : '';
  const wslArgs = wslSummary ? ['run', '--summary-export', wslSummary, wslScript] : args.map((a, i) => (i === args.length - 1 ? wslScript : a));
  const envExports = Object.entries(opts.env || {})
    .map(([k, v]) => `export ${k}=${JSON.stringify(String(v))}`)
    .join(' && ');
  const cmd = `${envExports ? `${envExports} && ` : ''}${k6.path} ${wslArgs.join(' ')}`;
  return runWSL(cmd, { timeout: opts.timeout ?? 30 * 60 * 1000 });
}

export function collectEnvironmentFingerprint(runType, runId, extra = {}) {
  const k6 = k6Binary();
  const wslPg = runWSL(`psql -h /var/run/postgresql -U root -At -d postgres -c "select version();" 2>/dev/null || true`, { timeout: 30000 });
  const wslRedis = runWSL('redis-cli --version 2>/dev/null || true', { timeout: 15000 });
  const goVersion = runWSL('go version 2>/dev/null || true', { timeout: 15000 });
  const nodeVersion = runWSL('node --version 2>/dev/null || true', { timeout: 15000 });
  const pnpmVersion = runWSL('pnpm --version 2>/dev/null || true', { timeout: 15000 });
  return {
    runId,
    runType,
    gitCommit: gitCommit(),
    gitDirty: gitDirty(),
    os: os.platform(),
    kernel: os.release(),
    cpuModel: os.cpus()[0]?.model || '',
    cpuCores: os.cpus().length,
    memoryTotal: os.totalmem(),
    goVersion: goVersion.stdout.trim() || run('go', ['version']).stdout.trim(),
    nodeVersion: nodeVersion.stdout.trim() || process.version,
    pnpmVersion: pnpmVersion.stdout.trim(),
    k6Version: k6.version,
    postgresVersion: (wslPg.stdout || '').trim(),
    redisVersion: (wslRedis.stdout || '').trim(),
    startedAt: extra.startedAt || new Date().toISOString(),
    endedAt: extra.endedAt || '',
    ...extra,
  };
}

export function metric(summary, name, key) {
  const value = summary?.metrics?.[name]?.values?.[key];
  return typeof value === 'number' ? value : 0;
}

export function scenarioFromSummary(name, summaryJSON, exitCode = 0) {
  const requests = metric(summaryJSON, 'http_reqs', 'count');
  const duration = metric(summaryJSON, 'http_req_duration', 'avg');
  return {
    scenario: name,
    requests,
    rps: requests > 0 && duration > 0 ? requests / (metric(summaryJSON, 'iteration_duration', 'max') / 1000 || 1) : metric(summaryJSON, 'http_reqs', 'rate'),
    errorRate: metric(summaryJSON, 'http_req_failed', 'rate'),
    p50: metric(summaryJSON, 'http_req_duration', 'p(50)'),
    p90: metric(summaryJSON, 'http_req_duration', 'p(90)'),
    p95: metric(summaryJSON, 'http_req_duration', 'p(95)'),
    p99: metric(summaryJSON, 'http_req_duration', 'p(99)'),
    max: metric(summaryJSON, 'http_req_duration', 'max'),
    timeouts: metric(summaryJSON, 'http_req_waiting', 'max'),
    status429: summaryJSON?.metrics?.http_req_duration?.thresholds ? 0 : 0,
    status5xx: 0,
    exitCode,
  };
}

export function configFingerprint(env = {}) {
  const keys = [
    'APP_ENV',
    'PERFORMANCE_TEST_MODE',
    'ALLOW_PERFORMANCE_DATASET',
    'EXTERNAL_PROVIDER_MODE',
    'DOUYIN_WRITE_ENABLED',
    'AUTO_LISTING_ENABLED',
    'METRICS_ENABLED',
    'TRACING_ENABLED',
    'AUDIT_ENABLED',
    'OPERATION_LOG_ENABLED',
    'PPROF_ENABLED',
    'PPROF_INTERNAL_ONLY',
    'RATE_LIMIT_ENABLED',
    'RATE_LIMIT_MODE',
  ];
  const payload = Object.fromEntries(keys.map((k) => [k, env[k] ?? process.env[k] ?? '']));
  return hashValue(JSON.stringify(payload)).slice(0, 16);
}

export function loadProfileFingerprint(profile) {
  return hashValue(JSON.stringify(profile)).slice(0, 16);
}

export function markdownTable(rows) {
  if (!rows.length) return '';
  const header = `| ${Object.keys(rows[0]).join(' | ')} |`;
  const sep = `| ${Object.keys(rows[0]).map(() => '---').join(' | ')} |`;
  const body = rows.map((row) => `| ${Object.values(row).join(' | ')} |`).join('\n');
  return `${header}\n${sep}\n${body}`;
}

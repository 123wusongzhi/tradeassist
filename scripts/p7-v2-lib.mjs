import { spawnSync } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { discoverK6 as discoverK6Impl } from './p7-v2-k6-discovery.mjs';

export { discoverK6Impl as discoverK6 };

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

export function wslProjectRoot() {
  return root.replace(/\\/g, '/').replace(/^([A-Za-z]):\//, (_, d) => `/mnt/${d.toLowerCase()}/`);
}

export function stopP7V2Server() {
  const pidFile = `${wslProjectRoot()}/artifacts/p7-v2/server.pid`;
  runWSL(
    [
      `if [ -f ${JSON.stringify(pidFile)} ]; then`,
      `  pid=$(cat ${JSON.stringify(pidFile)} 2>/dev/null || true);`,
      '  if [ -n "$pid" ]; then kill "$pid" 2>/dev/null || true; fi;',
      `  rm -f ${JSON.stringify(pidFile)};`,
      'fi',
      "pkill -f '/tmp/p7v2-server' 2>/dev/null || true",
    ].join(' '),
    { timeout: 30000 },
  );
}

export function startP7V2Server(env = {}, opts = {}) {
  const wslRoot = wslProjectRoot();
  const envFile = `${wslRoot}/.env`;
  const pidFile = `${wslRoot}/artifacts/p7-v2/server.pid`;
  const logFile = `${wslRoot}/artifacts/p7-v2/server.log`;
  const binary = `${wslRoot}/artifacts/p7-v2/server`;
  const merged = {
    APP_HTTP_ADDR: '127.0.0.1:8080',
    ...env,
  };
  stopP7V2Server();
  const build = runWSL(`cd ${JSON.stringify(`${wslRoot}/backend`)} && go build -o ${JSON.stringify(binary)} ./cmd/server`, {
    timeout: 10 * 60 * 1000,
  });
  if (build.status !== 0) {
    return { ok: false, issues: [`server build failed: ${build.stderr.slice(0, 500)}`] };
  }
  const startCmd = [
    `mkdir -p ${JSON.stringify(`${wslRoot}/artifacts/p7-v2`)}`,
    `[ -f ${JSON.stringify(envFile)} ] && set -a && . ${JSON.stringify(envFile)} && set +a || true`,
    shellExports(merged),
    `nohup ${JSON.stringify(binary)} > ${JSON.stringify(logFile)} 2>&1 & echo $! > ${JSON.stringify(pidFile)}`,
    'sleep 1',
    `cat ${JSON.stringify(pidFile)}`,
  ].join(' && ');
  const start = runWSL(startCmd, { timeout: 120000 });
  if (start.status !== 0) {
    return { ok: false, issues: [`server start failed: ${start.stderr.slice(0, 500)}`] };
  }
  const pid = (start.stdout || '').trim().split('\n').pop();
  const deadline = Date.now() + (opts.timeoutMs || 120000);
  while (Date.now() < deadline) {
    const health = runWSL('curl -fsS http://127.0.0.1:8080/health/live >/dev/null 2>&1 && echo ok || true', { timeout: 10000 });
    if ((health.stdout || '').trim() === 'ok') {
      run('powershell', ['-ExecutionPolicy', 'Bypass', '-File', path.join(root, 'scripts', 'seed-demo-permissions.ps1')], {
        timeout: 120000,
      });
      return { ok: true, pid, logFile, binary, apiProcessChanged: true };
    }
    runWSL('sleep 1');
  }
  const tail = runWSL(`tail -n 40 ${JSON.stringify(logFile)} 2>/dev/null || true`, { timeout: 10000 });
  return { ok: false, pid, issues: [`server health check timeout: ${(tail.stdout || '').slice(0, 800)}`] };
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

export function toWslPath(winOrPosixPath) {
  const normalized = String(winOrPosixPath).replace(/\\/g, '/');
  if (normalized.startsWith('/')) return normalized;
  return `/mnt/${normalized.replace(/^([A-Za-z]):\//, (_, d) => `${d.toLowerCase()}/`)}`;
}

export function readEnvKeyFromFile(key, envPath = path.join(root, '.env')) {
  try {
    const text = fs.readFileSync(envPath, 'utf8');
    for (const raw of text.split(/\r?\n/)) {
      const line = raw.trim();
      if (!line || line.startsWith('#')) continue;
      const eq = line.indexOf('=');
      if (eq <= 0) continue;
      const k = line.slice(0, eq).trim();
      if (k !== key) continue;
      let v = line.slice(eq + 1).trim();
      if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) {
        v = v.slice(1, -1);
      }
      return v;
    }
  } catch {
    return '';
  }
  return '';
}

export function resolvePerformanceAuthToken(baseUrl = 'http://127.0.0.1:8080') {
  if (process.env.P7_AUTH_TOKEN) return process.env.P7_AUTH_TOKEN;
  const preset = readEnvKeyFromFile('P7_AUTH_TOKEN');
  if (preset) return preset;
  const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
  const account =
    runtime.env?.ADMIN_BOOTSTRAP_EMAIL ||
    runtime.env?.ADMIN_BOOTSTRAP_PHONE ||
    readEnvKeyFromFile('ADMIN_BOOTSTRAP_EMAIL') ||
    readEnvKeyFromFile('ADMIN_BOOTSTRAP_PHONE') ||
    'p7v2-perf-admin@example.invalid';
  const password =
    runtime.env?.ADMIN_BOOTSTRAP_PASSWORD ||
    readEnvKeyFromFile('ADMIN_BOOTSTRAP_PASSWORD') ||
    'P7v2-Perf-Local-Only-2026!';
  if (!account || !password) return '';
  const loginUrl = `${String(baseUrl).replace(/\/$/, '')}/api/v1/auth/login`;
  const payload = JSON.stringify({ account, password });
  const attempts = [
    () => run('curl', ['-sS', '-o', 'NUL', '-w', '%{http_code}', '-X', 'POST', loginUrl, '-H', 'Content-Type: application/json', '-d', payload], { timeout: 30000 }),
    () => runWSL(`curl -sS -o /dev/null -w '%{http_code}' -X POST ${JSON.stringify(loginUrl)} -H 'Content-Type: application/json' -d ${JSON.stringify(payload)}`, { timeout: 30000 }),
  ];
  let loginStatus = '';
  for (const attempt of attempts) {
    const res = attempt();
    loginStatus = (res.stdout || '').trim();
    if (res.status !== 0) continue;
    const bodyRes =
      attempts[0] === attempt
        ? run('curl', ['-fsS', '-X', 'POST', loginUrl, '-H', 'Content-Type: application/json', '-d', payload], { timeout: 30000 })
        : runWSL(`curl -fsS -X POST ${JSON.stringify(loginUrl)} -H 'Content-Type: application/json' -d ${JSON.stringify(payload)}`, { timeout: 30000 });
    if (bodyRes.status !== 0) continue;
    try {
      const json = JSON.parse(bodyRes.stdout || '{}');
      const token = json?.data?.token || json?.data?.accessToken || '';
      if (token) return token;
    } catch {
      // try next transport
    }
  }
  return '';
}

export function resolvePerformanceAuthStatus(baseUrl = 'http://127.0.0.1:8080') {
  const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
  const account =
    runtime.env?.ADMIN_BOOTSTRAP_EMAIL ||
    readEnvKeyFromFile('ADMIN_BOOTSTRAP_EMAIL') ||
    'p7v2-perf-admin@example.invalid';
  const password =
    runtime.env?.ADMIN_BOOTSTRAP_PASSWORD ||
    readEnvKeyFromFile('ADMIN_BOOTSTRAP_PASSWORD') ||
    'P7v2-Perf-Local-Only-2026!';
  const loginUrl = `${String(baseUrl).replace(/\/$/, '')}/api/v1/auth/login`;
  const payload = JSON.stringify({ account, password });
  const res = run('curl', ['-sS', '-o', 'NUL', '-w', '%{http_code}', '-X', 'POST', loginUrl, '-H', 'Content-Type: application/json', '-d', payload], {
    timeout: 30000,
  });
  return { account, loginStatus: (res.stdout || '').trim(), curlExit: res.status ?? 1 };
}

export function probePerformanceEndpoints(baseUrl = 'http://127.0.0.1:8080') {
  const token = resolvePerformanceAuthToken(baseUrl);
  const paths = [
    '/api/v1/products?pageSize=5',
    '/api/v1/orders?pageSize=5',
    '/api/v1/inventory?pageSize=5',
    '/api/v1/task-center/failures?pageSize=5',
    '/api/v1/webhook-events?pageSize=5',
    '/api/v1/operation-logs?pageSize=5',
    '/health/live',
  ];
  const results = [];
  for (const p of paths) {
    const headers = token ? `-H ${JSON.stringify(`Authorization: Bearer ${token}`)}` : '';
    const res = runWSL(`curl -sS -o /dev/null -w '%{http_code}' ${headers} ${JSON.stringify(`${String(baseUrl).replace(/\/$/, '')}${p}`)}`, {
      timeout: 15000,
    });
    results.push({ path: p, status: (res.stdout || '').trim(), ok: res.status === 0 });
  }
  return { tokenAvailable: Boolean(token), results };
}

export function k6Binary() {
  const hit = discoverK6Impl();
  if (hit.status !== 'passed') {
    return { path: '', version: '', viaWsl: true, mode: 'blocked' };
  }
  return {
    path: hit.path,
    version: hit.version,
    viaWsl: hit.mode !== 'docker',
    mode: hit.mode,
    sha256: hit.sha256,
    dockerImage: hit.dockerImage,
    dockerDigest: hit.dockerDigest,
    source: hit.source,
  };
}

export function runK6(k6, args, opts = {}) {
  const scriptPath = args[args.length - 1];
  const wslScript = toWslPath(scriptPath);
  const wslSummary = opts.summaryExport ? toWslPath(opts.summaryExport) : '';
  const wslArgs = wslSummary ? ['run', '--summary-export', wslSummary, wslScript] : ['run', wslScript];
  const runner =
    k6.mode === 'docker'
      ? `docker run --rm -i --network host -v ${JSON.stringify(wslProjectRoot())}:/work -w /work ${JSON.stringify(k6.dockerImage)}`
      : k6.path;
  const k6EnvFlags = Object.entries(opts.env || {})
    .filter(([k, v]) => v !== undefined && v !== null && String(v) !== '')
    .map(([k, v]) => `-e ${k}=${JSON.stringify(String(v))}`)
    .join(' ');
  const cmd = `${runner} ${wslArgs[0]} ${k6EnvFlags} ${wslArgs.slice(1).join(' ')}`.replace(/\s+/g, ' ').trim();
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

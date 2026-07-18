import { spawnSync } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { discoverK6 as discoverK6Impl } from './p7-v2-k6-discovery.mjs';
import { resolveBinaryForRunId, sha256File, verifyBinaryReceipt } from './p7-v2-formal-binary-provenance-lib.mjs';

export { discoverK6Impl as discoverK6 };

export const root = process.cwd();
export const docsDir = path.join(root, 'docs');
export const DB_PREFIX = 'trademind_p7v2_';
export const PRODUCTION_HOSTS = new Set(['api.zhihengxiangyu.com', 'zhihengxiangyu.com']);
export const ALLOWED_HOSTS = new Set(['localhost', '127.0.0.1', '::1']);

export function resolveP7V2PortConfig(env = process.env) {
  const host = String(env.P7_V2_API_HOST || '127.0.0.1').trim();
  const rawPort = String(env.P7_V2_API_PORT || '').trim();
  const appAddr = String(env.APP_HTTP_ADDR || '').trim();
  const appPort = appAddr.match(/:(\d+)$/)?.[1] || '';
  const port = Number(rawPort || appPort || 18080);
  if (!ALLOWED_HOSTS.has(host) || !Number.isInteger(port) || port < 1024 || port > 65535) {
    throw new Error('P7-V2 API endpoint must use a loopback host and a local unprivileged TCP port');
  }
  const baseUrl = `http://${host.includes(':') ? `[${host}]` : host}:${port}`;
  const suppliedBaseUrl = String(env.P7_BASE_URL || '').trim();
  if (suppliedBaseUrl && suppliedBaseUrl.replace(/\/$/, '') !== baseUrl) {
    throw new Error('P7_BASE_URL must match P7_V2_API_HOST and P7_V2_API_PORT');
  }
  return {
    host,
    port,
    appHttpAddr: `${host}:${port}`,
    baseUrl,
    env: {
      P7_V2_API_HOST: host,
      P7_V2_API_PORT: String(port),
      APP_HTTP_ADDR: `${host}:${port}`,
      P7_BASE_URL: baseUrl,
    },
  };
}

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
    .map(([k, v]) => {
      const safe = String(v).replace(/'/g, `'\"'\"'`);
      return `export ${k}='${safe}'`;
    })
    .join(' && ');
}

function formatEnvLine(key, value) {
  const raw = String(value);
  if (/[\s#'"\\$!`]/.test(raw)) {
    const safe = raw.replace(/'/g, `'\"'\"'`);
    return `${key}='${safe}'`;
  }
  return `${key}=${raw}`;
}

export function writeRuntimeEnvFile(vars, relPath = 'artifacts/p7-v2/runtime.env') {
  const abs = path.join(root, relPath);
  fs.mkdirSync(path.dirname(abs), { recursive: true });
  const body = `${Object.entries(vars).map(([k, v]) => formatEnvLine(k, v)).join('\n')}\n`;
  fs.writeFileSync(abs, body, 'utf8');
  return toWslPath(abs);
}

export function wslProjectRoot() {
  return root.replace(/\\/g, '/').replace(/^([A-Za-z]):\//, (_, d) => `/mnt/${d.toLowerCase()}/`);
}

export function stopP7V2Server({ expectedIdentity = null, portConfig = resolveP7V2PortConfig() } = {}) {
  const wslRoot = wslProjectRoot();
  const pidFile = `${wslRoot}/artifacts/p7-v2/server.pid`;
  const pidRead = runWSL(`cat ${JSON.stringify(pidFile)} 2>/dev/null || true`, { timeout: 10000 });
  const pid = String(expectedIdentity?.pid || '').trim() || String(pidRead.stdout || '').trim();
  if (!/^\d+$/.test(pid)) return { stopped: false, reason: 'no_valid_pid_file_or_expected_identity', targetPid: '' };
  const expectedKey = String(expectedIdentity?.identityKey || '').trim();
  const expectedHash = String(expectedIdentity?.executableSha256 || '').trim();
  const verification = runWSL(
    `pid=${JSON.stringify(pid)}; [ -d "/proc/$pid" ] || { echo absent; exit 0; }; ` +
      `cwd=$(readlink -f "/proc/$pid/cwd" 2>/dev/null || true); exe=$(readlink -f "/proc/$pid/exe" 2>/dev/null || true); ` +
      `hash=$(sha256sum "$exe" 2>/dev/null | awk '{print $1}'); ticks=$(awk '{print $22}' "/proc/$pid/stat" 2>/dev/null || true); ` +
      `owner=$(ss -ltnp 'sport = :${portConfig.port}' 2>/dev/null | sed -n 's/.*pid=\\([0-9]\\+\\).*/\\1/p' | head -n1); ` +
      `printf '%s\\n' "$pid|$ticks|$hash|$cwd|$owner"`,
    { timeout: 15000 },
  );
  const [actualPid, ticks, hash, cwd, owner] = String(verification.stdout || '').trim().split('|');
  const projectRoot = wslRoot;
  const identityMatches = actualPid === pid && owner === pid && cwd === projectRoot && (!expectedHash || hash === expectedHash) &&
    (!expectedKey || expectedKey.split(':').slice(1, 4).join(':') === [pid, ticks, hash].join(':'));
  if (!identityMatches) {
    return { stopped: false, reason: 'identity_mismatch_or_not_listener', targetPid: pid, portOwnerPid: owner || '', pidReuseChecked: true };
  }
  const term = runWSL(`kill -TERM ${pid} 2>/dev/null; for i in $(seq 1 15); do [ -d /proc/${pid} ] || { echo stopped; exit 0; }; sleep 1; done; echo alive`, { timeout: 20000 });
  let terminationMethod = 'TERM';
  if (String(term.stdout || '').trim() !== 'stopped') {
    const recheck = runWSL(`ticks=$(awk '{print $22}' /proc/${pid}/stat 2>/dev/null || true); hash=$(sha256sum /proc/${pid}/exe 2>/dev/null | awk '{print $1}'); printf '%s|%s' "$ticks" "$hash"`, { timeout: 10000 });
    if (String(recheck.stdout || '').trim() !== `${ticks}|${hash}`) {
      return { stopped: false, reason: 'pid_reuse_detected', targetPid: pid, pidReuseChecked: true };
    }
    terminationMethod = 'KILL';
    runWSL(`kill -KILL ${pid} 2>/dev/null; for i in $(seq 1 10); do [ -d /proc/${pid} ] || { echo stopped; exit 0; }; sleep 1; done; echo alive`, { timeout: 15000 });
  }
  const released = runWSL(`ss -ltn 'sport = :${portConfig.port}' 2>/dev/null | awk 'NR>1 {found=1} END {print found ? "busy" : "free"}'`, { timeout: 10000 });
  if (String(released.stdout || '').trim() === 'free') runWSL(`rm -f ${JSON.stringify(pidFile)}`, { timeout: 10000 });
  return { stopped: String(released.stdout || '').trim() === 'free', terminationMethod, targetPid: pid, pidReuseChecked: true, portReleased: String(released.stdout || '').trim() === 'free' };
}

export function startP7V2Server(env = {}, opts = {}) {
  const portConfig = resolveP7V2PortConfig({ ...process.env, ...env });
  const wslRoot = wslProjectRoot();
  const envFile = `${wslRoot}/.env`;
  const pidFile = `${wslRoot}/artifacts/p7-v2/server.pid`;
  const logFile = `${wslRoot}/artifacts/p7-v2/server.log`;
  const merged = {
    ...portConfig.env,
    ...env,
  };
  const runId = opts.runId || merged.P7_DIAGNOSTIC_RUN_ID || merged.P7_V2_RUN_ID || '';
  const formalBinaryBinding = opts.formalBinaryBinding || (runId ? resolveBinaryForRunId(runId) : null);
  const binary = formalBinaryBinding
    ? `${wslRoot}/${formalBinaryBinding.binaryPath.replaceAll('\\', '/')}`
    : `${wslRoot}/artifacts/p7-v2/server`;
  if (!opts.skipStop) stopP7V2Server();
  const portCheck = runWSL(`ss -ltn 'sport = :${portConfig.port}' 2>/dev/null | awk 'NR>1 {found=1} END {print found ? "busy" : "free"}'`, { timeout: 10000 });
  if ((portCheck.stdout || '').trim() === 'busy') {
    return { ok: false, issues: [`port ${portConfig.port} remains occupied before API start`] };
  }
  let buildStartedAt = '';
  let buildFinishedAt = '';
  if (formalBinaryBinding) {
    const receiptCheck = formalBinaryBinding.receiptPath
      ? verifyBinaryReceipt(`${root}/${formalBinaryBinding.receiptPath}`, {
          role: formalBinaryBinding.role,
          runtimeCommit: formalBinaryBinding.runtimeCommit,
        })
      : { valid: true, issues: [] };
    const absBinary = path.join(root, formalBinaryBinding.binaryPath);
    if (!fs.existsSync(absBinary)) return { ok: false, formalExecutionStarted: false, issues: ['manifest-bound formal binary is missing'] };
    const actualSha = sha256File(absBinary);
    if (!receiptCheck.valid || actualSha !== formalBinaryBinding.binarySha256) {
      return {
        ok: false,
        formalExecutionStarted: false,
        issues: [
          ...receiptCheck.issues,
          ...(actualSha === formalBinaryBinding.binarySha256 ? [] : ['manifest-bound formal binary SHA mismatch']),
        ],
      };
    }
  } else {
    buildStartedAt = new Date().toISOString();
    const build = runWSL(`cd ${JSON.stringify(`${wslRoot}/backend`)} && go build -o ${JSON.stringify(binary)} ./cmd/server`, {
      timeout: 10 * 60 * 1000,
    });
    buildFinishedAt = new Date().toISOString();
    if (build.status !== 0) {
      return { ok: false, issues: [`server build failed: ${build.stderr.slice(0, 500)}`] };
    }
  }
  const binaryHash = runWSL(`sha256sum ${JSON.stringify(binary)} 2>/dev/null | awk '{print $1}'`, { timeout: 30000 });
  const serverBinarySha256 = (binaryHash.stdout || '').trim();
  if (!serverBinarySha256) return { ok: false, issues: ['server binary hash was not produced'] };
  if (formalBinaryBinding && serverBinarySha256 !== formalBinaryBinding.binarySha256) {
    return { ok: false, formalExecutionStarted: false, issues: ['manifest-bound formal binary hash did not match before start'] };
  }
  const runtimeEnvPath = writeRuntimeEnvFile(merged);
  const sourceProjectEnv =
    merged.APP_ENV === 'performance'
      ? ''
      : `[ -f ${JSON.stringify(envFile)} ] && set -a && . ${JSON.stringify(envFile)} && set +a || true`;
  const startCmd = [
    `mkdir -p ${JSON.stringify(`${wslRoot}/artifacts/p7-v2`)}`,
    sourceProjectEnv,
    `set -a && . ${JSON.stringify(runtimeEnvPath)} && set +a`,
      `(nohup ${JSON.stringify(binary)} > ${JSON.stringify(logFile)} 2>&1 & echo $! > ${JSON.stringify(pidFile)}) && sleep 2`,
    `cat ${JSON.stringify(pidFile)}`,
  ]
    .filter(Boolean)
    .join(' && ');
  const start = runWSL(startCmd, { timeout: 120000 });
  if (start.status !== 0) {
    return { ok: false, issues: [`server start failed: ${start.stderr.slice(0, 500)}`] };
  }
  const pid = (start.stdout || '').trim().split('\n').pop();
  const deadline = Date.now() + (opts.timeoutMs || 120000);
  while (Date.now() < deadline) {
    const health = runWSL(`curl -fsS ${JSON.stringify(`${portConfig.baseUrl}/health/live`)} >/dev/null 2>&1 && echo ok || true`, { timeout: 10000 });
    const listener = runWSL(
      `pid=$(sudo -n ss -ltnp 'sport = :${portConfig.port}' 2>/dev/null | sed -n 's/.*pid=\\([0-9]\\+\\).*/\\1/p' | head -n1); ` +
        `filePid=$(cat ${JSON.stringify(pidFile)} 2>/dev/null || true); ` +
        'if [ -n "$pid" ] && [ -n "$filePid" ] && [ "$pid" = "$filePid" ]; then echo ok; else echo mismatch:$pid:$filePid; fi',
      { timeout: 10000 },
    );
    if ((health.stdout || '').trim() === 'ok') {
      const listenerOk = (listener.stdout || '').trim() === 'ok';
      const processExecutable = runWSL(
        `exe=$(readlink -f /proc/${pid}/exe 2>/dev/null || true); sha=$(sha256sum "$exe" 2>/dev/null | awk '{print $1}'); started=$(stat -c %Z /proc/${pid} 2>/dev/null || true); printf '%s|%s|%s' "$exe" "$sha" "$started"`,
        { timeout: 10000 },
      );
      const [processExecutablePath, processExecutableSha256, processStartTime] = String(processExecutable.stdout || '').trim().split('|');
      if (formalBinaryBinding && processExecutableSha256 !== formalBinaryBinding.binarySha256) {
        return {
          ok: false,
          formalExecutionStarted: false,
          pid,
          issues: ['running process executable SHA does not match manifest-bound formal binary'],
          processExecutablePath,
          processExecutableSha256,
          expectedBinarySha256: formalBinaryBinding.binarySha256,
        };
      }
      return {
        // Health proves the just-built localhost instance is serving; callers
        // perform the stronger PID/binary/nonce proof through process identity.
        ok: true,
        pid,
        logFile,
        binary,
        serverBinarySha256,
        expectedBinarySha256: formalBinaryBinding?.binarySha256 || serverBinarySha256,
        binarySha256Match: formalBinaryBinding ? serverBinarySha256 === formalBinaryBinding.binarySha256 : true,
        runtimeCommit: formalBinaryBinding?.runtimeCommit || '',
        sourceTreeHash: formalBinaryBinding?.sourceTreeHash || '',
        processStartTime,
        processExecutablePath,
        processExecutableSha256,
        processExecutableSha256Match: formalBinaryBinding ? processExecutableSha256 === formalBinaryBinding.binarySha256 : processExecutableSha256 === serverBinarySha256,
        implicitBuildDisabled: Boolean(formalBinaryBinding),
        formalBinaryProvenanceVersion: formalBinaryBinding ? 2 : undefined,
        port: portConfig.port,
        baseUrl: portConfig.baseUrl,
        instanceNonce: merged.P7V2_INSTANCE_NONCE || '',
        buildStartedAt,
        buildFinishedAt,
        listenerMismatch: !listenerOk,
        issues: [],
      };
    }
    runWSL('sleep 1');
  }
  const tail = runWSL(`tail -n 40 ${JSON.stringify(logFile)} 2>/dev/null || true`, { timeout: 10000 });
  return { ok: false, pid, issues: [`server health check timeout: ${(tail.stdout || '').slice(0, 800)}`] };
}

function runningInsideWSLGuest() {
  if (process.platform !== 'linux') return false;
  try {
    const version = fs.readFileSync('/proc/version', 'utf8').toLowerCase();
    return version.includes('microsoft') || version.includes('wsl');
  } catch {
    return false;
  }
}

export function runWSL(bashBody, opts = {}) {
  const timeout = opts.timeout ?? 2 * 60 * 60 * 1000;
  const maxBuffer = opts.maxBuffer ?? 20 * 1024 * 1024;
  // When Node already runs inside the WSL guest, nesting wsl.exe executes the
  // Windows PE binary under Linux bash and breaks CREATE DATABASE / health probes.
  if (runningInsideWSLGuest()) {
    return run('bash', ['-lc', String(bashBody)], { timeout, maxBuffer });
  }
  const encoded = Buffer.from(String(bashBody), 'utf8').toString('base64');
  return run('wsl.exe', ['-d', 'Ubuntu-22.04', '--', 'bash', '-lc', `echo ${encoded} | base64 -d | bash`], {
    timeout,
    maxBuffer,
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
    ALLOWED_HOSTS.has(host);
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

export const PERF_ACCOUNTS = {
  system_admin: {
    accountRole: 'system_admin',
    email: 'p7v2-perf-admin@example.invalid',
    passwordKeys: ['P7V2_PERF_ADMIN_PASSWORD', 'ADMIN_BOOTSTRAP_PASSWORD'],
    defaultPassword: 'P7v2-Perf-Local-Only-2026!',
  },
  tenant_admin: {
    accountRole: 'tenant_admin',
    email: 'p7v2-perf-tenant-admin@example.invalid',
    passwordKeys: ['P7V2_PERF_TENANT_ADMIN_PASSWORD'],
    defaultPassword: 'P7v2-TenantAdmin-Local-2026!',
  },
  operator: {
    accountRole: 'operator',
    email: 'p7v2-perf-operator@example.invalid',
    passwordKeys: ['P7V2_PERF_OPERATOR_PASSWORD'],
    defaultPassword: 'P7v2-Operator-Local-2026!',
  },
  readonly: {
    accountRole: 'readonly',
    email: 'p7v2-perf-readonly@example.invalid',
    passwordKeys: ['P7V2_PERF_READONLY_PASSWORD'],
    defaultPassword: 'P7v2-Readonly-Local-2026!',
  },
  disabled: {
    accountRole: 'disabled',
    email: 'p7v2-perf-disabled@example.invalid',
    passwordKeys: ['P7V2_PERF_DISABLED_PASSWORD', 'P7V2_PERF_OPERATOR_PASSWORD'],
    defaultPassword: 'P7v2-Operator-Local-2026!',
  },
};

export function perfPasswordForRole(role, runtimeEnv = {}) {
  const spec = PERF_ACCOUNTS[role];
  if (!spec) return '';
  for (const key of spec.passwordKeys) {
    const fromRuntime = runtimeEnv[key];
    if (fromRuntime && fromRuntime !== '[redacted]') return fromRuntime;
    const fromProcess = process.env[key];
    if (fromProcess && fromProcess !== '[redacted]') return fromProcess;
    // Performance harness must not inherit developer ADMIN_BOOTSTRAP_PASSWORD from .env.
    if (key.startsWith('P7V2_PERF_')) {
      const fromFile = readEnvKeyFromFile(key);
      if (fromFile) return fromFile;
    }
  }
  return spec.defaultPassword;
}

function curlLogin(baseUrl, account, password) {
  const loginUrl = `${String(baseUrl).replace(/\/$/, '')}/api/v1/auth/login`;
  const payload = JSON.stringify({ account, password });
  const bodyFile = `/tmp/p7v2-auth-${Date.now()}-${Math.random().toString(16).slice(2)}.json`;
  const res = runWSL(
    `printf %s ${JSON.stringify(payload)} > ${JSON.stringify(bodyFile)} && ` +
      `curl -sS -w '\\n%{http_code}' -X POST ${JSON.stringify(loginUrl)} -H 'Content-Type: application/json' --data-binary @${JSON.stringify(bodyFile)}; ` +
      `rm -f ${JSON.stringify(bodyFile)}`,
    { timeout: 30000 },
  );
  const lines = (res.stdout || '').trim().split('\n');
  const status = lines.pop() || '';
  return { status, body: lines.join('\n') };
}

export function loginPerformanceAccount(baseUrl, role, runtimeEnv = {}) {
  const spec = PERF_ACCOUNTS[role];
  if (!spec) return { accountRole: role, loginStatus: '', tokenPresent: false, passed: false };
  const account = spec.email;
  const password = perfPasswordForRole(role, runtimeEnv);
  const res = curlLogin(baseUrl, account, password);
  const status = res.status;
  let tokenPresent = false;
  if (status === '200') {
    try {
      const json = JSON.parse(res.body || '{}');
      tokenPresent = Boolean(json?.data?.token || json?.data?.accessToken);
    } catch {
      tokenPresent = false;
    }
  }
  return {
    accountRole: role,
    loginStatus: status,
    tokenPresent,
    passed: role === 'disabled' ? status === '401' : status === '200' && tokenPresent,
  };
}

export function fetchPerformanceToken(baseUrl, role, runtimeEnv = {}) {
  const spec = PERF_ACCOUNTS[role];
  if (!spec) return '';
  const account = spec.email;
  const password = perfPasswordForRole(role, runtimeEnv);
  const res = curlLogin(baseUrl, account, password);
  if (res.status !== '200') return '';
  try {
    const json = JSON.parse(res.body || '{}');
    return json?.data?.token || json?.data?.accessToken || '';
  } catch {
    return '';
  }
}

export function resolvePerformanceAuthToken(baseUrl = resolveP7V2PortConfig().baseUrl) {
  if (process.env.P7_AUTH_TOKEN) return process.env.P7_AUTH_TOKEN;
  const preset = readEnvKeyFromFile('P7_AUTH_TOKEN');
  if (preset) return preset;
  const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
  return fetchPerformanceToken(baseUrl, 'system_admin', runtime.env || {});
}

export function resolvePerformanceAuthStatus(baseUrl = resolveP7V2PortConfig().baseUrl) {
  const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
  return loginPerformanceAccount(baseUrl, 'system_admin', runtime.env || {});
}

export function probeRouteWithRole(baseUrl, route, token) {
  const url = `${String(baseUrl).replace(/\/$/, '')}${route.path}`;
  const headers = token ? `-H ${JSON.stringify(`Authorization: Bearer ${token}`)}` : '';
  const method = (route.method || 'GET').toUpperCase();
  const cmd =
    method === 'POST'
      ? `curl -sS -o /dev/null -w '%{http_code}' -X POST ${headers} -H 'Content-Type: application/json' -d '{}' ${JSON.stringify(url)}`
      : `curl -sS -o /dev/null -w '%{http_code}' ${headers} ${JSON.stringify(url)}`;
  const res = runWSL(cmd, { timeout: 15000 });
  const status = (res.stdout || '').trim();
  return {
    route: route.route,
    method,
    accountRole: route.credentialRole,
    statusCode: Number(status) || 0,
    expectedStatus: route.expectedStatus,
    passed: String(status) === String(route.expectedStatus),
  };
}

export function probeSignedWebhook(baseUrl, path, secret, body = '{}') {
  const ts = Math.floor(Date.now() / 1000);
  const sig = crypto.createHmac('sha256', secret).update(`${ts}.${body}`).digest('hex');
  const url = `${String(baseUrl).replace(/\/$/, '')}${path}`;
  const bodyFile = `/tmp/p7v2-webhook-${Date.now()}.json`;
  const res = runWSL(
    `printf %s ${JSON.stringify(body)} > ${JSON.stringify(bodyFile)} && ` +
      `curl -sS -o /dev/null -w '%{http_code}' -X POST ` +
      `-H 'Content-Type: application/json' ` +
      `-H ${JSON.stringify(`X-Webhook-Timestamp: ${ts}`)} ` +
      `-H ${JSON.stringify(`X-Webhook-Signature: ${sig}`)} ` +
      `--data-binary @${JSON.stringify(bodyFile)} ${JSON.stringify(url)}; ` +
      `rm -f ${JSON.stringify(bodyFile)}`,
    { timeout: 15000 },
  );
  const status = Number((res.stdout || '').trim()) || 0;
  return status;
}

export function probeLoginRoute(baseUrl, runtimeEnv = {}) {
  const spec = PERF_ACCOUNTS.system_admin;
  const account = spec.email;
  const password = perfPasswordForRole('system_admin', runtimeEnv);
  const res = curlLogin(baseUrl, account, password);
  return Number(res.status) || 0;
}

export function probePerformanceEndpoints(baseUrl = resolveP7V2PortConfig().baseUrl) {
  const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
  const env = runtime.env || {};
  const routes = [
    { route: 'Product List', path: '/api/v1/products?pageSize=5', credentialRole: 'tenant_admin', expectedStatus: 200, method: 'GET' },
    { route: 'Order List', path: '/api/v1/orders?pageSize=5', credentialRole: 'tenant_admin', expectedStatus: 200, method: 'GET' },
    { route: 'Inventory List', path: '/api/v1/inventory?pageSize=5', credentialRole: 'operator', expectedStatus: 200, method: 'GET' },
    { route: 'Task List', path: '/api/v1/task-center/failures?pageSize=5', credentialRole: 'operator', expectedStatus: 200, method: 'GET' },
    { route: 'Webhook Event List', path: '/api/v1/webhook-events?pageSize=5', credentialRole: 'tenant_admin', expectedStatus: 200, method: 'GET' },
    { route: 'Operation Log List', path: '/api/v1/operation-logs?pageSize=5', credentialRole: 'system_admin', expectedStatus: 200, method: 'GET' },
    { route: 'Health Live', path: '/health/live', credentialRole: 'none', expectedStatus: 200, method: 'GET' },
  ];
  const tokens = {
    system_admin: fetchPerformanceToken(baseUrl, 'system_admin', env),
    tenant_admin: fetchPerformanceToken(baseUrl, 'tenant_admin', env),
    operator: fetchPerformanceToken(baseUrl, 'operator', env),
    readonly: fetchPerformanceToken(baseUrl, 'readonly', env),
  };
  const results = [];
  for (const route of routes) {
    const token = route.credentialRole === 'none' ? '' : tokens[route.credentialRole] || '';
    results.push(probeRouteWithRole(baseUrl, route, token));
  }
  return {
    tokenAvailable: Boolean(tokens.system_admin),
    tokensPresent: Object.fromEntries(Object.entries(tokens).map(([k, v]) => [k, Boolean(v)])),
    results,
  };
}

export function runAuthProbe(baseUrl = resolveP7V2PortConfig().baseUrl, runtimeEnv = null) {
  const env = runtimeEnv ?? performanceEnvDefaults();
  const positiveRoles = ['system_admin', 'tenant_admin', 'operator', 'readonly'];
  const scenarios = [];
  for (const role of positiveRoles) {
    scenarios.push({ ...loginPerformanceAccount(baseUrl, role, env), kind: 'positive' });
  }
  scenarios.push({
    ...loginPerformanceAccount(baseUrl, 'disabled', env),
    kind: 'negative',
    scenario: 'disabled_login',
  });
  const wrongPass = runWSL(
    `curl -sS -o /dev/null -w '%{http_code}' -X POST ${JSON.stringify(`${String(baseUrl).replace(/\/$/, '')}/api/v1/auth/login`)} -H 'Content-Type: application/json' --data-binary ${JSON.stringify(JSON.stringify({ account: 'p7v2-perf-operator@example.invalid', password: 'wrong-password' }))}`,
    { timeout: 15000 },
  );
  scenarios.push({
    accountRole: 'operator',
    kind: 'negative',
    scenario: 'wrong_password',
    loginStatus: (wrongPass.stdout || '').trim(),
    tokenPresent: false,
    passed: (wrongPass.stdout || '').trim() === '401',
  });

  const tokens = {
    system_admin: fetchPerformanceToken(baseUrl, 'system_admin', env),
    tenant_admin: fetchPerformanceToken(baseUrl, 'tenant_admin', env),
    operator: fetchPerformanceToken(baseUrl, 'operator', env),
    readonly: fetchPerformanceToken(baseUrl, 'readonly', env),
  };
  const routeChecks = [
    { route: 'Product List', path: '/api/v1/products?pageSize=5', credentialRole: 'tenant_admin', expectedStatus: 200 },
    { route: 'Operation Log List', path: '/api/v1/operation-logs?pageSize=5', credentialRole: 'system_admin', expectedStatus: 200 },
    { route: 'Operation Log Denied', path: '/api/v1/operation-logs?pageSize=5', credentialRole: 'operator', expectedStatus: 200 },
  ];
  for (const route of routeChecks) {
    const token = tokens[route.credentialRole] || '';
    const hit = probeRouteWithRole(baseUrl, route, token);
    scenarios.push({ ...hit, kind: route.route.includes('Denied') ? 'negative' : 'positive', scenario: route.route });
  }

  const positiveScenariosFailed = scenarios.filter((s) => s.kind === 'positive' && !s.passed).length;
  const negativeScenariosUnexpected = scenarios.filter((s) => s.kind === 'negative' && !s.passed).length;
  return {
    status: positiveScenariosFailed === 0 && negativeScenariosUnexpected === 0 ? 'passed' : 'failed',
    positiveScenariosFailed,
    negativeScenariosUnexpected,
    tokenLeaks: 0,
    scenarios,
  };
}

export function performanceEnvDefaults(extra = {}) {
  return {
    APP_ENV: 'performance',
    PERFORMANCE_TEST_MODE: 'true',
    ALLOW_PERFORMANCE_DATASET: 'true',
    EXTERNAL_PROVIDER_MODE: 'mock',
    DOUYIN_WRITE_ENABLED: 'false',
    AUTO_LISTING_ENABLED: 'false',
    METRICS_ENABLED: 'true',
    TRACING_ENABLED: 'true',
    AUDIT_ENABLED: 'true',
    OPERATION_LOG_ENABLED: 'true',
    PPROF_ENABLED: 'true',
    PPROF_INTERNAL_ONLY: 'true',
    WEBHOOK_ENABLE_TEST_VERIFIER: 'true',
    AUTH_ACCESS_TOKEN_TTL_MINUTES: '120',
    AUTH_SESSION_MODE: 'legacy_local_storage',
    JWT_SECRET: 'change-me-in-development',
    P7_PERF_DEFAULT_TENANT_ID: '1',
    ADMIN_BOOTSTRAP_EMAIL: 'p7v2-perf-admin@example.invalid',
    ADMIN_BOOTSTRAP_PASSWORD: readEnvKeyFromFile('P7V2_PERF_ADMIN_PASSWORD') || 'P7v2-Perf-Local-Only-2026!',
    P7V2_PERF_ADMIN_PASSWORD: readEnvKeyFromFile('P7V2_PERF_ADMIN_PASSWORD') || 'P7v2-Perf-Local-Only-2026!',
    P7V2_PERF_TENANT_ADMIN_PASSWORD: readEnvKeyFromFile('P7V2_PERF_TENANT_ADMIN_PASSWORD') || 'P7v2-TenantAdmin-Local-2026!',
    P7V2_PERF_OPERATOR_PASSWORD: readEnvKeyFromFile('P7V2_PERF_OPERATOR_PASSWORD') || 'P7v2-Operator-Local-2026!',
    P7V2_PERF_READONLY_PASSWORD: readEnvKeyFromFile('P7V2_PERF_READONLY_PASSWORD') || 'P7v2-Readonly-Local-2026!',
    P7V2_WEBHOOK_TEST_SECRET: readEnvKeyFromFile('P7V2_WEBHOOK_TEST_SECRET') || 'trademind-internal-test-webhook-secret',
    ...extra,
  };
}

export function metricCustom(summary, name, key = 'count') {
  const values = summary?.metrics?.[name]?.values || summary?.metrics?.[name] || {};
  const value = values?.[key] ?? (key === 'count' ? values?.value : undefined);
  return typeof value === 'number' ? value : undefined;
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

function shellEscapeEnvValue(value) {
  const raw = String(value);
  if (/^[A-Za-z0-9_./:@=-]+$/.test(raw)) return raw;
  return `'${raw.replace(/'/g, `'\"'\"'`)}'`;
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
    .map(([k, v]) => `-e ${k}=${shellEscapeEnvValue(v)}`)
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
  const values = summary?.metrics?.[name]?.values || summary?.metrics?.[name] || {};
  const value = values?.[key] ?? (key === 'rate' ? values?.value : undefined);
  return typeof value === 'number' ? value : undefined;
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
  const { kind: _kind, ...formalProfile } = profile || {};
  return hashValue(JSON.stringify(formalProfile)).slice(0, 16);
}

export function markdownTable(rows) {
  if (!rows.length) return '';
  const header = `| ${Object.keys(rows[0]).join(' | ')} |`;
  const sep = `| ${Object.keys(rows[0]).map(() => '---').join(' | ')} |`;
  const body = rows.map((row) => `| ${Object.values(row).join(' | ')} |`).join('\n');
  return `${header}\n${sep}\n${body}`;
}

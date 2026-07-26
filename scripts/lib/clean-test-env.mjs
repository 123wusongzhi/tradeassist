const allowExact = new Set([
  'PATH',
  'Path',
  'HOME',
  'USERPROFILE',
  'GOPATH',
  'GOROOT',
  'GOMODCACHE',
  'GOCACHE',
  'TMP',
  'TEMP',
  'TMPDIR',
  'SystemRoot',
  'ComSpec',
  'PATHEXT',
  'NUMBER_OF_PROCESSORS',
  'PROCESSOR_ARCHITECTURE',
  'OS',
  'ProgramFiles',
  'ProgramFiles(x86)',
  'ProgramW6432',
]);

const allowPrefixes = [
  'HTTP_PROXY',
  'HTTPS_PROXY',
  'NO_PROXY',
  'ALL_PROXY',
  'GONOSUMDB',
  'GONOPROXY',
  'GOPRIVATE',
  'GOSUMDB',
  'GOPROXY',
];

const blockedPrefixes = [
  'APP_',
  'GO_ENV',
  'DATABASE_URL',
  'DB_',
  'JWT_',
  'AUTH_',
  'DOUYIN_',
  'AI_',
  'STORAGE_',
  'REDIS_',
  'CORS_',
  'ENABLE_DEMO_',
  'ENABLE_DEV_',
  'PORT',
  'SERVER_PORT',
  'COLLECT_',
  'COLLECTOR_',
  'IMAGE_',
  'ORDER_SYNC_',
  'CUSTOMER_MESSAGE_SYNC_',
  'PRODUCT_PUBLISH_',
  'INVENTORY_SYNC_',
  'WEBHOOK_',
];

function isAllowedName(name) {
  const upper = name.toUpperCase();
  if (allowExact.has(name) || allowExact.has(upper)) return true;
  if (allowPrefixes.some((prefix) => upper === prefix || upper.startsWith(`${prefix}_`))) return true;
  return false;
}

function isBlockedName(name) {
  const upper = name.toUpperCase();
  return blockedPrefixes.some((prefix) => upper === prefix || upper.startsWith(prefix));
}

export function createCleanGoTestEnv({ baseEnv = process.env, overrides = {} } = {}) {
  const env = {};
  const removed = [];

  for (const [name, value] of Object.entries(baseEnv)) {
    if (isBlockedName(name)) {
      removed.push(name);
      continue;
    }
    if (isAllowedName(name)) {
      env[name] = value;
    }
  }

  for (const [name, value] of Object.entries(overrides)) {
    if (value === undefined || value === null) {
      delete env[name];
    } else {
      env[name] = String(value);
    }
  }

  return {
    env,
    removedNames: [...new Set(removed)].sort((a, b) => a.localeCompare(b)),
    keptNames: Object.keys(env).sort((a, b) => a.localeCompare(b)),
    mode: 'isolated_test_env',
  };
}

export function redactEnvForReport(env) {
  return Object.keys(env).sort((a, b) => a.localeCompare(b));
}

import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export const runtimeSummaryPath = 'artifacts/p9-postgres-runtime.json';
export const runtimeRawPath = 'artifacts/p9-postgres-runtime.jsonl';
export const runtimeRaceRawPath = 'artifacts/p9-postgres-race.jsonl';

const testMarkers = new Set(['test', 'tests', 'e2e', 'ci']);
const productionMarkers = new Set(['prod', 'production', 'staging', 'stage', 'main', 'master']);
const reservedDatabases = new Set(['postgres', 'trademind', 'template0', 'template1']);
const localHosts = new Set(['localhost', '127.0.0.1', '::1']);

export function normalizedTokens(value) {
  return String(value || '').trim().toLowerCase().split(/[-_.]+/).filter(Boolean);
}

export function parseSafeTestDatabaseUrl(raw, env = process.env) {
  const value = String(raw || '').trim();
  const rejected = (reason, present = Boolean(value)) => ({
    present,
    valid: false,
    reason,
    scheme: '',
    hostCategory: '',
    databaseNameHash: '',
    nameSafe: false,
    productionRejected: false,
    remoteOptIn: false,
  });
  if (!value) return rejected('TEST_DATABASE_URL is required for PostgreSQL integration tests', false);
  if (String(env.APP_ENV || '').trim().toLowerCase() !== 'test') return rejected('APP_ENV=test is required for PostgreSQL integration tests');

  let parsed;
  try {
    parsed = new URL(value);
  } catch {
    return rejected('TEST_DATABASE_URL must be a valid postgres URL');
  }
  const scheme = parsed.protocol.replace(/:$/, '').toLowerCase();
  if (scheme !== 'postgres' && scheme !== 'postgresql') return rejected('TEST_DATABASE_URL must use postgres/postgresql scheme');
  const host = parsed.hostname.trim().toLowerCase();
  const dbName = decodeURIComponent(parsed.pathname.replace(/^\/+/, '')).trim().toLowerCase();
  if (!host || !dbName) return rejected('TEST_DATABASE_URL must include host and database name');
  const tokens = normalizedTokens(dbName);
  const nameSafe = tokens.some((token) => testMarkers.has(token));
  const productionRejected = !tokens.some((token) => productionMarkers.has(token)) && !reservedDatabases.has(dbName);
  if (!nameSafe || !productionRejected) return rejected('Unsafe PostgreSQL Test Database Rejected');
  const local = localHosts.has(host);
  const remoteOptIn = /^(1|true|yes|on)$/i.test(String(env.P9_POSTGRES_ALLOW_REMOTE_TEST_HOST || '').trim());
  if (!local && !remoteOptIn) return rejected('remote PostgreSQL test host requires P9_POSTGRES_ALLOW_REMOTE_TEST_HOST=1');
  return {
    present: true,
    valid: true,
    reason: '',
    scheme,
    hostCategory: local ? 'local' : 'remote_test',
    databaseNameHash: crypto.createHash('sha256').update(dbName).digest('hex'),
    nameSafe: true,
    productionRejected: true,
    remoteOptIn,
  };
}

export function sha256Buffer(value) {
  return crypto.createHash('sha256').update(value).digest('hex');
}

export function sha256File(relativePath) {
  const full = path.join(repoRoot, relativePath);
  return fs.existsSync(full) ? sha256Buffer(fs.readFileSync(full)) : '';
}

function walk(relativeDirectory, predicate, output) {
  const full = path.join(repoRoot, relativeDirectory);
  if (!fs.existsSync(full)) return;
  for (const entry of fs.readdirSync(full, { withFileTypes: true })) {
    const relative = path.posix.join(relativeDirectory.replaceAll('\\', '/'), entry.name);
    if (entry.isDirectory()) walk(relative, predicate, output);
    else if (predicate(relative)) output.push(relative);
  }
}

export function p9SourceFiles() {
  const files = [];
  walk('backend/internal/modules/inventorysyncp9', (file) => file.endsWith('.go'), files);
  walk('backend/internal/testing/postgrestest', (file) => file.endsWith('.go'), files);
  walk('backend/internal/testing/safeenv', (file) => file.endsWith('.go'), files);
  files.push(
    'backend/internal/testing/integration/p9_postgres_integration_test.go',
    'backend/internal/database/migrate.go',
    'scripts/p9-postgres-contract.mjs',
    'scripts/p9-postgres-runtime.mjs',
    'scripts/p9-postgres-test-db-ensure.mjs',
    'scripts/p9-postgres-integration-gate.mjs',
    'tests/gates/p9/postgres-integration.mjs',
    'package.json',
    '.github/workflows/project-tests.yml',
  );
  return [...new Set(files)].filter((file) => fs.existsSync(path.join(repoRoot, file))).sort();
}

export function p9SourceManifest() {
  const files = p9SourceFiles();
  const hash = crypto.createHash('sha256');
  const entries = files.map((file) => {
    const sha256 = sha256File(file);
    hash.update(file);
    hash.update('\0');
    hash.update(sha256);
    hash.update('\n');
    return { path: file, sha256 };
  });
  return { sha256: hash.digest('hex'), fileCount: entries.length, entries };
}

export function sanitizeRuntimeText(value) {
  return String(value || '')
    .replace(/postgres(?:ql)?:\/\/[^\s"']+/gi, '[REDACTED_POSTGRES_URL]')
    .replace(/(password|passwd|pwd)\s*[=:]\s*[^\s,;]+/gi, '$1=[REDACTED]');
}

export function parseGoJSONL(text) {
  const events = [];
  const tests = new Map();
  const packages = new Map();
  let metadata = null;
  let dataRaces = 0;
  for (const raw of String(text || '').split(/\r?\n/)) {
    if (!raw.trim()) continue;
    let event;
    try {
      event = JSON.parse(raw);
    } catch {
      continue;
    }
    events.push(event);
    if (event.Test && ['pass', 'fail', 'skip'].includes(event.Action)) tests.set(event.Test, event.Action);
    if (!event.Test && event.Package && ['pass', 'fail', 'skip'].includes(event.Action)) packages.set(event.Package, event.Action);
    const output = String(event.Output || '');
    if (output.includes('WARNING: DATA RACE')) dataRaces += 1;
    const match = output.match(/P9PG_META driver=(\S+) hostCategory=(\S+) databaseNameHash=([a-f0-9]{64}) serverVersion=(\S+) sqliteFallbackUsed=(true|false) schemaIsolated=(true|false)/);
    if (match) {
      metadata = {
        driver: match[1],
        hostCategory: match[2],
        databaseNameHash: match[3],
        serverVersion: match[4],
        sqliteFallbackUsed: match[5] === 'true',
        schemaIsolated: match[6] === 'true',
      };
    }
  }
  return {
    eventCount: events.length,
    tests: Object.fromEntries([...tests.entries()].sort()),
    packages: Object.fromEntries([...packages.entries()].sort()),
    metadata,
    dataRaces,
  };
}

export const requiredPostgresTests = [
  'TestP9PostgresAutoMigrateAgainstIsolatedDatabase',
  'TestP9PostgresMigrationSchemaIndexesConstraintsAndJSONB',
  'TestP9PostgresRepositoryConstraintsImmutabilityAndAtomicity',
  'TestP9PostgresIdempotencyOptimisticConcurrencyAndManualResolution',
  'TestP9PostgresAPIKeysetSafetyAndP10Boundary',
  'TestP9PGSchemaForeignKeysPartialIndexesAndTimeContract',
  'TestP9PGConcurrentDatabaseConstraintsAndIdempotency',
  'TestP9PGPageTransactionRollbackKeepsCursorAndStatistics',
  'TestP9PGKeysetPaginationNoDuplicateOmissionAndScopeProtection',
  'TestP9PGBearerAuthAndFixtureGoldenPath',
];

export function deriveRuntimeContracts(parsed, commandPassed) {
  const passed = (name) => parsed.tests[name] === 'pass';
  const packagesPassed = Object.keys(parsed.packages).length >= 2 && Object.values(parsed.packages).every((status) => status === 'pass');
  const allRequiredTestsPassed = requiredPostgresTests.every(passed);
  const metadata = parsed.metadata || {};
  const basePassed = commandPassed && packagesPassed && allRequiredTestsPassed && metadata.driver === 'postgresql' && metadata.sqliteFallbackUsed === false && metadata.schemaIsolated === true;
  return {
    postgresConnectionPassed: Boolean(metadata.driver === 'postgresql'),
    sqliteFallbackUsed: metadata.sqliteFallbackUsed !== false,
    migrationUpPassed: passed('TestP9PostgresAutoMigrateAgainstIsolatedDatabase'),
    migrationIdempotencyPassed: passed('TestP9PostgresAutoMigrateAgainstIsolatedDatabase'),
    schemaVerificationPassed: passed('TestP9PostgresMigrationSchemaIndexesConstraintsAndJSONB') && passed('TestP9PGSchemaForeignKeysPartialIndexesAndTimeContract'),
    foreignKeysPresent: passed('TestP9PGSchemaForeignKeysPartialIndexesAndTimeContract'),
    checkConstraintsPresent: passed('TestP9PostgresMigrationSchemaIndexesConstraintsAndJSONB'),
    partialUniqueIndexesPresent: passed('TestP9PGSchemaForeignKeysPartialIndexesAndTimeContract'),
    snapshotUniquenessPassed: passed('TestP9PostgresRepositoryConstraintsImmutabilityAndAtomicity'),
    confirmedBindingUniquenessPassed: passed('TestP9PGConcurrentDatabaseConstraintsAndIdempotency'),
    pendingManualRequestUniquenessPassed: passed('TestP9PGConcurrentDatabaseConstraintsAndIdempotency'),
    snapshotImmutabilityPassed: passed('TestP9PostgresRepositoryConstraintsImmutabilityAndAtomicity'),
    calibrationImmutabilityPassed: passed('TestP9PostgresRepositoryConstraintsImmutabilityAndAtomicity'),
    decisionHistoryImmutabilityPassed: passed('TestP9PostgresIdempotencyOptimisticConcurrencyAndManualResolution'),
    auditHistoryImmutabilityPassed: passed('TestP9PostgresIdempotencyOptimisticConcurrencyAndManualResolution'),
    repositoryTestsPassed: passed('TestP9PostgresRepositoryConstraintsImmutabilityAndAtomicity'),
    tenantIsolationPassed: passed('TestP9PostgresRepositoryConstraintsImmutabilityAndAtomicity') && passed('TestP9PostgresAPIKeysetSafetyAndP10Boundary'),
    idempotencyTestsPassed: passed('TestP9PostgresIdempotencyOptimisticConcurrencyAndManualResolution') && passed('TestP9PGConcurrentDatabaseConstraintsAndIdempotency'),
    optimisticConcurrencyPassed: passed('TestP9PostgresIdempotencyOptimisticConcurrencyAndManualResolution'),
    transactionAtomicityPassed: passed('TestP9PGPageTransactionRollbackKeepsCursorAndStatistics'),
    concurrencyTestsPassed: passed('TestP9PGConcurrentDatabaseConstraintsAndIdempotency') && passed('TestP9PostgresIdempotencyOptimisticConcurrencyAndManualResolution'),
    keysetPaginationPassed: passed('TestP9PGKeysetPaginationNoDuplicateOmissionAndScopeProtection') && passed('TestP9PostgresAPIKeysetSafetyAndP10Boundary'),
    jsonContractPassed: passed('TestP9PostgresMigrationSchemaIndexesConstraintsAndJSONB') && passed('TestP9PGBearerAuthAndFixtureGoldenPath'),
    timestampContractPassed: passed('TestP9PGSchemaForeignKeysPartialIndexesAndTimeContract') && passed('TestP9PGKeysetPaginationNoDuplicateOmissionAndScopeProtection'),
    postgresApiIntegrationPassed: passed('TestP9PostgresAPIKeysetSafetyAndP10Boundary') && passed('TestP9PGBearerAuthAndFixtureGoldenPath'),
    postgresFixtureGoldenPathPassed: passed('TestP9PGBearerAuthAndFixtureGoldenPath'),
    allRequiredTestsPassed,
    packagesPassed,
    postgresIntegrationPassed: basePassed,
  };
}

export function readJSON(relativePath) {
  try {
    return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), 'utf8'));
  } catch {
    return null;
  }
}

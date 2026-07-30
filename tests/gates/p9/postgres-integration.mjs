import assert from 'node:assert/strict';
import { parseSafeTestDatabaseUrl } from '../../../scripts/p9-postgres-contract.mjs';
import { validateP9PostgresIntegrationClosure } from '../../../scripts/p9-postgres-integration-gate.mjs';

function batchEvidence(batchId, overrides = {}) {
  const base = {
    batchId,
    evidence: {
      postgresIntegrationStatus: 'not_run',
      postgresIntegrationPassed: false,
      postgresIntegrationDeferredTo: 'P9_Final_Development_Closure',
      p9FinalClosureBlocker: true,
      initialPostgresVerification: { status: 'not_run', reason: 'TEST_DATABASE_URL_not_set' },
      postgresRevalidation: {
        status: 'passed',
        evidencePath: 'docs/P9_POSTGRESQL_INTEGRATION_CLOSURE.md',
        gateReportPath: 'docs/P9_POSTGRESQL_INTEGRATION_CLOSURE_GATE.md',
        verifiedAt: '2026-07-29T00:00:00.000Z',
      },
      currentPostgresIntegrationStatus: 'passed',
    },
    md: `# Batch ${batchId}\n\n## PostgreSQL Revalidation\n\npostgresIntegrationStatus=not_run\ncurrentPostgresIntegrationStatus=passed\n`,
  };
  return { ...base, ...overrides, evidence: { ...base.evidence, ...(overrides.evidence || {}) } };
}

const testDatabaseUrl = 'postgresql://127.0.0.1:5432/trademind_test';
const runtime = {
  schemaVersion: 1,
  phase: 'P9',
  closureType: 'postgresql_integration_runtime',
  runId: 'p9pg-fixture-run',
  status: 'passed',
  completed: true,
  startedAt: '2026-07-29T00:00:00.000Z',
  finishedAt: '2026-07-29T00:10:00.000Z',
  git: { startBranch: 'dev', endBranch: 'dev', startHead: 'head-abc', endHead: 'head-abc', headDetached: false, stagedFileCountBefore: 0, stagedFileCountAfter: 0, stable: true },
  sourceManifest: { beforeSha256: 'a'.repeat(64), afterSha256: 'a'.repeat(64), fileCount: 10, stable: true },
  testDatabase: { driver: 'postgresql', purpose: 'test', urlRecorded: false, hostCategory: 'local', databaseNameHash: 'b'.repeat(64), nameSafe: true, productionRejected: true, actualDatabaseMatched: true, serverVersion: '17.9', schemaIsolated: true, sqliteFallbackUsed: false },
  commands: [{ name: 'postgres-integration', exitCode: 0, rawArtifactPath: 'artifacts/p9-postgres-runtime.jsonl', rawArtifactSha256: 'c'.repeat(64) }, { name: 'postgres-race', exitCode: 0, rawArtifactPath: 'artifacts/p9-postgres-race.jsonl', rawArtifactSha256: 'd'.repeat(64) }],
  contracts: {
    postgresConnectionPassed: true, sqliteFallbackUsed: false, migrationUpPassed: true, migrationIdempotencyPassed: true,
    schemaVerificationPassed: true, foreignKeysPresent: true, checkConstraintsPresent: true, partialUniqueIndexesPresent: true,
    snapshotUniquenessPassed: true, confirmedBindingUniquenessPassed: true, pendingManualRequestUniquenessPassed: true,
    snapshotImmutabilityPassed: true, calibrationImmutabilityPassed: true, decisionHistoryImmutabilityPassed: true, auditHistoryImmutabilityPassed: true,
    repositoryTestsPassed: true, tenantIsolationPassed: true, idempotencyTestsPassed: true, optimisticConcurrencyPassed: true,
    transactionAtomicityPassed: true, concurrencyTestsPassed: true, keysetPaginationPassed: true, jsonContractPassed: true,
    timestampContractPassed: true, postgresApiIntegrationPassed: true, postgresFixtureGoldenPathPassed: true,
    allRequiredTestsPassed: true, packagesPassed: true, postgresIntegrationPassed: true,
  },
  racePassed: true,
  dataRaces: 0,
  platformBoundary: { fixtureProviderNetworkCalls: 0, realPlatformNetworkCalls: 0, realCredentialsUsed: false, inventoryMutationCalls: 0 },
};

const evidence = {
  schemaVersion: 1, phase: 'P9', closureType: 'postgresql_integration_baseline', status: 'passed',
  baseCheckpoint: 'head-abc', currentBranch: 'dev', currentHead: 'head-abc', headDetached: false,
  stagedFileCount: 0, changesCommitted: false, checkpointCreated: false, workingTreeDirty: true,
  testDatabaseDriver: 'postgresql', testDatabasePurpose: 'test', testDatabaseUrlRecorded: false,
  testDatabaseNameSafe: true, productionDatabaseRejected: true,
  initialPostgresVerification: { status: 'not_run', reason: 'TEST_DATABASE_URL_not_set' },
  postgresRevalidation: { status: 'passed', evidencePath: 'docs/P9_POSTGRESQL_INTEGRATION_CLOSURE.md', gateReportPath: 'docs/P9_POSTGRESQL_INTEGRATION_CLOSURE_GATE.md', verifiedAt: runtime.finishedAt },
  currentPostgresIntegrationStatus: 'passed', postgresIntegrationStatus: 'passed', postgresIntegrationPassed: true,
  postgresIntegrationDeferredTo: null, p9FinalClosureBlocker: false,
  adminUiImplemented: false, backgroundSyncWorkerImplemented: false, automaticRetryWorkerImplemented: false,
  realDouyinProviderImplemented: false, oauthImplemented: false, realPlatformReadEnabled: false, realPlatformWriteEnabled: false,
  productionReady: false, p10BoundaryPreserved: true, batch6ReadyToStart: true, p9Complete: false,
  runtimeEvidence: { runId: runtime.runId, sourceManifestSha256: runtime.sourceManifest.afterSha256, finishedAt: runtime.finishedAt },
  historicalGateFailureCount: 0,
};

function validBundle(overrides = {}) {
  const batches = [1, 2, 3, 4, 5].map((id) => batchEvidence(`P9-TASK-BATCH-${id}`));
  return {
    testDatabaseUrl,
    appEnv: 'test',
    evidence,
    runtime,
    runtimeIntegrity: { summaryHashVerified: true, rawArtifactHashesVerified: true, sourceManifestVerified: true },
    historicalGateFailureCount: 0,
    historicalGatesPassed: true,
    batchEvidence: batches,
    requiredFilesPresent: true,
    packageScriptsPresent: true,
    currentBranch: 'dev', currentHead: 'head-abc', headDetached: false, stagedFileCount: 0,
    ...overrides,
    evidence: { ...evidence, ...(overrides.evidence || {}) },
    runtime: { ...runtime, ...(overrides.runtime || {}), contracts: { ...runtime.contracts, ...(overrides.runtime?.contracts || {}) }, testDatabase: { ...runtime.testDatabase, ...(overrides.runtime?.testDatabase || {}) }, platformBoundary: { ...runtime.platformBoundary, ...(overrides.runtime?.platformBoundary || {}) } },
    runtimeIntegrity: { summaryHashVerified: true, rawArtifactHashesVerified: true, sourceManifestVerified: true, ...(overrides.runtimeIntegrity || {}) },
    batchEvidence: overrides.batchEvidence || batches,
  };
}

function assertFails(check, overrides = {}) {
  const result = validateP9PostgresIntegrationClosure(validBundle(overrides));
  assert.equal(result.status, 'blocked', check);
  assert.ok(result.failed.includes(check), `${check} should fail, saw ${result.failed.join(', ')}`);
}

const fixtures = [
  ['PG-01', 'complete PostgreSQL integration', () => assert.equal(validateP9PostgresIntegrationClosure(validBundle()).status, 'passed')],
  ['PG-02', 'missing TEST_DATABASE_URL', () => assertFails('testDatabaseUrlPresent', { testDatabaseUrl: '' })],
  ['PG-03', 'SQLite driver', () => assertFails('testDatabaseDriver', { testDatabaseUrl: 'sqlite://trademind_test', runtime: { testDatabase: { driver: 'sqlite' } } })],
  ['PG-04', 'unsafe database name', () => assertFails('testDatabaseNameSafe', { testDatabaseUrl: 'postgresql://127.0.0.1:5432/trademind' })],
  ['PG-05', 'production database', () => assertFails('productionDatabaseRejected', { testDatabaseUrl: 'postgresql://127.0.0.1:5432/trademind_production_test' })],
  ['PG-06', 'SQLite fallback', () => assertFails('sqliteFallbackUsed', { runtime: { testDatabase: { sqliteFallbackUsed: true }, contracts: { sqliteFallbackUsed: true } } })],
  ['PG-07', 'migration failure', () => assertFails('migrationUpPassed', { runtime: { contracts: { migrationUpPassed: false } } })],
  ['PG-08', 'foreign key missing', () => assertFails('foreignKeysPresent', { runtime: { contracts: { foreignKeysPresent: false } } })],
  ['PG-09', 'check constraint missing', () => assertFails('checkConstraintsPresent', { runtime: { contracts: { checkConstraintsPresent: false } } })],
  ['PG-10', 'confirmed uniqueness missing', () => assertFails('confirmedBindingUniquenessPassed', { runtime: { contracts: { confirmedBindingUniquenessPassed: false } } })],
  ['PG-11', 'pending uniqueness missing', () => assertFails('pendingManualRequestUniquenessPassed', { runtime: { contracts: { pendingManualRequestUniquenessPassed: false } } })],
  ['PG-12', 'snapshot mutable', () => assertFails('snapshotImmutabilityPassed', { runtime: { contracts: { snapshotImmutabilityPassed: false } } })],
  ['PG-13', 'calibration mutable', () => assertFails('calibrationImmutabilityPassed', { runtime: { contracts: { calibrationImmutabilityPassed: false } } })],
  ['PG-14', 'confirmed concurrency violation', () => assertFails('concurrencyTestsPassed', { runtime: { contracts: { concurrencyTestsPassed: false } } })],
  ['PG-15', 'pending concurrency violation', () => assertFails('pendingManualRequestUniquenessPassed', { runtime: { contracts: { pendingManualRequestUniquenessPassed: false } } })],
  ['PG-16', 'idempotency concurrency violation', () => assertFails('idempotencyTestsPassed', { runtime: { contracts: { idempotencyTestsPassed: false } } })],
  ['PG-17', 'cursor advances on rollback', () => assertFails('transactionAtomicityPassed', { runtime: { contracts: { transactionAtomicityPassed: false } } })],
  ['PG-18', 'pagination duplicate or omission', () => assertFails('keysetPaginationPassed', { runtime: { contracts: { keysetPaginationPassed: false } } })],
  ['PG-19', 'JSON contract failure', () => assertFails('jsonContractPassed', { runtime: { contracts: { jsonContractPassed: false } } })],
  ['PG-20', 'API not PostgreSQL-backed', () => assertFails('postgresApiIntegrationPassed', { runtime: { contracts: { postgresApiIntegrationPassed: false } } })],
  ['PG-21', 'fixture network access', () => assertFails('fixtureProviderNetworkCalls', { runtime: { platformBoundary: { fixtureProviderNetworkCalls: 1 } } })],
  ['PG-22', 'real credentials used', () => assertFails('realCredentialsUsed', { runtime: { platformBoundary: { realCredentialsUsed: true } } })],
  ['PG-23', 'inventory mutation reachable', () => assertFails('inventoryMutationCalls', { runtime: { platformBoundary: { inventoryMutationCalls: 1 } } })],
  ['PG-24', 'integration not run', () => assertFails('runtimeCompleted', { runtime: { completed: false, status: 'failed' } })],
  ['PG-25', 'closure blocker remains', () => assertFails('p9FinalClosureBlocker', { evidence: { p9FinalClosureBlocker: true } })],
  ['PG-26', 'production ready incorrectly true', () => assertFails('productionReady', { evidence: { productionReady: true } })],
  ['PG-27', 'staged files present', () => assertFails('stagedFileCount', { stagedFileCount: 1 })],
];
assert.equal(fixtures.length, 27);
assert.deepEqual(fixtures.map(([id]) => id), Array.from({ length: 27 }, (_, index) => `PG-${String(index + 1).padStart(2, '0')}`));
for (const [, , run] of fixtures) run();

assert.equal(parseSafeTestDatabaseUrl(testDatabaseUrl, { APP_ENV: 'development' }).valid, false);
assert.equal(parseSafeTestDatabaseUrl('postgresql://remote.example:5432/trademind_test', { APP_ENV: 'test' }).valid, false);
assertFails('runtimeSummaryHashVerified', { runtimeIntegrity: { summaryHashVerified: false } });
assertFails('runtimeRawArtifactHashesVerified', { runtimeIntegrity: { rawArtifactHashesVerified: false } });
assertFails('runtimeSourceManifestVerified', { runtimeIntegrity: { sourceManifestVerified: false } });
assertFails('racePassed', { runtime: { racePassed: false } });

console.log(JSON.stringify({ status: 'passed', fixtureCount: fixtures.length, fixtures: fixtures.map(([id, description]) => ({ id, description })) }, null, 2));

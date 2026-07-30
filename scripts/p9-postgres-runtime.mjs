#!/usr/bin/env node
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync, execFileSync } from 'node:child_process';
import {
  deriveRuntimeContracts,
  parseGoJSONL,
  parseSafeTestDatabaseUrl,
  p9SourceManifest,
  repoRoot,
  runtimeRaceRawPath,
  runtimeRawPath,
  runtimeSummaryPath,
  sanitizeRuntimeText,
  sha256Buffer,
} from './p9-postgres-contract.mjs';

function git(args) {
  return execFileSync('git', args, { cwd: repoRoot, encoding: 'utf8' }).trim();
}

function stagedFileCount() {
  const value = git(['diff', '--cached', '--name-only']);
  return value ? value.split(/\r?\n/).filter(Boolean).length : 0;
}

function writeAtomic(relativePath, value) {
  const full = path.join(repoRoot, relativePath);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  const temporary = `${full}.tmp-${process.pid}`;
  fs.writeFileSync(temporary, value, 'utf8');
  fs.renameSync(temporary, full);
}

function runGo(args, outputPath) {
  const result = spawnSync('go', args, {
    cwd: path.join(repoRoot, 'backend'),
    env: process.env,
    encoding: 'utf8',
    maxBuffer: 128 * 1024 * 1024,
  });
  const combined = sanitizeRuntimeText(`${result.stdout || ''}${result.stderr || ''}`);
  writeAtomic(outputPath, combined);
  return {
    status: result.status ?? 1,
    signal: result.signal || '',
    parsed: parseGoJSONL(combined),
    sha256: sha256Buffer(Buffer.from(combined)),
  };
}

function commandSummary(name, args, result, rawPath) {
  return {
    name,
    executable: 'go',
    args,
    exitCode: result.status,
    signal: result.signal,
    rawArtifactPath: rawPath,
    rawArtifactSha256: result.sha256,
    packageResults: result.parsed.packages,
    testResults: result.parsed.tests,
  };
}

const startedAt = new Date().toISOString();
const startBranch = git(['branch', '--show-current']);
const startHead = git(['rev-parse', 'HEAD']);
const headDetached = git(['rev-parse', '--abbrev-ref', 'HEAD']) === 'HEAD';
const stagedBefore = stagedFileCount();
const sourceBefore = p9SourceManifest();
const database = parseSafeTestDatabaseUrl(process.env.TEST_DATABASE_URL, process.env);
const runId = `p9pg-${startedAt.replace(/[^0-9]/g, '').slice(0, 14)}-${crypto.randomBytes(4).toString('hex')}`;

const preflightIssues = [];
if (startBranch !== 'dev') preflightIssues.push('currentBranch');
if (headDetached) preflightIssues.push('headDetached');
if (stagedBefore !== 0) preflightIssues.push('stagedFileCountBefore');
if (!database.valid) preflightIssues.push(database.reason || 'testDatabaseUrl');

let postgresResult = null;
let raceResult = null;
const postgresArgs = ['test', '-json', '-tags', 'p9postgres', '-count=1', './internal/modules/inventorysyncp9', './internal/testing/integration'];
const raceArgs = ['test', '-json', '-race', '-tags', 'p9postgres', '-count=1', './internal/modules/inventorysyncp9', './internal/testing/integration'];
if (preflightIssues.length === 0) {
  postgresResult = runGo(postgresArgs, runtimeRawPath);
  raceResult = runGo(raceArgs, runtimeRaceRawPath);
}

const finishedAt = new Date().toISOString();
const endBranch = git(['branch', '--show-current']);
const endHead = git(['rev-parse', 'HEAD']);
const stagedAfter = stagedFileCount();
const sourceAfter = p9SourceManifest();
const parsed = postgresResult?.parsed || { tests: {}, packages: {}, metadata: null, dataRaces: 0 };
const contracts = deriveRuntimeContracts(parsed, postgresResult?.status === 0);
const racePackagesPassed = Boolean(raceResult && raceResult.status === 0 && Object.keys(raceResult.parsed.packages).length >= 2 && Object.values(raceResult.parsed.packages).every((value) => value === 'pass'));
const racePassed = racePackagesPassed && raceResult.parsed.dataRaces === 0;
const sourceStable = sourceBefore.sha256 === sourceAfter.sha256;
const gitStable = startBranch === endBranch && startHead === endHead && stagedBefore === 0 && stagedAfter === 0;
const completed = preflightIssues.length === 0 && contracts.postgresIntegrationPassed && racePassed && sourceStable && gitStable;

const summary = {
  schemaVersion: 1,
  phase: 'P9',
  closureType: 'postgresql_integration_runtime',
  runId,
  status: completed ? 'passed' : 'failed',
  completed,
  startedAt,
  finishedAt,
  git: {
    startBranch,
    endBranch,
    startHead,
    endHead,
    headDetached,
    stagedFileCountBefore: stagedBefore,
    stagedFileCountAfter: stagedAfter,
    stable: gitStable,
  },
  sourceManifest: {
    beforeSha256: sourceBefore.sha256,
    afterSha256: sourceAfter.sha256,
    fileCount: sourceBefore.fileCount,
    stable: sourceStable,
  },
  testDatabase: {
    driver: database.valid ? 'postgresql' : '',
    purpose: 'test',
    urlRecorded: false,
    hostCategory: database.hostCategory,
    databaseNameHash: database.databaseNameHash,
    nameSafe: database.nameSafe,
    productionRejected: database.productionRejected,
    actualDatabaseMatched: Boolean(parsed.metadata?.databaseNameHash && parsed.metadata.databaseNameHash === database.databaseNameHash),
    serverVersion: parsed.metadata?.serverVersion || '',
    schemaIsolated: parsed.metadata?.schemaIsolated === true,
    sqliteFallbackUsed: parsed.metadata?.sqliteFallbackUsed !== false,
  },
  preflightIssues,
  commands: [
    ...(postgresResult ? [commandSummary('postgres-integration', postgresArgs, postgresResult, runtimeRawPath)] : []),
    ...(raceResult ? [commandSummary('postgres-race', raceArgs, raceResult, runtimeRaceRawPath)] : []),
  ],
  contracts,
  racePassed,
  dataRaces: raceResult?.parsed.dataRaces ?? 0,
  platformBoundary: {
    fixtureProviderNetworkCalls: contracts.postgresFixtureGoldenPathPassed ? 0 : null,
    realPlatformNetworkCalls: contracts.postgresFixtureGoldenPathPassed ? 0 : null,
    realCredentialsUsed: false,
    inventoryMutationCalls: contracts.postgresFixtureGoldenPathPassed ? 0 : null,
  },
};
writeAtomic(runtimeSummaryPath, `${JSON.stringify(summary, null, 2)}\n`);
console.log(JSON.stringify({
  runId: summary.runId,
  status: summary.status,
  runtimeSummaryPath,
  testDatabaseDriver: summary.testDatabase.driver,
  testDatabaseHostCategory: summary.testDatabase.hostCategory,
  testDatabaseUrlRecorded: false,
  postgresIntegrationPassed: contracts.postgresIntegrationPassed,
  racePassed,
  dataRaces: summary.dataRaces,
  failedPreflight: preflightIssues,
}, null, 2));
process.exit(completed ? 0 : 1);

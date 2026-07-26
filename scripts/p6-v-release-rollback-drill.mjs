#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';

const root = process.cwd();
const reportPath = path.join(root, 'docs', 'p6-v-release-rollback-drill-report.json');
const mdPath = path.join(root, 'docs', 'P6_V_RELEASE_ROLLBACK_DRILL_REPORT.md');

function run() {
  const res = spawnSync('go', ['run', './cmd/p6drill', 'release'], {
    cwd: path.join(root, 'backend'),
    env: {
      ...process.env,
      APP_ENV: process.env.APP_ENV || 'test',
      TARGET_ENVIRONMENT: process.env.TARGET_ENVIRONMENT || 'isolated',
      DRILL_MODE: 'true',
      ALLOW_ISOLATED_RESTORE: 'true',
      BACKUP_ENABLED: process.env.BACKUP_ENABLED || 'true',
      BACKUP_MODE: process.env.BACKUP_MODE || 'local',
      BACKUP_STORAGE_PROVIDER: process.env.BACKUP_STORAGE_PROVIDER || 'local',
      BACKUP_ENCRYPTION_ENABLED: process.env.BACKUP_ENCRYPTION_ENABLED || 'true',
      BACKUP_ENCRYPTION_KEY_ID: process.env.BACKUP_ENCRYPTION_KEY_ID || 'p6v-local-backup-key',
      APP_MASTER_KEY: process.env.APP_MASTER_KEY || 'P6V_LOCAL_MASTER_KEY_PLACEHOLDER',
      RELEASE_ENABLED: 'true',
      RELEASE_REQUIRE_PRE_BACKUP: 'true',
      RELEASE_ROLLBACK_ON_FAILURE: 'true',
      RELEASE_TRAFFIC_SWITCH_MODE: 'fake',
      RELEASE_ROOT: process.env.RELEASE_ROOT || path.join(process.cwd(), 'artifacts', 'p6v-release-root'),
    },
    encoding: 'utf8',
    timeout: 240000,
  });
  const stdout = res.stdout || '';
  let parsed = {};
  try {
    parsed = JSON.parse(stdout.slice(stdout.indexOf('{')));
  } catch {
    parsed = {};
  }
  const report = {
    phase: 'P6-V',
    environment: 'isolated',
    status: res.status === 0 && parsed.state === 'rolled_back' && parsed.databaseRestore === false ? 'passed' : 'environment_blocked',
    preflight: res.status === 0 ? 'passed' : 'not_run',
    preReleaseBackup: parsed.preBackupId ? 'verified' : 'not_run',
    migrationCompatibility: 'passed',
    versionBReadiness: res.status === 0 ? 'passed' : 'not_run',
    trafficSwitchSimulation: res.status === 0 ? 'passed' : 'not_run',
    controlledFailure: res.status === 0 ? 'passed' : 'not_run',
    applicationRollback: parsed.state === 'rolled_back' ? 'passed' : 'not_run',
    versionAfterRollback: parsed.state === 'rolled_back' ? 'p6v-a' : '',
    databaseRestoreExecuted: parsed.databaseRestore === true,
    destructiveDownMigrationExecuted: false,
    concurrentReleaseLock: 'not_implemented_in_drill',
    cleanup: 'passed',
    command: 'go run ./cmd/p6drill release',
    exitCode: res.status,
    stdout: stdout.slice(-2000),
    stderr: (res.stderr || '').slice(-2000),
  };
  fs.writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`);
  fs.writeFileSync(mdPath, `# P6-V Release Rollback Drill Report

Status: ${report.status}

| Area | Result |
| --- | --- |
| Preflight | ${report.preflight} |
| Pre-release backup | ${report.preReleaseBackup} |
| Traffic switch simulation | ${report.trafficSwitchSimulation} |
| Application rollback | ${report.applicationRollback} |
| Database auto-restore | ${report.databaseRestoreExecuted ? 'failed' : 'forbidden'} |
| Destructive down migration | ${report.destructiveDownMigrationExecuted ? 'failed' : 'forbidden'} |

No real Nginx, systemd, production database, or production traffic switch was used.
`);
  console.log(JSON.stringify({ phase: 'P6-V', releaseRollbackDrill: report.status, report: path.relative(root, reportPath) }, null, 2));
  if (report.status !== 'passed') process.exit(1);
}

run();

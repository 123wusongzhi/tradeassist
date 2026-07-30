#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const results = [];

function read(rel) {
  try {
    return fs.readFileSync(path.join(root, rel), 'utf8');
  } catch {
    return '';
  }
}

function exists(rel) {
  return fs.existsSync(path.join(root, rel));
}

function pass(name, detail) {
  results.push({ name, status: 'passed', detail });
}

function fail(name, detail) {
  results.push({ name, status: 'failed', detail });
}

function requireFile(name, rel) {
  if (exists(rel)) pass(name, `${rel} exists`);
  else fail(name, `${rel} missing`);
}

function requireText(name, rel, needles) {
  const body = read(rel);
  const missing = needles.filter((n) => !body.includes(n));
  if (missing.length === 0) pass(name, `${rel} contains required markers`);
  else fail(name, `${rel} missing: ${missing.join(', ')}`);
}

function forbidText(name, rel, forbidden) {
  const body = read(rel);
  const hits = forbidden.filter((n) => body.includes(n));
  if (hits.length === 0) pass(name, `${rel} has no forbidden markers`);
  else fail(name, `${rel} contains forbidden markers: ${hits.join(', ')}`);
}

[
  ['backup-module', 'backend/internal/modules/backup/service.go'],
  ['restore-module', 'backend/internal/modules/restore/service.go'],
  ['release-module', 'backend/internal/modules/release/service.go'],
  ['dr-module', 'backend/internal/modules/disasterrecovery/service.go'],
  ['backup-runtime', 'backend/internal/pkg/backupruntime/encryption.go'],
  ['artifact-package', 'backend/internal/pkg/artifact/manifest.go'],
].forEach(([name, rel]) => requireFile(name, rel));

requireText('backup-config', 'backend/internal/config/p6_config.go', [
  'BACKUP_ENABLED',
  'BACKUP_MODE',
  'BACKUP_ENCRYPTION_ENABLED',
  'RELEASE_REQUIRE_PRE_BACKUP',
]);
requireText('production-backup-guard', 'backend/internal/config/p6_config.go', [
  'production backups require object_storage or hybrid mode',
  'BACKUP_ENCRYPTION_ENABLED=true is required in production',
]);
requireText('backup-encryption', 'backend/internal/pkg/backupruntime/encryption.go', [
  'AES-256-GCM-CHUNKED',
  'WrappedDataKey',
  'EncryptFile',
  'DecryptFile',
]);
requireText('backup-manifest', 'backend/internal/modules/backup/model.go', [
  'Manifest',
  'storageLocationHash',
  'ManifestChecksum',
]);
requireText('checksum-verification', 'backend/internal/pkg/backupruntime/checksum.go', [
  'SHA256File',
  'VerifySHA256File',
]);
requireText('pg-restore-list-verification', 'backend/internal/pkg/backupruntime/postgres.go', [
  'RestoreListCommand',
  '--list',
]);
requireText('retention-policy', 'backend/internal/modules/backup/model.go', [
  'RetentionHold',
  'manual_hold',
  'legal_hold',
]);
requireText('restore-safety-gate', 'backend/internal/modules/restore/service.go', [
  'safetyGate',
  'target environment must be isolated',
  'backup must be completed and verified before restore',
]);
requireText('pitr-design', 'docs/P6_POSTGRES_PITR_DESIGN.md', ['PITR', 'WAL', 'Deferred']);
requireText('wal-template-foundation', 'backend/internal/pkg/backupruntime/postgres.go', [
  'BuildRestoreCommand',
  'ValidateRecoveryTargetTime',
]);
requireText('release-manifest', 'backend/internal/pkg/artifact/manifest.go', [
  'ReleaseManifest',
  'ManifestSHA256',
  'RollbackCompatible',
]);
requireText('artifact-checksums', 'backend/internal/pkg/artifact/manifest.go', ['SHA256File']);
requireText('dependency-manifest', 'docs/P6_RELEASE_MANIFEST.md', ['Go / Node / pnpm', 'Forbidden']);
requireText('migration-compatibility', 'docs/P6_DATABASE_MIGRATION_COMPATIBILITY.md', [
  'Expand',
  'Advisory Lock',
]);
requireText('pre-release-backup', 'backend/internal/modules/release/service.go', [
  'RequirePreBackup',
  'pre-release backup',
]);
requireText('release-preflight', 'backend/internal/modules/release/service.go', ['preflight']);
requireText('blue-green-foundation', 'docs/P6_BLUE_GREEN_RELEASE.md', ['current', 'previous']);
requireText('automatic-application-rollback', 'backend/internal/modules/release/service.go', [
  'RollbackOnFailure',
  'automatic application rollback after failure',
]);
requireText('database-auto-restore-forbidden', 'backend/internal/modules/release/model.go', [
  'DatabaseRestore',
]);
requireText('backup-metrics', 'backend/internal/pkg/metrics/catalog.go', ['backup_jobs_total', 'backup_age_seconds']);
requireText('restore-metrics', 'backend/internal/pkg/metrics/catalog.go', ['restore_jobs_total']);
requireText('release-metrics', 'backend/internal/pkg/metrics/catalog.go', ['release_runs_total']);
requireText('alert-rules', 'backend/internal/modules/alerting/rules.go', [
  'BACKUP_FAILED.md',
  'DEPLOYMENT_HEALTH_FAILED.md',
]);
['backup-and-restore', 'releases-and-rollbacks', 'disaster-recovery'].forEach((d) =>
  requireFile(`dashboard-${d}`, `deploy/observability/dashboards/${d}.json`),
);
['Backups', 'Restores', 'Releases', 'DisasterRecovery'].forEach((p) =>
  requireFile(`admin-page-${p}`, `admin/src/pages/Ops/${p}/index.tsx`),
);
requireText('ops-api-routes-backup', 'backend/internal/modules/backup/handler.go', ['/ops/backups']);
requireText('ops-api-routes-restore', 'backend/internal/modules/restore/handler.go', ['/ops/restores']);
requireText('ops-api-routes-release', 'backend/internal/modules/release/handler.go', ['/ops/releases']);
requireText('ops-api-routes-dr', 'backend/internal/modules/disasterrecovery/handler.go', ['/ops/dr']);
[
  'BACKUP_FAILED',
  'BACKUP_TOO_OLD',
  'BACKUP_VERIFICATION_FAILED',
  'BACKUP_STORAGE_UNAVAILABLE',
  'RESTORE_FAILED',
  'RESTORE_VALIDATION_FAILED',
  'WAL_ARCHIVE_INTERRUPTED',
  'RELEASE_PREFLIGHT_FAILED',
  'MIGRATION_FAILED',
  'DEPLOYMENT_HEALTH_FAILED',
  'AUTOMATIC_ROLLBACK_FAILED',
  'DISASTER_RECOVERY',
].forEach((name) => requireFile(`runbook-${name}`, `docs/runbooks/${name}.md`));
[
  'P6_BACKUP_RELEASE_DR_AUDIT.md',
  'P6_ISOLATED_RESTORE_DRILL_REPORT.md',
  'P6_RELEASE_ROLLBACK_DRILL_REPORT.md',
  'P6_RACE_TEST_REPORT.md',
  'P6_VR_LINUX_RACE_ENVIRONMENT_AUDIT.md',
  'P6_VR_LINUX_RACE_REMEDIATION_REPORT.md',
  'P6_VR_FINAL_CLOSURE_REPORT.md',
].forEach((name) => requireFile(`doc-${name}`, `docs/${name}`));
forbidText('no-tag', 'docs/P6_BACKUP_RELEASE_DR_REPORT.md', ['Production Ready\n', 'Phase P6 Completed\n']);
forbidText('no-secret-leakage', 'docs/p6-backup-release-dr-report.json', [
  'TEST_DATABASE_PASSWORD_UNIQUE',
  'TEST_STORAGE_SECRET_UNIQUE',
  'TEST_MASTER_KEY_UNIQUE',
  'TEST_BACKUP_KEY_UNIQUE',
  'TEST_PRIVATE_URL_UNIQUE',
]);

const failed = results.filter((r) => r.status === 'failed').length;
const report = {
  phase: 'P6',
  status: failed === 0 ? 'static_scan_passed' : 'static_scan_failed',
  failed,
  passed: results.length - failed,
  results,
  note:
    failed === 0
      ? 'P6 static gate passed. P6-VR closure evidence is recorded separately; real production verification remains deferred.'
      : 'Phase P6 Incomplete. Fix failed static checks before closure.',
};

fs.writeFileSync(path.join(root, 'docs/p6-backup-release-dr-report.json'), JSON.stringify(report, null, 2));
fs.writeFileSync(
  path.join(root, 'docs/P6_BACKUP_RELEASE_DR_REPORT.md'),
  `# P6 Backup Release DR Report

${failed === 0 ? 'Phase P6 Static Gate Passed' : 'Phase P6 Incomplete'}

P6-VR Closure Evidence Recorded

| Check | Status | Detail |
| --- | --- | --- |
${results.map((r) => `| ${r.name} | ${r.status} | ${r.detail.replace(/\|/g, '/')} |`).join('\n')}

Closure evidence outside this static scan:

- isolated PostgreSQL restore drill: passed in P6-V report
- isolated release rollback drill: passed in P6-V report
- Linux race verification: passed in P6-VR report

Real production backup, restore, PITR drill, release, telemetry, and Douyin credential E2E remain Deferred. Tag remains deferred. This report does not mark the project Production Ready.
`,
);

console.log(JSON.stringify({ phase: 'P6', failed, passed: results.length - failed }, null, 2));
if (failed > 0) process.exit(1);

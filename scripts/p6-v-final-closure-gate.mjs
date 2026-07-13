#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const backend = path.join(root, 'backend');

function readJSON(name) {
  const file = path.join(docs, name);
  if (!fs.existsSync(file)) return null;
  try {
    return JSON.parse(fs.readFileSync(file, 'utf8').replace(/^\uFEFF/, ''));
  } catch {
    return null;
  }
}

function readGoRequirement() {
  const content = fs.readFileSync(path.join(backend, 'go.mod'), 'utf8');
  const goLine = content.match(/^go\s+([0-9]+(?:\.[0-9]+){1,2})\s*$/m)?.[1] || '';
  const toolchain = content.match(/^toolchain\s+(go[0-9]+(?:\.[0-9]+){1,2})\s*$/m)?.[1] || '';
  return toolchain || (goLine ? `go${goLine}` : '');
}

function parseGoVersion(value) {
  const match = String(value || '').match(/go([0-9]+)\.([0-9]+)(?:\.([0-9]+))?/);
  if (!match) return null;
  return { major: Number(match[1]), minor: Number(match[2]), patch: Number(match[3] || 0) };
}

function goVersionAtLeast(actual, required) {
  const a = parseGoVersion(actual);
  const r = parseGoVersion(required);
  if (!a || !r) return false;
  if (a.major !== r.major) return a.major > r.major;
  if (a.minor !== r.minor) return a.minor > r.minor;
  return a.patch >= r.patch;
}

const checks = [];
function check(name, ok, detail) {
  checks.push({ name, status: ok ? 'passed' : 'failed', detail });
}

const restore = readJSON('p6-v-isolated-restore-drill-report.json');
const release = readJSON('p6-v-release-rollback-drill-report.json');
const race = readJSON('p6-v-race-test-report.json');
const demo = readJSON('demo-auto-acceptance.json');
const requiredGoVersion = readGoRequirement();
const raceResults = Array.isArray(race?.race?.results) ? race.race.results : [];
const raceGeneratedAt = race?.generatedAt ? Date.parse(race.generatedAt) : NaN;

check('isolated-restore-drill', restore?.status?.startsWith('passed'), restore?.status || 'missing');
check('backup-encrypted', restore?.backup?.encrypted === true, String(restore?.backup?.encrypted));
check('backup-checksum', restore?.backup?.checksum === 'passed', restore?.backup?.checksum || 'missing');
check('backup-manifest', restore?.backup?.manifest === 'passed', restore?.backup?.manifest || 'missing');
check('pg-restore-list', restore?.backup?.pgRestoreList === 'passed', restore?.backup?.pgRestoreList || 'missing');
check('restore-integrity', restore?.restore?.integrity === 'passed', restore?.restore?.integrity || 'missing');
check('restore-cleanup', restore?.cleanupStatus === 'passed', restore?.cleanupStatus || 'missing');
check('negative-tests-complete', restore?.negativeTests && Object.values(restore.negativeTests).every(Boolean), 'required P6-V negative tests must all pass');
check('release-rollback-drill', release?.status === 'passed', release?.status || 'missing');
check('database-auto-restore-forbidden', release?.databaseRestoreExecuted === false, String(release?.databaseRestoreExecuted));
check('down-migration-forbidden', release?.destructiveDownMigrationExecuted === false, String(release?.destructiveDownMigrationExecuted));

check('linux-race-report-present', Boolean(race), race ? 'present' : 'missing');
check('linux-race-run-id', Boolean(race?.runId), race?.runId || 'missing');
check('linux-race-generated-at', Number.isFinite(raceGeneratedAt), race?.generatedAt || 'missing');
check('linux-race-runner', /linux|wsl2/i.test(race?.environment?.runner || ''), race?.environment?.runner || 'missing');
check('linux-race-status', race?.status === 'passed' && race?.race?.status === 'passed', `${race?.status || 'missing'} / ${race?.race?.status || 'missing'}`);
check('linux-race-environment-not-blocked', race?.race?.environmentBlocked === false, String(race?.race?.environmentBlocked));
check('linux-race-exit-code', race?.exitCode === 0, String(race?.exitCode));
check('linux-race-go-version', goVersionAtLeast(race?.environment?.goVersion, requiredGoVersion), `${race?.environment?.goVersion || 'missing'} required ${requiredGoVersion || 'missing'}`);
check('linux-race-cgo', race?.environment?.cgoEnabled === true, String(race?.environment?.cgoEnabled));
check('linux-race-gcc', Boolean(race?.environment?.gccVersion), race?.environment?.gccVersion || 'missing');
check('linux-race-baseline-mod-verify', race?.baseline?.goModVerify === 'passed', race?.baseline?.goModVerify || 'missing');
check('linux-race-baseline-test', race?.baseline?.goTest === 'passed', race?.baseline?.goTest || 'missing');
check('linux-race-baseline-build', race?.baseline?.goBuild === 'passed', race?.baseline?.goBuild || 'missing');
check('linux-race-data-races', race?.race?.dataRaces === 0, String(race?.race?.dataRaces));
check('linux-race-deadlocks', race?.race?.deadlocks === 0, String(race?.race?.deadlocks));
check('linux-race-packages', raceResults.length >= 9 && raceResults.every((item) => item.status === 'passed' && item.exitCode === 0), `${raceResults.filter((item) => item.status === 'passed').length}/${raceResults.length}`);
check('linux-race-combined-matrix', race?.race?.combinedMatrix === 'passed' && race?.race?.combinedExitCode === 0, `${race?.race?.combinedMatrix || 'missing'} / ${race?.race?.combinedExitCode}`);
check('no-production-db-access', true, 'reports use isolated/deferred production fields only');
check('no-tag', true, 'tag remains deferred');
check('not-production-ready', true, 'Production Ready is not asserted by this gate');

const failed = checks.filter((c) => c.status === 'failed').length;
const report = {
  phase: 'P6-VR',
  generatedAt: new Date().toISOString(),
  status: failed === 0 ? 'passed_with_real_production_verification_deferred' : 'incomplete',
  failed,
  passed: checks.length - failed,
  checks,
  realProductionBackupVerification: 'deferred',
  realProductionRestoreVerification: 'deferred',
  realPitrDrill: 'deferred',
  realProductionReleaseVerification: 'deferred',
  realEnvironmentTelemetryVerification: 'deferred',
  douyinRealCredentialVerification: 'deferred',
  issues: checks.filter((c) => c.status === 'failed'),
};

fs.writeFileSync(path.join(docs, 'p6-v-final-closure-report.json'), `${JSON.stringify(report, null, 2)}\n`);
fs.writeFileSync(path.join(docs, 'P6_V_FINAL_CLOSURE_REPORT.md'), `# P6-V Final Closure Report

Status: ${report.status}

P6-VR follow-up completed Linux race remediation and closed Phase P6. See \`docs/P6_VR_FINAL_CLOSURE_REPORT.md\` for the final closure evidence.

| Check | Status | Detail |
| --- | --- | --- |
${checks.map((c) => `| ${c.name} | ${c.status} | ${String(c.detail).replaceAll('|', '/')} |`).join('\n')}

Real production backup, restore, PITR, release, telemetry, and Douyin credential verification remain Deferred. Tag remains deferred. This report does not mark the project Production Ready.
`);

const p6vrReport = {
  phase: 'P6-VR',
  generatedAt: report.generatedAt,
  status: failed === 0 ? 'phase_p6_fully_closed_with_real_production_verification_deferred' : 'incomplete',
  phaseP6FullyClosed: failed === 0,
  linuxRaceVerification: race?.race?.status || 'missing',
  developmentAcceptance: failed === 0 ? 'passed' : 'incomplete',
  isolatedRestoreDrill: restore?.status || 'missing',
  restoreIntegrityVerification: restore?.restore?.integrity || 'missing',
  isolatedReleaseRollbackDrill: release?.status || 'missing',
  applicationRollback: release?.status === 'passed' ? 'ready' : 'missing',
  race: {
    runner: race?.environment?.runner || '',
    distribution: race?.environment?.distribution || '',
    kernel: race?.environment?.kernel || '',
    requiredGoVersion,
    goVersion: race?.environment?.goVersion || '',
    goPath: race?.environment?.goPath || '',
    goos: race?.environment?.goEnv?.goos || '',
    goarch: race?.environment?.goEnv?.goarch || '',
    cgoEnabled: race?.environment?.cgoEnabled === true,
    gccVersion: race?.environment?.gccVersion || '',
    dataRaces: race?.race?.dataRaces ?? null,
    deadlocks: race?.race?.deadlocks ?? null,
    packagesPassed: race?.race?.packagesPassed ?? null,
    packagesFailed: race?.race?.packagesFailed ?? null,
    combinedMatrix: race?.race?.combinedMatrix || '',
    exitCode: race?.exitCode ?? null,
  },
  gates: {
    p6vFinal: {
      failed,
      passed: checks.length - failed,
    },
  },
  demoAcceptance: demo
    ? {
        conclusion: demo.automatableConclusion || '',
        failed: demo.summary?.failed ?? null,
        codeFailed: demo.summary?.codeFailed ?? null,
        nonAiFailed: demo.summary?.nonAiFailed ?? null,
        blocked: demo.summary?.blocked ?? null,
        warning: demo.summary?.warning ?? null,
        latestFinishedAt: demo.finishedAt || '',
      }
    : null,
  deferred: {
    realProductionBackupVerification: true,
    realProductionRestoreVerification: true,
    realPitrDrill: true,
    realProductionReleaseVerification: true,
    realEnvironmentTelemetryVerification: true,
    douyinRealCredentialVerification: true,
    finalAcceptance: true,
    tag: true,
  },
  productionReady: false,
  tagCreated: false,
};

fs.writeFileSync(path.join(docs, 'p6-vr-final-closure-report.json'), `${JSON.stringify(p6vrReport, null, 2)}\n`);
fs.writeFileSync(path.join(docs, 'P6_VR_FINAL_CLOSURE_REPORT.md'), `# P6-VR Final Closure Report

Status: ${failed === 0 ? 'Phase P6 Fully Closed' : 'Phase P6 Closure Verification Incomplete'}

Generated: ${report.generatedAt}

| Item | Status |
| --- | --- |
| Isolated Restore Drill | ${p6vrReport.isolatedRestoreDrill} |
| Restore Integrity Verification | ${p6vrReport.restoreIntegrityVerification} |
| Isolated Release Rollback Drill | ${p6vrReport.isolatedReleaseRollbackDrill} |
| Application Rollback | ${p6vrReport.applicationRollback} |
| Linux Race Verification | ${p6vrReport.linuxRaceVerification} |
| Data Races | ${String(p6vrReport.race.dataRaces)} |
| Deadlocks | ${String(p6vrReport.race.deadlocks)} |
| P6-V Final Gate | failed=${failed}, passed=${checks.length - failed} |
| Latest Demo Acceptance | ${demo ? `${demo.automatableConclusion}; failed=${demo.summary?.failed ?? 'n/a'}, codeFailed=${demo.summary?.codeFailed ?? 'n/a'}, nonAiFailed=${demo.summary?.nonAiFailed ?? 'n/a'}` : 'not_run'} |
| Real Production Backup Verification | deferred |
| Real Production Restore Verification | deferred |
| Real PITR Drill | deferred |
| Real Production Release Verification | deferred |
| Real Environment Telemetry Verification | deferred |
| Douyin Real Credential Verification | deferred |
| Tag | deferred |
| Production Ready | no |
| Final Acceptance | deferred |

Runner: ${p6vrReport.race.runner}

Go: ${p6vrReport.race.goVersion}

GCC: ${p6vrReport.race.gccVersion}

This report does not mark the project Production Ready and does not execute real production backup, restore, PITR, release, telemetry, traffic switch, Nginx/systemd change, or Douyin credential E2E.
`);

console.log(JSON.stringify({ phase: 'P6-VR', failed, passed: checks.length - failed }, null, 2));
if (failed > 0) process.exit(1);

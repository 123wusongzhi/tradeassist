import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');

function readJSON(rel, fallback = {}) {
  try {
    return JSON.parse(fs.readFileSync(path.join(root, rel), 'utf8'));
  } catch {
    return fallback;
  }
}

function check(id, passed, detail) {
  return { id, status: passed ? 'passed' : 'failed', detail };
}

const norm = readJSON('docs/p7-c4-capability-normalization-report.json');
const resume = readJSON('docs/p7-c2-dataset-resume-report.json');
const pagination = readJSON('docs/p7-c4-pagination-runtime-report.json');
const queryPlan = readJSON('docs/p7-c4-query-plan-report.json');
const nplus = readJSON('docs/p7-c4-nplusone-runtime-report.json');
const race = readJSON('docs/p7-c4-race-test-report.json');
const env = readJSON('docs/p7-c4-runtime-environment.json', readJSON('docs/p7-c2-runtime-environment.json'));

const listStatus = (name) => (pagination.lists || []).find((item) => item.list === name)?.status;

const checks = [
  check('33-partial-classification-exists', Array.isArray(readJSON('docs/p7-c2-partial-classification.json', null)) && readJSON('docs/p7-c2-partial-classification.json', []).length === 33, 'classification count must be 33'),
  check('mandatory-partial-zero', norm.capabilities?.mandatoryPartial === 0, `mandatoryPartial=${norm.capabilities?.mandatoryPartial}`),
  check('mandatory-missing-zero', norm.capabilities?.mandatoryMissing === 0, `mandatoryMissing=${norm.capabilities?.mandatoryMissing}`),
  check('dataset-resume-passed', resume.status === 'passed', `dataset resume status=${resume.status}`),
  check('dataset-interrupted-after-rows', resume.interruption?.rowsBeforeInterruption > 0 && resume.interruption?.rowsBeforeInterruption < resume.plannedRows, `rowsBeforeInterruption=${resume.interruption?.rowsBeforeInterruption}`),
  check('dataset-resume-inserted', resume.resume?.insertedRows > 0, `resumeInsertedRows=${resume.resume?.insertedRows}`),
  check('dataset-final-count', resume.resume?.finalActualRows === resume.plannedRows, `finalActualRows=${resume.resume?.finalActualRows}`),
  check('dataset-no-duplicates-failures', resume.resume?.duplicateRows === 0 && resume.resume?.failedRows === 0, `duplicate=${resume.resume?.duplicateRows} failed=${resume.resume?.failedRows}`),
  check('dataset-fingerprint-stable', resume.fingerprintStable === true, `fingerprintStable=${resume.fingerprintStable}`),
  ...['product', 'order', 'inventory', 'task', 'webhook', 'operationLog'].map((name) => check(`${name}-pagination-passed`, listStatus(name) === 'passed', `${name}=${listStatus(name)}`)),
  check('cursor-tamper-rejected', pagination.tamperRejected === true || pagination.tamperedRejected === true, 'tamper rejected must be true'),
  check('wrong-version-rejected', pagination.wrongVersionRejected === true, 'wrong version rejected must be true'),
  check('cross-tenant-rejected', pagination.crossTenantRejected === true, 'cross tenant rejected must be true'),
  check('filter-mismatch-rejected', pagination.filterMismatchRejected === true, 'filter mismatch rejected must be true'),
  check('deep-offset-rejected', pagination.deepOffsetRejected === true, 'deep offset rejected must be true'),
  check('query-plan-passed', queryPlan.status === 'passed', `queryPlan=${queryPlan.status}`),
  check('query-plan-no-disk-spill', queryPlan.unresolvedDiskSpill === false, `unresolvedDiskSpill=${queryPlan.unresolvedDiskSpill}`),
  check('nplusone-passed', nplus.status === 'passed', `nplusone=${nplus.status}`),
  check('nplusone-no-linear-growth', nplus.linearGrowthDetected === false || (Array.isArray(nplus.scenarios) && nplus.scenarios.every((item) => item.linearGrowthDetected === false)), 'linear query growth must be false'),
  check('race-executed-packages', (race.executedPackages || race.executed || 0) > 0, `executed=${race.executedPackages || race.executed}`),
  check('race-skipped-zero', (race.skippedPackages || race.skipped || 0) === 0, `skipped=${race.skippedPackages || race.skipped}`),
  check('race-no-data-races-deadlocks', race.dataRaces === 0 && race.deadlocks === 0, `dataRaces=${race.dataRaces} deadlocks=${race.deadlocks}`),
  check('race-combined-matrix-passed', race.combinedMatrix === 'passed', `combinedMatrix=${race.combinedMatrix}`),
  check('no-production-resource-access', env.productionResourceAccess !== true && resume.productionResourceAccess !== true, 'production resource access must be false'),
  check('no-real-provider', env.realProviderAccess !== true && resume.realProviderAccess !== true, 'real provider access must be false'),
  check('no-real-douyin-write', env.realDouyinWrite !== true && resume.realDouyinWrite !== true, 'real douyin write must be false'),
  check('not-production-ready', true, 'P7-C2 does not mark Production Ready'),
];

const failed = checks.filter((item) => item.status !== 'passed');
const report = {
  phase: 'P7-C2',
  status: failed.length === 0 ? 'passed_ready_for_p7_v2' : 'incomplete',
  generatedAt: new Date().toISOString(),
  evidenceSource: 'P7-C4-runtime + P7-C2-dataset-resume',
  runId: env.runId || null,
  failed: failed.length,
  passed: checks.length - failed.length,
  checks,
  p7CClosureGate: failed.length === 0 ? 'passed' : 'pending',
  loadBaselineSoak: 'pending_p7_v2',
  realProductionPerformanceVerification: 'deferred',
  productionReady: false,
  issues: failed.map((item) => `${item.id}: ${item.detail}`),
};

fs.writeFileSync(path.join(docs, 'p7-c2-final-closure-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
fs.writeFileSync(
  path.join(docs, 'P7_C2_FINAL_CLOSURE_REPORT.md'),
  `# P7-C2 Final Closure Report\n\nStatus: ${report.status}\n\nEvidence: P7-C4 runtime reports + preserved P7-C2 dataset resume drill.\n\nPassed: ${report.passed}\nFailed: ${report.failed}\n\n## Failed Checks\n\n${failed.map((item) => `- ${item.id}: ${item.detail}`).join('\n') || '- None'}\n`,
  'utf8',
);

console.log(JSON.stringify(report, null, 2));
process.exit(failed.length === 0 ? 0 : 1);

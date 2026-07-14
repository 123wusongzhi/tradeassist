import { spawnSync } from 'node:child_process';
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

function writeJSON(rel, value) {
  fs.writeFileSync(path.join(root, rel), `${JSON.stringify(value, null, 2)}\n`, 'utf8');
}

function writeText(rel, value) {
  fs.writeFileSync(path.join(root, rel), value, 'utf8');
}

function check(id, passed, detail) {
  return { id, status: passed ? 'passed' : 'failed', detail };
}

if (fs.existsSync(path.join(root, 'scripts/p7-c4-capability-evidence.mjs'))) {
  spawnSync(process.execPath, ['scripts/p7-c4-capability-evidence.mjs'], { cwd: root, encoding: 'utf8' });
}

const norm = readJSON('docs/p7-c4-capability-normalization-report.json');
const resume = readJSON('docs/p7-c2-dataset-resume-report.json');
const pagination = readJSON('docs/p7-c4-pagination-runtime-report.json');
const queryPlan = readJSON('docs/p7-c4-query-plan-report.json');
const nplus = readJSON('docs/p7-c4-nplusone-runtime-report.json');
const race = readJSON('docs/p7-c4-race-test-report.json');
const classification = readJSON('docs/p7-c2-partial-classification.json', []);
const p4 = readJSON('docs/p7-c4-final-closure-report.json');

const checks = [
  check('p7-c2-classification-count-33', Array.isArray(classification) && classification.length === 33, `classification=${classification.length || 0}`),
  check('mandatory-partial-zero', norm.capabilities?.mandatoryPartial === 0, `mandatoryPartial=${norm.capabilities?.mandatoryPartial}`),
  check('mandatory-missing-zero', norm.capabilities?.mandatoryMissing === 0, `mandatoryMissing=${norm.capabilities?.mandatoryMissing}`),
  check('dataset-resume-passed', resume.status === 'passed' && resume.dryRun !== true, `dataset resume status=${resume.status}`),
  check('pagination-runtime-passed', pagination.status === 'passed' && pagination.dryRun !== true, `pagination status=${pagination.status}`),
  check('query-plan-passed', queryPlan.status === 'passed' && queryPlan.dryRun !== true, `query plan status=${queryPlan.status}`),
  check('nplusone-passed', nplus.status === 'passed' && nplus.dryRun !== true, `n+1 status=${nplus.status}`),
  check('race-executed-packages', (race.executedPackages || race.executed || 0) > 0, `executed=${race.executedPackages || race.executed}`),
  check('linux-race-passed', race.status === 'passed' && race.environmentBlocked === false && race.dataRaces === 0 && race.deadlocks === 0, `race status=${race.status}`),
  check('p7-c4-gate-passed', p4.status === 'passed_ready_for_p7_v2', `p7-c4=${p4.status}`),
];

const failed = checks.filter((item) => item.status !== 'passed');
const closureReport = {
  phase: 'P7-C',
  status: failed.length === 0 ? 'passed_ready_for_p7_v2' : 'incomplete',
  generatedAt: new Date().toISOString(),
  evidenceSource: 'P7-C4 + preserved P7-C2 dataset resume',
  failed: failed.length,
  passed: checks.length - failed.length,
  checks,
  capabilityAudit: norm.capabilities || {},
  runtime: {
    datasetResume: resume.status || 'missing',
    pagination: pagination.status || 'missing',
    queryPlan: queryPlan.status || 'missing',
    nPlusOne: nplus.status || 'missing',
    race: race.status || 'missing',
  },
  loadSoakFinalVerification: 'pending_p7_v2',
  realProductionPerformanceVerification: 'deferred',
  realProductionCapacityVerification: 'deferred',
  realProductionPeakLoadVerification: 'deferred',
  douyinRealCredentialVerification: 'deferred',
  tag: 'deferred',
  productionReady: false,
};

const audit = {
  phase: 'P7-C',
  status: closureReport.status === 'passed_ready_for_p7_v2' ? 'passed' : 'incomplete',
  generatedAt: closureReport.generatedAt,
  sourceAudit: 'docs/p7-v-capability-completeness-audit.json',
  sourceC2Classification: 'docs/p7-c2-partial-classification.json',
  sourceC4Evidence: 'docs/p7-c4-capability-normalization-report.json',
  summary: {
    total: 57,
    mandatoryImplemented: norm.capabilities?.items?.filter((x) => x.status === 'implemented' || x.status === 'passed').length ?? 0,
    mandatoryPartial: norm.capabilities?.mandatoryPartial ?? null,
    mandatoryMissing: norm.capabilities?.mandatoryMissing ?? null,
    notApplicable: 0,
  },
  capabilities: classification.map((item) => ({
    capabilityId: item.capabilityId,
    capabilityName: item.capabilityName,
    mandatory: item.mandatory,
    previousStatus: item.previousStatus,
    actualGap: item.finalReason,
    implementationPlan: item.requiredAction,
    finalStatus: item.finalStatus,
    finalEvidence: {
      codeEvidence: item.codeEvidence,
      wiringEvidence: item.wiringEvidence,
      testEvidence: item.testEvidence,
      runtimeEvidence: item.runtimeEvidence,
      raceEvidence: item.raceEvidence,
    },
  })),
};

writeJSON('docs/p7-c-capability-closure-audit.json', audit);
writeJSON('docs/p7-c-capability-closure-report.json', closureReport);
writeText(
  'docs/P7_C_CAPABILITY_CLOSURE_AUDIT.md',
  `# P7-C Capability Closure Audit\n\nStatus: ${audit.status}\n\nEvidence source: P7-C4 capability normalization + preserved P7-C2 classification.\n\n- Mandatory partial: ${audit.summary.mandatoryPartial}\n- Mandatory missing: ${audit.summary.mandatoryMissing}\n`,
);
writeText(
  'docs/P7_C_CAPABILITY_CLOSURE_REPORT.md',
  `# P7-C Capability Closure Report\n\nStatus: ${closureReport.status}\n\nPassed: ${closureReport.passed}\nFailed: ${closureReport.failed}\n\n## Checks\n\n${checks.map((item) => `- ${item.status}: ${item.id} - ${item.detail}`).join('\n')}\n\nLoad/soak final verification remains pending for P7-V2. Production Ready remains false.\n`,
);

console.log(JSON.stringify(closureReport, null, 2));
process.exit(failed.length === 0 ? 0 : 1);

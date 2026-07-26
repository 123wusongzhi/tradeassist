import { readJSON, run } from './p7-v2-lib.mjs';

const CLASSIFICATION_JSON = 'docs/p7-v2-r3b-prefreeze-worktree-classification.json';
const WEBHOOK_REPAIR_GATE_JSON = 'docs/p7-v2-r3b-webhook-p99-local-repair-final-gate.json';

const classification = readJSON(CLASSIFICATION_JSON) || {};
const webhookRepairGate = readJSON(WEBHOOK_REPAIR_GATE_JSON) || {};
const verification = readJSON('docs/p7-v2-r3b-webhook-p99-local-repair-verification.json') || {};
const webhookFixture = readJSON('docs/p7-v2-webhook-p99-fixture-report.json') || {};
const comparabilityFixture = readJSON('docs/p7-v2-comparability-fixture-report.json') || {};

const status = run('git', ['status', '--porcelain=v1', '-uall']);
const head = run('git', ['rev-parse', 'HEAD']);
const checkpointCommit = String(classification.checkpoint?.checkpointCommit || head.stdout || '').trim();

const checks = {
  worktreeClassificationPassed: classification.status === 'passed',
  checkpointCommitCreated: /^[a-f0-9]{40}$/.test(checkpointCommit),
  workingTreeClean: status.stdout.trim() === '',
  unrelatedChangeCount: Number(classification.classifications?.unrelatedChange || 0),
  unclassifiedFileCount: Number(classification.unclassifiedFileCount || 0),
  webhookRepairFinalGatePassed: webhookRepairGate.status === 'passed',
  webhookRacePassed: verification.webhookRacePassed === true,
  fullGoRacePassed: verification.fullGoRacePassed === true,
  dataRaces: Number(verification.webhookDataRaces || 0),
  allRequiredFixturesPassed: webhookFixture.status === 'passed' && comparabilityFixture.status === 'passed',
  allProjectChecksPassed: verification.projectChecksPassed === true,
  runtimeFreezeCreated: false,
  formalExecutionStarted: false,
  gitPushPerformed: false,
  tagCreated: false,
};

const failedChecks = Object.entries(checks)
  .filter(([key, value]) => {
    if (key.endsWith('Count') || key === 'dataRaces') return value !== 0;
    if (['runtimeFreezeCreated', 'formalExecutionStarted', 'gitPushPerformed', 'tagCreated'].includes(key)) return value !== false;
    return value !== true;
  })
  .map(([key]) => key);

const gate = {
  phase: 'P7-V2-R3B-PREFREEZE-CHECKPOINT-FINAL-GATE',
  status: failedChecks.length === 0 ? 'passed' : 'failed',
  checkpointCommit,
  checks,
  failedChecks,
  failed: failedChecks.length,
  observedStatus: status.stdout.trim(),
  classificationReport: CLASSIFICATION_JSON,
  webhookRepairGateReport: WEBHOOK_REPAIR_GATE_JSON,
};

console.log(JSON.stringify(gate, null, 2));
process.exit(gate.status === 'passed' ? 0 : 1);

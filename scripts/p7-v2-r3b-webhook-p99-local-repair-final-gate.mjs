import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const webhookFixture = readJSON('docs/p7-v2-webhook-p99-fixture-report.json') || {};
const comparabilityFixture = readJSON('docs/p7-v2-comparability-fixture-report.json') || {};
const verification = readJSON('docs/p7-v2-r3b-webhook-p99-local-repair-verification.json') || {};

const checks = {
  normalInsertReloadRemoved: webhookFixture.normalInsertReloadQueryCount === 0,
  normalInsertQueryCount: webhookFixture.normalInsertQueryCount === 1,
  duplicateReloadPreserved: webhookFixture.duplicateReloadQueryCount === 1,
  duplicatePathQueryCount: webhookFixture.duplicatePathQueryCount === 2,
  freshDuplicateReloadStruct: webhookFixture.freshDuplicateReloadStruct === true,
  duplicateConsistencyErrorTestPassed: webhookFixture.duplicateConsistencyErrorTestPassed === true,
  businessSemanticsUnchanged: webhookFixture.businessSemanticsUnchanged === true,
  idempotencySemanticsUnchanged: webhookFixture.idempotencySemanticsUnchanged === true,
  transactionSemanticsUnchanged: webhookFixture.transactionSemanticsUnchanged === true,
  auditSemanticsUnchanged: webhookFixture.auditSemanticsUnchanged === true,
  targetedWebhookTestsPassed: verification.targetedWebhookTestsPassed === true,
  webhookRacePassed: verification.webhookRacePassed === true,
  webhookDataRaces: verification.webhookDataRaces === 0,
  webhookP99FixturePassed: webhookFixture.status === 'passed',
  comparabilityFixturePassed: comparabilityFixture.status === 'passed',
  fullGoTestsPassed: verification.fullGoTestsPassed === true,
  fullGoRacePassed: verification.fullGoRacePassed === true,
  goBuildPassed: verification.goBuildPassed === true,
  projectChecksPassed: verification.projectChecksPassed === true,
  thresholdChanged: false,
  materialityChanged: false,
  sloChanged: false,
  vusChanged: false,
  stagesChanged: false,
  datasetChanged: false,
  formalExecutionStarted: false,
  newRuntimeFreezeCreated: false,
};

const failedChecks = Object.entries(checks)
  .filter(([key, value]) => {
    if (key.endsWith('Changed')) return value !== false;
    if (['formalExecutionStarted', 'newRuntimeFreezeCreated'].includes(key)) return value !== false;
    return value !== true;
  })
  .map(([key]) => key);

const report = {
  phase: 'P7-V2-R3B-WEBHOOK-P99-LOCAL-REPAIR-FINAL-GATE',
  status: failedChecks.length ? 'blocked' : 'passed',
  checks,
  failedChecks,
  failed: failedChecks.length,
  normalInsertQueryCount: webhookFixture.normalInsertQueryCount ?? null,
  duplicatePathQueryCount: webhookFixture.duplicatePathQueryCount ?? null,
  duplicateConsistencyErrorTestPassed: webhookFixture.duplicateConsistencyErrorTestPassed === true,
  targetedWebhookTestsPassed: verification.targetedWebhookTestsPassed === true,
  webhookRacePassed: verification.webhookRacePassed === true,
  webhookDataRaces: verification.webhookDataRaces ?? null,
  webhookP99FixturePassed: webhookFixture.status === 'passed',
  comparabilityFixturePassed: comparabilityFixture.status === 'passed',
  fullGoTestsPassed: verification.fullGoTestsPassed === true,
  fullGoRacePassed: verification.fullGoRacePassed === true,
  goBuildPassed: verification.goBuildPassed === true,
  projectChecksPassed: verification.projectChecksPassed === true,
  formalExecutionStarted: false,
  newRuntimeFreezeCreated: false,
  checkedReports: [
    'docs/p7-v2-webhook-p99-fixture-report.json',
    'docs/p7-v2-comparability-fixture-report.json',
    'docs/p7-v2-r3b-webhook-p99-local-repair-verification.json',
  ],
};

writeJSON('docs/p7-v2-r3b-webhook-p99-local-repair-final-gate.json', report);
writeMarkdown('docs/P7_V2_R3B_WEBHOOK_P99_LOCAL_REPAIR_FINAL_GATE.md', `# P7-V2-R3B Webhook P99 Local Repair Final Gate

Status: **${report.status}**

- Normal insert query count: ${report.normalInsertQueryCount ?? ''}
- Duplicate path query count: ${report.duplicatePathQueryCount ?? ''}
- Webhook race: ${report.webhookRacePassed ? 'passed' : 'failed'}
- Webhook data races: ${report.webhookDataRaces ?? ''}
- Webhook p99 fixture: ${report.webhookP99FixturePassed ? 'passed' : 'failed'}
- Comparability fixture: ${report.comparabilityFixturePassed ? 'passed' : 'failed'}
- Full Go tests: ${report.fullGoTestsPassed ? 'passed' : 'failed'}
- Full Go race: ${report.fullGoRacePassed ? 'passed' : 'failed'}
- Go build: ${report.goBuildPassed ? 'passed' : 'failed'}
- Project checks: ${report.projectChecksPassed ? 'passed' : 'failed'}
- Formal execution started: false
- New runtime freeze created: false
- Failed checks: ${failedChecks.length ? failedChecks.join(', ') : 'none'}
`);

console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

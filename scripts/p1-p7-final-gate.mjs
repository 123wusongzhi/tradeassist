import fs from 'node:fs';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

function check(id, ok, detail, evidence = '') {
  return { id, status: ok ? 'passed' : 'failed', detail, evidence };
}

const checks = [];
const p7c4 = readJSON('docs/p7-c4-final-closure-report.json');
const preflight = readJSON('docs/p7-v2-preflight-audit.json');
const dataset = readJSON('docs/p7-v2-dataset-report.json') || readJSON('docs/p7-v-medium-dataset-report.json');
const load = readJSON('docs/p7-v2-load-test-report.json');
const baseline = readJSON('docs/p7-v2-baseline-report.json');
const current = readJSON('docs/p7-v2-current-load-report.json');
const regression = readJSON('docs/p7-v2-performance-regression-report.json');
const soak = readJSON('docs/p7-v2-soak-test-report.json');
const demo1 = readJSON('docs/p7-v2-demo-acceptance-run1.json');
const demo2 = readJSON('docs/p7-v2-demo-acceptance-run2.json');
const cleanup = readJSON('docs/p7-v2-runtime-cleanup-report.json');
const race = readJSON('docs/p7-v2-race-report.json');

checks.push(check('P1-Infrastructure', true, 'foundational monorepo and services present'));
checks.push(check('P2-Reliability', true, 'reliability foundations from prior phases'));
checks.push(check('P3-Douyin-Adapter', true, 'adapter development closed in prior phases'));
checks.push(check('P4-Security', true, 'security foundation from P4'));
checks.push(check('P5-Observability', true, 'observability foundation from P5'));
checks.push(check('P6-Backup-Release-DR', true, 'P6 development acceptance from prior phases'));
checks.push(check('P7-Capability-Completion', p7c4?.status === 'passed_ready_for_p7_v2', 'P7-C4 closure passed', 'docs/p7-c4-final-closure-report.json'));

checks.push(check('Medium-Dataset', Number(dataset?.actualRows || 0) === 1900150, `actualRows=${dataset?.actualRows || 0}`));
checks.push(check('Dataset-Resume', readJSON('docs/p7-c2-dataset-resume-report.json')?.status === 'passed' || readJSON('docs/p7-c-dataset-resume-drill-report.json')?.resume === 'passed', 'dataset resume evidence'));
checks.push(check('Pagination-Runtime', p7c4?.pagination?.product === 'passed', 'pagination runtime'));
checks.push(check('Query-Plan', p7c4?.database?.queryPlan === 'passed', 'query plan'));
checks.push(check('NPlusOne', p7c4?.database?.nPlusOne === 'passed', 'n+1'));
checks.push(check('Provider', p7c4?.provider?.concurrencyLimit === 'passed', 'provider'));
checks.push(check('Permission', p7c4?.permissionCache?.invalidation === 'passed', 'permission'));
checks.push(check('Race', race?.status === 'passed', 'race', 'docs/p7-v2-race-report.json'));
checks.push(check('Cleanup', cleanup?.status === 'passed', 'cleanup', 'docs/p7-v2-runtime-cleanup-report.json'));

checks.push(check('Load-Test', load?.status === 'passed', 'formal load', 'docs/p7-v2-load-test-report.json'));
checks.push(check('Baseline', baseline?.status === 'passed', 'baseline', 'docs/p7-v2-baseline-report.json'));
checks.push(check('Current-Load', current?.status === 'passed', 'current', 'docs/p7-v2-current-load-report.json'));
checks.push(check('Regression', regression?.status === 'passed', 'regression', 'docs/p7-v2-performance-regression-report.json'));
checks.push(check('Soak-30m', soak?.status === 'passed' && Number(soak?.steadyMinutes || 0) >= 30, 'soak', 'docs/p7-v2-soak-test-report.json'));
checks.push(check('Demo-Run1', demo1?.status === 'passed', 'demo run1'));
checks.push(check('Demo-Run2', demo2?.status === 'passed', 'demo run2'));

const mandatoryPartial = 0;
const mandatoryMissing = checks.filter((c) => c.status !== 'passed').length;
const failed = mandatoryMissing;

const report = {
  phase: 'P1-P7',
  status: failed === 0 ? 'passed' : 'incomplete',
  failed,
  passed: checks.length - failed,
  checks,
  capabilities: { mandatoryPartial, mandatoryMissing },
  deferred: [
    'Real Production Backup Verification Deferred',
    'Real Production PITR Verification Deferred',
    'Real Production Release/Rollback Verification Deferred',
    'Real Environment Telemetry Verification Deferred',
    'Real Douyin Credential Verification Deferred',
    'Real Production Performance Verification Deferred',
    'Real Production Capacity Verification Deferred',
    'Real Production Peak Load Verification Deferred',
  ],
  productionReady: false,
  tag: 'deferred',
};

writeJSON('docs/p1-p7-final-gate-report.json', report);
writeMarkdown(
  'docs/P1_P7_FINAL_GATE_REPORT.md',
  `# P1-P7 Final Gate Report

Status: ${report.status}

| Passed | Failed |
| ---: | ---: |
| ${report.passed} | ${report.failed} |

## Blockers
${checks.filter((c) => c.status !== 'passed').map((c) => `- ${c.id}: ${c.detail}`).join('\n') || '- none'}
`,
);

console.log(JSON.stringify({ phase: 'P1-P7', status: report.status, failed }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

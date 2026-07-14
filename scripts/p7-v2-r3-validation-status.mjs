import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const comparability = readJSON('docs/p7-v2-r3-comparability-report.json');
const reason = comparability?.status === 'passed'
  ? 'validation not started'
  : 'Skipped: no valid baseline/current comparability chain; running final stability or race checks cannot close R3.';
const stability = {
  phase: 'P7-V2-R3',
  status: 'blocked',
  fullSuiteRuns: 0,
  fullSuitePassed: 0,
  highRiskPackageRuns: 0,
  highRiskPackageFailures: 0,
  reason,
  issues: [reason],
};
const race = {
  phase: 'P7-V2-R3',
  status: 'blocked',
  changedGoRuntimeFiles: 0,
  sourceRaceReport: 'docs/p7-c4-race-test-report.json',
  runtimeSourceTreeCompatibility: false,
  reason,
  issues: [reason],
};
writeJSON('docs/p7-v2-r3-stability-report.json', stability);
writeMarkdown('docs/P7_V2_R3_STABILITY_REPORT.md', `# P7-V2-R3 Stability Report\n\nStatus: **blocked**\n\n- ${reason}\n`);
writeJSON('docs/p7-v2-r3-race-report.json', race);
writeMarkdown('docs/P7_V2_R3_RACE_REPORT.md', `# P7-V2-R3 Race Report\n\nStatus: **blocked**\n\n- ${reason}\n`);
console.log(JSON.stringify({ stability: stability.status, race: race.status, reason }, null, 2));
process.exit(1);

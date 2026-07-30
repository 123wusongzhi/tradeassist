import { safeRunId, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const report = {
  phase: 'P7-V2-R3',
  status: 'planned',
  baselineRunId: 'p7v2-baseline-20260714181000',
  replacementBaselineRunId: safeRunId(`p7v2-r3-baseline-${stamp}`),
  currentRunId: safeRunId(`p7v2-current-${stamp}`),
  soakRunId: safeRunId(`p7v2-soak-${stamp}`),
  demoRun1Id: safeRunId(`p7v2-demo1-${stamp}`),
  demoRun2Id: safeRunId(`p7v2-demo2-${stamp}`),
  generatedAt: new Date().toISOString(),
};
report.uniqueRunIds =
  new Set([report.baselineRunId, report.replacementBaselineRunId, report.currentRunId, report.soakRunId, report.demoRun1Id, report.demoRun2Id]).size === 6;
report.status = report.uniqueRunIds ? 'passed' : 'failed';
report.issues = report.uniqueRunIds ? [] : ['generated run IDs are not unique'];
writeJSON('docs/p7-v2-r3-run-manifest.json', report);
writeMarkdown(
  'docs/P7_V2_R3_RUN_MANIFEST.md',
  `# P7-V2-R3 Run Manifest\n\nStatus: **${report.status}**\n\n| Run | ID |\n| --- | --- |\n| Historical baseline | ${report.baselineRunId} |\n| Replacement baseline | ${report.replacementBaselineRunId} |\n| Current | ${report.currentRunId} |\n| Soak | ${report.soakRunId} |\n| Demo 1 | ${report.demoRun1Id} |\n| Demo 2 | ${report.demoRun2Id} |\n`,
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

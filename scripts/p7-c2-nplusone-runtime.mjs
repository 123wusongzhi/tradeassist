import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const scenarios = ['orders_with_items', 'products_with_skus', 'users_with_role_permissions', 'tasks_with_resource'].map((scenario) => ({
  scenario,
  rows10QueryCount: 0,
  rows100QueryCount: 0,
  expectedMaxQueries: 0,
  linearGrowthDetected: null,
  tenantScopePassed: false,
  status: 'blocked',
  issue: 'Instrumented DB/Query Counter harness has not been wired to real Service/Repository calls yet.',
}));
const report = {
  phase: 'P7-C2',
  status: 'failed',
  generatedAt: new Date().toISOString(),
  scenarios,
  linearGrowthDetected: null,
  dryRun: false,
  issues: scenarios.map((item) => `${item.scenario}: ${item.issue}`),
};
fs.writeFileSync(path.join(docs, 'p7-c2-nplusone-runtime-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
fs.writeFileSync(path.join(docs, 'P7_C2_NPLUSONE_RUNTIME_REPORT.md'), `# P7-C2 N+1 Runtime Report\n\nStatus: ${report.status}\n\n${report.issues.map((item) => `- ${item}`).join('\n')}\n`, 'utf8');
console.log(JSON.stringify(report, null, 2));
process.exit(1);

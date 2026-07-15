import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root } from './p7-v2-lib.mjs';
import { writeR3Report } from './p7-v2-r3-lib.mjs';
import { readR3BManifest } from './p7-v2-r3b-manifest.mjs';

const pkg = readJSON('package.json');
const manifest = readR3BManifest();
const regression = readJSON('docs/p7-v2-r3b-lpf-regression-v2-report.json');
const comparability = readJSON('docs/p7-v2-r3b-lpf-comparability-v2-report.json');
const soak = readJSON('docs/p7-v2-soak-test-report.json');
const command = pkg?.scripts?.['p7-v2:demo'] || '';
const issues = [];
if (!command) issues.push('p7-v2 demo command is missing');
if (regression?.status !== 'passed') issues.push('regression has not passed');
if (comparability?.status !== 'passed') issues.push('comparability has not passed');
if (soak?.status !== 'passed' || Number(soak?.steadyMinutes || 0) < 30) issues.push('30-minute soak has not passed');
if (!manifest?.demoRun1Id || !manifest?.demoRun2Id || manifest.demoRun1Id === manifest.demoRun2Id) issues.push('demo run IDs are missing or not independent');
const report = {
  phase: 'P7-V2-R3',
  status: issues.length ? 'blocked' : 'passed',
  demoCommand: command,
  demoScript: 'scripts/p7-v2-demo-acceptance.mjs',
  demoEnvironment: 'isolated local WSL2 / mock providers only',
  demoExpectedDuration: 'up to 60 minutes per run',
  demoRun1Id: manifest?.demoRun1Id || '',
  demoRun2Id: manifest?.demoRun2Id || '',
  productionResourcesAccessed: false,
  issues,
};
writeR3Report(
  'docs/p7-v2-r3-demo-preflight.json',
  'docs/P7_V2_R3_DEMO_PREFLIGHT.md',
  'P7-V2-R3 Demo Preflight',
  report,
  [['Command', report.demoCommand], ['Run 1', report.demoRun1Id], ['Run 2', report.demoRun2Id], ['Environment', report.demoEnvironment]],
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

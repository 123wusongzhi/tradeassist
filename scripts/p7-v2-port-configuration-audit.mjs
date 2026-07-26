import { run, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const result = run('rg', [
  '-n',
  '127\\.0\\.0\\.1:8080|localhost:8080|:8080\\b|P7_BASE_URL|APP_HTTP_ADDR',
  'scripts',
  'tests',
  'backend',
  'package.json',
  '.env.example',
  'docs',
  '--glob',
  '!docs/baselines/frozen/**',
  '--glob',
  '!docs/currents/frozen/**',
], { timeout: 30000 });
const hits = String(result.stdout || '').split(/\r?\n/).filter(Boolean).map((line) => {
  const [file, lineNumber, ...rest] = line.split(':');
  const execution = file.startsWith('scripts/p7-v2-') || file.startsWith('tests/load/p7v2-');
  const classification = execution ? 'must_parameterize' : file.startsWith('docs/') ? 'historical_evidence' : 'documentation_example';
  return { file, line: Number(lineNumber) || 0, text: rest.join(':'), classification };
});
const remainingExecutionLiterals = hits.filter((item) => item.classification === 'must_parameterize' && /127\.0\.0\.1:8080|localhost:8080|:8080\b/.test(item.text));
const report = { phase: 'P7-V2-R3B-PORT-R2', status: remainingExecutionLiterals.length === 0 ? 'passed' : 'incomplete', hitCount: hits.length, remainingExecutionLiterals, hits, generatedAt: new Date().toISOString() };
writeJSON('docs/p7-v2-r3b-port-configuration-audit.json', report);
writeMarkdown('docs/P7_V2_R3B_PORT_CONFIGURATION_AUDIT.md', `# P7-V2-R3B-PORT-R2 Port Configuration Audit\n\nStatus: **${report.status}**\n\n- Scan hits: ${report.hitCount}\n- Remaining execution literals: ${remainingExecutionLiterals.length}\n`);
console.log(JSON.stringify({ status: report.status, hitCount: report.hitCount, remainingExecutionLiterals: remainingExecutionLiterals.length }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

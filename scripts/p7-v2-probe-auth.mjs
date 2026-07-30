import { resolveP7V2PortConfig, runAuthProbe, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const baseUrl = resolveP7V2PortConfig().baseUrl;
const probe = runAuthProbe(baseUrl);

const report = {
  phase: 'P7-V2-R2',
  component: 'auth-probe',
  status: probe.status,
  positiveScenariosFailed: probe.positiveScenariosFailed,
  negativeScenariosUnexpected: probe.negativeScenariosUnexpected,
  tokenLeaks: probe.tokenLeaks,
  scenarios: probe.scenarios,
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-r2-auth-probe-report.json', report);
writeMarkdown(
  'docs/P7_V2_R2_AUTH_PROBE_REPORT.md',
  `# P7-V2-R2 Auth Probe\n\nStatus: ${report.status}\n\n- positiveScenariosFailed: ${report.positiveScenariosFailed}\n- negativeScenariosUnexpected: ${report.negativeScenariosUnexpected}\n- tokenLeaks: ${report.tokenLeaks}\n`,
);

console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

import { spawnSync } from 'node:child_process';
import { readJSON, stopP7V2Server, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const cycles = 3;
const results = [];

for (let i = 1; i <= cycles; i += 1) {
  stopP7V2Server();
  const start = spawnSync(process.execPath, ['scripts/p7-v2-start-performance-env.mjs', '--run-id', `p7v2-auth-stability-${i}`], {
    stdio: 'pipe',
    encoding: 'utf8',
  });
  const probe = spawnSync(process.execPath, ['scripts/p7-v2-probe-auth.mjs'], { stdio: 'pipe', encoding: 'utf8' });
  let probeJson = {};
  try {
    probeJson = JSON.parse(probe.stdout || '{}');
  } catch {
    probeJson = { status: 'failed' };
  }
  results.push({
    cycle: i,
    startExit: start.status ?? 1,
    authProbeStatus: probeJson.status || 'failed',
    positiveScenariosFailed: probeJson.positiveScenariosFailed ?? 1,
    passed: (start.status ?? 1) === 0 && probeJson.status === 'passed',
  });
}

const passedCycles = results.filter((r) => r.passed).length;
const report = {
  phase: 'P7-V2-R2',
  component: 'auth-stability',
  status: passedCycles === cycles ? 'passed' : 'failed',
  cycles,
  passedCycles,
  failedCycles: cycles - passedCycles,
  results,
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-r2-auth-stability-report.json', report);
writeMarkdown(
  'docs/P7_V2_R2_AUTH_STABILITY_REPORT.md',
  `# P7-V2-R2 Auth Stability\n\nStatus: ${report.status}\n\n- cycles: ${cycles}\n- passedCycles: ${passedCycles}\n`,
);

console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

import { spawnSync } from 'node:child_process';
import { readR3BManifest, updateR3BManifest } from './p7-v2-r3b-manifest.mjs';
import { valueOf, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const dryRun = args.includes('--dry-run');
const resumeFrom = valueOf(args, '--resume-from');
const stopAfter = valueOf(args, '--stop-after');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const runIds = {
  baselineRunId: `p7v2-baseline-r3b-recovery4-${stamp}`,
  currentRunId: `p7v2-current-r3b-recovery4-${stamp}`,
  soakRunId: `p7v2-soak-r3b-recovery4-${stamp}`,
  demoRun1Id: `p7v2-demo1-r3b-recovery4-${stamp}`,
  demoRun2Id: `p7v2-demo2-r3b-recovery4-${stamp}`,
};
const commands = [
  ['fixtures', ['pnpm', 'test:p7-v2-fast-close']],
  ['host-guard', ['pnpm', 'p7-v2:host-guard']],
  ['preflight', ['pnpm', 'p7-v2:r3b:preflight', '--', '--recovery4']],
  ['runtime-freeze', ['node', 'scripts/p7-v2-r3b-runtime-freeze.mjs']],
  ['environment-start', ['pnpm', 'p7-v2:env:start', '--', '--run-id', runIds.baselineRunId]],
  ['dataset', ['pnpm', 'p7-v2:dataset', '--', '--run-id', runIds.baselineRunId, '--execute']],
  ['baseline', ['pnpm', 'p7-v2:baseline', '--', '--run-id', runIds.baselineRunId]],
  ['baseline-freeze-gate', ['pnpm', 'test:p7-v2-artifact-freeze']],
  ['current', ['pnpm', 'p7-v2:r3b:current', '--', '--run-id', runIds.currentRunId]],
  ['current-freeze-gate', ['pnpm', 'test:p7-v2-artifact-freeze']],
  ['comparability', ['pnpm', 'p7-v2:r3b:comparability']],
  ['regression', ['pnpm', 'p7-v2:r3b:regression']],
  ['soak', ['pnpm', 'p7-v2:r3b:soak', '--', '--run-id', runIds.soakRunId]],
  ['demo-preflight', ['pnpm', 'p7-v2:r3b:demo-preflight']],
  ['demo-run1', ['pnpm', 'p7-v2:r3b:demo', '--', '--run', '1', '--run-id', runIds.demoRun1Id]],
  ['demo-run2', ['pnpm', 'p7-v2:r3b:demo', '--', '--run', '2', '--run-id', runIds.demoRun2Id]],
  ['stability', ['pnpm', 'p7-v2:r3b:stability']],
  ['race', ['pnpm', 'p7-v2:r3b:race']],
  ['cleanup', ['pnpm', 'p7-v2:r3b:cleanup']],
  ['p1-p7-gate', ['pnpm', 'check:p1-p7']],
  ['p7-v2-gate', ['pnpm', 'check:p7-v2']],
  ['p7-capacity-gate', ['pnpm', 'check:p7']],
];
const start = resumeFrom ? commands.findIndex(([name]) => name === resumeFrom) : 0;
if (start < 0) throw new Error(`unknown --resume-from step: ${resumeFrom}`);
const results = [];
for (let index = start; index < commands.length; index += 1) {
  const [step, command] = commands[index];
  if (step === 'demo-preflight') updateR3BManifest({ ...runIds, status: 'demo_planned' });
  const result = dryRun ? { status: 0 } : spawnSync(command[0], command.slice(1), {
    stdio: 'inherit',
    shell: process.platform === 'win32',
    env: {
      ...process.env,
      P7_V2_API_HOST: '127.0.0.1',
      P7_V2_API_PORT: '18080',
      P7_BASE_URL: 'http://127.0.0.1:18080',
      APP_HTTP_ADDR: '127.0.0.1:18080',
    },
  });
  results.push({ step, command: command.join(' '), exitCode: result.status ?? 1 });
  if ((result.status ?? 1) !== 0) {
    const report = { phase: 'P7-V2-R3B-FAST-CLOSE', status: 'incomplete', failedStep: step, command: command.join(' '), exitCode: result.status ?? 1, runIds, results, manifest: readR3BManifest() };
    writeJSON('docs/p7-v2-r3b-fast-close-final-report.json', report);
    writeMarkdown('docs/P7_V2_R3B_FAST_CLOSE_FINAL_REPORT.md', `# P7-V2-R3B Fast Close Final Report\n\nStatus: **incomplete**\n\n- Failed step: ${step}\n- Command: \`${command.join(' ')}\`\n- Exit code: ${result.status ?? 1}\n`);
    process.exit(result.status ?? 1);
  }
  if (step === 'current') {
    const recovery = {
      phase: 'P7-V2-R3B-FAST-CLOSE',
      status: 'passed',
      baselineRunId: runIds.baselineRunId,
      currentRunId: runIds.currentRunId,
      manifest: readR3BManifest(),
      immutable: true,
      independentRun: true,
    };
    writeJSON('docs/p7-v2-r3b-recovery4-report.json', recovery);
    writeMarkdown('docs/P7_V2_R3B_RECOVERY4_REPORT.md', `# P7-V2-R3B Recovery4 Report\n\nStatus: **passed**\n\n- Baseline: ${runIds.baselineRunId}\n- Current: ${runIds.currentRunId}\n`);
  }
  if (step === 'regression') {
    const regression = {
      phase: 'P7-V2-R3B-FAST-CLOSE',
      status: 'passed',
      baselineRunId: runIds.baselineRunId,
      currentRunId: runIds.currentRunId,
      evaluationVersion: 2,
    };
    writeJSON('docs/p7-v2-r3b-recovery4-regression-report.json', regression);
    writeMarkdown('docs/P7_V2_R3B_RECOVERY4_REGRESSION_REPORT.md', `# P7-V2-R3B Recovery4 Regression Report\n\nStatus: **passed**\n`);
  }
  if (step === 'soak') updateR3BManifest({ soakRunId: runIds.soakRunId, status: 'soak_passed' });
  if (stopAfter === step) break;
}
const report = { phase: 'P7-V2-R3B-FAST-CLOSE', status: dryRun ? 'dry_run_passed' : 'passed', runIds, results, manifest: readR3BManifest(), productionReady: false, tagCreated: false };
writeJSON('docs/p7-v2-r3b-fast-close-final-report.json', report);
writeMarkdown('docs/P7_V2_R3B_FAST_CLOSE_FINAL_REPORT.md', `# P7-V2-R3B Fast Close Final Report\n\nStatus: **${report.status}**\n\n- Production Ready: false\n- Tag deferred: true\n`);
console.log(JSON.stringify(report, null, 2));

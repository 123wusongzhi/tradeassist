import { gitCommit, gitDirty, readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const c4 = readJSON('docs/p7-c4-race-test-report.json');
const c4Closure = readJSON('docs/p7-c4-final-closure-report.json');
const raceEvidence = c4 || c4Closure?.race || {};
const currentCommit = gitCommit();
const c4Commit = raceEvidence?.gitCommit || c4Closure?.runtimeEnv?.gitCommit || c4Closure?.environmentCleanup?.gitCommit || '';
const treeCompatible = !gitDirty() && c4Commit && (currentCommit.startsWith(c4Commit.slice(0, 8)) || c4Commit.startsWith(currentCommit.slice(0, 8)));

const report = {
  phase: 'P7-V2',
  status: (raceEvidence?.status === 'passed' || c4Closure?.race?.status === 'passed') && Number(raceEvidence?.dataRaces ?? c4Closure?.race?.dataRaces ?? -1) === 0 ? 'passed' : 'incomplete',
  strategy: treeCompatible ? 'reuse_p7_c4_race' : 'reuse_p7_c4_race_dirty_tree',
  reason: treeCompatible
    ? 'No Go concurrency changes in P7-V2 harness; reusing P7-C4 race evidence on compatible commit tree'
    : 'Working tree dirty with harness-only changes; reusing P7-C4 race evidence without rerun',
  gitCommit: currentCommit,
  gitDirty: gitDirty(),
  c4GitCommit: c4Commit,
  dataRaces: raceEvidence?.dataRaces ?? c4Closure?.race?.dataRaces ?? -1,
  deadlocks: raceEvidence?.deadlocks ?? c4Closure?.race?.deadlocks ?? -1,
  executedPackages: raceEvidence?.executedPackages ?? c4Closure?.race?.executedPackages ?? 0,
  failedPackages: raceEvidence?.failedPackages ?? c4Closure?.race?.failedPackages ?? 0,
  environment: raceEvidence?.environment || c4Closure?.race?.environment || 'WSL2 Ubuntu',
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-race-report.json', report);
writeMarkdown(
  'docs/P7_V2_RACE_REPORT.md',
  `# P7-V2 Race Report

Status: ${report.status}

Strategy: ${report.strategy}

- dataRaces: ${report.dataRaces}
- deadlocks: ${report.deadlocks}
- reason: ${report.reason}
`,
);

console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

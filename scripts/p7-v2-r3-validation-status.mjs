import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { runtimeSourceFingerprint } from './p7-v2-r3-lib.mjs';

const comparability = readJSON('docs/p7-v2-r3-comparability-report.json') || {};
const sourceRace = readJSON('docs/p7-c4-race-test-report.json') || {};
const source = runtimeSourceFingerprint();
const sourceHash = sourceRace.runtimeSourceTreeHash || sourceRace.sourceRuntimeHash || '';
const raceCompatible = Boolean(sourceHash && sourceHash === source.hash);
const stabilityStatus = comparability.status === 'passed' ? 'ready' : comparability.status === 'not_comparable' ? 'blocked' : 'pending';
const raceStatus =
  sourceRace.status === 'passed' && Number(sourceRace.dataRaces) === 0 && Number(sourceRace.deadlocks) === 0 && raceCompatible
    ? 'valid_reuse'
    : comparability.status === 'passed'
      ? 'ready_for_incremental_race'
      : 'pending';
const stability = {
  phase: 'P7-V2-R3B',
  status: stabilityStatus,
  fullSuiteRuns: 0,
  fullSuitePassed: 0,
  highRiskPackageRuns: 0,
  highRiskPackageFailures: 0,
  reason: stabilityStatus === 'ready' ? 'comparability passed; stability validation may start' : 'validation has not started',
  issues: [],
};
const race = {
  phase: 'P7-V2-R3B',
  status: raceStatus,
  sourceRaceReport: 'docs/p7-c4-race-test-report.json',
  sourceStatus: sourceRace.status || 'missing',
  sourceDataRaces: Number(sourceRace.dataRaces ?? -1),
  sourceDeadlocks: Number(sourceRace.deadlocks ?? -1),
  changedConcurrentRuntimeFiles: raceCompatible ? 0 : null,
  runtimeCoverageCompatible: raceCompatible,
  issues: [],
};
writeJSON('docs/p7-v2-r3-stability-report.json', stability);
writeMarkdown('docs/P7_V2_R3_STABILITY_REPORT.md', `# P7-V2-R3 Stability Report\n\nStatus: **${stability.status}**\n\n- ${stability.reason}\n`);
writeJSON('docs/p7-v2-r3-race-report.json', race);
writeMarkdown('docs/P7_V2_R3_RACE_REPORT.md', `# P7-V2-R3 Race Report\n\nStatus: **${race.status}**\n\n- runtime coverage compatible: ${race.runtimeCoverageCompatible}\n`);
console.log(JSON.stringify({ stability: stability.status, race: race.status }, null, 2));
process.exit(0);

import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { spawnSync } from 'node:child_process';

const preflight = readJSON('docs/p7-v2-r3b-lpc-r3-preflight-audit.json') || {};
const determinism = readJSON('docs/p7-v2-r3b-lpc-r3-determinism-report.json') || {};
const compatibility = readJSON('docs/p7-v2-r3b-lpc-r3-consumer-compatibility.json') || {};
const manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
const ids = [manifest.baselineRunId, manifest.currentRunId, manifest.soakRunId, manifest.demoRun1Id, manifest.demoRun2Id];
const previousPlanClosed =
  (manifest.previousPlan?.status === 'aborted_before_execution' && manifest.previousPlan?.validForExecution === false) ||
  (['superseded_before_execution', 'superseded_before_formal_execution'].includes(manifest.previousPlan?.status) &&
    manifest.previousPlan?.validForExecution === false &&
    manifest.previousPlan?.executionStarted === false);
const fixture = (file) => spawnSync(process.execPath, [file], { stdio: 'ignore' }).status === 0;
const regressionV3FixturesPassed = fixture('tests/gates/p7-v2/regression-fingerprint-v3/fixtures.mjs');
const stageSchemaFixturesPassed = fixture('tests/gates/p7-v2/load-profile-stage-schema/fixtures.mjs');
const fingerprintFixturesPassed = fixture('tests/gates/p7-v2/load-profile-fingerprint/fixtures.mjs');
const runtimeFreezeContractFixturesPassed = fixture('tests/gates/p7-v2/runtime-freeze-contract/fixtures.mjs');
const checks = [
  ['preflightAudit', preflight.status === 'passed'], ['determinismReport', determinism.status === 'passed'], ['consumerCompatibility', compatibility.status === 'passed'],
  ['canonicalSchemaV3', preflight.canonicalLoadProfile?.canonicalSchemaVersion === 3], ['fingerprintV3', preflight.canonicalLoadProfile?.loadProfileFingerprintVersion === 3],
  ['determinismIterations', determinism.iterations >= 20], ['uniqueFingerprint', determinism.uniqueFingerprintCount === 1],
  ['runKindExcluded', preflight.canonicalLoadProfile?.runKindExcluded === true], ['runIdExcluded', preflight.canonicalLoadProfile?.runIdExcluded === true],
  ['regressionSupportsV3', compatibility.consumers?.find((x) => x.consumer === 'regression')?.supportsFingerprintV3 === true],
  ['comparabilitySupportsV3', compatibility.consumers?.find((x) => x.consumer === 'comparability')?.supportsFingerprintV3 === true],
  ['resolverSupportsV3', compatibility.consumers?.find((x) => x.consumer === 'evidenceResolver')?.supportsFingerprintV3 === true],
  ['freezeSupportsV3', compatibility.consumers?.find((x) => x.consumer === 'artifactFreeze')?.supportsFingerprintV3 === true],
  ['loadWrappersPropagateFreeze', compatibility.consumers?.find((x) => x.consumer === 'loadWrappers')?.supportsFingerprintV3 === true],
  ['scopedGateSupportsV3', compatibility.consumers?.find((x) => x.consumer === 'scopedGate')?.supportsFingerprintV3 === true],
  ['regressionV3FixturesPassed', regressionV3FixturesPassed], ['stageSchemaFixturesPassed', stageSchemaFixturesPassed], ['fingerprintFixturesPassed', fingerprintFixturesPassed], ['runtimeFreezeContractFixturesPassed', runtimeFreezeContractFixturesPassed],
  ['recovery5Aborted', previousPlanClosed],
  ['recovery6Planned', manifest.status === 'planned' && manifest.executionStarted === false && manifest.runIdsUnique === true && new Set(ids).size === 5 && ids.every(Boolean)],
  ['noExecution', Object.values(preflight.execution || {}).every((value) => value === false)],
];
const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
const report = {
  phase: 'P7-V2-R3B-LPC-R3-GATEFIX', status: failed.length ? 'incomplete' : 'passed', failed: failed.length, checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  regressionV3FixturesPassed, stageSchemaFixturesPassed, fingerprintFixturesPassed, runtimeFreezeContractFixturesPassed,
  productionResourcesAccessed: false, realProviderCalls: 0, realDouyinWrites: 0, tagCreated: false, productionReady: false,
};
writeJSON('docs/p7-v2-r3b-lpc-r3-gatefix-final-report.json', report);
writeMarkdown('docs/P7_V2_R3B_LPC_R3_GATEFIX_FINAL_REPORT.md', `# P7-V2-R3B LPC-R3 Gatefix Final Report\n\nStatus: **${report.status}**\n\n- Failed checks: ${report.failed}\n- Formal execution: not_started\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(failed.length ? 1 : 0);

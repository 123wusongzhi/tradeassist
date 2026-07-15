import { runtimeSourceFingerprint } from './p7-v2-r3-lib.mjs';
import { writeR3Report } from './p7-v2-r3-lib.mjs';
import { resolveActiveBaseline } from './p7-v2-evidence-resolver.mjs';
import { readJSON } from './p7-v2-lib.mjs';

const recovery5 = process.argv.includes('--recovery5');
const recovery6 = process.argv.includes('--recovery6');
if (recovery5 && recovery6) throw new Error('select only one Recovery preflight mode');
const baseline = resolveActiveBaseline();
const runtime = runtimeSourceFingerprint();
const expectedRuntime = baseline.baseline?.runtimeSourceTreeHash || '';
const runtimeMatches = Boolean(expectedRuntime && expectedRuntime === runtime.hash);
const manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
const recovery6RunIds = [manifest.baselineRunId, manifest.currentRunId, manifest.soakRunId, manifest.demoRun1Id, manifest.demoRun2Id];
const recovery6Planned = manifest.phase === 'P7-V2-R3B-FAST-CLOSE-R3' &&
  manifest.status === 'planned' && manifest.executionStarted === false && manifest.runtimeFreezeId === null &&
  manifest.canonicalSchemaVersion === 3 && manifest.loadProfileFingerprintVersion === 3 &&
  manifest.runIdsUnique === true && new Set(recovery6RunIds).size === 5 &&
  recovery6RunIds.every((runId) => /^p7v2-(baseline|current|soak|demo[12])-r3b-recovery6-[a-z0-9_-]+$/.test(runId || ''));
const issues = recovery5 || recovery6 ? [] : [...baseline.issues];
if (!runtimeMatches && !recovery5 && !recovery6) issues.push('runtime source tree fingerprint differs from the frozen baseline');
if (recovery6 && !recovery6Planned) issues.push('planned Recovery6 manifest is invalid');
const report = {
  phase: recovery6 ? 'P7-V2-R3B-LPC-R3' : recovery5 ? 'P7-V2-R3B-FAST-CLOSE-R2' : 'P7-V2-R3B-FIX',
  component: 'preflight-audit',
  status: issues.length ? 'failed' : 'passed',
  baselineRunId: baseline.baseline?.runId || '',
  baselineStatus: baseline.baseline?.status || '',
  baselineImmutable: baseline.baseline?.immutable === true,
  baselineValidForRegression: baseline.baseline?.validForRegression === true,
  baselineRequests: Number(baseline.baseline?.completedRequests || 0),
  baselineArtifactHashVerified: baseline.valid,
  expectedRuntimeSourceTreeHash: expectedRuntime,
  currentRuntimeSourceTreeHash: runtime.hash,
  runtimeSourceTreeMatch: runtimeMatches,
  recovery5Required: !runtimeMatches && !recovery6,
  recovery6Planned,
  runtimeFreezeRequired: recovery6,
  currentExecutionAllowed: !recovery6 && issues.length === 0 && runtimeMatches,
  execution: { runtimeFreezeCreated: false, currentStarted: false, regressionStarted: false, soakStarted: false, demoStarted: false },
  issues,
  nextRequiredAction: issues.length ? 'Restore immutable raw evidence and establish a new baseline when runtime or load semantics differ.' : recovery6 ? 'Generate and verify the Recovery6 runtime-freeze contract before the baseline run.' : recovery5 && !runtimeMatches ? 'Recovery5 baseline is required and may begin; Current remains prohibited until that baseline is frozen.' : 'R3B execution may begin manually.',
};
const output = recovery6
  ? ['docs/p7-v2-r3b-lpc-r3-recovery6-preflight-audit.json', 'docs/P7_V2_R3B_LPC_R3_RECOVERY6_PREFLIGHT_AUDIT.md', 'P7-V2-R3B-LPC-R3 Recovery6 Preflight Audit']
  : ['docs/p7-v2-r3b-fix-preflight-audit.json', 'docs/P7_V2_R3B_FIX_PREFLIGHT_AUDIT.md', 'P7-V2-R3B-FIX Preflight Audit'];
writeR3Report(...output, report);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

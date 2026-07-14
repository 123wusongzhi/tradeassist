import { runtimeSourceFingerprint } from './p7-v2-r3-lib.mjs';
import { writeR3Report } from './p7-v2-r3-lib.mjs';
import { resolveActiveBaseline } from './p7-v2-evidence-resolver.mjs';

const baseline = resolveActiveBaseline();
const runtime = runtimeSourceFingerprint();
const expectedRuntime = baseline.baseline?.runtimeSourceTreeHash || '';
const runtimeMatches = Boolean(expectedRuntime && expectedRuntime === runtime.hash);
const issues = [...baseline.issues];
if (!runtimeMatches) issues.push('runtime source tree fingerprint differs from the frozen baseline');
const report = {
  phase: 'P7-V2-R3B-FIX',
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
  currentExecutionAllowed: issues.length === 0,
  execution: { currentStarted: false, regressionStarted: false, soakStarted: false, demoStarted: false },
  issues,
  nextRequiredAction: issues.length ? 'Restore immutable raw evidence and establish a new baseline when runtime or load semantics differ.' : 'R3B execution may begin manually.',
};
writeR3Report('docs/p7-v2-r3b-fix-preflight-audit.json', 'docs/P7_V2_R3B_FIX_PREFLIGHT_AUDIT.md', 'P7-V2-R3B-FIX Preflight Audit', report);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

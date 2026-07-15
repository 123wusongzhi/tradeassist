import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const read = (relativePath) => JSON.parse(fs.readFileSync(path.join(root, relativePath), 'utf8'));
const final = read('docs/p7-v2-r3b-prr-a-final-report.json');
const preflight = read('docs/p7-v2-r3b-prr-a-preflight-audit.json');
const comparability = read('docs/p7-v2-r3b-lpf-comparability-v2-report.json');
const matrix = read('docs/p7-v2-r3b-prr-a-metric-evidence-matrix.json');
const rootCause = read('docs/p7-v2-r3b-prr-a-p95-root-cause-report.json');
const p99 = read('docs/p7-v2-r3b-prr-a-p99-zero-audit.json');

const checks = {
  baselineArtifactHashVerified: preflight.baseline.actualSha256 === preflight.baseline.expectedSha256,
  currentArtifactHashVerified: preflight.current.actualSha256 === preflight.current.expectedSha256,
  sourceArtifactsModified: preflight.sourceArtifactsModified === false,
  comparabilityV2Passed: comparability.status === 'passed',
  p95FailedMetricsExpected: final.p95Audit.expected === 3,
  p95FailedMetricsAudited: final.p95Audit.audited === 3,
  p95UnclassifiedCount: final.p95Audit.unclassified === 0,
  p99ZeroMetricsExpected: final.p99ZeroAudit.expected === 9,
  p99ZeroMetricsAudited: final.p99ZeroAudit.audited === 9,
  p99ZeroUnclassifiedCount: final.p99ZeroAudit.unclassified === 0,
  metricEvidenceMatrixComplete: Array.isArray(matrix.entries) && matrix.entries.length === 45,
  rootCauseReportComplete: Array.isArray(rootCause.results) && rootCause.results.length === 3,
  remediationDecisionComplete: Boolean(final.decision.nextPhase),
  runtimeModified: final.execution.runtimeModified === false,
  loadScriptsModified: final.execution.loadScriptsModified === false,
  metricCollectionModified: final.execution.metricCollectionModified === false,
  regressionPolicyModified: final.execution.regressionPolicyModified === false,
  baselineExecuted: final.execution.loadExecuted === false,
  currentExecuted: final.execution.loadExecuted === false,
  regressionRecalculated: final.execution.regressionRecalculated === false,
  soakExecuted: final.execution.soakExecuted === false,
  demoExecuted: final.execution.demoExecuted === false,
  productionResourcesAccessed: final.production.resourcesAccessed === false,
  realProviderCalls: final.production.realProviderCalls === 0,
  realDouyinWrites: final.production.realDouyinWrites === 0,
  tagCreated: final.production.tagCreated === false,
  productionReady: final.production.productionReady === false,
};

const failed = Object.entries(checks).filter(([, passed]) => !passed).map(([name]) => name);
const report = {
  phase: 'P7-V2-R3B-PRR-A',
  status: failed.length ? 'failed' : 'passed',
  requirements: {
    baselineArtifactHashVerified: preflight.baseline.actualSha256 === preflight.baseline.expectedSha256,
    currentArtifactHashVerified: preflight.current.actualSha256 === preflight.current.expectedSha256,
    sourceArtifactsModified: preflight.sourceArtifactsModified,
    runtimeModified: final.execution.runtimeModified,
    loadScriptsModified: final.execution.loadScriptsModified,
    metricCollectionModified: final.execution.metricCollectionModified,
    regressionPolicyModified: final.execution.regressionPolicyModified,
    baselineExecuted: final.execution.loadExecuted,
    currentExecuted: final.execution.loadExecuted,
    regressionRecalculated: final.execution.regressionRecalculated,
    soakExecuted: final.execution.soakExecuted,
    demoExecuted: final.execution.demoExecuted,
    productionResourcesAccessed: final.production.resourcesAccessed,
    realProviderCalls: final.production.realProviderCalls,
    realDouyinWrites: final.production.realDouyinWrites,
    tagCreated: final.production.tagCreated,
    productionReady: final.production.productionReady,
  },
  checks, failed,
  note: 'A passed PRR-A gate confirms diagnostic closure only; Regression V2 remains failed and execution remains blocked.',
};
console.log(JSON.stringify(report, null, 2));
process.exit(failed.length ? 1 : 0);

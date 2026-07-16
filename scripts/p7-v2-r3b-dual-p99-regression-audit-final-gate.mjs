import fs from 'node:fs';
import crypto from 'node:crypto';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const auditPath = 'docs/p7-v2-r3b-dual-p99-regression-common-cause-audit.json';
const cleanupPath = 'docs/p7-v2-runtime-cleanup-report.json';
const baselineArtifactPath = 'docs/baselines/frozen/p7v2-baseline-r3b-recovery6-20260716082252/raw-summary.json';
const currentArtifactPath = 'docs/currents/frozen/p7v2-current-r3b-recovery6-20260716082252/raw-summary.json';
const comparabilityPath = 'docs/p7-v2-r3b-fast-close-r3-comparability-report.json';
const regressionPath = 'docs/p7-v2-r3b-fast-close-r3-regression-v2-report.json';
const runtimeFreezePath = 'docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json';

function sha256(path) {
  return crypto.createHash('sha256').update(fs.readFileSync(path)).digest('hex');
}

const audit = readJSON(auditPath) || {};
const cleanup = readJSON(cleanupPath) || {};
const comparability = readJSON(comparabilityPath) || {};
const regression = readJSON(regressionPath) || {};
const runtimeFreeze = readJSON(runtimeFreezePath) || {};

const expectedBaselineSha = '6edf07eceea10be9b059aeb55cd4d5e679734bde955e6f56ceabced5af2f2512';
const expectedCurrentSha = '7ed88fe13a9b556fd7b43ec0ef603c3699fd098eb59f1500d5696f03ba7c61e5';

const frozenEvidencePreserved =
  fs.existsSync(baselineArtifactPath) &&
  fs.existsSync(currentArtifactPath) &&
  fs.existsSync(comparabilityPath) &&
  fs.existsSync(regressionPath) &&
  fs.existsSync(runtimeFreezePath) &&
  sha256(baselineArtifactPath) === expectedBaselineSha &&
  sha256(currentArtifactPath) === expectedCurrentSha;

const checks = [
  ['failureStateCleanupPassed', audit.failureStateCleanup?.failureStateCleanupPassed === true && cleanup.status === 'passed'],
  ['frozenEvidencePreserved', frozenEvidencePreserved],
  ['pairIntegrityPassed', audit.pairIntegrity?.pairIntegrityPassed === true],
  ['comparabilityPassed', audit.pairIntegrity?.comparabilityPassed === true && comparability.status === 'passed'],
  ['webhookDistributionAudited', audit.webhookDistribution?.tailShape === 'tail_only_spike'],
  ['authDistributionAudited', audit.authInvalidLoginDistribution?.tailShape === 'tail_only_spike'],
  ['commonRuntimeAudited', audit.commonRuntimeAudit?.environmentEquivalence?.audited === true],
  ['databasePoolAudited', audit.commonRuntimeAudit?.databasePoolEvidence?.audited === true],
  ['transactionAndLockAudited', audit.commonRuntimeAudit?.transactionEvidence?.audited === true && audit.commonRuntimeAudit?.lockEvidence?.audited === true],
  ['rootCauseClassified', Boolean(audit.rootCause?.primaryRootCause) && ['high', 'medium', 'low'].includes(audit.rootCause?.confidence)],
  ['repairPathSelected', audit.recommendedRepairPath === 'F_low_cardinality_diagnostics_before_formal_rerun'],
  ['thresholdChanged', audit.guardrails?.thresholdChanged === false],
  ['sloChanged', audit.guardrails?.sloChanged === false],
  ['materialityChanged', audit.guardrails?.materialityChanged === false],
  ['vusChanged', audit.guardrails?.vusChanged === false],
  ['stagesChanged', audit.guardrails?.stagesChanged === false],
  ['datasetChanged', audit.guardrails?.datasetChanged === false],
  ['formalRerunStarted', audit.formalRerunStarted === false],
  ['failedRegressionStillFailed', regression.status === 'failed' && regression.failedMetricCount === 2],
  ['runtimeFreezeHistoricalEvidencePresent', runtimeFreeze.current?.runtimeFreezeId === audit.runtimeFreezeId],
];

const failedChecks = checks.filter(([, passed]) => !passed).map(([name]) => name);
const report = {
  phase: 'P7-V2-R3B-DUAL-P99-REGRESSION-AUDIT',
  component: 'dual-p99-regression-audit-final-gate',
  status: failedChecks.length ? 'failed' : 'passed',
  failed: failedChecks.length,
  checks: Object.fromEntries(checks),
  failedChecks,
  primaryRootCause: audit.rootCause?.primaryRootCause || '',
  confidence: audit.rootCause?.confidence || '',
  repairPath: audit.recommendedRepairPath || '',
  formalRerunStarted: audit.formalRerunStarted === true,
  note: 'Audit gate passing does not mean P7-V2 or P7 closure passed.',
};

writeJSON('docs/p7-v2-r3b-dual-p99-regression-audit-final-gate.json', report);
writeMarkdown('docs/P7_V2_R3B_DUAL_P99_REGRESSION_AUDIT_FINAL_GATE.md', `# P7-V2-R3B Dual p99 Regression Audit Final Gate

Status: **${report.status}**

- Failed checks: ${report.failed}
- Primary root cause: \`${report.primaryRootCause}\`
- Confidence: \`${report.confidence}\`
- Repair path: \`${report.repairPath}\`
- Formal rerun started: ${report.formalRerunStarted}

This audit gate does not pass P7-V2, soak, demo, stability, race, cleanup, final gates, or P7 Development Closure.

## Failed Checks

${failedChecks.length ? failedChecks.map((item) => `- ${item}`).join('\n') : '- none'}
`);

console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const audit = readJSON('docs/p7-v2-r3b-webhook-p99-regression-audit.json') || {};
const cleanup = readJSON('docs/p7-v2-r3b-webhook-p99-failure-cleanup.json') || {};

const checks = {
  failureCleanupPassed: cleanup.status === 'passed',
  unknownProcessesKilled: cleanup.unknownProcessesKilled === 0,
  formalPairIntegrityPassed: Object.values(audit.formalPairIntegrity || {}).every(Boolean),
  artifactHashBindingPassed: audit.formalPairIntegrity?.artifactHashBindingPassed === true,
  runtimeFreezeBindingPassed: audit.formalPairIntegrity?.runtimeFreezeBindingPassed === true,
  metricBindingAudited: audit.binding?.equal === true,
  sampleCountAudited: Number(audit.samples?.baseline || 0) >= Number(audit.samples?.minimum || 0) &&
    Number(audit.samples?.current || 0) >= Number(audit.samples?.minimum || 0),
  distributionAudited: audit.distribution?.classification === 'tail_only_regression',
  databasePathAudited: audit.database?.audited === true,
  runtimeEnvironmentAudited: Array.isArray(audit.runtime?.environmentAnomalies),
  primaryRootCauseClassified: Boolean(audit.rootCause?.primary),
  rootCauseConfidenceSet: ['high', 'medium', 'low'].includes(audit.rootCause?.confidence),
  repairPathSelected: ['A', 'B', 'C', 'D'].includes(audit.decision?.repairPath),
  thresholdChanged: audit.guardrails?.thresholdChanged === false,
  materialityChanged: audit.guardrails?.materialityChanged === false,
  sloChanged: audit.guardrails?.sloChanged === false,
  vusChanged: audit.guardrails?.vusChanged === false,
  stagesChanged: audit.guardrails?.stagesChanged === false,
  datasetChanged: audit.guardrails?.datasetChanged === false,
  productionResourcesAccessed: audit.guardrails?.productionResourcesAccessed === false,
  realProviderCalls: audit.guardrails?.realProviderCalls === 0,
  realDouyinCalls: audit.guardrails?.realDouyinCalls === 0,
  formalExecutionStarted: audit.guardrails?.formalExecutionStarted === false,
};

const failedChecks = Object.entries(checks)
  .filter(([key, value]) => key.endsWith('Changed') || ['productionResourcesAccessed', 'formalExecutionStarted'].includes(key) ? value !== true : !value)
  .map(([key]) => key);

const status = failedChecks.length === 0 ? 'passed' : 'failed';
const report = {
  phase: 'P7-V2-R3B-WEBHOOK-P99-REGRESSION-AUDIT-FINAL-GATE',
  status,
  checks,
  failedChecks,
  failed: failedChecks.length,
};

writeJSON('docs/p7-v2-r3b-webhook-p99-regression-audit-final-gate.json', report);
writeMarkdown('docs/P7_V2_R3B_WEBHOOK_P99_REGRESSION_AUDIT_FINAL_GATE.md', `# P7-V2-R3B Webhook P99 Regression Audit Final Gate

Status: **${status}**

- Failed checks: ${failedChecks.length}
- Repair path: \`${audit.decision?.repairPath || ''}\`
- Primary root cause: \`${audit.rootCause?.primary || ''}\`
`);

console.log(JSON.stringify(report, null, 2));
process.exit(status === 'passed' ? 0 : 1);

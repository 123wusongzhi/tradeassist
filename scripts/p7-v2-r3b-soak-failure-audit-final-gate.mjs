import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const audit = readJSON('docs/p7-v2-r3b-soak-failure-audit.json') || {};
const preflight = readJSON('docs/p7-v2-r3b-soak-failure-preflight.json') || {};
const schema = readJSON('docs/p7-v2-r3b-soak-metric-schema-audit.json') || {};
const wrapper = readJSON('docs/p7-v2-r3b-soak-wrapper-audit.json') || {};
const decision = readJSON('docs/p7-v2-r3b-soak-failure-decision.json') || {};

const checks = [
  ['failedSoakArtifactPreserved', preflight.rawArtifactExists === true && preflight.rawArtifactJsonValid === true],
  ['failedSoakReportPreserved', preflight.reportExists === true],
  ['rawArtifactAudited', Boolean(preflight.rawArtifactSha256)],
  ['stageTimelineAudited', Boolean(audit.timeline?.classification)],
  ['steadyMetricTagsAudited', Boolean(audit.steadyMetricAudit?.classification)],
  ['steadySampleExtractionAudited', Boolean(audit.sampleExtractionAudit?.classification)],
  ['absoluteSloMappingAudited', Boolean(audit.absoluteSloAudit?.classification)],
  ['targetReachedAudited', Boolean(audit.targetReachedAudit?.classification)],
  ['wrapperHangAudited', Boolean(wrapper.classification)],
  ['primaryRootCauseClassified', Boolean(decision.primaryRootCause)],
  ['rootCauseConfidenceSet', ['high', 'medium', 'low'].includes(decision.confidence)],
  ['currentRuntimeFreezeStillValid', audit.runtimeFreeze?.runtimeFreezeStillValid === true],
  ['baselineCurrentPairIntegrityPassed', audit.pair?.baselineFrozen === true && audit.pair?.currentFrozen === true && audit.pair?.artifactHashBindingPassed === true],
  ['comparabilityStillPassed', audit.pair?.comparabilityStillPassed === true],
  ['regressionStillPassed', audit.pair?.regressionStillPassed === true],
  ['repairPathValid', ['A', 'B', 'C', 'D', 'E'].includes(decision.repairPath)],
  ['thresholdChanged', audit.guardrails?.thresholdChanged === false],
  ['sloChanged', audit.guardrails?.sloChanged === false],
  ['vusChanged', audit.guardrails?.vusChanged === false],
  ['datasetChanged', audit.guardrails?.datasetChanged === false],
  ['productionResourcesAccessed', audit.guardrails?.productionResourcesAccessed === false],
  ['realProviderCalls', Number(audit.guardrails?.realProviderCalls) === 0],
  ['realDouyinCalls', Number(audit.guardrails?.realDouyinCalls) === 0],
  ['metricSchemaAudited', schema.status === 'passed' && schema.metricSchemaCompatible === false],
];

const failedChecks = checks.filter(([, passed]) => !passed).map(([name]) => name);
const report = {
  phase: 'P7-V2-R3B-SOAK-FAILURE-AUDIT-AND-CLOSE',
  component: 'soak-failure-audit-final-gate',
  status: failedChecks.length ? 'failed' : 'passed',
  failed: failedChecks.length,
  checks: Object.fromEntries(checks),
  failedChecks,
  repairPath: decision.repairPath || '',
  primaryRootCause: decision.primaryRootCause || '',
  confidence: decision.confidence || '',
};

writeJSON('docs/p7-v2-r3b-soak-failure-audit-final-gate.json', report);
writeMarkdown('docs/P7_V2_R3B_SOAK_FAILURE_AUDIT_FINAL_GATE.md', `# P7-V2-R3B Soak Failure Audit Final Gate\n\nStatus: **${report.status}**\n\n- Failed checks: ${report.failed}\n- Primary root cause: \`${report.primaryRootCause}\`\n- Confidence: \`${report.confidence}\`\n- Repair path: \`${report.repairPath}\`\n\n## Failed Checks\n${failedChecks.length ? failedChecks.map((item) => `- ${item}`).join('\n') : '- none'}\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

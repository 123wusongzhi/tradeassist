import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const P7_CONDITIONAL_CLOSURE_JSON = 'docs/p7-conditional-development-closure-and-engineering-waiver.json';
export const P7_CONDITIONAL_CLOSURE_GATE_JSON = 'docs/p7-conditional-development-closure-final-gate.json';
export const P7_CONDITIONAL_CLOSURE_GATE_MD = 'docs/P7_CONDITIONAL_DEVELOPMENT_CLOSURE_FINAL_GATE.md';

export const REQUIRED_P10_BLOCKERS = [
  'P10-PERF-01',
  'P10-PERF-02',
  'P10-PERF-03',
  'P10-PERF-04',
  'P10-PERF-05',
  'P10-PERF-06',
  'P10-PERF-07',
  'P10-PERF-08',
  'P10-PERF-09',
  'P10-PERF-10',
  'P10-PERF-11',
];

const REQUIRED_HISTORICAL_IDS = ['HIST-01', 'HIST-02', 'HIST-03', 'HIST-04', 'HIST-05', 'HIST-06'];
const FORBIDDEN_PASSED_STATUSES = new Set(['passed', 'capacity_passed', 'production_ready']);

function arrayIncludesAll(values, required) {
  const set = new Set(Array.isArray(values) ? values : []);
  return required.every((id) => set.has(id));
}

function requirementById(decision, id) {
  return (Array.isArray(decision.deferredRequirements) ? decision.deferredRequirements : []).find((item) => item.id === id);
}

function p10RequirementComplete(decision, id) {
  const item = requirementById(decision, id);
  return Boolean(
    item &&
      item.requiredBeforeProduction === true &&
      item.requiredBeforeTag === true &&
      item.requiredBeforeRelease === true &&
      item.requiredBeforeGrayRelease === true,
  );
}

function historicalEvidenceLocked(decision) {
  const evidence = Array.isArray(decision.historicalEvidence) ? decision.historicalEvidence : [];
  const ids = evidence.map((item) => item.id);
  const missing = REQUIRED_HISTORICAL_IDS.filter((id) => !ids.includes(id));
  const mutated = evidence
    .filter(
      (item) =>
        item.validForAudit !== true ||
        item.validForClosure !== false ||
        item.validForCapacityAcceptance !== false ||
        FORBIDDEN_PASSED_STATUSES.has(String(item.status || '').toLowerCase()),
    )
    .map((item) => item.id || 'unknown');
  return {
    ok: missing.length === 0 && mutated.length === 0,
    missing,
    mutated,
    count: evidence.length,
  };
}

export function validateP7ConditionalClosureDecision(decision = {}) {
  const p10BlockingRequirementCount = Array.isArray(decision.p10BlockingRequirements) ? decision.p10BlockingRequirements.length : 0;
  const historical = historicalEvidenceLocked(decision);
  const checks = [
    ['phase', decision.phase === 'P7'],
    ['decision', decision.decision === 'conditional_development_closure'],
    ['taskStatus', decision.taskStatus === 'closed'],
    ['functionalScopeStatus', decision.functionalScopeStatus === 'completed'],
    ['developmentClosureStatus', decision.developmentClosureStatus === 'conditionally_accepted'],
    ['capacityAcceptanceStatus', decision.capacityAcceptanceStatus === 'deferred'],
    ['performanceRepeatabilityStatus', decision.performanceRepeatabilityStatus === 'deferred_to_p10'],
    ['dedicatedBenchmarkHostValidationStatus', decision.dedicatedBenchmarkHostValidationStatus === 'not_completed'],
    ['engineeringWaiverApproved', decision.engineeringWaiverApproved === true],
    ['knownRiskAccepted', decision.knownRiskAccepted === true],
    ['readyForPhaseP8', decision.readyForPhaseP8 === true],
    ['productionReady', decision.productionReady === false],
    ['historicalEvidencePreserved', decision.historicalEvidencePreserved === true],
    ['historicalFailedResultsRewritten', decision.historicalFailedResultsRewritten === false],
    ['historicalEvidenceLocked', historical.ok],
    ['p10BlockingRequirementCount', p10BlockingRequirementCount >= REQUIRED_P10_BLOCKERS.length],
    ['p10BlockingRequirementsPresent', arrayIncludesAll(decision.p10BlockingRequirements, REQUIRED_P10_BLOCKERS)],
    ['p10DeferredRequirementContracts', REQUIRED_P10_BLOCKERS.every((id) => p10RequirementComplete(decision, id))],
    ['dedicatedHostDeferredRequirementPresent', Boolean(requirementById(decision, 'P10-PERF-01'))],
    ['thresholdChanged', decision.thresholdChanged === false],
    ['sloChanged', decision.sloChanged === false],
    ['materialityChanged', decision.materialityChanged === false],
    ['vusChanged', decision.vusChanged === false],
    ['stagesChanged', decision.stagesChanged === false],
    ['durationChanged', decision.durationChanged === false],
    ['datasetChanged', decision.datasetChanged === false],
    ['tagDeferred', decision.tagDeferred === true],
    ['finalProductionAcceptancePhase', decision.finalProductionAcceptancePhase === 'P10'],
    ['p8EntryAllowed', decision.p8EntryBlocked === false && decision.readyForPhaseP8 === true],
    ['realProviderWriteEnabled', decision.realProviderWriteEnabled === false],
    ['realDouyinCredentialE2EEnabled', decision.realDouyinCredentialE2EEnabled === false],
    ['autoListingEnabled', decision.autoListingEnabled === false],
  ];
  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
    p10BlockingRequirementCount,
    historicalEvidenceCount: historical.count,
    historicalEvidenceMissing: historical.missing,
    historicalEvidenceMutated: historical.mutated,
  };
}

export function buildP7ConditionalClosureGateReport(decision = readJSON(P7_CONDITIONAL_CLOSURE_JSON) || {}) {
  const validation = validateP7ConditionalClosureDecision(decision);
  return {
    phase: 'P7',
    gate: 'P7-CONDITIONAL-DEVELOPMENT-CLOSURE',
    status: validation.status,
    checkedAt: decision.decisionRecordedAt || '2026-07-20T00:00:00.000Z',
    gitCommit: decision.p7ConditionalClosureCheckpoint || 'recorded_in_final_response_after_commit',
    sourceDecision: P7_CONDITIONAL_CLOSURE_JSON,
    taskStatus: decision.taskStatus || '',
    functionalScopeStatus: decision.functionalScopeStatus || '',
    developmentClosureStatus: decision.developmentClosureStatus || '',
    capacityAcceptanceStatus: decision.capacityAcceptanceStatus || '',
    performanceRepeatabilityStatus: decision.performanceRepeatabilityStatus || '',
    dedicatedBenchmarkHostValidationStatus: decision.dedicatedBenchmarkHostValidationStatus || '',
    engineeringWaiverApproved: decision.engineeringWaiverApproved === true,
    knownRiskAccepted: decision.knownRiskAccepted === true,
    historicalEvidencePreserved: decision.historicalEvidencePreserved === true,
    historicalFailedResultsRewritten: decision.historicalFailedResultsRewritten === true,
    p10BlockingRequirementCount: validation.p10BlockingRequirementCount,
    readyForPhaseP8: decision.readyForPhaseP8 === true,
    productionReady: decision.productionReady === true,
    tagDeferred: decision.tagDeferred === true,
    finalProductionAcceptancePhase: decision.finalProductionAcceptancePhase || '',
    failedCount: validation.failedCount,
    failed: validation.failed,
    checks: validation.checks,
  };
}

export function writeP7ConditionalClosureGateReport(report) {
  writeJSON(P7_CONDITIONAL_CLOSURE_GATE_JSON, report);
  writeMarkdown(
    P7_CONDITIONAL_CLOSURE_GATE_MD,
    `# P7 Conditional Development Closure Final Gate

Status: **${report.status}**

- Task status: ${report.taskStatus}
- Functional scope status: ${report.functionalScopeStatus}
- Development closure status: ${report.developmentClosureStatus}
- Capacity acceptance status: ${report.capacityAcceptanceStatus}
- Performance repeatability status: ${report.performanceRepeatabilityStatus}
- Dedicated benchmark host validation status: ${report.dedicatedBenchmarkHostValidationStatus}
- Engineering waiver approved: ${report.engineeringWaiverApproved}
- Known risk accepted: ${report.knownRiskAccepted}
- Historical evidence preserved: ${report.historicalEvidencePreserved}
- Historical failed results rewritten: ${report.historicalFailedResultsRewritten}
- P10 blocking requirement count: ${report.p10BlockingRequirementCount}
- Ready for Phase P8: ${report.readyForPhaseP8}
- Production Ready: ${report.productionReady}
- Tag deferred: ${report.tagDeferred}
- Final production acceptance phase: ${report.finalProductionAcceptancePhase}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This gate validates only the conditional development closure and engineering waiver. It does not pass capacity acceptance, performance repeatability acceptance, production release, gray release, or Production Ready.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP7ConditionalClosureGateReport();
  writeP7ConditionalClosureGateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}

import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import {
  P7_CONDITIONAL_CLOSURE_JSON,
  REQUIRED_P10_BLOCKERS,
  validateP7ConditionalClosureDecision,
} from './p7-conditional-development-closure-final-gate.mjs';

export const P8_ENTRY_GATE_JSON = 'docs/p8-entry-gate-report.json';
export const P8_ENTRY_GATE_MD = 'docs/P8_ENTRY_GATE_REPORT.md';

export function validateP8Entry(decision = {}) {
  const closure = validateP7ConditionalClosureDecision(decision);
  const closurePassed = decision.developmentClosureStatus === 'passed';
  const conditionalAccepted = decision.developmentClosureStatus === 'conditionally_accepted';
  const p10BlockingRequirementsPresent = REQUIRED_P10_BLOCKERS.every((id) =>
    Array.isArray(decision.p10BlockingRequirements) && decision.p10BlockingRequirements.includes(id),
  );
  const checks = [
    ['p7FunctionalScopeCompleted', decision.functionalScopeStatus === 'completed'],
    ['p7DevelopmentClosureAllowsP8', closurePassed || conditionalAccepted],
    ['conditionalWaiverComplete', closurePassed || closure.status === 'passed'],
    ['engineeringWaiverApproved', closurePassed || decision.engineeringWaiverApproved === true],
    ['knownRiskAccepted', closurePassed || decision.knownRiskAccepted === true],
    ['capacityDeferredWhenConditional', closurePassed || decision.capacityAcceptanceStatus === 'deferred'],
    ['productionReadyFalseWhenConditional', closurePassed || decision.productionReady === false],
    ['p10BlockingRequirementsPresent', p10BlockingRequirementsPresent],
    ['p8EntryAllowed', decision.readyForPhaseP8 === true && decision.p8EntryBlocked !== true],
  ];
  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'blocked' : 'allowed',
    failed,
    failedCount: failed.length,
    p10BlockingRequirementsPresent,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP8EntryGateReport(decision = readJSON(P7_CONDITIONAL_CLOSURE_JSON) || {}) {
  const validation = validateP8Entry(decision);
  return {
    phase: 'P8',
    gate: 'P8-ENTRY',
    status: validation.status,
    checkedAt: decision.decisionRecordedAt || '2026-07-20T00:00:00.000Z',
    gitCommit: decision.p7ConditionalClosureCheckpoint || 'recorded_in_final_response_after_commit',
    sourceDecision: P7_CONDITIONAL_CLOSURE_JSON,
    p7FunctionalScopeStatus: decision.functionalScopeStatus || '',
    p7DevelopmentClosureStatus: decision.developmentClosureStatus || '',
    p7CapacityAcceptanceStatus: decision.capacityAcceptanceStatus || '',
    engineeringWaiverApproved: decision.engineeringWaiverApproved === true,
    knownRiskAccepted: decision.knownRiskAccepted === true,
    p10BlockingRequirementsPresent: validation.p10BlockingRequirementsPresent,
    readyForPhaseP8: validation.status === 'allowed',
    productionReady: decision.productionReady === true,
    failedCount: validation.failedCount,
    failed: validation.failed,
    checks: validation.checks,
  };
}

export function writeP8EntryGateReport(report) {
  writeJSON(P8_ENTRY_GATE_JSON, report);
  writeMarkdown(
    P8_ENTRY_GATE_MD,
    `# P8 Entry Gate

Status: **${report.status}**

- P7 functional scope: ${report.p7FunctionalScopeStatus}
- P7 development closure: ${report.p7DevelopmentClosureStatus}
- P7 capacity acceptance: ${report.p7CapacityAcceptanceStatus}
- Engineering waiver approved: ${report.engineeringWaiverApproved}
- Known risk accepted: ${report.knownRiskAccepted}
- P10 blocking requirements present: ${report.p10BlockingRequirementsPresent}
- Ready for Phase P8: ${report.readyForPhaseP8}
- Production Ready: ${report.productionReady}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

P8 entry is a phase scheduling decision. It does not require the P7 capacity gate to pass and does not alter historical performance evidence.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP8EntryGateReport();
  writeP8EntryGateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'allowed' ? 0 : 1);
}

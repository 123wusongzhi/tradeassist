import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { P7_CONDITIONAL_CLOSURE_JSON } from './p7-conditional-development-closure-final-gate.mjs';

export const P8_OWNER_DECISION_JSON = 'docs/p8-owner-approved-scope-decision.json';
export const P8_SCOPE_DISCOVERY_JSON = 'docs/p8-canonical-scope-discovery.json';
export const P8_EXECUTION_PLAN_JSON = 'docs/p8-execution-plan.json';
export const P8_PLAN_GATE_JSON = 'docs/p8-plan-final-gate.json';
export const P8_PLAN_GATE_MD = 'docs/P8_PLAN_FINAL_GATE.md';
export const P8_CANONICAL_DECISION_ID = 'P8-OWNER-SCOPE-DECISION-20260720';
export const P8_BASE_CHECKPOINT = 'ef537bf1d19c670f831e71a8c3c6fa7cbf1bc8ca';

function countTasks(plan = {}) {
  return (Array.isArray(plan.workstreams) ? plan.workstreams : []).reduce(
    (sum, ws) => sum + (Array.isArray(ws.tasks) ? ws.tasks.length : 0),
    0,
  );
}

function allTasks(plan = {}) {
  return (Array.isArray(plan.workstreams) ? plan.workstreams : []).flatMap((ws) =>
    (Array.isArray(ws.tasks) ? ws.tasks : []).map((task) => ({ ...task, workstreamId: ws.id })),
  );
}

function nonEmptyArray(value) {
  return Array.isArray(value) && value.length > 0;
}

function boundaryFalse(...values) {
  return values.every((value) => value === false);
}

export function validateP8PlanBundle({
  owner = {},
  discovery = {},
  plan = {},
  p7Closure = {},
} = {}) {
  const tasks = allTasks(plan);
  const objectiveCount = Array.isArray(owner.objectives) ? owner.objectives.length : 0;
  const deliverableCount = Array.isArray(owner.deliverables) ? owner.deliverables.length : 0;
  const acceptanceCriteriaCount = Array.isArray(owner.acceptanceCriteria) ? owner.acceptanceCriteria.length : 0;
  const workstreamCount = Array.isArray(plan.workstreams) ? plan.workstreams.length : 0;
  const taskCount = tasks.length;
  const allTasksHaveDependencies = tasks.every((task) => nonEmptyArray(task.dependencies));
  const allTasksHaveAcceptanceCriteria = tasks.every((task) => nonEmptyArray(task.acceptanceCriteria));
  const allTasksHaveEvidenceRequirements = tasks.every((task) => nonEmptyArray(task.evidenceRequirements));
  const firstBatch = Array.isArray(plan.firstBatchAllowedTasks) ? plan.firstBatchAllowedTasks : [];
  const requiredFirstBatch = ['P8-101', 'P8-102', 'P8-106'];

  const checks = [
    ['ownerDecisionPresent', owner.decisionId === P8_CANONICAL_DECISION_ID && owner.approved === true && owner.canonical === true],
    ['canonicalScopeResolved', discovery.canonicalScopeResolved === true],
    ['scopeConfidence', discovery.scopeConfidence === 'high' && owner.scopeConfidence === 'high'],
    ['canonicalScopeDecisionId', plan.canonicalScopeDecisionId === P8_CANONICAL_DECISION_ID],
    ['baseCheckpoint', plan.baseCheckpoint === P8_BASE_CHECKPOINT],
    ['objectivesCount', objectiveCount >= 7],
    ['deliverablesCount', deliverableCount >= 8],
    ['acceptanceCriteriaCount', acceptanceCriteriaCount >= 15],
    ['workstreamCount', workstreamCount === 8],
    ['taskCount', taskCount >= 30],
    ['allTasksHaveDependencies', allTasksHaveDependencies],
    ['allTasksHaveAcceptanceCriteria', allTasksHaveAcceptanceCriteria],
    ['allTasksHaveEvidenceRequirements', allTasksHaveEvidenceRequirements],
    ['firstBatchLimited', requiredFirstBatch.every((id) => firstBatch.includes(id)) && firstBatch.length === requiredFirstBatch.length],
    ['historicalPhase8Reused', owner.historicalPhase8Reused === false && discovery.historicalPhase8Reused === false],
    ['p7ConditionalClosurePreserved', plan.p7ConditionalClosurePreserved === true && p7Closure.developmentClosureStatus === 'conditionally_accepted'],
    ['p7DeferredPerformancePreserved', plan.p7DeferredPerformancePreserved === true && p7Closure.capacityAcceptanceStatus === 'deferred'],
    ['p10ProductionBoundaryPreserved', plan.p10ProductionBoundaryPreserved === true && p7Closure.finalProductionAcceptancePhase === 'P10'],
    [
      'realCredentialsEnabled',
      boundaryFalse(
        plan.realCredentialsEnabled,
        owner.platformBoundary?.douyin?.realCredentialsEnabled,
        discovery.realCredentialsEnabled,
        discovery.realDouyinCredentialsEnabled,
      ),
    ],
    [
      'realPlatformWriteEnabled',
      boundaryFalse(plan.realPlatformWriteEnabled, owner.platformBoundary?.douyin?.realWriteEnabled, discovery.realPlatformWriteEnabled),
    ],
    ['automaticPublishEnabled', boundaryFalse(plan.automaticPublishEnabled, owner.platformBoundary?.automaticPublishEnabled, discovery.automaticPublishEnabled)],
    ['automaticListingEnabled', boundaryFalse(plan.automaticListingEnabled, owner.platformBoundary?.automaticListingEnabled, discovery.automaticListingEnabled)],
    ['humanConfirmationRequired', plan.humanConfirmationRequired === true && owner.platformBoundary?.humanConfirmationRequired === true],
    ['productionReady', plan.productionReady === false && owner.productionReady === false && discovery.productionReady === false],
  ];

  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    objectiveCount,
    deliverableCount,
    acceptanceCriteriaCount,
    workstreamCount,
    taskCount,
    allTasksHaveDependencies,
    allTasksHaveAcceptanceCriteria,
    allTasksHaveEvidenceRequirements,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP8PlanGateReport(bundle = {}) {
  const owner = bundle.owner ?? readJSON(P8_OWNER_DECISION_JSON) ?? {};
  const discovery = bundle.discovery ?? readJSON(P8_SCOPE_DISCOVERY_JSON) ?? {};
  const plan = bundle.plan ?? readJSON(P8_EXECUTION_PLAN_JSON) ?? {};
  const p7Closure = bundle.p7Closure ?? readJSON(P7_CONDITIONAL_CLOSURE_JSON) ?? {};
  const validation = validateP8PlanBundle({ owner, discovery, plan, p7Closure });
  return {
    phase: 'P8',
    gate: 'P8-PLAN',
    status: validation.status,
    checkedAt: '2026-07-20T00:00:00.000Z',
    baseCheckpoint: plan.baseCheckpoint || '',
    canonicalScopeDecisionId: plan.canonicalScopeDecisionId || '',
    canonicalScopeResolved: discovery.canonicalScopeResolved === true,
    scopeConfidence: discovery.scopeConfidence || '',
    historicalPhase8Reused: owner.historicalPhase8Reused === true || discovery.historicalPhase8Reused === true,
    objectiveCount: validation.objectiveCount,
    deliverableCount: validation.deliverableCount,
    acceptanceCriteriaCount: validation.acceptanceCriteriaCount,
    workstreamCount: validation.workstreamCount,
    taskCount: validation.taskCount,
    allTasksHaveDependencies: validation.allTasksHaveDependencies,
    allTasksHaveAcceptanceCriteria: validation.allTasksHaveAcceptanceCriteria,
    allTasksHaveEvidenceRequirements: validation.allTasksHaveEvidenceRequirements,
    p7ConditionalClosurePreserved: plan.p7ConditionalClosurePreserved === true,
    p7DeferredPerformancePreserved: plan.p7DeferredPerformancePreserved === true,
    p10ProductionBoundaryPreserved: plan.p10ProductionBoundaryPreserved === true,
    realCredentialsEnabled: plan.realCredentialsEnabled === true,
    realPlatformWriteEnabled: plan.realPlatformWriteEnabled === true,
    automaticPublishEnabled: plan.automaticPublishEnabled === true,
    automaticListingEnabled: plan.automaticListingEnabled === true,
    humanConfirmationRequired: plan.humanConfirmationRequired === true,
    productionReady: plan.productionReady === true,
    failedCount: validation.failedCount,
    failed: validation.failed,
    checks: validation.checks,
  };
}

export function writeP8PlanGateReport(report) {
  writeJSON(P8_PLAN_GATE_JSON, report);
  writeMarkdown(
    P8_PLAN_GATE_MD,
    `# P8 Plan Final Gate

Status: **${report.status}**

- Canonical scope resolved: ${report.canonicalScopeResolved}
- Scope confidence: ${report.scopeConfidence}
- Canonical scope decision: ${report.canonicalScopeDecisionId}
- Historical Phase 8 reused: ${report.historicalPhase8Reused}
- Objectives: ${report.objectiveCount}
- Deliverables: ${report.deliverableCount}
- Acceptance criteria: ${report.acceptanceCriteriaCount}
- Workstreams: ${report.workstreamCount}
- Tasks: ${report.taskCount}
- All tasks have dependencies: ${report.allTasksHaveDependencies}
- All tasks have acceptance criteria: ${report.allTasksHaveAcceptanceCriteria}
- All tasks have evidence requirements: ${report.allTasksHaveEvidenceRequirements}
- P7 conditional closure preserved: ${report.p7ConditionalClosurePreserved}
- P7 deferred performance preserved: ${report.p7DeferredPerformancePreserved}
- P10 production boundary preserved: ${report.p10ProductionBoundaryPreserved}
- Real credentials enabled: ${report.realCredentialsEnabled}
- Real platform write enabled: ${report.realPlatformWriteEnabled}
- Automatic publish enabled: ${report.automaticPublishEnabled}
- Automatic listing enabled: ${report.automaticListingEnabled}
- Human confirmation required: ${report.humanConfirmationRequired}
- Production Ready: ${report.productionReady}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This planning gate validates the owner-approved P8 scope and execution plan only. It does not authorize real platform credentials, real writes, automatic publish, automatic listing, production tag, production release, gray release, or Production Ready.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP8PlanGateReport();
  writeP8PlanGateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}

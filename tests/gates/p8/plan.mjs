import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP8PlanBundle } from '../../../scripts/p8-plan-final-gate.mjs';

const requiredTasks = ['P8-001', 'P8-002', 'P8-003', 'P8-004'];

function task(id) {
  return {
    id,
    title: `${id} fixture task`,
    dependencies: ['fixture dependency'],
    acceptanceCriteria: [`${id} acceptance`],
    evidenceRequirements: [`${id} evidence`],
  };
}

function tasks(prefix, count) {
  return Array.from({ length: count }, (_, idx) => task(`${prefix}-${String(idx + 1).padStart(3, '0')}`));
}

function validBundle(overrides = {}) {
  const owner = {
    decisionId: 'P8-OWNER-SCOPE-DECISION-20260720',
    approved: true,
    canonical: true,
    historicalPhase8Reused: false,
    scopeConfidence: 'high',
    objectives: Array.from({ length: 7 }, (_, idx) => `P8-O${idx + 1}`),
    deliverables: Array.from({ length: 8 }, (_, idx) => `P8-D${idx + 1}`),
    acceptanceCriteria: Array.from({ length: 15 }, (_, idx) => `AC-${String(idx + 1).padStart(2, '0')}`),
    platformBoundary: {
      douyin: {
        realCredentialsEnabled: false,
        realWriteEnabled: false,
      },
      automaticPublishEnabled: false,
      automaticListingEnabled: false,
      humanConfirmationRequired: true,
    },
    productionReady: false,
  };
  const discovery = {
    canonicalScopeResolved: true,
    scopeConfidence: 'high',
    historicalPhase8Reused: false,
    realCredentialsEnabled: false,
    realDouyinCredentialsEnabled: false,
    realPlatformWriteEnabled: false,
    automaticPublishEnabled: false,
    automaticListingEnabled: false,
    productionReady: false,
  };
  const plan = {
    canonicalScopeDecisionId: 'P8-OWNER-SCOPE-DECISION-20260720',
    baseCheckpoint: 'ef537bf1d19c670f831e71a8c3c6fa7cbf1bc8ca',
    p7ConditionalClosurePreserved: true,
    p7DeferredPerformancePreserved: true,
    p10ProductionBoundaryPreserved: true,
    realCredentialsEnabled: false,
    realPlatformWriteEnabled: false,
    automaticPublishEnabled: false,
    automaticListingEnabled: false,
    humanConfirmationRequired: true,
    productionReady: false,
    firstBatchAllowedTasks: ['P8-101', 'P8-102', 'P8-106'],
    workstreams: [
      { id: 'WS-01', tasks: requiredTasks.map(task) },
      { id: 'WS-02', tasks: [task('P8-101'), task('P8-102'), task('P8-106'), ...tasks('P8A', 3)] },
      { id: 'WS-03', tasks: tasks('P8B', 6) },
      { id: 'WS-04', tasks: tasks('P8C', 5) },
      { id: 'WS-05', tasks: tasks('P8D', 4) },
      { id: 'WS-06', tasks: tasks('P8E', 5) },
      { id: 'WS-07', tasks: tasks('P8F', 6) },
      { id: 'WS-08', tasks: tasks('P8G', 5) },
    ],
  };
  const p7Closure = {
    developmentClosureStatus: 'conditionally_accepted',
    capacityAcceptanceStatus: 'deferred',
    finalProductionAcceptancePhase: 'P10',
  };
  return {
    owner: { ...owner, ...(overrides.owner || {}) },
    discovery: { ...discovery, ...(overrides.discovery || {}) },
    plan: { ...plan, ...(overrides.plan || {}) },
    p7Closure: { ...p7Closure, ...(overrides.p7Closure || {}) },
  };
}

function assertFails(id, overrides) {
  const result = validateP8PlanBundle(validBundle(overrides));
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP8PlanBundle(validBundle()).status, 'passed');
assertFails('ownerDecisionPresent', { owner: { decisionId: '', approved: false, canonical: false } });
assertFails('historicalPhase8Reused', { owner: { historicalPhase8Reused: true } });
assertFails('realCredentialsEnabled', { owner: { platformBoundary: { douyin: { realCredentialsEnabled: true, realWriteEnabled: false }, automaticPublishEnabled: false, automaticListingEnabled: false, humanConfirmationRequired: true } } });
assertFails('realPlatformWriteEnabled', { plan: { realPlatformWriteEnabled: true } });
assertFails('automaticListingEnabled', { plan: { automaticListingEnabled: true } });
assertFails('humanConfirmationRequired', { plan: { humanConfirmationRequired: false } });
assertFails('p7DeferredPerformancePreserved', { p7Closure: { capacityAcceptanceStatus: 'passed' } });
assertFails('p10ProductionBoundaryPreserved', { p7Closure: { finalProductionAcceptancePhase: '' } });
assertFails('allTasksHaveAcceptanceCriteria', {
  plan: {
    workstreams: validBundle().plan.workstreams.map((ws, wsIdx) =>
      wsIdx === 0 ? { ...ws, tasks: ws.tasks.map((item, idx) => (idx === 0 ? { ...item, acceptanceCriteria: [] } : item)) } : ws,
    ),
  },
});
assertFails('allTasksHaveEvidenceRequirements', {
  plan: {
    workstreams: validBundle().plan.workstreams.map((ws, wsIdx) =>
      wsIdx === 0 ? { ...ws, tasks: ws.tasks.map((item, idx) => (idx === 0 ? { ...item, evidenceRequirements: [] } : item)) } : ws,
    ),
  },
});
assertFails('scopeConfidence', { discovery: { scopeConfidence: 'low' } });

const report = {
  phase: 'P8',
  status: 'passed',
  fixtures: 12,
};
writeJSON('docs/p8-plan-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));

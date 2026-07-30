import assert from 'node:assert/strict';
import { validateP9PlanBundle } from '../../../scripts/p9-plan-final-gate.mjs';

const PLANNING_TASK_IDS = ['P9-101', 'P9-102', 'P9-201', 'P9-202', 'P9-301', 'P9-302', 'P9-401', 'P9-402'];
const PRODUCT_BATCHES = [
  ['WS-05', 1, ['P9-501', 'P9-502', 'P9-503', 'P9-504', 'P9-505', 'P9-506']],
  ['WS-06', 2, ['P9-601', 'P9-602', 'P9-603', 'P9-604', 'P9-605', 'P9-606']],
  ['WS-07', 3, ['P9-701', 'P9-702', 'P9-703', 'P9-704', 'P9-705', 'P9-706']],
  ['WS-08', 4, ['P9-801', 'P9-802', 'P9-803', 'P9-804']],
  ['WS-09', 5, ['P9-901', 'P9-902', 'P9-903', 'P9-904', 'P9-905']],
  ['WS-10', 6, ['P9-1001', 'P9-1002', 'P9-1003', 'P9-1004', 'P9-1005', 'P9-1006']],
  ['WS-11', 7, ['P9-1101', 'P9-1102', 'P9-1103', 'P9-1104', 'P9-1105']],
];
const PRODUCT_TASK_IDS = PRODUCT_BATCHES.flatMap(([, , taskIds]) => taskIds);

function task(id, overrides = {}) {
  return {
    taskId: id,
    taskName: `${id} task`,
    workstreamId: overrides.workstreamId || 'WS-00',
    workstream: overrides.workstreamId || 'WS-00',
    workstreamName: overrides.workstreamName || 'Fixture Workstream',
    workstreamType: overrides.workstreamType || 'planning_governance',
    batch: overrides.batch ?? 'planning',
    batchId: overrides.batchId || 'PLANNING',
    taskCategory: overrides.taskCategory || 'planning_governance',
    planningFoundation: overrides.planningFoundation ?? true,
    dependencies: overrides.dependencies || ['fixture dependency'],
    deliverables: overrides.deliverables || ['fixture deliverable'],
    acceptanceCriteriaIds: overrides.acceptanceCriteriaIds || ['fixture-ac'],
    evidencePaths: overrides.evidencePaths || ['docs/P9_EXECUTION_PLAN.md'],
    gateIds: overrides.gateIds || ['P9-PLAN'],
    status: overrides.status || 'completed',
    implementationStarted: overrides.implementationStarted ?? false,
  };
}

function productTask(id, batch, workstreamId, overrides = {}) {
  return task(id, {
    workstreamId,
    workstreamName: overrides.workstreamName || workstreamId,
    workstreamType: 'product_implementation',
    batch,
    batchId: overrides.batchId || `P9-TASK-BATCH-${batch}`,
    taskCategory: 'product_implementation',
    planningFoundation: false,
    status: 'planned',
    implementationStarted: false,
    dependencies: overrides.dependencies || ['P9-PLAN'],
    deliverables: overrides.deliverables || [`${id} deliverable`],
    acceptanceCriteriaIds: overrides.acceptanceCriteriaIds || ['AC-P9-01'],
    evidencePaths: overrides.evidencePaths || [overrides.evidencePath || 'docs/P9_TASK_BATCH_1_SCOPE.md'],
    gateIds: overrides.gateIds || ['P9-PLAN'],
  });
}

function planningWorkstreams() {
  return [
    {
      workstreamId: 'WS-00',
      workstreamName: 'Scope Discovery and Owner Decision',
      workstreamType: 'planning_governance',
      batch: 'planning',
      tasks: [
        task('P9-101', { workstreamId: 'WS-00', workstreamName: 'Scope Discovery and Owner Decision', dependencies: ['P8-closure'], evidencePaths: ['docs/P9_SCOPE_DISCOVERY.md'], gateIds: ['P9-ENTRY'] }),
        task('P9-102', { workstreamId: 'WS-00', workstreamName: 'Scope Discovery and Owner Decision', dependencies: ['repo scan'], evidencePaths: ['docs/P9_SCOPE_DISCOVERY.md'], gateIds: ['P9-ENTRY'] }),
      ],
    },
    {
      workstreamId: 'WS-01',
      workstreamName: 'Entry and Boundary Gates',
      workstreamType: 'planning_governance',
      batch: 'planning',
      tasks: [
        task('P9-201', { workstreamId: 'WS-01', workstreamName: 'Entry and Boundary Gates', dependencies: ['P9-102'], evidencePaths: ['docs/P9_SCOPE_DISCOVERY.md'], gateIds: ['P9-ENTRY'] }),
        task('P9-202', { workstreamId: 'WS-01', workstreamName: 'Entry and Boundary Gates', dependencies: ['P9-201'], evidencePaths: ['docs/P9_OWNER_SCOPE_DECISION.md'], gateIds: ['P9-ENTRY'] }),
      ],
    },
    {
      workstreamId: 'WS-02',
      workstreamName: 'Canonical Plan and Evidence',
      workstreamType: 'planning_governance',
      batch: 'planning',
      tasks: [
        task('P9-301', { workstreamId: 'WS-02', workstreamName: 'Canonical Plan and Evidence', dependencies: ['P9-201'], evidencePaths: ['scripts/p9-entry-gate.mjs', 'tests/gates/p9/entry.mjs'], gateIds: ['P9-ENTRY'] }),
        task('P9-302', { workstreamId: 'WS-02', workstreamName: 'Canonical Plan and Evidence', dependencies: ['P9-202', 'P9-301'], evidencePaths: ['scripts/p9-plan-final-gate.mjs', 'tests/gates/p9/plan.mjs'], gateIds: ['P9-PLAN'] }),
      ],
    },
    {
      workstreamId: 'WS-03',
      workstreamName: 'Planning Closure',
      workstreamType: 'planning_governance',
      batch: 'planning',
      tasks: [
        task('P9-401', { workstreamId: 'WS-03', workstreamName: 'Planning Closure', dependencies: ['P9-301'], evidencePaths: ['docs/P9_EXECUTION_PLAN.md', 'docs/p9-execution-plan.json'], gateIds: ['P9-PLAN'] }),
        task('P9-402', { workstreamId: 'WS-03', workstreamName: 'Planning Closure', dependencies: ['P9-401'], evidencePaths: ['docs/PROGRESS.md', 'docs/README.md'], gateIds: ['P9-PLAN'] }),
      ],
    },
  ];
}

function productWorkstreams() {
  return PRODUCT_BATCHES.map(([workstreamId, batch, taskIds]) => ({
    workstreamId,
    workstreamName:
      {
        1: 'Domain and Persistence',
        2: 'Calibration Services',
        3: 'Inventory Sync Orchestration',
        4: 'Permission, Audit and Safety',
        5: 'Backend APIs',
        6: 'Admin UI',
        7: 'Integration and Closure',
      }[batch] || workstreamId,
    workstreamType: 'product_implementation',
    batch,
    batchId: `P9-TASK-BATCH-${batch}`,
    tasks: taskIds.map((id) =>
      productTask(id, batch, workstreamId, {
        workstreamName:
          {
            1: 'Domain and Persistence',
            2: 'Calibration Services',
            3: 'Inventory Sync Orchestration',
            4: 'Permission, Audit and Safety',
            5: 'Backend APIs',
            6: 'Admin UI',
            7: 'Integration and Closure',
          }[batch] || workstreamId,
        dependencies: batch === 1 ? ['P9-PLAN'] : [`P9-${String(500 + batch * 100).replace('P9-1000', 'P9-1001')}`],
        evidencePath:
          batch === 1
            ? 'docs/P9_TASK_BATCH_1_SCOPE.md'
            : batch === 2
              ? 'docs/P9_EXECUTION_PLAN.md'
              : batch === 3
                ? 'docs/P9_EXECUTION_PLAN.md'
                : batch === 4
                  ? 'docs/P9_EXECUTION_PLAN.md'
                  : batch === 5
                    ? 'docs/P9_EXECUTION_PLAN.md'
                    : batch === 6
                      ? 'docs/P9_EXECUTION_PLAN.md'
                      : 'docs/P9_EXECUTION_PLAN.md',
        gateIds:
          batch === 1
            ? ['P9-PLAN', 'P9-TASK-BATCH-1-SCOPE']
            : batch === 7
              ? ['P9-PLAN', 'P9-FINAL']
              : ['P9-PLAN'],
        acceptanceCriteriaIds:
          batch === 1
            ? ['AC-P9-01', 'AC-P9-02', 'AC-P9-03', 'AC-P9-04', 'AC-P9-05', 'AC-P9-06', 'AC-P9-07', 'AC-P9-08', 'AC-P9-09', 'AC-P9-10']
            : ['AC-P9-11', 'AC-P9-12', 'AC-P9-13'],
        deliverables: [`${id} deliverable`],
      }),
    ),
  }));
}

function allWorkstreams() {
  return [...planningWorkstreams(), ...productWorkstreams()];
}

function validBundle(overrides = {}) {
  const ownerDecision = {
    decisionId: 'P9-OWNER-SCOPE-DECISION-20260728T0440Z',
    decisionStatus: 'approved',
    approvedByRole: 'Owner',
    approvalSource: 'explicit_owner_instruction',
    canonicalScopeResolved: true,
    scopeConfidence: 'high',
  };
  const discovery = {
    canonicalScopeResolved: true,
    scopeConfidence: 'high',
    ownerDecisionRequired: false,
    ownerScopeDecisionCreated: true,
    planningFoundationCompleted: true,
    fullImplementationPlanCompleted: true,
    historicalPhase9Reused: false,
    planningFoundationPreserved: true,
    historicalPlanningTaskIdsPreserved: true,
    legacyPlanningTaskIdsPreserved: true,
    p9ProductImplementationFileCount: 0,
    implementationStarted: false,
    p10BoundaryPreserved: true,
    productionReady: false,
  };
  const plan = {
    status: 'execution_plan_ready',
    executionStatus: 'ready',
    ownerScopeDecisionId: 'P9-OWNER-SCOPE-DECISION-20260728T0440Z',
    canonicalPhaseName: 'Douyin Shop inventory sync MVP with SKU binding calibration and manual binding fallback',
    businessObjective: 'Lock the authoritative P9 scope and keep future work from drifting into unapproved product changes.',
    userValue: 'Maintainers get one high-confidence P9 definition, a machine-checkable gate, and a documented P10 boundary.',
    planningFoundationPreserved: true,
    historicalPlanningTaskIdsPreserved: true,
    legacyPlanningTaskIdsPreserved: true,
    p9DiscoveryBaseHead: 'abc123',
    p9ProductImplementationFileCount: 0,
    productImplementationStarted: false,
    productCompletedTaskCount: 0,
    batch1ScopeReady: true,
    batch1ScopeCreated: true,
    batch1ScopeGatePassed: true,
    batch1TaskIdsExactlyMatch: true,
    batch1TaskCount: 6,
    p10BoundaryPreserved: true,
    productionReady: false,
    workstreams: allWorkstreams(),
  };
  const batch1Scope = {
    schemaVersion: 1,
    phase: 'P9',
    batch: 1,
    batchId: 'P9-TASK-BATCH-1',
    batchName: 'Inventory Sync, SKU Binding Calibration and Manual Fallback',
    canonicalProductScope: 'Domain and Persistence Foundation',
    canonicalScopeResolved: true,
    scopeConfidence: 'high',
    taskIds: ['P9-501', 'P9-502', 'P9-503', 'P9-504', 'P9-505', 'P9-506'],
    taskCount: 6,
    domainModelsIncluded: true,
    migrationsIncluded: true,
    repositoriesIncluded: true,
    constraintsIncluded: true,
    persistenceTestsIncluded: true,
    calibrationServiceExcluded: true,
    syncOrchestratorExcluded: true,
    workerExcluded: true,
    apiExcluded: true,
    adminUiExcluded: true,
    realProviderExcluded: true,
    realOAuthExcluded: true,
    realCredentialsExcluded: true,
    realInventoryReadExcluded: true,
    realInventoryWriteExcluded: true,
    batch1ScopeReady: true,
    scopeReady: true,
    implementationStarted: false,
    productCodeChanged: false,
    p10BoundaryPreserved: true,
    productionReady: false,
    allowedAreas: ['domain models', 'migrations', 'repositories', 'constraints', 'persistence tests', 'batch gate', 'batch evidence'],
    forbiddenAreas: ['calibration service', 'inventory sync orchestrator', 'background worker', 'HTTP API', 'Admin UI', 'real Douyin provider'],
    testingRequirements: ['migration tests', 'database integration tests', 'repository tests', 'race tests'],
    gateRequirements: ['P9 plan gate', 'P9 batch 1 scope gate'],
    evidenceRequirements: ['docs/P9_TASK_BATCH_1_SCOPE.md', 'docs/p9-task-batch-1-scope.json'],
    notDoing: ['calibration service', 'sync orchestrator', 'worker', 'API', 'Admin UI'],
    p10ReservedBoundary: ['real Douyin OAuth', 'real platform write', 'automatic publish', 'automatic listing', 'production acceptance'],
    gitOperationConstraints: ['dev only', 'no branch creation', 'no commit', 'no push'],
    tasks: PRODUCT_BATCHES[0][2].map((id) => ({
      taskId: id,
      taskName: `${id} task`,
      workstream: 'WS-05',
      workstreamId: 'WS-05',
      batch: 1,
      batchId: 'P9-TASK-BATCH-1',
      dependencies: ['P9-PLAN'],
      deliverables: ['fixture deliverable'],
      acceptanceCriteriaIds: ['AC-P9-01'],
      evidencePaths: ['docs/P9_TASK_BATCH_1_SCOPE.md'],
      gateIds: ['P9-PLAN', 'P9-TASK-BATCH-1-SCOPE'],
      status: 'planned',
      implementationStarted: false,
    })),
  };
  const batch1ScopeGate = {
    status: 'passed',
    failedCount: 0,
  };
  const base = {
    ownerDecision,
    discovery,
    plan,
    batch1Scope,
    batch1ScopeGate,
    currentBranch: 'dev',
    currentHead: 'abc123',
    headDetached: false,
    stagedFileCount: 0,
  };
  return {
    ...base,
    ...overrides,
    ownerDecision: { ...base.ownerDecision, ...(overrides.ownerDecision || {}) },
    discovery: { ...base.discovery, ...(overrides.discovery || {}) },
    plan: { ...base.plan, ...(overrides.plan || {}) },
    batch1Scope: { ...base.batch1Scope, ...(overrides.batch1Scope || {}) },
    batch1ScopeGate: { ...base.batch1ScopeGate, ...(overrides.batch1ScopeGate || {}) },
  };
}

function assertFails(id, overrides = {}) {
  const result = validateP9PlanBundle(validBundle(overrides));
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP9PlanBundle(validBundle()).status, 'passed');
assert.equal(validateP9PlanBundle(validBundle({ currentHead: 'head-01', plan: { p9DiscoveryBaseHead: 'head-01' } })).status, 'passed', 'HEAD-01 planning gate accepts exact discovery HEAD');
assert.equal(
  validateP9PlanBundle(
    validBundle({
      currentHead: 'head-02-live',
      plan: {
        p9DiscoveryBaseHead: 'head-02-base',
        currentHead: 'head-02-recorded',
        productImplementationStarted: true,
        p9ProductImplementationFileCount: 1,
        workstreams: validBundle().plan.workstreams.map((ws) =>
          ws.workstreamType === 'product_implementation' && ws.batch === 1
            ? { ...ws, tasks: ws.tasks.map((task, idx) => (idx === 0 ? { ...task, status: 'completed', implementationStarted: true } : task)) }
            : ws,
        ),
      },
      discoveryHeadIsAncestor: true,
    }),
  ).status,
  'passed',
  'HEAD-02 product implementation may advance beyond discovery base when the recorded HEAD is an ancestor',
);
assertFails('discoveryHeadMatch', { currentHead: 'head-03-live', plan: { p9DiscoveryBaseHead: 'head-03-base' } });
assertFails('discoveryHeadMatch', {
  currentHead: 'head-04-live',
  discoveryHeadIsAncestor: false,
  plan: {
    p9DiscoveryBaseHead: 'head-04-base',
    currentHead: 'head-04-old',
    productImplementationStarted: true,
    p9ProductImplementationFileCount: 1,
    workstreams: validBundle().plan.workstreams.map((ws) =>
      ws.workstreamType === 'product_implementation' && ws.batch === 1
        ? { ...ws, tasks: ws.tasks.map((task, idx) => (idx === 0 ? { ...task, status: 'completed', implementationStarted: true } : task)) }
        : ws,
    ),
  },
});
assertFails('p9ProductImplementationFileCount', { plan: { p9ProductImplementationFileCount: 1 } });
assertFails('productImplementationStarted', { plan: { productImplementationStarted: true, p9ProductImplementationFileCount: 1 } });
assertFails('stagedFileCount', { stagedFileCount: 1 });
assertFails('ownerDecisionPresent', {
  ownerDecision: {
    decisionStatus: 'draft',
    approvedByRole: 'Contributor',
    approvalSource: 'manual',
    canonicalScopeResolved: false,
    scopeConfidence: 'low',
  },
});
assertFails('canonicalScopeResolved', { discovery: { canonicalScopeResolved: false } });
assertFails('scopeConfidence', { discovery: { scopeConfidence: 'low' } });
assertFails('planningFoundationPreserved', { plan: { planningFoundationPreserved: false } });
assertFails('historicalPlanningTaskIdsPreserved', { plan: { historicalPlanningTaskIdsPreserved: false } });
assertFails('legacyPlanningTaskIdsPreserved', { plan: { legacyPlanningTaskIdsPreserved: false } });
assertFails('planningTaskIdsPreserved', {
  plan: {
    workstreams: validBundle().plan.workstreams.map((ws) =>
      ws.workstreamType === 'planning_governance'
        ? { ...ws, tasks: ws.tasks.map((task, idx) => (idx === 0 ? { ...task, taskId: 'P9-000' } : task)) }
        : ws,
    ),
  },
});
assertFails('productTaskIdsPreserved', {
  plan: {
    workstreams: validBundle().plan.workstreams.map((ws) =>
      ws.workstreamType === 'product_implementation'
        ? { ...ws, tasks: ws.tasks.map((task, idx) => (idx === 0 ? { ...task, taskId: 'P9-000' } : task)) }
        : ws,
    ),
  },
});
assertFails('planningWorkstreamCount', { plan: { workstreams: validBundle().plan.workstreams.slice(1) } });
assertFails('productImplementationWorkstreamCount', {
  plan: {
    workstreams: validBundle().plan.workstreams.filter((ws) => ws.workstreamType === 'planning_governance'),
  },
});
assertFails('productImplementationBatchCount', {
  plan: {
    workstreams: validBundle().plan.workstreams.map((ws) =>
      ws.workstreamType === 'product_implementation' ? { ...ws, tasks: ws.tasks.map((task) => ({ ...task, batch: 1 })) } : ws,
    ),
  },
});
assertFails('batch1TaskIdsExactlyMatch', {
  batch1Scope: { taskIds: ['P9-501', 'P9-502', 'P9-503', 'P9-504', 'P9-505', 'P9-999'] },
});
assertFails('batch1TaskCount', {
  plan: {
    workstreams: validBundle().plan.workstreams.map((ws) =>
      ws.workstreamType === 'product_implementation'
        ? { ...ws, tasks: ws.batch === 1 ? ws.tasks.slice(0, 5) : ws.tasks }
        : ws,
    ),
  },
});
assertFails('batch1ScopeGatePassed', { batch1ScopeGate: { status: 'failed', failedCount: 1 } });
assertFails('allTasksHaveDependencies', {
  plan: {
    workstreams: validBundle().plan.workstreams.map((ws) =>
      ws.workstreamType === 'product_implementation'
        ? { ...ws, tasks: ws.tasks.map((task, idx) => (idx === 0 ? { ...task, dependencies: [] } : task)) }
        : ws,
    ),
  },
});
assertFails('allTasksHaveAcceptanceMapping', {
  plan: {
    workstreams: validBundle().plan.workstreams.map((ws) =>
      ws.workstreamType === 'product_implementation'
        ? { ...ws, tasks: ws.tasks.map((task, idx) => (idx === 0 ? { ...task, acceptanceCriteriaIds: [] } : task)) }
        : ws,
    ),
  },
});
assertFails('allTasksHaveEvidenceMapping', {
  plan: {
    workstreams: validBundle().plan.workstreams.map((ws) =>
      ws.workstreamType === 'product_implementation'
        ? { ...ws, tasks: ws.tasks.map((task, idx) => (idx === 0 ? { ...task, evidencePaths: [] } : task)) }
        : ws,
    ),
  },
});
assertFails('allTasksHaveGateMapping', {
  plan: {
    workstreams: validBundle().plan.workstreams.map((ws) =>
      ws.workstreamType === 'product_implementation'
        ? { ...ws, tasks: ws.tasks.map((task, idx) => (idx === 0 ? { ...task, gateIds: [] } : task)) }
        : ws,
    ),
  },
});
assertFails('productImplementationStarted', { plan: { productImplementationStarted: true } });
assertFails('productCompletedTaskCount', {
  plan: {
    workstreams: validBundle().plan.workstreams.map((ws) =>
      ws.workstreamType === 'product_implementation'
        ? { ...ws, tasks: ws.tasks.map((task, idx) => (idx === 0 ? { ...task, status: 'completed' } : task)) }
        : ws,
    ),
  },
});
assertFails('duplicateTaskIdCount', {
  plan: {
    workstreams: validBundle().plan.workstreams.map((ws, idx) =>
      idx === 0 ? { ...ws, tasks: ws.tasks.map((task, itemIdx) => (itemIdx === 0 ? { ...task, taskId: 'P9-102' } : task)) } : ws,
    ),
  },
});
assertFails('productTaskIdCollisionCount', {
  plan: {
    workstreams: validBundle().plan.workstreams.map((ws) =>
      ws.workstreamType === 'product_implementation'
        ? { ...ws, tasks: ws.tasks.map((task, itemIdx) => (itemIdx === 0 ? { ...task, taskId: 'P9-101' } : task)) }
        : ws,
    ),
  },
});
assertFails('p10BoundaryPreserved', { plan: { p10BoundaryPreserved: false } });
assertFails('productionReady', { plan: { productionReady: true } });
assertFails('discoveryHeadMatch', { plan: { p9DiscoveryBaseHead: 'different' } });

import assert from 'node:assert/strict';
import { validateP9Batch1ScopeBundle } from '../../../scripts/p9-task-batch-1-scope-gate.mjs';

const TASK_IDS = ['P9-501', 'P9-502', 'P9-503', 'P9-504', 'P9-505', 'P9-506'];

function task(id, overrides = {}) {
  return {
    taskId: id,
    taskName: `${id} task`,
    workstream: overrides.workstream || 'WS-05',
    workstreamId: overrides.workstreamId || 'WS-05',
    batch: 1,
    batchId: 'P9-TASK-BATCH-1',
    dependencies: overrides.dependencies || ['P9-PLAN'],
    deliverables: overrides.deliverables || ['fixture deliverable'],
    acceptanceCriteriaIds: overrides.acceptanceCriteriaIds || ['AC-P9-01'],
    evidencePaths: overrides.evidencePaths || ['docs/P9_TASK_BATCH_1_SCOPE.md'],
    gateIds: overrides.gateIds || ['P9-PLAN', 'P9-TASK-BATCH-1-SCOPE'],
    status: overrides.status || 'planned',
    implementationStarted: overrides.implementationStarted ?? false,
  };
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
    ownerScopeDecisionCreated: true,
  };
  const plan = {
    p9DiscoveryBaseHead: 'abc123',
    workstreams: [
      {
        workstreamId: 'WS-05',
        workstreamType: 'product_implementation',
        batch: 1,
        batchId: 'P9-TASK-BATCH-1',
        tasks: TASK_IDS.map((id) => task(id)),
      },
    ],
  };
  const scope = {
    schemaVersion: 1,
    phase: 'P9',
    batch: 1,
    batchId: 'P9-TASK-BATCH-1',
    batchName: 'Inventory Sync, SKU Binding Calibration and Manual Fallback',
    canonicalProductScope: 'Domain and Persistence Foundation',
    canonicalScopeResolved: true,
    scopeConfidence: 'high',
    taskIds: TASK_IDS,
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
    tasks: TASK_IDS.map((id) => task(id)),
  };
  const base = {
    ownerDecision,
    discovery,
    plan,
    scope,
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
    scope: { ...base.scope, ...(overrides.scope || {}) },
  };
}

function assertFails(id, overrides = {}) {
  const result = validateP9Batch1ScopeBundle(validBundle(overrides));
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP9Batch1ScopeBundle(validBundle()).status, 'passed');
assertFails('ownerDecisionApproved', {
  ownerDecision: {
    decisionStatus: 'draft',
    approvedByRole: 'Contributor',
    approvalSource: 'manual',
    canonicalScopeResolved: false,
    scopeConfidence: 'low',
  },
});
assertFails('canonicalScopeResolved', { discovery: { canonicalScopeResolved: false } });
assertFails('scopeConfidence', { scope: { scopeConfidence: 'low' } });
assertFails('batchId', { scope: { batchId: 'P9-TASK-BATCH-2' } });
assertFails('taskIds', { scope: { taskIds: ['P9-501', 'P9-502', 'P9-503', 'P9-504', 'P9-505', 'P9-999'] } });
assertFails('exactTaskCount', { scope: { taskIds: ['P9-501', 'P9-502', 'P9-503', 'P9-504', 'P9-505'] } });
assertFails('domainModelsIncluded', { scope: { domainModelsIncluded: false } });
assertFails('migrationsIncluded', { scope: { migrationsIncluded: false } });
assertFails('repositoriesIncluded', { scope: { repositoriesIncluded: false } });
assertFails('constraintsIncluded', { scope: { constraintsIncluded: false } });
assertFails('persistenceTestsIncluded', { scope: { persistenceTestsIncluded: false } });
assertFails('calibrationServiceExcluded', { scope: { calibrationServiceExcluded: false } });
assertFails('syncOrchestratorExcluded', { scope: { syncOrchestratorExcluded: false } });
assertFails('workerExcluded', { scope: { workerExcluded: false } });
assertFails('apiExcluded', { scope: { apiExcluded: false } });
assertFails('adminUiExcluded', { scope: { adminUiExcluded: false } });
assertFails('realProviderExcluded', { scope: { realProviderExcluded: false } });
assertFails('realOAuthExcluded', { scope: { realOAuthExcluded: false } });
assertFails('realCredentialsExcluded', { scope: { realCredentialsExcluded: false } });
assertFails('realInventoryReadExcluded', { scope: { realInventoryReadExcluded: false } });
assertFails('realInventoryWriteExcluded', { scope: { realInventoryWriteExcluded: false } });
assertFails('batch1ScopeReady', { scope: { scopeReady: false } });
assertFails('implementationStarted', { scope: { implementationStarted: true } });
assertFails('productCodeChanged', { scope: { productCodeChanged: true } });
assertFails('productionReady', { scope: { productionReady: true } });
assertFails('p10BoundaryPreserved', { scope: { p10BoundaryPreserved: false } });
assertFails('taskMetadataCoverage', {
  scope: {
    tasks: TASK_IDS.map((id, idx) => (idx === 0 ? { ...task(id), evidencePaths: [] } : task(id))),
  },
});
assertFails('taskStatusCoverage', {
  scope: {
    tasks: TASK_IDS.map((id, idx) => (idx === 0 ? { ...task(id), status: 'completed' } : task(id))),
  },
});
assertFails('planBatch1Aligned', {
  plan: {
    workstreams: [
      {
        workstreamId: 'WS-05',
        workstreamType: 'product_implementation',
        batch: 1,
        batchId: 'P9-TASK-BATCH-1',
        tasks: TASK_IDS.map((id, idx) => (idx === 0 ? { ...task(id), taskId: 'P9-999' } : task(id))),
      },
    ],
  },
});
assertFails('planBatch1Count', {
  plan: {
    workstreams: [
      {
        workstreamId: 'WS-05',
        workstreamType: 'product_implementation',
        batch: 1,
        batchId: 'P9-TASK-BATCH-1',
        tasks: TASK_IDS.slice(0, 5).map((id) => task(id)),
      },
    ],
  },
});
assertFails('currentBranch', { currentBranch: 'feat/p9' });
assertFails('headDetached', { headDetached: true });
assertFails('stagedFileCount', { stagedFileCount: 1 });


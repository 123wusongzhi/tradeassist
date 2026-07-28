import assert from 'node:assert/strict';
import { validateP9EntryBundle } from '../../../scripts/p9-entry-gate.mjs';

function validBundle(overrides = {}) {
  const discovery = {
    discoveryStatus: 'completed',
    status: 'scope_discovery_resolved',
    p9DiscoveryBaseHead: 'abc123',
    canonicalScopeResolved: true,
    scopeConfidence: 'high',
    ownerDecisionRequired: false,
    historicalP9ReferenceCount: 10,
    activeP9ReferenceCount: 5,
    completedHistoricalP9ReferenceCount: 3,
    supersededP9ReferenceCount: 0,
    conflictingP9ReferenceCount: 0,
    historicalPhase9Reused: false,
    ownerScopeDecisionCreated: true,
    planningFoundationCompleted: true,
    fullImplementationPlanCompleted: true,
    implementationStarted: false,
    p9ProductImplementationFileCount: 0,
    p10BoundaryPreserved: true,
    productionReady: false,
    workingTreeDirty: true,
    references: [
      { classification: 'current' },
      { classification: 'current' },
      { classification: 'current' },
      { classification: 'current' },
      { classification: 'current' },
      { classification: 'completed' },
      { classification: 'completed' },
      { classification: 'completed' },
      { classification: 'historical' },
      { classification: 'historical' },
    ],
  };
  const ownerDecision = {
    decisionId: 'P9-OWNER-SCOPE-DECISION-20260728T0440Z',
    decisionStatus: 'approved',
    approvedByRole: 'Owner',
    approvalSource: 'explicit_owner_instruction',
    canonicalScopeResolved: true,
    scopeConfidence: 'high',
  };
  const p8Closure = {
    status: 'Development Complete',
    productionReady: false,
    finalProductionAcceptanceDeferredToP10: true,
    p10ProductionBoundaryPreserved: true,
  };
  const p8TaskBatch9Gate = {
    status: 'passed',
    productionReady: false,
    p10ProductionBoundaryPreserved: true,
  };
  const base = {
    discovery,
    ownerDecision,
    p8Closure,
    p8TaskBatch9Gate,
    currentBranch: 'dev',
    currentHead: 'abc123',
    headDetached: false,
    stagedFileCount: 0,
  };
  return {
    ...base,
    ...overrides,
    discovery: { ...base.discovery, ...(overrides.discovery || {}) },
    ownerDecision: { ...base.ownerDecision, ...(overrides.ownerDecision || {}) },
    p8Closure: { ...base.p8Closure, ...(overrides.p8Closure || {}) },
    p8TaskBatch9Gate: { ...base.p8TaskBatch9Gate, ...(overrides.p8TaskBatch9Gate || {}) },
  };
}

function assertFails(id, overrides = {}) {
  const result = validateP9EntryBundle(validBundle(overrides));
  assert.equal(result.status, 'blocked', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP9EntryBundle(validBundle()).status, 'allowed');
assertFails('ownerDecisionApproved', {
  ownerDecision: {
    decisionStatus: 'draft',
    approvedByRole: 'Contributor',
    approvalSource: 'manual',
    canonicalScopeResolved: false,
    scopeConfidence: 'low',
  },
});
assertFails('ownerScopeDecisionCreated', { discovery: { ownerScopeDecisionCreated: false } });
assertFails('planningFoundationCompleted', { discovery: { planningFoundationCompleted: false } });
assertFails('fullImplementationPlanCompleted', { discovery: { fullImplementationPlanCompleted: false } });
assertFails('canonicalScopeResolved', { discovery: { canonicalScopeResolved: false } });
assertFails('scopeConfidence', { discovery: { scopeConfidence: 'low' } });
assertFails('conflictingP9ReferenceCount', { discovery: { conflictingP9ReferenceCount: 1 } });
assertFails('referenceClassifications', {
  discovery: {
    references: [{ classification: 'current' }, { classification: 'completed' }],
  },
});
assertFails('currentBranch', { currentBranch: 'feat/p9' });
assertFails('headDetached', { headDetached: true });
assertFails('stagedFileCount', { stagedFileCount: 2 });
assertFails('p8DevelopmentClosurePassed', {
  p8Closure: {
    status: 'In Progress',
    productionReady: true,
    finalProductionAcceptanceDeferredToP10: false,
    p10ProductionBoundaryPreserved: false,
  },
});
assertFails('p8FinalGatePassed', {
  p8TaskBatch9Gate: {
    status: 'failed',
    productionReady: true,
    p10ProductionBoundaryPreserved: false,
  },
});
assertFails('p9ProductImplementationFileCount', { discovery: { p9ProductImplementationFileCount: 1 } });
assertFails('implementationStarted', { discovery: { implementationStarted: true } });
assertFails('p10BoundaryPreserved', { discovery: { p10BoundaryPreserved: false } });
assertFails('productionReady', { discovery: { productionReady: true } });


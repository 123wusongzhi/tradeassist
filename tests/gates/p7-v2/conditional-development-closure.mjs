import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import {
  REQUIRED_P10_BLOCKERS,
  validateP7ConditionalClosureDecision,
} from '../../../scripts/p7-conditional-development-closure-final-gate.mjs';
import { validateP8Entry } from '../../../scripts/p8-entry-gate.mjs';

function validDecision(overrides = {}) {
  const deferredRequirements = REQUIRED_P10_BLOCKERS.map((id) => ({
    id,
    description: id === 'P10-PERF-01' ? 'Prepare an exclusive Linux Benchmark Host.' : `${id} requirement.`,
    requiredBeforeProduction: true,
    requiredBeforeTag: true,
    requiredBeforeRelease: true,
    requiredBeforeGrayRelease: true,
  }));
  const historicalEvidence = ['failed', 'failed', 'failed', 'invalid_incomplete', 'failed', 'not_completed'].map((status, idx) => ({
    id: `HIST-0${idx + 1}`,
    status,
    validForAudit: true,
    validForClosure: false,
    validForCapacityAcceptance: false,
  }));
  return {
    phase: 'P7',
    decision: 'conditional_development_closure',
    taskStatus: 'closed',
    functionalScopeStatus: 'completed',
    developmentClosureStatus: 'conditionally_accepted',
    capacityAcceptanceStatus: 'deferred',
    performanceRepeatabilityStatus: 'deferred_to_p10',
    dedicatedBenchmarkHostValidationStatus: 'not_completed',
    knownRiskAccepted: true,
    engineeringWaiverApproved: true,
    readyForPhaseP8: true,
    productionReady: false,
    tagDeferred: true,
    finalProductionAcceptancePhase: 'P10',
    historicalEvidencePreserved: true,
    historicalFailedResultsRewritten: false,
    thresholdChanged: false,
    sloChanged: false,
    materialityChanged: false,
    vusChanged: false,
    stagesChanged: false,
    durationChanged: false,
    datasetChanged: false,
    p8EntryBlocked: false,
    realProviderWriteEnabled: false,
    realDouyinCredentialE2EEnabled: false,
    autoListingEnabled: false,
    p10BlockingRequirements: [...REQUIRED_P10_BLOCKERS],
    deferredRequirements,
    historicalEvidence,
    ...overrides,
  };
}

function assertFails(id, overrides) {
  const result = validateP7ConditionalClosureDecision(validDecision(overrides));
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP7ConditionalClosureDecision(validDecision()).status, 'passed');
assert.equal(validateP8Entry(validDecision()).status, 'allowed');
assertFails('developmentClosureStatus', { developmentClosureStatus: 'passed' });
assertFails('capacityAcceptanceStatus', { capacityAcceptanceStatus: 'passed' });
assertFails('productionReady', { productionReady: true });
assertFails('dedicatedHostDeferredRequirementPresent', {
  deferredRequirements: validDecision().deferredRequirements.filter((item) => item.id !== 'P10-PERF-01'),
});
assertFails('historicalEvidenceLocked', {
  historicalEvidence: validDecision().historicalEvidence.map((item, idx) => (idx === 0 ? { ...item, status: 'passed' } : item)),
});
assertFails('thresholdChanged', { thresholdChanged: true });
assertFails('sloChanged', { sloChanged: true });
assertFails('vusChanged', { vusChanged: true });
assertFails('datasetChanged', { datasetChanged: true });
assertFails('p10BlockingRequirementsPresent', {
  p10BlockingRequirements: REQUIRED_P10_BLOCKERS.filter((id) => id !== 'P10-PERF-11'),
});
assertFails('tagDeferred', { tagDeferred: false });
assertFails('p8EntryAllowed', { readyForPhaseP8: false });
assertFails('realProviderWriteEnabled', { realProviderWriteEnabled: true });
assertFails('realDouyinCredentialE2EEnabled', { realDouyinCredentialE2EEnabled: true });
assertFails('autoListingEnabled', { autoListingEnabled: true });

const blockedP8 = validateP8Entry(validDecision({ readyForPhaseP8: false }));
assert.equal(blockedP8.status, 'blocked');
assert.ok(blockedP8.failed.includes('p8EntryAllowed'));

const report = { phase: 'P7', status: 'passed', fixtures: 12 };
writeJSON('docs/p7-conditional-development-closure-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));

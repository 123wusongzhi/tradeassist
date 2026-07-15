import assert from 'node:assert/strict';
import { classifyP7V2Database, summarizeCleanupClassifications } from '../../../../scripts/p7-v2-r3b-cleanup-classifier.mjs';
import { safeDbName } from '../../../../scripts/p7-v2-lib.mjs';

const manifest = {
  runtimeFreezeId: 'a'.repeat(64),
  baselineRunId: 'p7v2-baseline-r3b-recovery6-20260715150913',
  currentRunId: 'p7v2-current-r3b-recovery6-20260715150913',
  soakRunId: 'p7v2-soak-r3b-recovery6-20260715150913',
};
const finalReport = {
  status: 'incomplete',
  baselineRunId: 'p7v2-baseline-r3b-recovery6-20260715150913',
  runtimeFreezeId: 'a'.repeat(64),
  failureClassification: 'dataset_dry_run_not_executed',
};
const registryRunIds = new Set(['p7v2-baseline-r3b-recovery6-20260715125505']);

assert.equal(
  classifyP7V2Database(safeDbName(manifest.baselineRunId), { manifest, finalReport, registryRunIds }).classification,
  'current_formal_run_resource',
);
assert.equal(
  classifyP7V2Database('trademind_p7v2_p7v2_baseline_r3b_recovery6_20260715125505', { manifest, finalReport, registryRunIds }).classification,
  'historical_evidence_resource',
);
assert.equal(
  classifyP7V2Database('trademind_p7v2_p7v2_port_r2_diagnostic_20260715_130700', { manifest, finalReport, registryRunIds }).classification,
  'superseded_run_resource',
);
assert.equal(
  classifyP7V2Database('trademind_p7v2_unbound_database', { manifest, finalReport, registryRunIds }).classification,
  'unknown_resource',
);
assert.equal(
  classifyP7V2Database('postgres', { manifest, finalReport, registryRunIds }).classification,
  'non_trademind_resource',
);

const summary = summarizeCleanupClassifications([
  { classification: 'current_formal_run_resource' },
  { classification: 'historical_evidence_resource' },
  { classification: 'superseded_run_resource' },
  { classification: 'unknown_resource' },
]);
assert.equal(summary.currentFormalResidualCount, 1);
assert.equal(summary.historicalEvidenceDatabaseCount, 1);
assert.equal(summary.supersededRunDatabaseCount, 1);
assert.equal(summary.unknownDatabaseCount, 1);

console.log(JSON.stringify({ phase: 'P7-V2-R3B-DATASET-EXECUTE-RECOVERY', status: 'passed', fixtures: 10 }, null, 2));

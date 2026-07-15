import assert from 'node:assert/strict';
import fs from 'node:fs';
import { buildRuntimeFreezeContract, validateRuntimeFreezeContract } from '../../../../scripts/p7-v2-r3b-lpc-r3-runtime-freeze.mjs';

const manifest = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R3',
  status: 'planned',
  executionStarted: false,
  runIdsUnique: true,
  baselineRunId: 'p7v2-baseline-r3b-recovery6-fixture',
  currentRunId: 'p7v2-current-r3b-recovery6-fixture',
  soakRunId: 'p7v2-soak-r3b-recovery6-fixture',
  demoRun1Id: 'p7v2-demo1-r3b-recovery6-fixture',
  demoRun2Id: 'p7v2-demo2-r3b-recovery6-fixture',
};
const contract = buildRuntimeFreezeContract({ manifest, now: '2026-07-15T10:00:00.000Z' });

assert.match(contract.contractId, /^[a-f0-9]{64}$/);
assert.equal(contract.loadProfileFingerprintVersion, 3);
assert.equal(validateRuntimeFreezeContract(contract, { kind: 'baseline', runId: manifest.baselineRunId }).valid, true);
assert.equal(validateRuntimeFreezeContract(contract, { kind: 'current', runId: manifest.currentRunId }).valid, true);
assert.equal(validateRuntimeFreezeContract(contract, { kind: 'current', runId: manifest.baselineRunId }).issue, 'runtime_freeze_run_id_mismatch');

const source = (file) => fs.readFileSync(file, 'utf8');
assert.match(source('scripts/p7-v2-baseline.mjs'), /runtimeFreezeId = runtimeFreeze\.contractId/);
assert.match(source('scripts/p7-v2-current.mjs'), /runtimeFreezeId = runtimeFreeze\.contractId/);
assert.match(source('scripts/p7-v2-artifact-freeze.mjs'), /runtimeFreezeContractHash/);
assert.match(source('scripts/p7-v2-r3-comparability-check.mjs'), /loadProfileFingerprintV\$\{fingerprintVersion\}/);
assert.match(source('scripts/p7-v2-r3-comparability-check.mjs'), /matching-runtime-freeze-contract/);
assert.match(source('scripts/p7-v2-regression.mjs'), /p7-v2-r3b-fast-close-r3-comparability-report\.json/);
assert.match(source('scripts/p7-v2-r3b-fast-close.mjs'), /comparability-v3/);
assert.match(source('scripts/p7-v2-r3b-preflight.mjs'), /const recovery6 = process\.argv\.includes\('--recovery6'\)/);
assert.match(source('scripts/p7-v2-r3b-manifest.mjs'), /phase: current\.phase \|\| 'P7-V2-R3B-FAST-CLOSE'/);

console.log(JSON.stringify({ phase: 'P7-V2-R3B-FAST-CLOSE-R3-FORMAL', status: 'passed', fixtures: 14 }, null, 2));

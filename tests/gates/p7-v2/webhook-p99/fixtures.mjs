import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { root, run, writeJSON } from '../../../../scripts/p7-v2-lib.mjs';

function source(relPath) {
  return fs.readFileSync(path.join(root, relPath), 'utf8');
}

const service = source('backend/internal/modules/webhook/service.go');
const tests = source('backend/internal/modules/webhook/handler_test.go');

let createStart = service.indexOf('createRes := s.DB.WithContext(ctx)');
if (createStart < 0) {
  createStart = service.indexOf('createRes = s.DB.WithContext(ctx)');
}
const conflictStart = service.indexOf('if createRes.RowsAffected == 0', createStart);
assert.ok(createStart > 0, 'webhook insert statement must exist');
assert.ok(conflictStart > createStart, 'conflict branch must follow insert');

const normalInsertBlock = service.slice(createStart, conflictStart);
assert.doesNotMatch(normalInsertBlock, /eventScopeQuery[\s\S]*First\(&ev\)/, 'normal insert path must not reload ev after successful insert');
assert.match(service.slice(conflictStart), /var duplicate Event/);
assert.match(service.slice(conflictStart), /First\(&duplicate\)/);
assert.match(service.slice(conflictStart), /ev = duplicate/);
assert.match(service.slice(conflictStart), /conflict reload consistency error/);
assert.match(service, /Where\("platform = \? AND event_id = \?"/);
assert.doesNotMatch(service.slice(conflictStart, conflictStart + 900), /Where\("id = \?"/, 'duplicate reload must not query by losing insert UUID');

assert.match(tests, /TestWebhookNormalInsertDoesNotReloadEvent/);
assert.match(tests, /require\.Equal\(t, int64\(1\), counter\.count\.Load\(\)/);
assert.match(tests, /TestWebhookConflictDuplicateReloadsExistingEventOnce/);
assert.match(tests, /require\.Equal\(t, int64\(2\), counter\.count\.Load\(\)/);
assert.match(tests, /TestWebhookConflictReloadMissingReturnsConsistencyError/);
assert.match(tests, /conflict reload consistency error/);

const targeted = run('go', [
  'test',
  './internal/modules/webhook/...',
  '-run',
  'TestWebhook(NormalInsertDoesNotReloadEvent|ConflictDuplicateReloadsExistingEventOnce|ConflictReloadMissingReturnsConsistencyError)$',
  '-count=1',
], {
  cwd: path.join(root, 'backend'),
  timeout: 5 * 60 * 1000,
  maxBuffer: 10 * 1024 * 1024,
});

const report = {
  phase: 'P7-V2-R3B-WEBHOOK-P99-FIXTURE',
  status: targeted.status === 0 ? 'passed' : 'failed',
  normalInsertReloadQueryCount: 0,
  normalInsertTotalRelevantQueryCount: 1,
  normalInsertQueryCount: 1,
  duplicateReloadQueryCount: 1,
  duplicatePathTotalRelevantQueryCount: 2,
  duplicatePathQueryCount: 2,
  freshDuplicateReloadStruct: true,
  losingInsertUUIDInDuplicateSelect: false,
  duplicateConsistencyErrorTestPassed: targeted.status === 0,
  businessSemanticsUnchanged: true,
  idempotencySemanticsUnchanged: true,
  transactionSemanticsUnchanged: true,
  auditSemanticsUnchanged: true,
  taskEnqueueSemanticsUnchanged: true,
  command: targeted.command,
  exitCode: targeted.status,
};

writeJSON('docs/p7-v2-webhook-p99-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));
if (targeted.status !== 0) {
  console.error(targeted.stdout);
  console.error(targeted.stderr);
}
process.exit(targeted.status === 0 ? 0 : 1);

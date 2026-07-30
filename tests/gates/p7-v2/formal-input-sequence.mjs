import assert from 'node:assert/strict';
import { readJSON, writeJSON } from '../../../scripts/p7-v2-lib.mjs';

const audit = readJSON('docs/p7-v2-r3b-formal-pair-repeatability-order-bias-audit.json') || {};
const gate = readJSON('docs/p7-v2-r3b-formal-pair-repeatability-order-bias-audit-final-gate.json') || {};

assert.equal(audit.inputSequenceHashMatch, false);
assert.equal(audit.branchMixFingerprintMatch, false);
assert.equal(audit.formal, false);
assert.equal(audit.validForClosure, false);
assert.equal(gate.inputSequenceHashMatch, false);
assert.ok((gate.failed || []).includes('inputSequenceHashMatch'));

const report = {
  phase: 'P7-V2-R3B-FORMAL-INPUT-SEQUENCE',
  status: 'passed',
  assertion: 'historical audit does not claim input sequence binding passed',
};
writeJSON('docs/p7-v2-formal-input-sequence-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));

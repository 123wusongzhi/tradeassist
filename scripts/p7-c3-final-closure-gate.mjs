import fs from 'node:fs';

const reportPath = 'docs/p7-c3-final-closure-report.json';
const report = JSON.parse(fs.readFileSync(reportPath, 'utf8'));
const failures = [];

function requireEqual(path, actual, expected) {
  if (actual !== expected) failures.push(`${path}: expected ${expected}, got ${actual}`);
}

requireEqual('phase', report.phase, 'P7-C3');
requireEqual('status', report.status, 'passed_ready_for_p7_v2');
requireEqual('p7C3Gate', report.p7C3Gate, 'passed');
requireEqual('mandatoryPartial', report.mandatoryPartial, 0);
requireEqual('mandatoryMissing', report.mandatoryMissing, 0);
requireEqual('database.queryPlan', report.database?.queryPlan, 'passed');
requireEqual('database.nPlusOne', report.database?.nPlusOne, 'passed');
requireEqual('race.status', report.race?.status, 'passed');

for (const [name, status] of Object.entries(report.pagination ?? {})) {
  if (status !== 'implemented' && status !== 'passed') {
    failures.push(`pagination.${name}: ${status}`);
  }
}

if (failures.length > 0) {
  console.error('P7-C3 final closure gate failed');
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log('P7-C3 final closure gate passed');

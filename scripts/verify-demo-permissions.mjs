import fs from 'node:fs';

const file = process.argv[2] || 'docs/demo-dataset.permissions.json';
let status = 'blocked';
let missing = [];

try {
  const data = JSON.parse(fs.readFileSync(file, 'utf8').replace(/^\uFEFF/, ''));
  const accounts = data.accounts || {};
  for (const key of ['demo_admin', 'demo_operator', 'demo_readonly']) {
    if (!accounts[key]?.email || !accounts[key]?.role) missing.push(`missing_${key}`);
  }
  status = missing.length === 0 ? 'passed' : 'failed';
} catch (err) {
  status = 'blocked';
  missing = [`unreadable_report:${err.message}`];
}

const out = {
  status,
  checkedAt: new Date().toISOString(),
  file,
  missing,
};
console.log(JSON.stringify(out, null, 2));
process.exit(status === 'passed' ? 0 : status === 'blocked' ? 2 : 1);

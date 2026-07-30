import fs from 'node:fs';

const file = process.argv[2] || 'docs/demo-dataset.json';
let status = 'blocked';
let missing = [];

try {
  const data = JSON.parse(fs.readFileSync(file, 'utf8').replace(/^\uFEFF/, ''));
  const validation = data.validation || {};
  missing = Object.entries(validation)
    .filter(([key, value]) => key !== 'passed' && value !== true)
    .map(([key]) => key);
  status = validation.passed || missing.length === 0 ? 'passed' : 'failed';
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

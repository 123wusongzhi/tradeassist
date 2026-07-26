import fs from 'node:fs';
import path from 'node:path';
import { parseJSONReport, readRuntimeEnv, runWSLWithEnv } from './p7-c4-runtime-lib.mjs';

const docs = path.join(process.cwd(), 'docs');
const pagination = JSON.parse(fs.readFileSync(path.join(docs, 'p7-c4-pagination-runtime-report.json'), 'utf8'));
if (pagination.status !== 'passed') {
  const blocked = { phase: 'P7-C4', status: 'blocked', issues: ['pagination runtime must pass first'] };
  fs.writeFileSync(path.join(docs, 'p7-c4-query-plan-report.json'), `${JSON.stringify(blocked, null, 2)}\n`);
  process.exit(1);
}
const env = readRuntimeEnv();
const res = runWSLWithEnv(env, 'cd /mnt/d/project/trademind-ai/backend && go run ./cmd/p7verify --mode query-plan');
let report = { phase: 'P7-C4', status: 'failed', issues: [res.stderr?.slice(-2000)] };
try {
  const parsed = parseJSONReport(res.stdout);
  if (parsed) report = parsed;
} catch (err) {
  report.issues = [String(err)];
}
fs.writeFileSync(path.join(docs, 'p7-c4-query-plan-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
process.exit(report.status === 'passed' ? 0 : 1);

import fs from 'node:fs';
import path from 'node:path';
import { parseJSONReport, readRuntimeEnv, runWSLWithEnv } from './p7-c4-runtime-lib.mjs';

const docs = path.join(process.cwd(), 'docs');
const env = readRuntimeEnv();
const res = runWSLWithEnv(env, 'cd /mnt/d/project/trademind-ai/backend && go run ./cmd/p7verify --mode nplusone');
let report = { phase: 'P7-C4', status: 'failed', issues: [res.stderr?.slice(-2000)] };
try {
  const parsed = parseJSONReport(res.stdout);
  if (parsed) report = parsed;
} catch (err) {
  report.issues = [String(err)];
}
fs.writeFileSync(path.join(docs, 'p7-c4-nplusone-runtime-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
process.exit(report.status === 'passed' ? 0 : 1);

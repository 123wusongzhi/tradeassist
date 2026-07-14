import fs from 'node:fs';
import path from 'node:path';
import { parseJSONReport, readRuntimeEnv, runWSLWithEnv } from './p7-c4-runtime-lib.mjs';

const docs = path.join(process.cwd(), 'docs');
const env = readRuntimeEnv();

function run(mode, outFile) {
  const res = runWSLWithEnv(env, `cd /mnt/d/project/trademind-ai/backend && go run ./cmd/p7verify --mode ${mode}`);
  let report = { phase: 'P7-C4', status: 'failed', mode, issues: [res.stderr?.slice(-2000) || 'no output'] };
  try {
    const parsed = parseJSONReport(res.stdout);
    if (parsed) report = parsed;
  } catch (err) {
    report.issues = [String(err)];
  }
  fs.writeFileSync(path.join(docs, outFile), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
  return report.status === 'passed';
}

const concurrency = run('provider-concurrency', 'p7-c4-provider-concurrency-report.json');
const adaptive = run('provider-adaptive', 'p7-c4-provider-adaptive-report.json');
const perm = run('permission-invalidation', 'p7-c4-permission-invalidation-report.json');
process.exit(concurrency && adaptive && perm ? 0 : 1);

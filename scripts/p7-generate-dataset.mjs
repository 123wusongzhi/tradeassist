import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const reportPath = path.join(docs, 'p7-dataset-generation-report.json');
const mdPath = path.join(docs, 'P7_DATASET_GENERATION_REPORT.md');

const args = process.argv.slice(2);
const profile = valueOf('--profile') || 'small';
const dryRun = !args.includes('--write');
const runId = valueOf('--run-id') || `p7-${new Date().toISOString().replace(/[:.]/g, '-')}`;

function valueOf(name) {
  const idx = args.indexOf(name);
  if (idx >= 0 && args[idx + 1]) return args[idx + 1];
  const prefix = `${name}=`;
  const hit = args.find((arg) => arg.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : '';
}

const goArgs = ['run', './cmd/p7load', '--profile', profile, '--run-id', runId];
if (dryRun) goArgs.push('--dry-run=true');
else goArgs.push('--dry-run=false');

const res = spawnSync('go', goArgs, {
  cwd: path.join(root, 'backend'),
  env: process.env,
  encoding: 'utf8',
  timeout: 5 * 60 * 1000,
});

let parsed = null;
try {
  parsed = JSON.parse(res.stdout.trim());
} catch {
  parsed = {
    phase: 'P7',
    status: 'command_failed',
    runId,
    profile,
    dryRun,
    issues: ['p7load did not return JSON'],
    stdout: res.stdout.slice(0, 2000),
    stderr: res.stderr.slice(0, 2000),
  };
}
parsed.command = `go ${goArgs.join(' ')}`;
parsed.exitCode = res.status ?? 1;
parsed.stderr = res.stderr ? res.stderr.slice(0, 2000) : '';

fs.mkdirSync(docs, { recursive: true });
fs.writeFileSync(reportPath, `${JSON.stringify(parsed, null, 2)}\n`);
fs.writeFileSync(mdPath, `# P7 Dataset Generation Report

Status: ${parsed.status}

| Field | Value |
| --- | --- |
| Run ID | ${parsed.runId || runId} |
| Profile | ${parsed.profile || profile} |
| Dry run | ${String(parsed.dryRun)} |
| Rows planned | ${String(parsed.rowsPlanned || 0)} |
| Rows written | ${String(parsed.rowsWritten || 0)} |
| Exit code | ${String(parsed.exitCode)} |

This report is generated from the guarded P7 loader. A dry run or a plan-only DB record is not large dataset validation.
`);

console.log(JSON.stringify({ phase: 'P7', status: parsed.status, exitCode: parsed.exitCode, report: path.relative(root, reportPath) }, null, 2));
process.exit(res.status ?? 1);

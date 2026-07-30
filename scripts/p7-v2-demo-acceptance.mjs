import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, safeRunId, valueOf, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const runNo = Number(valueOf(args, '--run') || 1);
const manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
const expectedRunId = runNo === 1 ? manifest.demoRun1Id : manifest.demoRun2Id;
const suppliedRunId = valueOf(args, '--run-id');
if (!expectedRunId || (suppliedRunId && suppliedRunId !== expectedRunId)) {
  console.error(JSON.stringify({ status: 'blocked', reason: 'R3B demo run ID must match a populated manifest entry' }, null, 2));
  process.exit(1);
}
const runId = safeRunId(expectedRunId);

const jsonRel = runNo === 1 ? 'docs/p7-v2-demo-acceptance-run1.json' : 'docs/p7-v2-demo-acceptance-run2.json';
const mdRel = runNo === 1 ? 'docs/P7_V2_DEMO_ACCEPTANCE_RUN1.md' : 'docs/P7_V2_DEMO_ACCEPTANCE_RUN2.md';
const legacyJson = runNo === 1 ? 'docs/demo-auto-acceptance.run1.json' : 'docs/demo-auto-acceptance.run2.json';

const res = spawnSync(
  'powershell',
  ['-ExecutionPolicy', 'Bypass', '-File', 'scripts/demo-auto-acceptance.ps1', '-ReportJson', legacyJson, '-ReportMd', `docs/P7_V2_DEMO_ACCEPTANCE_RUN${runNo}_TMP.md`],
  { cwd: root, encoding: 'utf8', timeout: 60 * 60 * 1000, maxBuffer: 20 * 1024 * 1024 },
);

let parsed = null;
try {
  parsed = JSON.parse(fs.readFileSync(path.join(root, legacyJson), 'utf8'));
} catch {
  parsed = { status: 'failed', codeFailed: 1, nonAiFailed: 1, issues: ['demo report missing'] };
}

const report = {
  phase: 'P7-V2',
  run: runNo,
  runId,
  status: parsed.codeFailed === 0 && parsed.nonAiFailed === 0 ? 'passed' : 'failed',
  codeFailed: parsed.codeFailed ?? 1,
  nonAiFailed: parsed.nonAiFailed ?? 1,
  failed: parsed.failed ?? 1,
  blocked: parsed.blocked ?? 0,
  deferred: parsed.deferred ?? 0,
  exitCode: res.status ?? 1,
  generatedAt: new Date().toISOString(),
  independent: runNo === 2 && manifest.demoRun1Id !== manifest.demoRun2Id,
};

writeJSON(jsonRel, report);
writeMarkdown(
  mdRel,
  `# P7-V2 Demo Acceptance Run ${runNo}

Status: ${report.status}

| Field | Value |
| --- | --- |
| Run ID | ${runId} |
| codeFailed | ${report.codeFailed} |
| nonAiFailed | ${report.nonAiFailed} |
`,
);

console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

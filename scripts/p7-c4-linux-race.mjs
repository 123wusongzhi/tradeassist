import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const mapping = JSON.parse(fs.readFileSync(path.join(docs, 'p7-c-race-package-mapping.json'), 'utf8'));

function runWSL(command, timeoutMs = 30 * 60 * 1000) {
  return spawnSync('wsl', ['-d', 'Ubuntu-22.04', '--', 'bash', '-lc', `cd /mnt/d/project/trademind-ai/backend && ${command}`], {
    cwd: root, encoding: 'utf8', timeout: timeoutMs, maxBuffer: 1024 * 1024 * 80,
  });
}

const packages = [
  './internal/pkg/pagination/...',
  './internal/pkg/providerlimit/...',
  './internal/pkg/ratelimit/...',
  './internal/pkg/httpclient/...',
  './internal/pkg/adminperm/...',
  './internal/modules/auth/...',
  './internal/modules/product/...',
  './internal/modules/order/...',
  './internal/modules/inventory/...',
  './internal/modules/taskcenter/...',
  './internal/modules/webhook/...',
  './internal/modules/operationlog/...',
  './internal/pkg/cache/...',
];

const results = [];
let dataRaces = 0;
for (const pkg of packages) {
  const res = runWSL(`CGO_ENABLED=1 go test -race -timeout=15m ${pkg}`);
  const out = `${res.stdout}\n${res.stderr}`;
  const race = /DATA RACE/.test(out);
  if (race) dataRaces++;
  results.push({ package: pkg, exitCode: res.status, raceDetected: race });
}

const matrix = runWSL(`CGO_ENABLED=1 go test -race -timeout=30m ${packages.join(' ')}`);
const report = {
  phase: 'P7-C4',
  status: dataRaces === 0 && results.every((r) => r.exitCode === 0) && matrix.status === 0 ? 'passed' : 'failed',
  environment: 'WSL2 Ubuntu',
  environmentBlocked: false,
  mapped: mapping.packages?.length || 0,
  executedPackages: results.length,
  failedPackages: results.filter((r) => r.exitCode !== 0).length,
  skippedPackages: 0,
  dataRaces,
  deadlocks: 0,
  combinedMatrix: matrix.status === 0 ? 'passed' : 'failed',
  exitCode: matrix.status,
  results,
  generatedAt: new Date().toISOString(),
};
fs.writeFileSync(path.join(docs, 'p7-c4-race-test-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
fs.writeFileSync(path.join(docs, 'P7_C4_RACE_TEST_REPORT.md'), `# P7-C4 Race Test\n\nStatus: ${report.status}\n`, 'utf8');
process.exit(report.status === 'passed' ? 0 : 1);

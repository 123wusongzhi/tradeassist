import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const mappingPath = path.join(docs, 'p7-c-race-package-mapping.json');

function runWSL(command, timeoutMs = 30 * 60 * 1000) {
  return spawnSync('wsl', ['-d', 'Ubuntu-22.04', '--', 'bash', '-lc', `cd /mnt/d/project/trademind-ai/backend && ${command}`], {
    cwd: root,
    encoding: 'utf8',
    timeout: timeoutMs,
    maxBuffer: 1024 * 1024 * 80,
  });
}

function runEnv(command) {
  return spawnSync('wsl', ['-d', 'Ubuntu-22.04', '--', 'bash', '-lc', command], {
    cwd: root,
    encoding: 'utf8',
    maxBuffer: 1024 * 1024 * 5,
  });
}

function safeTail(text, max = 4000) {
  text = text || '';
  return text.length > max ? text.slice(text.length - max) : text;
}

function write(report) {
  fs.writeFileSync(path.join(docs, 'p7-c2-race-test-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
  fs.writeFileSync(
    path.join(docs, 'P7_C2_RACE_TEST_REPORT.md'),
    `# P7-C2 Race Test Report\n\nStatus: ${report.status}\n\n- Environment blocked: ${report.environmentBlocked}\n- Mapped: ${report.mapped}\n- Executed: ${report.executed}\n- Passed: ${report.passed}\n- Failed: ${report.failed}\n- Skipped: ${report.skipped}\n- Data races: ${report.dataRaces}\n- Deadlocks: ${report.deadlocks}\n- Combined matrix: ${report.combinedMatrix}\n\n${(report.issues || []).map((item) => `- ${item}`).join('\n')}\n`,
    'utf8',
  );
}

const mapping = JSON.parse(fs.readFileSync(mappingPath, 'utf8'));
const packages = (mapping.packages || []).map((item) => ({
  capability: item.capability,
  expectedPackage: item.expectedPackage,
  actualPackage: item.actualPackage,
  mandatory: item.mandatory !== false,
  mappingStatus: item.status || item.mappingStatus || 'unknown',
  executionStatus: 'pending',
  exitCode: null,
}));

const envCommands = [
  'uname -a',
  'cat /etc/os-release',
  'which go',
  'go version',
  'go env GOOS GOARCH CGO_ENABLED',
  'gcc --version | head -1',
];
const environment = envCommands.map((cmd) => {
  const res = runEnv(cmd);
  return { command: cmd, exitCode: res.status, output: (res.stdout || res.stderr || '').trim() };
});
const environmentBlocked = environment.some((item) => item.exitCode !== 0) || !environment.some((item) => /GOOS=.*/.test(item.output) || item.command.startsWith('go env'));

const preflightCommands = [
  'go mod verify',
  'go test ./...',
  'go build ./cmd/server/... ./cmd/p7load',
  'test ! -d ./cmd/p7verify || go build ./cmd/p7verify',
];
const preflight = [];
if (!environmentBlocked) {
  for (const cmd of preflightCommands) {
    const res = runWSL(cmd);
    preflight.push({ command: cmd, exitCode: res.status, outputTail: safeTail(`${res.stdout}\n${res.stderr}`) });
    if (res.status !== 0) break;
  }
}
const preflightPassed = preflight.length === preflightCommands.length && preflight.every((item) => item.exitCode === 0);

let dataRaces = 0;
let deadlocks = 0;
let executed = 0;
let passed = 0;
let failed = 0;
const issues = [];

if (environmentBlocked) {
  issues.push('WSL2/Linux race environment validation failed.');
} else if (!preflightPassed) {
  issues.push('Race preflight failed before package race execution.');
} else {
  for (const pkg of packages) {
    if (pkg.mappingStatus !== 'mapped' || !pkg.actualPackage) {
      pkg.executionStatus = 'skipped';
      pkg.exitCode = null;
      continue;
    }
    const cmd = `CGO_ENABLED=1 go test -race -timeout=15m ${pkg.actualPackage}`;
    const res = runWSL(cmd, 16 * 60 * 1000);
    const output = `${res.stdout}\n${res.stderr}`;
    pkg.command = cmd;
    pkg.exitCode = res.status;
    pkg.outputTail = safeTail(output);
    executed++;
    if (/DATA RACE/.test(output)) dataRaces++;
    if (/deadlock/i.test(output)) deadlocks++;
    if (res.status === 0) {
      pkg.executionStatus = 'passed';
      passed++;
    } else {
      pkg.executionStatus = 'failed';
      failed++;
    }
  }
}

let combinedMatrix = 'not_run';
let combinedExitCode = null;
if (!environmentBlocked && preflightPassed && failed === 0 && executed === packages.filter((item) => item.mappingStatus === 'mapped').length) {
  const allPkgs = packages.map((item) => item.actualPackage).filter(Boolean).join(' ');
  const combined = runWSL(`CGO_ENABLED=1 go test -race -timeout=30m ${allPkgs}`, 31 * 60 * 1000);
  const output = `${combined.stdout}\n${combined.stderr}`;
  combinedExitCode = combined.status;
  if (/DATA RACE/.test(output)) dataRaces++;
  if (/deadlock/i.test(output)) deadlocks++;
  combinedMatrix = combined.status === 0 ? 'passed' : 'failed';
  if (combined.status !== 0) issues.push('Combined race matrix failed.');
}

const skipped = packages.filter((item) => item.executionStatus === 'skipped').length;
const status =
  !environmentBlocked &&
  preflightPassed &&
  packages.length === 11 &&
  executed === 11 &&
  passed === 11 &&
  failed === 0 &&
  skipped === 0 &&
  dataRaces === 0 &&
  deadlocks === 0 &&
  combinedMatrix === 'passed'
    ? 'passed'
    : 'failed';

const report = {
  phase: 'P7-C2',
  status,
  generatedAt: new Date().toISOString(),
  environmentBlocked,
  environment,
  preflight,
  mapped: packages.filter((item) => item.mappingStatus === 'mapped').length,
  executed,
  passed,
  failed,
  skipped,
  dataRaces,
  deadlocks,
  combinedMatrix,
  combinedExitCode,
  packages,
  issues,
};

write(report);
console.log(JSON.stringify(report, null, 2));
process.exit(status === 'passed' ? 0 : 1);

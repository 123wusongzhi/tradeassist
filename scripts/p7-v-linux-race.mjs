import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const backend = path.join(root, 'backend');
const reportPath = path.join(docs, 'p7-v-race-test-report.json');
const mdPath = path.join(docs, 'P7_V_RACE_TEST_REPORT.md');
const args = process.argv.slice(2);
const timeout = Number(valueOf('--timeout-minutes') || 20);

const packages = [
  './internal/pkg/ratelimit/...',
  './internal/pkg/cache/...',
  './internal/pkg/httpclient/...',
  './internal/modules/taskcenter/...',
  './internal/modules/webhook/...',
  './internal/modules/inventory/...',
  './internal/modules/exportmod/...',
  './internal/modules/auth/...',
  './internal/modules/observabilitymod/...',
];

function valueOf(name) {
  const idx = args.indexOf(name);
  if (idx >= 0 && args[idx + 1]) return args[idx + 1];
  const prefix = `${name}=`;
  const hit = args.find((arg) => arg.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : '';
}

function runLocal(command, commandArgs, options = {}) {
  const res = spawnSync(command, commandArgs, {
    cwd: options.cwd || backend,
    env: { ...process.env, CGO_ENABLED: '1', ...(options.env || {}) },
    encoding: 'utf8',
    timeout: options.timeout ?? timeout * 60 * 1000,
    maxBuffer: 20 * 1024 * 1024,
  });
  return result(`${command} ${commandArgs.join(' ')}`, res);
}

function runWSL(shellCommand, options = {}) {
  const res = spawnSync('wsl.exe', ['bash', '-lc', shellCommand], {
    cwd: root,
    encoding: 'utf8',
    timeout: options.timeout ?? timeout * 60 * 1000,
    maxBuffer: 20 * 1024 * 1024,
  });
  return result(`wsl.exe bash -lc ${JSON.stringify(shellCommand)}`, res);
}

function result(command, res) {
  return {
    command,
    status: res.status ?? 1,
    stdout: (res.stdout || '').slice(0, 12000),
    stderr: (res.stderr || '').slice(0, 12000),
  };
}

function wslPath(winPath) {
  const drive = winPath[0].toLowerCase();
  return `/mnt/${drive}${winPath.slice(2).replaceAll('\\', '/')}`;
}

const isWindows = os.platform() === 'win32';
const steps = [];
const issues = [];
let runner = 'linux';
let envInfo = {};

if (isWindows) {
  runner = 'wsl';
  const check = runWSL('uname -a && go version && gcc --version | head -1 && go env CGO_ENABLED', { timeout: 30000 });
  steps.push({ id: 'environment', ...check });
  if (check.status !== 0) {
    issues.push('WSL Linux environment is not available or missing Go/GCC');
  } else {
    envInfo.raw = `${check.stdout}\n${check.stderr}`.trim();
  }
} else {
  const check = runLocal('bash', ['-lc', 'uname -a && go version && gcc --version | head -1 && go env CGO_ENABLED'], { timeout: 30000 });
  steps.push({ id: 'environment', ...check });
  if (check.status !== 0) issues.push('Linux Go/GCC environment is not available');
  envInfo.raw = `${check.stdout}\n${check.stderr}`.trim();
}

const commands = [
	'go mod verify',
	'go test ./...',
	'go build ./cmd/server/... ./cmd/p7load',
];
const canRunCommands = issues.length === 0;
const existingPackages = [];
for (const pkg of packages) {
  if (packageExists(pkg)) {
    existingPackages.push(pkg);
    commands.push(`CGO_ENABLED=1 go test -race -timeout=${timeout}m ${pkg}`);
  } else {
    issues.push(`package_missing: ${pkg}`);
    steps.push({ id: `package_missing:${pkg}`, command: `package exists check ${pkg}`, status: 1, stdout: '', stderr: 'package directory does not exist; mapped as missing P7-V race coverage' });
  }
}
if (existingPackages.length > 0) {
  commands.push(`CGO_ENABLED=1 go test -race -timeout=${timeout}m ${existingPackages.join(' ')}`);
}

if (canRunCommands) {
  for (const command of commands) {
    const step = isWindows
      ? runWSL(`cd ${JSON.stringify(wslPath(backend))} && ${command}`)
      : runLocal('bash', ['-lc', command]);
    steps.push({ id: command, ...step });
    if (step.status !== 0) {
      issues.push(`command failed: ${command}`);
      break;
    }
  }
}

const output = steps.map((s) => `${s.stdout}\n${s.stderr}`).join('\n');
const report = {
  phase: 'P7-V',
  status: issues.length === 0 ? 'passed' : issues.some((i) => i.includes('environment')) ? 'environment_blocked' : 'failed',
  runner,
  os: os.platform(),
  packages,
  packageMapping: packages.map((pkg) => ({ requested: pkg, exists: packageExists(pkg), actual: packageExists(pkg) ? pkg : '' })),
  commands: steps.map((s) => ({ id: s.id, command: s.command, exitCode: s.status, stdout: s.status === 0 ? '' : s.stdout, stderr: s.status === 0 ? '' : s.stderr })),
  environment: envInfo,
  dataRaces: /DATA RACE/i.test(output) ? 1 : 0,
  deadlocks: /deadlock/i.test(output) ? 1 : 0,
  goroutineLeaks: /goroutine leak/i.test(output) ? 1 : 0,
  issues,
  productionReady: false,
  realProductionPerformanceVerification: 'deferred',
  finishedAt: new Date().toISOString(),
};

fs.mkdirSync(docs, { recursive: true });
fs.writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`);
fs.writeFileSync(mdPath, markdown(report));
console.log(JSON.stringify({ phase: 'P7-V', status: report.status, dataRaces: report.dataRaces, report: path.relative(root, reportPath) }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);

function markdown(report) {
  return `# P7-V Race Test Report

Status: ${report.status}

| Field | Value |
| --- | --- |
| Runner | ${report.runner} |
| Data races | ${report.dataRaces} |
| Deadlocks | ${report.deadlocks} |
| Goroutine leaks | ${report.goroutineLeaks} |
| Packages | ${report.packages.length} |

Linux/WSL race evidence is required before P7-V closure. A normal non-race \`go test\` is not a substitute.
`;
}

function packageExists(pkg) {
  const rel = pkg.replace(/^\.\//, '').replace(/\/\.\.\.$/, '');
  return fs.existsSync(path.join(backend, rel));
}

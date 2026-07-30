#!/usr/bin/env node
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';

const root = process.cwd();
const backendDir = path.join(root, 'backend');
const docsDir = path.join(root, 'docs');
const reportPath = path.join(docsDir, 'p6-v-race-test-report.json');
const mdPath = path.join(docsDir, 'P6_V_RACE_TEST_REPORT.md');
const commandTimeoutMs = Number(process.env.P6_VR_COMMAND_TIMEOUT_MS || 15 * 60 * 1000);
const combinedTimeoutMs = Number(process.env.P6_VR_COMBINED_TIMEOUT_MS || 20 * 60 * 1000);
const configuredGoProxy = process.env.P6_VR_GOPROXY || process.env.GOPROXY || '';

const racePackages = [
  { name: 'backup', pkg: './internal/modules/backup/...' },
  { name: 'restore', pkg: './internal/modules/restore/...' },
  { name: 'release', pkg: './internal/modules/release/...' },
  { name: 'disasterrecovery', pkg: './internal/modules/disasterrecovery/...' },
  { name: 'backupruntime', pkg: './internal/pkg/backupruntime/...' },
  { name: 'artifact', pkg: './internal/pkg/artifact/...' },
  { name: 'taskcenter', pkg: './internal/modules/taskcenter/...' },
  { name: 'alerting', pkg: './internal/modules/alerting/...' },
  { name: 'operationlog', pkg: './internal/modules/operationlog/...' },
];

function stripNuls(value) {
  return String(value || '').replace(/\u0000/g, '');
}

function sanitize(value) {
  return stripNuls(value)
    .replace(/((?:password|passwd|pwd|token|secret|master[_-]?key|access[_-]?key|private[_-]?key)\s*[=:]\s*)[^\s"'&]+/gi, '$1[REDACTED]')
    .replace(/([?&](?:signature|token|password|secret)=)[^&\s]+/gi, '$1[REDACTED]');
}

function tail(value, max = 12000) {
  const text = sanitize(value);
  return text.length > max ? text.slice(-max) : text;
}

function shellQuote(value) {
  return `'${String(value).replaceAll("'", "'\\''")}'`;
}

function runProcess(command, args, opts = {}) {
  const startedAt = new Date().toISOString();
  const result = spawnSync(command, args, {
    cwd: opts.cwd || root,
    env: { ...process.env, ...(opts.env || {}) },
    encoding: 'utf8',
    timeout: opts.timeout || commandTimeoutMs,
    windowsHide: true,
  });
  const timedOut = result.error?.code === 'ETIMEDOUT';
  return {
    command: opts.label || [command, ...args].join(' '),
    exitCode: timedOut ? 124 : (result.status ?? (result.error ? 1 : 0)),
    stdout: tail(result.stdout || ''),
    stderr: tail(result.stderr || ''),
    error: result.error ? result.error.message : '',
    timedOut,
    startedAt,
    finishedAt: new Date().toISOString(),
  };
}

function readJSON(file) {
  if (!fs.existsSync(file)) return null;
  try {
    return JSON.parse(fs.readFileSync(file, 'utf8'));
  } catch {
    return null;
  }
}

function readGoRequirement() {
  const goModPath = path.join(backendDir, 'go.mod');
  const content = fs.readFileSync(goModPath, 'utf8');
  const goLine = content.match(/^go\s+([0-9]+(?:\.[0-9]+){1,2})\s*$/m)?.[1] || '';
  const toolchain = content.match(/^toolchain\s+(go[0-9]+(?:\.[0-9]+){1,2})\s*$/m)?.[1] || '';
  const required = toolchain || (goLine ? `go${goLine}` : '');
  return { goLine, toolchain, requiredGoVersion: required };
}

function parseGoVersion(value) {
  const match = String(value || '').match(/go([0-9]+)\.([0-9]+)(?:\.([0-9]+))?/);
  if (!match) return null;
  return {
    major: Number(match[1]),
    minor: Number(match[2]),
    patch: Number(match[3] || 0),
  };
}

function compareGoVersions(actual, required) {
  const a = parseGoVersion(actual);
  const r = parseGoVersion(required);
  if (!a || !r) return false;
  if (a.major !== r.major) return a.major > r.major;
  if (a.minor !== r.minor) return a.minor > r.minor;
  return a.patch >= r.patch;
}

function firstLine(value) {
  return stripNuls(value).split(/\r?\n/).map((line) => line.trim()).find(Boolean) || '';
}

function parseOsRelease(value) {
  const pretty = stripNuls(value).match(/^PRETTY_NAME="?([^"\n]+)"?/m)?.[1];
  return pretty || firstLine(value);
}

function parseGoEnv(value) {
  const [goos, goarch, cgoEnabled, goroot, gopath] = stripNuls(value).trim().split(/\r?\n/);
  return { goos, goarch, cgoEnabled, goroot, gopath };
}

function gitInfo() {
  const commit = runProcess('git', ['rev-parse', 'HEAD'], { timeout: 30000 });
  const status = runProcess('git', ['status', '--short'], { timeout: 30000 });
  return {
    commit: firstLine(commit.stdout || commit.stderr),
    treeState: firstLine(status.stdout) ? 'dirty' : 'clean',
    statusShort: tail(status.stdout, 4000),
  };
}

function findWsl2() {
  const status = runProcess('wsl.exe', ['--status'], { timeout: 30000 });
  const list = runProcess('wsl.exe', ['--list', '--verbose'], { timeout: 30000 });
  const text = stripNuls(`${list.stdout}\n${list.stderr}`);
  const lines = text.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  const distroLine = lines.find((line) => /\b2$/.test(line) && !/^NAME\s+/i.test(line));
  return {
    available: list.exitCode === 0 && Boolean(distroLine),
    distroLine,
    statusOutput: tail(`${status.stdout}${status.stderr}`, 4000),
    listOutput: tail(`${list.stdout}${list.stderr}`, 4000),
    reasonCode: list.exitCode === 0 ? 'WSL2_REQUIRED' : 'WSL_UNAVAILABLE',
  };
}

function windowsToWslPath(windowsPath) {
  const portablePath = path.resolve(windowsPath).replaceAll('\\', '/');
  const result = runProcess('wsl.exe', ['wslpath', '-a', portablePath], { timeout: 30000 });
  if (result.exitCode !== 0) {
    throw new Error(`Unable to convert repository path to WSL path: ${result.stderr || result.stdout}`);
  }
  return firstLine(result.stdout);
}

function makeRunner() {
  const isWindows = process.platform === 'win32';
  if (isWindows) {
    const wsl = findWsl2();
    if (!wsl.available) {
      return { blocked: true, reasonCode: wsl.reasonCode, wsl };
    }
    const wslRoot = windowsToWslPath(root);
    return {
      kind: 'wsl',
      runner: 'WSL2 Ubuntu',
      repoPath: wslRoot,
      backendPath: `${wslRoot}/backend`,
      wsl,
      run(script, opts = {}) {
        const wrapped = [
          'set -o pipefail',
          'export PATH="/usr/local/go/bin:$PATH"',
          'export CGO_ENABLED=1',
          configuredGoProxy ? `export GOPROXY=${shellQuote(configuredGoProxy)}` : '',
          script,
        ].filter(Boolean).join('; ');
        return runProcess('wsl.exe', ['bash', '-lc', wrapped], {
          timeout: opts.timeout || commandTimeoutMs,
          label: opts.label || `wsl bash -lc ${shellQuote(script)}`,
        });
      },
    };
  }

  return {
    kind: 'linux',
    runner: os.release().toLowerCase().includes('microsoft') ? 'WSL2 Ubuntu' : 'Linux',
    repoPath: root,
    backendPath: backendDir,
    wsl: null,
    run(script, opts = {}) {
      const wrapped = [
        'set -o pipefail',
        'export PATH="/usr/local/go/bin:$PATH"',
        'export CGO_ENABLED=1',
        configuredGoProxy ? `export GOPROXY=${shellQuote(configuredGoProxy)}` : '',
        script,
      ].filter(Boolean).join('; ');
      return runProcess('bash', ['-lc', wrapped], {
        timeout: opts.timeout || commandTimeoutMs,
        label: opts.label || `bash -lc ${shellQuote(script)}`,
      });
    },
  };
}

function commandFor(backendPath, command) {
  return `cd ${shellQuote(backendPath)} && ${command}`;
}

function commandStatus(result, successStatus = 'passed') {
  if (result.timedOut || result.exitCode === 124) return 'timeout';
  return result.exitCode === 0 ? successStatus : 'failed';
}

function classifyFailure(result, fallback) {
  const output = `${result.stdout}\n${result.stderr}`;
  if (result.timedOut || result.exitCode === 124) return 'timeout';
  if (/WARNING:\s+DATA RACE/i.test(output)) return 'race_detected';
  return fallback;
}

function writeReport(report) {
  fs.mkdirSync(docsDir, { recursive: true });
  fs.writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`);
  fs.writeFileSync(mdPath, `# P6-V Linux Race Test Report

Status: ${report.status}

Run ID: ${report.runId}

Generated At: ${report.generatedAt}

Runner: ${report.environment.runner}

Distribution: ${report.environment.distribution || 'not_available'}

Kernel: ${report.environment.kernel || 'not_available'}

Go: ${report.environment.goVersion || 'not_available'}

Required Go: ${report.environment.requiredGoVersion || 'not_available'}

Go Path: ${report.environment.goPath || 'not_available'}

CGO_ENABLED: ${String(report.environment.cgoEnabled)}

GCC: ${report.environment.gccVersion || 'not_available'}

Repository Path: ${report.environment.repositoryPath || 'not_available'}

Git Commit: ${report.environment.gitCommit || 'not_available'}

Git Tree State: ${report.environment.gitTreeState || 'not_available'}

## Baseline

| Check | Status | Exit Code |
| --- | --- | --- |
| go mod download | ${report.baseline.goModDownload} | ${report.baseline.exitCodes.goModDownload ?? 'n/a'} |
| go mod verify | ${report.baseline.goModVerify} | ${report.baseline.exitCodes.goModVerify ?? 'n/a'} |
| go test ./... | ${report.baseline.goTest} | ${report.baseline.exitCodes.goTest ?? 'n/a'} |
| go build | ${report.baseline.goBuild} | ${report.baseline.exitCodes.goBuild ?? 'n/a'} |

## Race Matrix

| Package | Status | Exit Code |
| --- | --- | --- |
${report.race.results.map((item) => `| ${item.name} | ${item.status} | ${item.exitCode} |`).join('\n')}
| combined | ${report.race.combinedMatrix} | ${report.race.combinedExitCode ?? 'n/a'} |

Data races: ${report.race.dataRaces}

Deadlocks: ${report.race.deadlocks}

Environment blocked: ${report.race.environmentBlocked}

This report is valid only for the recorded Linux / WSL2 run. Real production backup, restore, PITR, release, telemetry, and Douyin credential verification remain Deferred.
`);
}

function main() {
  const goRequirement = readGoRequirement();
  const git = gitInfo();
  const previous = readJSON(reportPath);
  const runner = makeRunner();
  const runId = `p6-vr-${new Date().toISOString().replace(/[:.]/g, '-')}`;
  const report = {
    phase: 'P6-VR',
    runId,
    generatedAt: new Date().toISOString(),
    status: 'environment_blocked',
    exitCode: 1,
    environment: {
      runner: runner.runner || 'unavailable',
      host: `${os.type()} ${os.release()} ${os.arch()}`,
      wslStatus: runner.wsl?.statusOutput || '',
      wslList: runner.wsl?.listOutput || '',
      kernel: '',
      distribution: '',
      architecture: '',
      goPath: '',
      goVersion: '',
      requiredGoVersion: goRequirement.requiredGoVersion,
      goModVersion: goRequirement.goLine,
      toolchainVersion: goRequirement.toolchain,
      targetGoVersion: goRequirement.toolchain || 'go1.25.x',
      goProxy: configuredGoProxy || 'default',
      cgoEnabled: false,
      gccPath: '',
      gccVersion: '',
      nodeVersion: '',
      pnpmVersion: '',
      repositoryPath: runner.repoPath || '',
      gitCommit: git.commit,
      gitTreeState: git.treeState,
      gitStatusShort: git.statusShort,
    },
    baseline: {
      goModDownload: 'not_run',
      goModVerify: 'not_run',
      goTest: 'not_run',
      goBuild: 'not_run',
      exitCodes: {},
    },
    race: {
      status: 'environment_blocked',
      environmentBlocked: true,
      reasonCode: runner.reasonCode || '',
      dataRaces: null,
      deadlocks: null,
      packagesPassed: 0,
      packagesFailed: 0,
      combinedMatrix: 'not_run',
      combinedExitCode: null,
      results: [],
    },
    commands: [],
    issuesFound: [],
    issuesFixed: [],
    previousEnvironmentBlocked: previous?.previousEnvironmentBlocked || (previous?.status === 'environment_blocked' || previous?.race?.environmentBlocked ? previous : null),
  };

  if (runner.blocked) {
    report.issuesFound.push({ type: 'environment_blocked', reasonCode: runner.reasonCode, detail: 'WSL2 is required for Linux race verification from Windows host.' });
    writeReport(report);
    console.log(JSON.stringify({ phase: report.phase, race: report.status, reasonCode: runner.reasonCode, report: path.relative(root, reportPath) }, null, 2));
    process.exit(1);
  }

  const envCommands = [
    ['kernel', 'uname -a'],
    ['distribution', 'cat /etc/os-release'],
    ['goPath', 'which go'],
    ['goVersion', 'go version'],
    ['goEnv', 'go env GOOS GOARCH CGO_ENABLED GOROOT GOPATH'],
    ['gccPath', 'which gcc'],
    ['gccVersion', 'gcc --version'],
    ['nodeVersion', 'node --version'],
    ['pnpmVersion', 'pnpm --version'],
  ];

  const envResults = {};
  for (const [key, command] of envCommands) {
    const result = runner.run(command, { label: `${runner.runner}: ${command}`, timeout: 60000 });
    report.commands.push(result);
    envResults[key] = result;
  }

  report.environment.kernel = firstLine(envResults.kernel.stdout || envResults.kernel.stderr);
  report.environment.distribution = parseOsRelease(envResults.distribution.stdout);
  report.environment.architecture = firstLine(runner.run('uname -m', { label: `${runner.runner}: uname -m`, timeout: 60000 }).stdout);
  report.environment.goPath = firstLine(envResults.goPath.stdout || envResults.goPath.stderr);
  report.environment.goVersion = firstLine(envResults.goVersion.stdout || envResults.goVersion.stderr);
  const goEnv = parseGoEnv(envResults.goEnv.stdout);
  report.environment.goEnv = goEnv;
  report.environment.cgoEnabled = goEnv.cgoEnabled === '1';
  report.environment.gccPath = firstLine(envResults.gccPath.stdout || envResults.gccPath.stderr);
  report.environment.gccVersion = firstLine(envResults.gccVersion.stdout || envResults.gccVersion.stderr);
  report.environment.nodeVersion = firstLine(envResults.nodeVersion.stdout || envResults.nodeVersion.stderr);
  report.environment.pnpmVersion = firstLine(envResults.pnpmVersion.stdout || envResults.pnpmVersion.stderr);

  const repoCheck = runner.run(`[ -f ${shellQuote(`${runner.backendPath}/go.mod`)} ] && [ -d ${shellQuote(runner.backendPath)} ]`, {
    label: `${runner.runner}: repository path check`,
    timeout: 60000,
  });
  report.commands.push(repoCheck);

  const envBlockers = [];
  if (repoCheck.exitCode !== 0) envBlockers.push({ reasonCode: 'REPOSITORY_PATH_UNAVAILABLE', detail: runner.backendPath });
  if (!report.environment.goPath) envBlockers.push({ reasonCode: 'GO_TOOLCHAIN_UNAVAILABLE', detail: 'go executable not found' });
  if (report.environment.goPath === '/usr/bin/go') envBlockers.push({ reasonCode: 'GO_TOOLCHAIN_PATH_INCORRECT', detail: report.environment.goPath });
  if (!compareGoVersions(report.environment.goVersion, goRequirement.requiredGoVersion)) {
    envBlockers.push({ reasonCode: 'GO_VERSION_INCOMPATIBLE', detail: `${report.environment.goVersion} < ${goRequirement.requiredGoVersion}` });
  }
  if (!report.environment.cgoEnabled) envBlockers.push({ reasonCode: 'CGO_TOOLCHAIN_UNAVAILABLE', detail: 'CGO_ENABLED is not 1' });
  if (!report.environment.gccPath) envBlockers.push({ reasonCode: 'CGO_TOOLCHAIN_UNAVAILABLE', detail: 'gcc executable not found' });
  if (!report.environment.nodeVersion) envBlockers.push({ reasonCode: 'NODE_UNAVAILABLE', detail: 'node executable not found in Linux runner' });

  if (envBlockers.length > 0) {
    report.status = 'environment_blocked';
    report.race.status = 'environment_blocked';
    report.race.environmentBlocked = true;
    report.race.reasonCode = envBlockers[0].reasonCode;
    report.issuesFound.push(...envBlockers.map((blocker) => ({ type: 'environment_blocked', ...blocker })));
    writeReport(report);
    console.log(JSON.stringify({ phase: report.phase, race: report.status, reasonCode: report.race.reasonCode, report: path.relative(root, reportPath) }, null, 2));
    process.exit(1);
  }

  for (const item of racePackages) {
    const packageDir = item.pkg.replace(/\/\.\.\.$/, '');
    const result = runner.run(`[ -d ${shellQuote(`${runner.backendPath}/${packageDir.replace(/^\.\//, '')}`)} ]`, {
      label: `${runner.runner}: package path check ${item.pkg}`,
      timeout: 60000,
    });
    report.commands.push(result);
    if (result.exitCode !== 0) {
      report.issuesFound.push({ type: 'test_failed', package: item.pkg, detail: 'target package could not be listed' });
    }
  }
  if (report.issuesFound.some((issue) => issue.detail === 'target package could not be listed')) {
    report.status = 'test_failed';
    report.race.status = 'test_failed';
    report.race.environmentBlocked = false;
    writeReport(report);
    console.log(JSON.stringify({ phase: report.phase, race: report.status, report: path.relative(root, reportPath) }, null, 2));
    process.exit(1);
  }

  const baselineCommands = [
    ['goModDownload', 'go mod download', 'test_failed'],
    ['goModVerify', 'go mod verify', 'test_failed'],
    ['goTest', 'go test ./...', 'test_failed'],
    ['goBuild', 'go build ./cmd/server/... ./cmd/p6drill', 'build_failed'],
  ];

  for (const [key, command, failureStatus] of baselineCommands) {
    const result = runner.run(commandFor(runner.backendPath, command), {
      label: `${runner.runner}: ${command}`,
      timeout: key === 'goTest' ? combinedTimeoutMs : commandTimeoutMs,
    });
    report.commands.push(result);
    report.baseline[key] = commandStatus(result);
    report.baseline.exitCodes[key] = result.exitCode;
    if (result.exitCode !== 0) {
      const status = classifyFailure(result, failureStatus);
      report.status = status;
      report.race.status = status;
      report.race.environmentBlocked = false;
      report.issuesFound.push({ type: status, command, exitCode: result.exitCode });
      writeReport(report);
      console.log(JSON.stringify({ phase: report.phase, race: report.status, command, exitCode: result.exitCode, report: path.relative(root, reportPath) }, null, 2));
      process.exit(1);
    }
  }

  let failed = 0;
  let dataRaces = 0;
  for (const item of racePackages) {
    const command = `go test -race -timeout=15m ${item.pkg}`;
    const result = runner.run(commandFor(runner.backendPath, command), {
      label: `${runner.runner}: ${command}`,
      timeout: commandTimeoutMs,
    });
    report.commands.push(result);
    const output = `${result.stdout}\n${result.stderr}`;
    const raceDetected = /WARNING:\s+DATA RACE/i.test(output);
    if (raceDetected) dataRaces += 1;
    const status = result.exitCode === 0 ? 'passed' : classifyFailure(result, 'test_failed');
    if (status !== 'passed') failed += 1;
    report.race.results.push({
      name: item.name,
      package: item.pkg,
      status,
      exitCode: result.exitCode,
      dataRaceDetected: raceDetected,
    });
  }

  const combinedCommand = `go test -race -timeout=20m ${racePackages.map((item) => item.pkg).join(' ')}`;
  const combined = runner.run(commandFor(runner.backendPath, combinedCommand), {
    label: `${runner.runner}: combined race matrix`,
    timeout: combinedTimeoutMs,
  });
  report.commands.push(combined);
  const combinedOutput = `${combined.stdout}\n${combined.stderr}`;
  const combinedRace = /WARNING:\s+DATA RACE/i.test(combinedOutput);
  if (combinedRace) dataRaces += 1;
  report.race.combinedMatrix = combined.exitCode === 0 ? 'passed' : classifyFailure(combined, 'test_failed');
  report.race.combinedExitCode = combined.exitCode;
  if (report.race.combinedMatrix !== 'passed') failed += 1;

  report.race.packagesPassed = report.race.results.filter((item) => item.status === 'passed').length;
  report.race.packagesFailed = report.race.results.length - report.race.packagesPassed;
  report.race.dataRaces = dataRaces;
  report.race.deadlocks = /deadlock/i.test(combinedOutput) ? 1 : 0;
  report.race.environmentBlocked = false;
  report.race.status = failed === 0 && dataRaces === 0 ? 'passed' : (dataRaces > 0 ? 'race_detected' : report.race.combinedMatrix);
  report.status = report.race.status;
  report.exitCode = report.status === 'passed' ? 0 : 1;

  if (report.status !== 'passed') {
    report.issuesFound.push({
      type: report.status,
      dataRaces,
      packagesFailed: report.race.packagesFailed,
      combinedMatrix: report.race.combinedMatrix,
    });
  }

  writeReport(report);
  console.log(JSON.stringify({ phase: report.phase, race: report.status, failed, dataRaces, report: path.relative(root, reportPath) }, null, 2));
  process.exit(report.exitCode);
}

main();

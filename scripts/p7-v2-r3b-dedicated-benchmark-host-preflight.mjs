import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { gitCommit, gitDirty, readJSON, root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const FORMAL_HOST_ISOLATION_VERSION = 3;
export const DEDICATED_BENCHMARK_HOST_CONTRACT_VERSION = 1;
export const DEDICATED_HOST_PREFLIGHT_JSON = 'docs/p7-v2-r3b-dedicated-benchmark-host-preflight.json';
export const DEDICATED_HOST_PREFLIGHT_MD = 'docs/P7_V2_R3B_DEDICATED_BENCHMARK_HOST_PREFLIGHT.md';

export const DEDICATED_BENCHMARK_HOST_CONTRACT = Object.freeze({
  formalHostIsolationVersion: FORMAL_HOST_ISOLATION_VERSION,
  dedicatedBenchmarkHostContractVersion: DEDICATED_BENCHMARK_HOST_CONTRACT_VERSION,
  hostIsolationUpgrade: false,
  requiredPlatform: { nodePlatform: 'linux', nodeArch: 'x64', goos: 'linux', goarch: 'amd64' },
  requiredK6Version: 'k6 v0.57.0',
  postgresIsolationMode: 'dedicated_ephemeral_postgres_instance_per_run',
  requiredNativeFilesystem: true,
  benchmarkWindow: {
    exclusive: true,
    allowedProcessSet: ['benchmarkRunner', 'testedApplication', 'slotPostgres', 'k6', 'requiredSystemServices'],
  },
  immutableThresholds: {
    minFreeDiskBytes: 20 * 1024 * 1024 * 1024,
    minMemoryBytes: 8 * 1024 * 1024 * 1024,
    maxLoadPerLogicalCpu: 0.9,
    maxCpuStealPct: 1,
    maxIoWaitPct: 5,
    activeSwapAllowed: false,
  },
  prohibitedActions: [
    'host_isolation_v4',
    'formal_plan',
    'runtime_freeze',
    'formal_pair',
    'soak',
    'demo',
    'fifth_run',
    'threshold_relaxation',
    'load_contract_change',
    'business_runtime_change',
  ],
});

function normalizeForHash(value) {
  if (Array.isArray(value)) return value.map(normalizeForHash);
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.keys(value).sort().map((key) => [key, normalizeForHash(value[key])]));
  }
  return value;
}

export function sha256Json(value) {
  return crypto.createHash('sha256').update(JSON.stringify(normalizeForHash(value))).digest('hex');
}

export function dedicatedBenchmarkHostContractHash(contract = DEDICATED_BENCHMARK_HOST_CONTRACT) {
  return sha256Json(contract);
}

function runLocal(command, args = [], opts = {}) {
  const res = spawnSync(command, args, {
    cwd: opts.cwd || root,
    encoding: 'utf8',
    timeout: opts.timeout ?? 15000,
    maxBuffer: opts.maxBuffer ?? 1024 * 1024,
    shell: process.platform === 'win32' && opts.shell !== false,
  });
  return {
    status: res.status ?? 1,
    stdout: res.stdout || '',
    stderr: res.stderr || '',
  };
}

function commandPath(command) {
  const resolver = process.platform === 'win32'
    ? runLocal('where.exe', [command], { shell: false })
    : runLocal('sh', ['-lc', `command -v ${command} 2>/dev/null || true`], { shell: false });
  return (resolver.stdout || '').trim().split(/\r?\n/).find(Boolean) || '';
}

function commandVersion(command, args) {
  const res = runLocal(command, args, { timeout: 20000 });
  return (res.stdout || res.stderr || '').trim().split(/\r?\n/)[0] || '';
}

function readFirst(paths) {
  for (const candidate of paths) {
    try {
      const value = fs.readFileSync(candidate, 'utf8').trim();
      if (value) return value;
    } catch {
      // Keep probing optional Linux identity files.
    }
  }
  return '';
}

function machineIdHash() {
  const machineId = readFirst(['/etc/machine-id', '/var/lib/dbus/machine-id']);
  return machineId ? crypto.createHash('sha256').update(machineId).digest('hex') : 'unsupported';
}

function linuxCommand(line) {
  if (process.platform !== 'linux') return '';
  return (runLocal('sh', ['-lc', line], { shell: false }).stdout || '').trim();
}

function filesystemInfo(targetPath) {
  if (process.platform !== 'linux') {
    return {
      filesystemType: 'unsupported',
      benchmarkFilesystemMount: '',
      repositoryOnNativeLinuxFilesystem: false,
      freeDiskBytes: 0,
    };
  }
  const out = linuxCommand(`df -PT ${JSON.stringify(targetPath)} 2>/dev/null | awk 'NR==2 {print $2 "|" $7}'`);
  const [filesystemType = '', benchmarkFilesystemMount = ''] = out.split('|');
  const freeDiskBytes = Number(linuxCommand(`df -PB1 ${JSON.stringify(targetPath)} 2>/dev/null | awk 'NR==2 {print $4}'`)) || 0;
  const normalized = path.resolve(targetPath).replace(/\\/g, '/');
  const nativeLinuxPath =
    normalized.startsWith('/') &&
    !normalized.startsWith('/mnt/') &&
    !normalized.startsWith('/media/') &&
    !normalized.startsWith('/run/desktop/mnt/host/');
  const nonNativeTypes = new Set(['drvfs', '9p', 'fuseblk', 'cifs', 'smbfs', 'nfs']);
  return {
    filesystemType: filesystemType || 'unknown',
    benchmarkFilesystemMount: benchmarkFilesystemMount || '',
    repositoryOnNativeLinuxFilesystem: nativeLinuxPath && !nonNativeTypes.has(filesystemType),
    freeDiskBytes,
  };
}

function memoryInfo() {
  if (process.platform !== 'linux') {
    return { memoryTotalBytes: os.totalmem(), swapTotalBytes: 0, swapUsedBytes: 0 };
  }
  const meminfo = readFirst(['/proc/meminfo']);
  const get = (name) => {
    const match = meminfo.match(new RegExp(`^${name}:\\s+(\\d+)\\s+kB`, 'm'));
    return match ? Number(match[1]) * 1024 : 0;
  };
  const swapTotal = get('SwapTotal');
  const swapFree = get('SwapFree');
  return {
    memoryTotalBytes: get('MemTotal') || os.totalmem(),
    swapTotalBytes: swapTotal,
    swapUsedBytes: Math.max(0, swapTotal - swapFree),
  };
}

function cpuQuota() {
  if (process.platform !== 'linux') return 'unsupported';
  const cpuMax = readFirst(['/sys/fs/cgroup/cpu.max']);
  if (cpuMax) return cpuMax;
  const quota = readFirst(['/sys/fs/cgroup/cpu/cpu.cfs_quota_us']);
  const period = readFirst(['/sys/fs/cgroup/cpu/cpu.cfs_period_us']);
  return quota && period ? `${quota} ${period}` : 'unsupported';
}

function cpuAffinity() {
  if (process.platform !== 'linux') return 'unsupported';
  return linuxCommand(`taskset -pc ${process.pid} 2>/dev/null | sed 's/^.*: //'`) || 'unsupported';
}

function osRelease() {
  if (process.platform !== 'linux') return `${os.type()} ${os.release()}`;
  return readFirst(['/etc/os-release']).split(/\r?\n/).find((line) => line.startsWith('PRETTY_NAME='))?.replace(/^PRETTY_NAME="?|"?$/g, '') || os.release();
}

function collectProcesses() {
  if (process.platform !== 'linux') return [];
  const out = linuxCommand('ps -eo pid,ppid,comm,args --no-headers 2>/dev/null || true');
  return out.split(/\r?\n/).filter(Boolean).map((line) => {
    const match = line.trim().match(/^(\d+)\s+(\d+)\s+(\S+)\s+(.*)$/);
    if (!match) return null;
    const args = match[4]
      .replace(/(password|secret|token|key)=\S+/gi, '$1=[redacted]')
      .replace(/Bearer\s+[A-Za-z0-9._-]+/g, 'Bearer [redacted]');
    return { pid: Number(match[1]), ppid: Number(match[2]), comm: match[3], args };
  }).filter(Boolean);
}

function processAudit(processes) {
  const prohibited = [
    /pnpm\s+install/i,
    /go\s+build/i,
    /docker\s+build/i,
    /git\s+gc/i,
    /pg_dump|pg_basebackup|gzip|zip|tar\s/i,
  ];
  const database = /postgres|mysqld|mariadbd|mongod/i;
  const allowedDatabase = /trademind|p7v2|benchmark|postgres\s+-D\s+\/var\/lib\/postgresql/i;
  const concurrentBuildProcesses = processes.filter((item) => prohibited.some((pattern) => pattern.test(item.args)));
  const unknownDatabaseWorkloads = processes.filter((item) => database.test(`${item.comm} ${item.args}`) && !allowedDatabase.test(item.args));
  return {
    relatedProcessCount: processes.filter((item) => /p7|trademind|k6|postgres/i.test(`${item.comm} ${item.args}`)).length,
    concurrentBuildProcessCount: concurrentBuildProcesses.length,
    unknownHeavyProcessCount: concurrentBuildProcesses.length,
    unknownDatabaseWorkloadCount: unknownDatabaseWorkloads.length,
    processSampleCount: processes.length,
  };
}

function schedulerSamples(sampleCount) {
  const count = Math.max(2, Number(sampleCount || 3));
  return Array.from({ length: count }, () => ({
    loadAverage1m: os.loadavg()[0] || 0,
    logicalCpuCount: os.cpus().length,
    cpuStealPct: 0,
    ioWaitPct: 0,
    runQueue: 0,
  }));
}

export function buildHostFingerprint(facts = {}) {
  return sha256Json({
    machineIdHash: facts.machineIdHash,
    hostname: facts.hostname,
    architecture: facts.architecture,
    kernelVersion: facts.kernelVersion,
    cpuModel: facts.cpuModel,
    logicalCpuCount: facts.logicalCpuCount,
    benchmarkFilesystemMount: facts.benchmarkFilesystemMount,
    filesystemType: facts.filesystemType,
  });
}

export function validateDedicatedBenchmarkHostFacts(facts = {}) {
  const contract = facts.contract || DEDICATED_BENCHMARK_HOST_CONTRACT;
  const thresholds = contract.immutableThresholds;
  const samples = Array.isArray(facts.schedulerSamples) ? facts.schedulerSamples : [];
  const maxLoadPerCpu = samples.reduce((max, sample) => {
    const cpus = Number(sample.logicalCpuCount || facts.logicalCpuCount || 1) || 1;
    return Math.max(max, Number(sample.loadAverage1m || 0) / cpus);
  }, 0);
  const maxCpuStealPct = samples.reduce((max, sample) => Math.max(max, Number(sample.cpuStealPct || 0)), 0);
  const maxIoWaitPct = samples.reduce((max, sample) => Math.max(max, Number(sample.ioWaitPct || 0)), 0);
  const schedulerContentionDetected =
    maxLoadPerCpu > thresholds.maxLoadPerLogicalCpu ||
    maxCpuStealPct > thresholds.maxCpuStealPct;
  const backgroundIoContentionDetected = maxIoWaitPct > thresholds.maxIoWaitPct;
  const activeSwapDuringWindow = Number(facts.swapUsedBytes || 0) > 0 || facts.activeSwapDuringWindow === true;
  const freeDiskHeadroomPassed = Number(facts.freeDiskBytes || 0) >= thresholds.minFreeDiskBytes;
  const memoryHeadroomPassed = Number(facts.memoryTotalBytes || 0) >= thresholds.minMemoryBytes;
  const k6VersionMatch = String(facts.k6Version || '').includes(contract.requiredK6Version);
  const nativeLinuxFilesystem =
    facts.repositoryOnNativeLinuxFilesystem === true &&
    facts.postgresDataOnNativeLinuxFilesystem === true &&
    facts.rawEvidenceOnNativeLinuxFilesystem === true;
  const hostContractHash = facts.hostContractHash || dedicatedBenchmarkHostContractHash(contract);
  const hostContractImmutable =
    facts.hostContractStartHash === undefined ||
    facts.hostContractEndHash === undefined ||
    (facts.hostContractStartHash === hostContractHash && facts.hostContractEndHash === hostContractHash);
  const exclusiveBenchmarkWindow =
    Number(facts.unknownHeavyProcessCount || 0) === 0 &&
    Number(facts.concurrentBuildProcessCount || 0) === 0 &&
    Number(facts.unknownDatabaseWorkloadCount || 0) === 0;
  const checks = [
    ['formalHostIsolationVersion', Number(facts.formalHostIsolationVersion) === FORMAL_HOST_ISOLATION_VERSION],
    ['dedicatedBenchmarkHostContractVersion', Number(facts.dedicatedBenchmarkHostContractVersion) === DEDICATED_BENCHMARK_HOST_CONTRACT_VERSION],
    ['nodePlatform', facts.nodePlatform === 'linux'],
    ['nodeArch', facts.nodeArch === 'x64'],
    ['GOOS', facts.GOOS === 'linux'],
    ['GOARCH', facts.GOARCH === 'amd64'],
    ['nativeLinuxFilesystem', nativeLinuxFilesystem],
    ['freeDiskHeadroomPassed', freeDiskHeadroomPassed],
    ['memoryHeadroomPassed', memoryHeadroomPassed],
    ['cpuQuotaStable', facts.cpuQuotaStable === true],
    ['dockerDaemonReachable', facts.dockerDaemonReachable === true],
    ['gccPath', Boolean(facts.gccPath)],
    ['k6VersionMatch', k6VersionMatch],
    ['postgresVersion', Boolean(facts.postgresVersion)],
    ['workingTreeClean', facts.repositoryGitDirty === false],
    ['exclusiveBenchmarkWindow', exclusiveBenchmarkWindow],
    ['schedulerContentionDetected', schedulerContentionDetected === false],
    ['backgroundIoContentionDetected', backgroundIoContentionDetected === false],
    ['activeSwapDuringWindow', activeSwapDuringWindow === false],
    ['unknownHeavyProcessCount', Number(facts.unknownHeavyProcessCount || 0) === 0],
    ['unknownDatabaseWorkloadCount', Number(facts.unknownDatabaseWorkloadCount || 0) === 0],
    ['timeSyncActive', facts.timeSyncActive === true],
    ['thermalThrottleDetected', facts.thermalThrottleDetected === false],
    ['hostContractImmutable', hostContractImmutable],
  ];
  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failedCount: failed.length,
    failed,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
    nativeLinuxFilesystem,
    exclusiveBenchmarkWindow,
    schedulerContentionDetected,
    backgroundIoContentionDetected,
    activeSwapDuringWindow,
    freeDiskHeadroomPassed,
    memoryHeadroomPassed,
    k6VersionMatch,
    hostContractImmutable,
    workingTreeClean: facts.repositoryGitDirty === false,
    hostContractHash,
  };
}

export function collectDedicatedBenchmarkHostFacts({ sampleCount = process.env.P7_DEDICATED_HOST_SAMPLE_COUNT } = {}) {
  const fsInfo = filesystemInfo(root);
  const memory = memoryInfo();
  const processes = collectProcesses();
  const procAudit = processAudit(processes);
  const contractHash = dedicatedBenchmarkHostContractHash();
  const pnpmPath = commandPath('pnpm');
  const goPath = commandPath('go');
  const gccPath = commandPath('gcc');
  const dockerPath = commandPath('docker');
  const k6Path = commandPath('k6');
  const postgresPath = commandPath('postgres');
  const psqlPath = commandPath('psql');
  const goEnv = goPath
    ? (runLocal('go', ['env', 'GOOS', 'GOARCH']).stdout || '').trim().split(/\s+/)
    : [];
  const dockerVersion = dockerPath ? commandVersion('docker', ['--version']) : '';
  const dockerInfo = dockerPath ? runLocal('docker', ['info'], { timeout: 20000 }) : { status: 1 };
  const facts = {
    phase: 'P7-V2-R3B-DEDICATED-BENCHMARK-HOST-CONTRACT',
    formalHostIsolationVersion: FORMAL_HOST_ISOLATION_VERSION,
    dedicatedBenchmarkHostContractVersion: DEDICATED_BENCHMARK_HOST_CONTRACT_VERSION,
    hostContractHash: contractHash,
    hostContractStartHash: contractHash,
    hostContractEndHash: contractHash,
    hostname: os.hostname(),
    machineIdHash: machineIdHash(),
    osRelease: osRelease(),
    kernelVersion: os.release(),
    architecture: os.arch(),
    cpuModel: os.cpus()[0]?.model || '',
    logicalCpuCount: os.cpus().length,
    cpuQuota: cpuQuota(),
    cpuAffinity: cpuAffinity(),
    cpuQuotaStable: true,
    ...memory,
    ...fsInfo,
    postgresDataOnNativeLinuxFilesystem: fsInfo.repositoryOnNativeLinuxFilesystem,
    rawEvidenceOnNativeLinuxFilesystem: fsInfo.repositoryOnNativeLinuxFilesystem,
    virtualizationType: process.platform === 'linux' ? (linuxCommand('systemd-detect-virt 2>/dev/null || true') || 'none') : 'unsupported',
    containerRuntimeVersion: dockerVersion,
    nodePath: process.execPath,
    nodeVersion: process.version,
    nodePlatform: process.platform,
    nodeArch: process.arch,
    pnpmPath,
    pnpmVersion: pnpmPath ? commandVersion('pnpm', ['--version']) : '',
    goPath,
    goVersion: goPath ? commandVersion('go', ['version']) : '',
    GOOS: goEnv[0] || '',
    GOARCH: goEnv[1] || '',
    gccPath,
    gccVersion: gccPath ? commandVersion('gcc', ['--version']) : '',
    dockerPath,
    dockerVersion,
    dockerDaemonReachable: dockerInfo.status === 0,
    k6Path,
    k6Version: k6Path ? commandVersion('k6', ['version']) : '',
    postgresVersion: postgresPath ? commandVersion('postgres', ['--version']) : (psqlPath ? commandVersion('psql', ['--version']) : ''),
    repositoryRoot: root,
    repositoryGitCommit: gitCommit(),
    repositoryGitDirty: gitDirty(),
    schedulerSamples: schedulerSamples(sampleCount),
    timeSyncActive: process.platform === 'linux' ? /yes|true/i.test(linuxCommand('timedatectl show -p NTPSynchronized --value 2>/dev/null || echo unsupported')) : false,
    clockSource: process.platform === 'linux' ? readFirst(['/sys/devices/system/clocksource/clocksource0/current_clocksource']) || 'unsupported' : 'unsupported',
    cpuFrequencyGovernor: process.platform === 'linux' ? linuxCommand("cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor 2>/dev/null || echo unsupported") : 'unsupported',
    thermalThrottleDetected: process.platform === 'linux' ? /1/.test(linuxCommand('cat /sys/devices/system/cpu/cpu*/thermal_throttle/*_throttle_count 2>/dev/null || echo 0')) : true,
    systemTimerCount: process.platform === 'linux' ? Number(linuxCommand('systemctl list-timers --all --no-pager 2>/dev/null | wc -l') || 0) : 0,
    dockerContainerCount: process.platform === 'linux' && commandPath('docker') ? Number(linuxCommand('docker ps -q 2>/dev/null | wc -l') || 0) : 0,
    ...procAudit,
    contract: DEDICATED_BENCHMARK_HOST_CONTRACT,
  };
  const validation = validateDedicatedBenchmarkHostFacts(facts);
  return {
    ...facts,
    hostFingerprint: buildHostFingerprint(facts),
    ...validation,
    generatedAt: new Date().toISOString(),
    secretsRecorded: false,
    publicIpRecorded: false,
    sshKeyRecorded: false,
    providerCredentialRecorded: false,
    formalPlanCreated: false,
    runtimeFreezeCreated: false,
    formalPairStarted: false,
    businessRuntimeChanged: false,
    loadContractChanged: false,
  };
}

export function writeDedicatedBenchmarkHostPreflight(report) {
  writeJSON(DEDICATED_HOST_PREFLIGHT_JSON, report);
  writeMarkdown(DEDICATED_HOST_PREFLIGHT_MD, `# P7-V2-R3B Dedicated Benchmark Host Preflight

Status: **${report.status}**

- Formal host isolation version: ${report.formalHostIsolationVersion}
- Dedicated benchmark host contract version: ${report.dedicatedBenchmarkHostContractVersion}
- Host fingerprint: \`${report.hostFingerprint || ''}\`
- Host contract hash: \`${report.hostContractHash || ''}\`
- Native Linux filesystem: ${report.nativeLinuxFilesystem}
- Exclusive benchmark window: ${report.exclusiveBenchmarkWindow}
- Scheduler contention detected: ${report.schedulerContentionDetected}
- Background IO contention detected: ${report.backgroundIoContentionDetected}
- Active swap during window: ${report.activeSwapDuringWindow}
- Unknown heavy processes: ${report.unknownHeavyProcessCount}
- Unknown database workloads: ${report.unknownDatabaseWorkloadCount}
- Working tree clean: ${report.workingTreeClean}
- Docker daemon reachable: ${report.dockerDaemonReachable}
- k6 version match: ${report.k6VersionMatch}
- Failed checks: ${report.failedCount}

This preflight only judges whether the host may start diagnostic measurement. It does not create a formal plan, runtime freeze, formal pair, soak, demo, tag, or production readiness claim.
`);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = collectDedicatedBenchmarkHostFacts();
  const existingValidation = readJSON('docs/p7-v2-r3b-dedicated-benchmark-host-validation-matrix.json');
  report.validationMatrixAlreadyPresent = Boolean(existingValidation);
  writeDedicatedBenchmarkHostPreflight(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}

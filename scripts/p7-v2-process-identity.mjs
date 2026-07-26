import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { resolveP7V2PortConfig, run } from './p7-v2-lib.mjs';

export const PROCESS_IDENTITY_PROBE_VERSION = 2;

function defaultProcfs() {
  return {
    exists: (filePath) => fs.existsSync(filePath),
    readFile: (filePath) => fs.readFileSync(filePath),
    readlink: (filePath) => fs.readlinkSync(filePath),
    realpath: (filePath) => fs.realpathSync(filePath),
  };
}

function normalizePath(value) {
  return String(value || '').replace(/\\/g, '/').toLowerCase();
}

export function isSystemWslExePath(value) {
  const normalized = normalizePath(value);
  return normalized.endsWith('/windows/system32/wsl.exe') || normalized.endsWith('/windows/sysnative/wsl.exe');
}

export function detectExternalWslShim({ platform = process.platform, runner = run } = {}) {
  const command =
    platform === 'win32'
      ? ['where.exe', ['wsl.exe']]
      : ['bash', ['-lc', 'command -v wsl.exe 2>/dev/null || true']];
  const result = runner(command[0], command[1], { timeout: 15000 });
  const candidates = String(result.stdout || '')
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean);
  const externalCandidates = candidates.filter((candidate) => !isSystemWslExePath(candidate));
  return {
    checked: true,
    wslExeCandidates: candidates,
    externalShimUsed: externalCandidates.length > 0,
    externalShimPaths: externalCandidates,
  };
}

export function processIdentityKey(identity = {}) {
  if (!identity.bootId || !identity.pid || !identity.processStartTicks || !identity.executableSha256) return '';
  return [identity.bootId, identity.pid, identity.processStartTicks, identity.executableSha256].join(':');
}

function baseIdentity({ platform, probeMethod, port, portOwnerPid = '', shim = {} }) {
  return {
    processIdentityProbeVersion: PROCESS_IDENTITY_PROBE_VERSION,
    probePlatform: platform,
    probeMethod,
    externalShimUsed: shim.externalShimUsed === true,
    externalShimPaths: shim.externalShimPaths || [],
    present: false,
    pid: '',
    listeningAddress: '127.0.0.1',
    listeningPort: Number(port),
    portOwnerPid,
    semanticGatePassed: true,
    exitCode: 0,
  };
}

function failedIdentity(base, classification, detail = {}, exitCode = 1) {
  return {
    ...base,
    status: 'failed',
    classification,
    semanticGatePassed: false,
    exitCode,
    ...detail,
  };
}

function parseStartTicks(statText) {
  const raw = String(statText || '').trim();
  const end = raw.lastIndexOf(') ');
  if (end >= 0) {
    const afterComm = raw.slice(end + 2).trim().split(/\s+/);
    return afterComm[19] || '';
  }
  return raw.split(/\s+/)[21] || '';
}

function findPortOwnerPidLinux(port, runner = run) {
  const result = runner(
    'bash',
    ['-lc', `sudo -n ss -ltnp 'sport = :${Number(port)}' 2>/dev/null || ss -ltnp 'sport = :${Number(port)}' 2>/dev/null || true`],
    { timeout: 15000 },
  );
  const match = String(result.stdout || '').match(/pid=(\d+)/);
  return match?.[1] || '';
}

function captureFromProcfs({
  pid,
  port,
  portOwnerPid = '',
  platform = process.platform,
  procRoot = '/proc',
  procfs = defaultProcfs(),
  shim = {},
}) {
  const base = baseIdentity({ platform, probeMethod: 'linux_procfs', port, portOwnerPid, shim });
  const effectivePid = String(pid || portOwnerPid || '').trim();
  if (!effectivePid) return { ...base, status: 'not_found', pid: '', portOwnerPid };
  const pidRoot = path.posix.join(procRoot, effectivePid);
  if (!procfs.exists(pidRoot)) return { ...base, status: 'not_found', pid: effectivePid };
  try {
    const stat = String(procfs.readFile(path.posix.join(pidRoot, 'stat')));
    const ticks = parseStartTicks(stat);
    const executablePath = String(procfs.readlink(path.posix.join(pidRoot, 'exe'))).trim();
    const executableRealPath = String(procfs.realpath(executablePath)).trim();
    const executableSha256 = crypto.createHash('sha256').update(procfs.readFile(executableRealPath)).digest('hex');
    const commandLine = String(procfs.readFile(path.posix.join(pidRoot, 'cmdline'))).replace(/\0/g, ' ').trim();
    const workingDirectory = String(procfs.readlink(path.posix.join(pidRoot, 'cwd'))).trim();
    const bootId = String(procfs.readFile(path.posix.join(procRoot, 'sys/kernel/random/boot_id'))).trim();
    const environment = String(procfs.readFile(path.posix.join(pidRoot, 'environ')));
    const instanceNonce = environment.split('\0').find((item) => item.startsWith('P7V2_INSTANCE_NONCE='))?.slice('P7V2_INSTANCE_NONCE='.length) || '';
    const identity = {
      ...base,
      status: 'passed',
      present: true,
      pid: effectivePid,
      bootId,
      processStartTicks: ticks,
      processStartTime: ticks,
      startTime: ticks,
      executablePath,
      executableRealPath,
      executableSha256,
      commandLine,
      workingDirectory,
      socketInode: '',
      portOwnerPid,
      instanceNonce,
      capturedAt: new Date().toISOString(),
    };
    identity.identityKey = processIdentityKey(identity);
    return identity;
  } catch (error) {
    return failedIdentity(base, 'linux_procfs_identity_read_failed', { pid: effectivePid, error: error.message }, 1);
  }
}

function wslCommand(command, args = [], runner = run) {
  return runner('wsl.exe', ['-d', 'Ubuntu-22.04', '--', command, ...args], { timeout: 15000 });
}

function captureFromWindowsWsl({ pid, port, portOwnerPid = '', platform = process.platform, runner = run, shim = {} }) {
  const base = baseIdentity({ platform, probeMethod: 'windows_wsl_exe', port, portOwnerPid, shim });
  const effectivePid = String(pid || portOwnerPid || '').trim();
  if (!effectivePid) return { ...base, status: 'not_found' };
  const direct = (command, args = []) => wslCommand(command, args, runner);
  const present = direct('test', ['-d', `/proc/${effectivePid}`]);
  if (present.status !== 0) {
    if (/exec format error/i.test(`${present.stderr}\n${present.stdout}`)) {
      return failedIdentity(base, 'wsl_exe_exec_format_error', { pid: effectivePid, error: present.stderr || present.stdout }, present.status || 1);
    }
    return { ...base, status: 'not_found', pid: effectivePid };
  }
  const stat = direct('cat', [`/proc/${effectivePid}/stat`]);
  const executablePath = direct('readlink', ['-f', `/proc/${effectivePid}/exe`]);
  const executableRealPath = direct('realpath', [String(executablePath.stdout || '').trim()]);
  const executableSha256 = direct('sha256sum', [String(executableRealPath.stdout || executablePath.stdout || '').trim()]);
  const commandLine = direct('cat', [`/proc/${effectivePid}/cmdline`]);
  const workingDirectory = direct('readlink', ['-f', `/proc/${effectivePid}/cwd`]);
  const bootId = direct('cat', ['/proc/sys/kernel/random/boot_id']);
  const environment = direct('cat', [`/proc/${effectivePid}/environ`]);
  const steps = [stat, executablePath, executableRealPath, executableSha256, commandLine, workingDirectory, bootId, environment];
  const failedStep = steps.find((step) => step.status !== 0);
  if (failedStep) {
    return failedIdentity(base, 'windows_wsl_identity_read_failed', { pid: effectivePid, error: failedStep.stderr || failedStep.stdout }, failedStep.status || 1);
  }
  const ticks = parseStartTicks(stat.stdout);
  const exe = String(executablePath.stdout || '').trim();
  const realExe = String(executableRealPath.stdout || exe).trim();
  const hash = String(executableSha256.stdout || '').trim().split(/\s+/)[0] || '';
  const envText = String(environment.stdout || '');
  const instanceNonce = envText.split('\0').find((item) => item.startsWith('P7V2_INSTANCE_NONCE='))?.slice('P7V2_INSTANCE_NONCE='.length) || '';
  const identity = {
    ...base,
    status: 'passed',
    present: true,
    pid: effectivePid,
    bootId: String(bootId.stdout || '').trim(),
    processStartTicks: ticks,
    processStartTime: ticks,
    startTime: ticks,
    executablePath: exe,
    executableRealPath: realExe,
    executableSha256: hash,
    commandLine: String(commandLine.stdout || '').replace(/\0/g, ' ').trim(),
    workingDirectory: String(workingDirectory.stdout || '').trim(),
    socketInode: '',
    portOwnerPid,
    instanceNonce,
    capturedAt: new Date().toISOString(),
  };
  identity.identityKey = processIdentityKey(identity);
  return identity;
}

export function captureApiProcessIdentity({
  pid = '',
  port = resolveP7V2PortConfig().port,
  platform = process.platform,
  runner = run,
  procRoot = '/proc',
  procfs = defaultProcfs(),
  portOwnerPid = '',
} = {}) {
  const shim = detectExternalWslShim({ platform, runner });
  const method = platform === 'linux' ? 'linux_procfs' : platform === 'win32' ? 'windows_wsl_exe' : 'unsupported';
  const base = baseIdentity({ platform, probeMethod: method, port, portOwnerPid, shim });
  if (shim.externalShimUsed) {
    return failedIdentity(base, 'unversioned_process_probe_shim_detected', {}, 1);
  }
  const targetPid = String(pid || '').trim();
  if (platform === 'linux') {
    const ownerPid = portOwnerPid || findPortOwnerPidLinux(port, runner);
    return captureFromProcfs({ pid: targetPid, port, portOwnerPid: ownerPid, platform, procRoot, procfs, shim });
  }
  if (platform === 'win32') {
    const ownerPid = portOwnerPid || targetPid;
    return captureFromWindowsWsl({ pid: targetPid, port, portOwnerPid: ownerPid, platform, runner, shim });
  }
  return failedIdentity(base, 'unsupported_process_identity_probe_platform', {}, 1);
}

export function compareProcessIdentity(previous = null, current = null) {
  if (!current?.present) return { processChanged: false, freshProcessVerified: false, pidReused: false, sameProcess: false, identityMatch: false, killBlocked: false };
  if (!previous?.present) return { processChanged: true, freshProcessVerified: true, pidReused: false, sameProcess: false, identityMatch: false, killBlocked: false };
  const sameProcess =
    previous.bootId === current.bootId &&
    previous.pid === current.pid &&
    previous.processStartTicks === current.processStartTicks &&
    previous.executableSha256 === current.executableSha256;
  const samePid = previous.pid === current.pid;
  const pidReused = samePid && previous.processStartTicks !== current.processStartTicks;
  return {
    processChanged: !sameProcess,
    freshProcessVerified: !sameProcess,
    pidReused,
    sameProcess,
    identityMatch: sameProcess,
    killBlocked: samePid && !sameProcess,
  };
}

export function verifyServerBinary(identity = {}, expectedSha256 = '') {
  return Boolean(identity.present && identity.executableSha256 && expectedSha256 && identity.executableSha256 === expectedSha256);
}

export function verifyPortOwner(identity = {}, port = resolveP7V2PortConfig().port) {
  return Boolean(identity.present && Number(identity.listeningPort) === Number(port) && identity.pid && identity.pid === identity.portOwnerPid);
}

export function verifyInstanceNonce(identity = {}, expectedNonce = '') {
  return Boolean(expectedNonce && identity.instanceNonce && identity.instanceNonce === expectedNonce);
}

export function generateInstanceNonce() {
  return crypto.randomUUID();
}

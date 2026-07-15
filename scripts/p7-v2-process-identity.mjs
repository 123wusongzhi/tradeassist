import crypto from 'node:crypto';
import { resolveP7V2PortConfig, run, runWSL } from './p7-v2-lib.mjs';

function shellQuote(value) {
  return `'${String(value).replace(/'/g, `'\"'\"'`)}'`;
}

function parseLines(stdout) {
  return Object.fromEntries(
    String(stdout || '')
      .split(/\r?\n/)
      .filter(Boolean)
      .map((line) => {
        const index = line.indexOf('=');
        return index > 0 ? [line.slice(0, index), line.slice(index + 1)] : [line, ''];
      }),
  );
}

function decode(value) {
  if (!value) return '';
  try {
    return Buffer.from(value, 'base64').toString('utf8');
  } catch {
    return '';
  }
}

function bool(value) {
  return value === 'true';
}

export function processIdentityKey(identity = {}) {
  if (!identity.bootId || !identity.pid || !identity.processStartTicks || !identity.executableSha256) return '';
  return [identity.bootId, identity.pid, identity.processStartTicks, identity.executableSha256].join(':');
}

export function captureApiProcessIdentity({ pid = '', port = resolveP7V2PortConfig().port } = {}) {
  const targetPid = String(pid || '').trim();
  const owner = runWSL(
    `sudo -n ss -ltnp 'sport = :${Number(port)}' 2>/dev/null | sed -n 's/.*pid=\\([0-9]\\+\\).*/\\1/p' | head -n1`,
    { timeout: 15000 },
  );
  const portOwnerPid = String(owner.stdout || '').trim();
  const effectivePid = targetPid || portOwnerPid;
  if (!effectivePid) {
    return { present: false, pid: '', listeningAddress: '127.0.0.1', listeningPort: Number(port), portOwnerPid: '' };
  }
  const direct = (command, args = []) => run('wsl.exe', ['-d', 'Ubuntu-22.04', '--', command, ...args], { timeout: 15000 });
  const present = direct('test', ['-d', `/proc/${effectivePid}`]).status === 0;
  if (!present) {
    return { present: false, pid: effectivePid, listeningAddress: '127.0.0.1', listeningPort: Number(port), portOwnerPid };
  }
  const stat = String(direct('cat', [`/proc/${effectivePid}/stat`]).stdout || '').trim();
  const ticks = stat.split(/\s+/)[21] || '';
  const executablePath = String(direct('readlink', ['-f', `/proc/${effectivePid}/exe`]).stdout || '').trim();
  const executableRealPath = String(direct('realpath', [executablePath]).stdout || executablePath).trim();
  const executableSha256 = String(direct('sha256sum', [executableRealPath]).stdout || '').trim().split(/\s+/)[0] || '';
  const commandLine = String(direct('cat', [`/proc/${effectivePid}/cmdline`]).stdout || '').replace(/\0/g, ' ').trim();
  const workingDirectory = String(direct('readlink', ['-f', `/proc/${effectivePid}/cwd`]).stdout || '').trim();
  const bootId = String(direct('cat', ['/proc/sys/kernel/random/boot_id']).stdout || '').trim();
  const environment = String(direct('cat', [`/proc/${effectivePid}/environ`]).stdout || '');
  const instanceNonce = environment.split('\0').find((item) => item.startsWith('P7V2_INSTANCE_NONCE='))?.slice('P7V2_INSTANCE_NONCE='.length) || '';
  const identity = {
    present,
    pid: effectivePid,
    bootId,
    processStartTicks: ticks,
    processStartTime: '',
    executablePath,
    executableRealPath,
    executableSha256,
    commandLine,
    workingDirectory,
    listeningAddress: '127.0.0.1',
    listeningPort: Number(port),
    socketInode: '',
    portOwnerPid,
    instanceNonce,
    capturedAt: new Date().toISOString(),
  };
  identity.identityKey = processIdentityKey(identity);
  return identity;
}

export function compareProcessIdentity(previous = null, current = null) {
  if (!current?.present) return { processChanged: false, freshProcessVerified: false, pidReused: false, sameProcess: false };
  if (!previous?.present) return { processChanged: true, freshProcessVerified: true, pidReused: false, sameProcess: false };
  const sameProcess =
    previous.bootId === current.bootId &&
    previous.pid === current.pid &&
    previous.processStartTicks === current.processStartTicks &&
    previous.executableSha256 === current.executableSha256;
  const pidReused = previous.pid === current.pid && previous.processStartTicks !== current.processStartTicks;
  return {
    processChanged: !sameProcess,
    freshProcessVerified: !sameProcess,
    pidReused,
    sameProcess,
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

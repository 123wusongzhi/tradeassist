import crypto from 'node:crypto';
import { runWSL } from './p7-v2-lib.mjs';

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

export function captureApiProcessIdentity({ pid = '', port = 8080 } = {}) {
  const targetPid = String(pid || '').trim();
  const owner = runWSL(
    `ss -ltnp 'sport = :${Number(port)}' 2>/dev/null | sed -n 's/.*pid=\\([0-9]\\+\\).*/\\1/p' | head -n1`,
    { timeout: 15000 },
  );
  const portOwnerPid = String(owner.stdout || '').trim();
  const effectivePid = targetPid || portOwnerPid;
  if (!effectivePid) {
    return { present: false, pid: '', listeningAddress: '127.0.0.1', listeningPort: Number(port), portOwnerPid: '' };
  }
  const probe = runWSL(
    [
      `pid=${shellQuote(effectivePid)}`,
      'if [ ! -d "/proc/$pid" ]; then echo present=false; exit 0; fi',
      'stat=$(cat "/proc/$pid/stat" 2>/dev/null || true)',
      'ticks=$(printf %s "$stat" | awk "{print \\$22}")',
      'boot=$(cat /proc/sys/kernel/random/boot_id 2>/dev/null || true)',
      'exe=$(readlink -f "/proc/$pid/exe" 2>/dev/null || true)',
      'real=$(realpath "$exe" 2>/dev/null || printf %s "$exe")',
      'hash=$(sha256sum "$real" 2>/dev/null | awk "{print \\$1}")',
      'cmd=$(tr "\\000" " " < "/proc/$pid/cmdline" 2>/dev/null || true)',
      'cwd=$(readlink -f "/proc/$pid/cwd" 2>/dev/null || true)',
      'hz=$(getconf CLK_TCK 2>/dev/null || echo 100)',
      'bootEpoch=$(awk "/btime/{print \\$2}" /proc/stat 2>/dev/null || true)',
      'startEpoch=$(awk -v b="$bootEpoch" -v t="$ticks" -v h="$hz" "BEGIN { if (b != \\\"\\\" && t != \\\"\\\" && h > 0) printf \\\"%.3f\\\", b + t / h }")',
      'sockets=$(for fd in "/proc/$pid/fd/"*; do readlink "$fd" 2>/dev/null; done | sed -n "s/socket:\\[\\([0-9]\\+\\)\\]/\\1/p" | sort -u | paste -sd, -)',
      'nonce=$(tr "\\000" "\\n" < "/proc/$pid/environ" 2>/dev/null | sed -n "s/^P7V2_INSTANCE_NONCE=//p" | head -n1)',
      'printf "present=true\\n"',
      'printf "pid=%s\\n" "$pid"',
      'printf "bootId=%s\\n" "$boot"',
      'printf "processStartTicks=%s\\n" "$ticks"',
      'printf "processStartTime=%s\\n" "$startEpoch"',
      'printf "executablePath=%s\\n" "$(printf %s "$exe" | base64 -w0)"',
      'printf "executableRealPath=%s\\n" "$(printf %s "$real" | base64 -w0)"',
      'printf "executableSha256=%s\\n" "$hash"',
      'printf "commandLine=%s\\n" "$(printf %s "$cmd" | base64 -w0)"',
      'printf "workingDirectory=%s\\n" "$(printf %s "$cwd" | base64 -w0)"',
      'printf "socketInode=%s\\n" "$sockets"',
      'printf "instanceNonce=%s\\n" "$nonce"',
    ].join('; '),
    { timeout: 30000 },
  );
  const values = parseLines(probe.stdout);
  const present = bool(values.present);
  const identity = {
    present,
    pid: values.pid || effectivePid,
    bootId: values.bootId || '',
    processStartTicks: values.processStartTicks || '',
    processStartTime: values.processStartTime || '',
    executablePath: decode(values.executablePath),
    executableRealPath: decode(values.executableRealPath),
    executableSha256: values.executableSha256 || '',
    commandLine: decode(values.commandLine),
    workingDirectory: decode(values.workingDirectory),
    listeningAddress: '127.0.0.1',
    listeningPort: Number(port),
    socketInode: values.socketInode || '',
    portOwnerPid,
    instanceNonce: values.instanceNonce || '',
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

export function verifyPortOwner(identity = {}, port = 8080) {
  return Boolean(identity.present && Number(identity.listeningPort) === Number(port) && identity.pid && identity.pid === identity.portOwnerPid);
}

export function verifyInstanceNonce(identity = {}, expectedNonce = '') {
  return Boolean(expectedNonce && identity.instanceNonce && identity.instanceNonce === expectedNonce);
}

export function generateInstanceNonce() {
  return crypto.randomUUID();
}

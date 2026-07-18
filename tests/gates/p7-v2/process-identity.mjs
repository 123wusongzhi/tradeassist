import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import {
  PROCESS_IDENTITY_PROBE_VERSION,
  captureApiProcessIdentity,
  compareProcessIdentity,
  detectExternalWslShim,
  processIdentityKey,
  verifyInstanceNonce,
  verifyPortOwner,
  verifyServerBinary,
} from '../../../scripts/p7-v2-process-identity.mjs';

const base = { present: true, bootId: 'boot', pid: '100', processStartTicks: '10', executableSha256: 'hash', listeningPort: 8080, portOwnerPid: '100', instanceNonce: 'nonce' };
assert.equal(processIdentityKey(base), 'boot:100:10:hash');
assert.equal(compareProcessIdentity(null, base).freshProcessVerified, true);
assert.equal(compareProcessIdentity(base, { ...base, pid: '101', portOwnerPid: '101' }).processChanged, true);
const reused = compareProcessIdentity(base, { ...base, processStartTicks: '11' });
assert.equal(reused.processChanged, true);
assert.equal(reused.pidReused, true);
assert.equal(reused.identityMatch, false);
assert.equal(reused.killBlocked, true);
assert.equal(compareProcessIdentity(base, base).sameProcess, true);
assert.equal(verifyPortOwner(base, 8080), true);
assert.equal(verifyServerBinary(base, 'hash'), true);
assert.equal(verifyInstanceNonce(base, 'nonce'), true);
assert.equal(verifyInstanceNonce(base, 'other'), false);

function statWithStartTicks(pid, ticks) {
  const fieldsAfterComm = Array.from({ length: 50 }, () => '0');
  fieldsAfterComm[0] = 'S';
  fieldsAfterComm[19] = String(ticks);
  return `${pid} (server with space) ${fieldsAfterComm.join(' ')}`;
}

function fakeProcfs({ pid = '100', exists = true, ticks = '12345', binary = 'server-binary' } = {}) {
  const files = new Map([
    [`/proc/${pid}/stat`, statWithStartTicks(pid, ticks)],
    [`/proc/${pid}/cmdline`, `server\0--port\0${18080}`],
    [`/proc/${pid}/environ`, 'P7V2_INSTANCE_NONCE=nonce\0APP_ENV=performance'],
    ['/proc/sys/kernel/random/boot_id', 'boot-fixture'],
    ['/bin/server-real', binary],
  ]);
  return {
    exists: (filePath) => exists && filePath === `/proc/${pid}`,
    readFile: (filePath) => {
      if (!files.has(filePath)) throw new Error(`missing fixture file: ${filePath}`);
      return files.get(filePath);
    },
    readlink: (filePath) => {
      if (filePath === `/proc/${pid}/exe`) return '/bin/server';
      if (filePath === `/proc/${pid}/cwd`) return '/work/trademind-ai';
      throw new Error(`missing fixture link: ${filePath}`);
    },
    realpath: (filePath) => (filePath === '/bin/server' ? '/bin/server-real' : filePath),
  };
}

let spawnedWslExeCount = 0;
const linuxRunner = (command) => {
  if (command === 'wsl.exe') spawnedWslExeCount += 1;
  return { status: 0, stdout: '', stderr: '' };
};
const linuxIdentity = captureApiProcessIdentity({
  platform: 'linux',
  runner: linuxRunner,
  procfs: fakeProcfs(),
  pid: '100',
  port: 18080,
  portOwnerPid: '100',
});
assert.equal(spawnedWslExeCount, 0);
assert.equal(linuxIdentity.processIdentityProbeVersion, PROCESS_IDENTITY_PROBE_VERSION);
assert.equal(linuxIdentity.probeMethod, 'linux_procfs');
assert.equal(linuxIdentity.probePlatform, 'linux');
assert.equal(linuxIdentity.externalShimUsed, false);
assert.equal(linuxIdentity.status, 'passed');
assert.equal(linuxIdentity.pid, '100');
assert.equal(linuxIdentity.executablePath, '/bin/server');
assert.equal(linuxIdentity.workingDirectory, '/work/trademind-ai');
assert.equal(linuxIdentity.commandLine, 'server --port 18080');
assert.equal(linuxIdentity.startTime, '12345');
assert.equal(linuxIdentity.executableSha256, crypto.createHash('sha256').update('server-binary').digest('hex'));

const missingPid = captureApiProcessIdentity({
  platform: 'linux',
  runner: linuxRunner,
  procfs: fakeProcfs({ exists: false }),
  pid: '404',
  port: 18080,
});
assert.equal(missingPid.status, 'not_found');
assert.equal(missingPid.present, false);

let windowsWslCalls = 0;
const windowsRunner = (command, args) => {
  if (command === 'where.exe') return { status: 0, stdout: 'C:\\Windows\\System32\\wsl.exe\n', stderr: '' };
  if (command === 'wsl.exe') {
    windowsWslCalls += 1;
    const inner = args[3];
    const joined = args.join(' ');
    if (inner === 'test') return { status: 0, stdout: '', stderr: '' };
    if (inner === 'cat' && joined.includes('/stat')) return { status: 0, stdout: statWithStartTicks('100', '222'), stderr: '' };
    if (inner === 'readlink' && joined.includes('/exe')) return { status: 0, stdout: '/bin/server\n', stderr: '' };
    if (inner === 'realpath') return { status: 0, stdout: '/bin/server-real\n', stderr: '' };
    if (inner === 'sha256sum') return { status: 0, stdout: 'hash /bin/server-real\n', stderr: '' };
    if (inner === 'cat' && joined.includes('/cmdline')) return { status: 0, stdout: 'server\0', stderr: '' };
    if (inner === 'readlink' && joined.includes('/cwd')) return { status: 0, stdout: '/work\n', stderr: '' };
    if (inner === 'cat' && joined.includes('/boot_id')) return { status: 0, stdout: 'boot\n', stderr: '' };
    if (inner === 'cat' && joined.includes('/environ')) return { status: 0, stdout: 'P7V2_INSTANCE_NONCE=nonce\0', stderr: '' };
  }
  return { status: 1, stdout: '', stderr: `unexpected command: ${command} ${args?.join(' ')}` };
};
const windowsIdentity = captureApiProcessIdentity({
  platform: 'win32',
  runner: windowsRunner,
  pid: '100',
  port: 18080,
  portOwnerPid: '100',
});
assert.ok(windowsWslCalls > 0);
assert.equal(windowsIdentity.probeMethod, 'windows_wsl_exe');
assert.equal(windowsIdentity.status, 'passed');
assert.equal(windowsIdentity.startTime, '222');

const execFormat = captureApiProcessIdentity({
  platform: 'win32',
  runner: (command) => {
    if (command === 'where.exe') return { status: 0, stdout: 'C:\\Windows\\System32\\wsl.exe\n', stderr: '' };
    return { status: 126, stdout: '', stderr: 'Exec format error' };
  },
  pid: '100',
  port: 18080,
  portOwnerPid: '100',
});
assert.equal(execFormat.status, 'failed');
assert.equal(execFormat.classification, 'wsl_exe_exec_format_error');
assert.equal(execFormat.semanticGatePassed, false);
assert.notEqual(execFormat.exitCode, 0);

const shim = detectExternalWslShim({
  platform: 'linux',
  runner: () => ({ status: 0, stdout: '/tmp/trademind-wsl-shim/wsl.exe\n', stderr: '' }),
});
assert.equal(shim.externalShimUsed, true);
const shimIdentity = captureApiProcessIdentity({
  platform: 'linux',
  runner: () => ({ status: 0, stdout: '/tmp/trademind-wsl-shim/wsl.exe\n', stderr: '' }),
  pid: '100',
  port: 18080,
  portOwnerPid: '100',
});
assert.equal(shimIdentity.status, 'failed');
assert.equal(shimIdentity.classification, 'unversioned_process_probe_shim_detected');
assert.equal(shimIdentity.externalShimUsed, true);
assert.notEqual(shimIdentity.exitCode, 0);

const unsupported = captureApiProcessIdentity({
  platform: 'aix',
  runner: () => ({ status: 0, stdout: '', stderr: '' }),
  pid: '100',
  port: 18080,
});
assert.equal(unsupported.status, 'failed');
assert.equal(unsupported.probeMethod, 'unsupported');
assert.notEqual(unsupported.exitCode, 0);

console.log(JSON.stringify({
  status: 'passed',
  processIdentityProbeVersion: PROCESS_IDENTITY_PROBE_VERSION,
  linuxProcfsFixturePassed: true,
  windowsWslFixturePassed: true,
  pidReuseFixturePassed: true,
  externalShimRejected: true,
  semanticFailureExitCodePassed: true,
  fixtures: 18,
}));

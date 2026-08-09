import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

import { runCapture } from './command.js';
import { sleep } from './port-cleanup.js';

export type ProcessIdentity = {
  pid: number;
  parentPid: number;
  startTime: string;
  commandLine: string;
  executablePath: string;
  workingDirectory: string;
};

export type ProcessController = {
  platform: NodeJS.Platform;
  inspect: (pid: number) => Promise<ProcessIdentity | undefined>;
  terminate: (
    expected: ProcessIdentity,
    options: { repoRoot: string; provenance: 'manifest' | 'repository' },
  ) => Promise<TerminateProcessResult>;
};

export type TerminateProcessResult = {
  pid: number;
  status: 'terminated' | 'not-running' | 'identity-changed' | 'refused' | 'failed';
  detail?: string;
};

type CaptureResult = { ok: boolean; out: string; err: string };

type LifecycleDependencies = {
  platform?: NodeJS.Platform;
  capture?: (cmd: string, args: string[]) => Promise<CaptureResult>;
  inspect?: (pid: number) => Promise<ProcessIdentity | undefined>;
  signal?: (pid: number, signal: NodeJS.Signals) => void;
  wait?: (ms: number) => Promise<void>;
  timeoutMs?: number;
};

function inspectablePid(pid: number): boolean {
  return Number.isSafeInteger(pid) && pid > 4;
}

function terminablePid(pid: number): boolean {
  return inspectablePid(pid) && pid !== process.pid;
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function numberValue(value: unknown): number {
  const parsed = typeof value === 'number' ? value : Number.parseInt(String(value ?? ''), 10);
  return Number.isSafeInteger(parsed) ? parsed : 0;
}

function parseIdentityJson(raw: string): ProcessIdentity | undefined {
  try {
    const value = JSON.parse(raw) as Record<string, unknown>;
    const pid = numberValue(value.pid);
    const startTime = stringValue(value.startTime);
    const commandLine = stringValue(value.commandLine);
    if (!inspectablePid(pid) || !startTime || !commandLine) return undefined;
    return {
      pid,
      parentPid: numberValue(value.parentPid),
      startTime,
      commandLine,
      executablePath: stringValue(value.executablePath),
      workingDirectory: stringValue(value.workingDirectory),
    };
  } catch {
    return undefined;
  }
}

async function inspectWindowsProcess(
  pid: number,
  capture: (cmd: string, args: string[]) => Promise<CaptureResult>,
): Promise<ProcessIdentity | undefined> {
  const script = [
    "$ErrorActionPreference = 'Stop'",
    `$item = Get-CimInstance Win32_Process -Filter 'ProcessId = ${pid}'`,
    'if ($null -eq $item) { exit 3 }',
    '$created = $item.CreationDate.ToUniversalTime().ToString(\'o\')',
    '[PSCustomObject]@{',
    '  pid = [int]$item.ProcessId',
    '  parentPid = [int]$item.ParentProcessId',
    '  startTime = $created',
    '  commandLine = [string]$item.CommandLine',
    '  executablePath = [string]$item.ExecutablePath',
    "  workingDirectory = ''",
    '} | ConvertTo-Json -Compress',
  ].join('\n');
  const result = await capture('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', script]);
  return result.ok ? parseIdentityJson(result.out) : undefined;
}

function readLinuxProcess(pid: number): ProcessIdentity | undefined {
  const procDir = `/proc/${pid}`;
  try {
    const stat = fs.readFileSync(path.posix.join(procDir, 'stat'), 'utf8');
    const commandEnd = stat.lastIndexOf(')');
    if (commandEnd < 0) return undefined;
    const fieldsAfterCommand = stat.slice(commandEnd + 2).trim().split(/\s+/);
    const parentPid = Number.parseInt(fieldsAfterCommand[1] ?? '', 10);
    const startTicks = fieldsAfterCommand[19] ?? '';
    const commandLine = fs
      .readFileSync(path.posix.join(procDir, 'cmdline'), 'utf8')
      .replace(/\0/g, ' ')
      .trim();
    if (!startTicks || !commandLine) return undefined;
    let executablePath = '';
    let workingDirectory = '';
    try {
      executablePath = fs.readlinkSync(path.posix.join(procDir, 'exe'));
    } catch {
      // Optional diagnostic field.
    }
    try {
      workingDirectory = fs.readlinkSync(path.posix.join(procDir, 'cwd'));
    } catch {
      // Optional diagnostic field.
    }
    return {
      pid,
      parentPid: Number.isSafeInteger(parentPid) ? parentPid : 0,
      startTime: `linux-ticks:${startTicks}`,
      commandLine,
      executablePath,
      workingDirectory,
    };
  } catch {
    return undefined;
  }
}

async function inspectPosixProcess(
  pid: number,
  capture: (cmd: string, args: string[]) => Promise<CaptureResult>,
): Promise<ProcessIdentity | undefined> {
  if (process.platform === 'linux' || fs.existsSync(`/proc/${pid}`)) {
    return readLinuxProcess(pid);
  }
  const result = await capture('ps', ['-p', String(pid), '-o', 'pid=', '-o', 'ppid=', '-o', 'lstart=', '-o', 'command=']);
  if (!result.ok || !result.out.trim()) return undefined;
  const match = result.out.trim().match(/^(\d+)\s+(\d+)\s+(.{24})\s+(.+)$/);
  if (!match) return undefined;
  return {
    pid: Number.parseInt(match[1] ?? '', 10),
    parentPid: Number.parseInt(match[2] ?? '', 10),
    startTime: `ps:${match[3]?.trim() ?? ''}`,
    commandLine: match[4]?.trim() ?? '',
    executablePath: '',
    workingDirectory: '',
  };
}

export async function inspectProcess(
  pid: number,
  options: { platform?: NodeJS.Platform; capture?: (cmd: string, args: string[]) => Promise<CaptureResult> } = {},
): Promise<ProcessIdentity | undefined> {
  if (!inspectablePid(pid)) return undefined;
  const platform = options.platform ?? process.platform;
  const capture = options.capture ?? runCapture;
  return platform === 'win32'
    ? inspectWindowsProcess(pid, capture)
    : inspectPosixProcess(pid, capture);
}

export function sameProcessIdentity(expected: ProcessIdentity, actual: ProcessIdentity | undefined): boolean {
  if (!actual) return false;
  return (
    expected.pid === actual.pid &&
    expected.startTime === actual.startTime &&
    expected.commandLine === actual.commandLine
  );
}

function normalizedPath(value: string, platform: NodeJS.Platform): string {
  const normalized = path.resolve(value).replaceAll('\\', '/').replace(/\/$/, '');
  return platform === 'win32' ? normalized.toLowerCase() : normalized;
}

function normalizedText(value: string, platform: NodeJS.Platform): string {
  const normalized = value.replaceAll('\\', '/');
  return platform === 'win32' ? normalized.toLowerCase() : normalized;
}

export function processBelongsToRepo(
  identity: ProcessIdentity,
  repoRoot: string,
  platform: NodeJS.Platform = process.platform,
): boolean {
  const root = normalizedPath(repoRoot, platform);
  const commandLine = normalizedText(identity.commandLine, platform);
  const executablePath = normalizedText(identity.executablePath, platform);
  const workingDirectory = identity.workingDirectory
    ? normalizedPath(identity.workingDirectory, platform)
    : '';
  const pathMatches =
    commandLine.includes(root) ||
    executablePath === root ||
    executablePath.startsWith(`${root}/`) ||
    workingDirectory === root ||
    workingDirectory.startsWith(`${root}/`);
  if (!pathMatches) return false;

  // A repository cwd alone is insufficient: only known local-dev entrypoints may be classified as TradeMind.
  const devSignature = /(?:scripts\/dev-(?:all|backend|admin|collector|opencli-bridge)\.(?:ts|js)|\bpnpm(?:\.cjs)?\b[^\r\n]*\bdev(?::(?:backend|admin|collector|opencli-bridge))?\b|\b(?:max|umi)\b[^\r\n]*\bdev\b|\btsx\b[^\r\n]*\bwatch\b[^\r\n]*src\/(?:opencli-bridge\/)?index\.ts|\bgo\b[^\r\n]*\brun\b[^\r\n]*cmd\/server)/i;
  return devSignature.test(commandLine);
}

async function waitUntilIdentityGone(
  expected: ProcessIdentity,
  inspect: (pid: number) => Promise<ProcessIdentity | undefined>,
  wait: (ms: number) => Promise<void>,
  timeoutMs: number,
): Promise<boolean> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const actual = await inspect(expected.pid);
    if (!sameProcessIdentity(expected, actual)) return true;
    await wait(100);
  }
  return !sameProcessIdentity(expected, await inspect(expected.pid));
}

export async function terminateProcessTree(
  expected: ProcessIdentity,
  options: { repoRoot: string; provenance: 'manifest' | 'repository' },
  dependencies: LifecycleDependencies = {},
): Promise<TerminateProcessResult> {
  if (!terminablePid(expected.pid)) {
    return { pid: expected.pid, status: 'refused', detail: 'invalid or protected PID' };
  }
  const platform = dependencies.platform ?? process.platform;
  const capture = dependencies.capture ?? runCapture;
  const inspect = dependencies.inspect ?? ((pid) => inspectProcess(pid, { platform, capture }));
  const wait = dependencies.wait ?? sleep;
  const timeoutMs = dependencies.timeoutMs ?? 5_000;

  const actual = await inspect(expected.pid);
  if (!actual) return { pid: expected.pid, status: 'not-running' };
  if (!sameProcessIdentity(expected, actual)) {
    return { pid: expected.pid, status: 'identity-changed', detail: 'PID was reused or command line changed' };
  }
  if (
    options.provenance === 'repository' &&
    !processBelongsToRepo(actual, options.repoRoot, platform)
  ) {
    return { pid: expected.pid, status: 'refused', detail: 'process is not owned by this repository' };
  }

  // Re-read immediately before signalling so PID reuse cannot cross the validation/kill boundary unnoticed.
  const beforeSignal = await inspect(expected.pid);
  if (!sameProcessIdentity(expected, beforeSignal)) {
    return { pid: expected.pid, status: 'identity-changed', detail: 'identity changed before termination' };
  }

  if (platform === 'win32') {
    const result = await capture('taskkill', ['/PID', String(expected.pid), '/T', '/F']);
    if (!result.ok && sameProcessIdentity(expected, await inspect(expected.pid))) {
      return { pid: expected.pid, status: 'failed', detail: result.err || result.out || 'taskkill failed' };
    }
  } else {
    const signal = dependencies.signal ?? ((pid, name) => process.kill(pid, name));
    try {
      signal(-expected.pid, 'SIGTERM');
    } catch {
      try {
        signal(expected.pid, 'SIGTERM');
      } catch {
        // The verification below decides whether the process is already gone.
      }
    }
    if (!(await waitUntilIdentityGone(expected, inspect, wait, timeoutMs))) {
      try {
        signal(-expected.pid, 'SIGKILL');
      } catch {
        try {
          signal(expected.pid, 'SIGKILL');
        } catch {
          // The final verification reports a clear failure.
        }
      }
    }
  }

  const gone = await waitUntilIdentityGone(expected, inspect, wait, timeoutMs);
  return gone
    ? { pid: expected.pid, status: 'terminated' }
    : { pid: expected.pid, status: 'failed', detail: 'process tree still exists after termination timeout' };
}

export async function waitForProcessIdentity(
  pid: number,
  controller: Pick<ProcessController, 'inspect'>,
  options: { attempts?: number; delayMs?: number } = {},
): Promise<ProcessIdentity | undefined> {
  const attempts = options.attempts ?? 20;
  const delayMs = options.delayMs ?? 50;
  for (let index = 0; index < attempts; index += 1) {
    const identity = await controller.inspect(pid);
    if (identity) return identity;
    if (index + 1 < attempts) await sleep(delayMs);
  }
  return undefined;
}

export const systemProcessController: ProcessController = {
  platform: process.platform,
  inspect: (pid) => inspectProcess(pid),
  terminate: (identity, options) => terminateProcessTree(identity, options),
};

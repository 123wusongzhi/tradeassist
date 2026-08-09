import { describe, expect, it, vi } from 'vitest';

import {
  inspectProcess,
  processBelongsToRepo,
  terminateProcessTree,
  type ProcessIdentity,
} from '../process-lifecycle.js';

function identity(overrides: Partial<ProcessIdentity> = {}): ProcessIdentity {
  return {
    pid: 101,
    parentPid: 50,
    startTime: '2026-08-09T00:00:00.000Z',
    commandLine: 'node C:/repo/trademind/scripts/dev-all.ts',
    executablePath: 'C:/Program Files/nodejs/node.exe',
    workingDirectory: 'C:/repo/trademind',
    ...overrides,
  };
}

describe('process identity safety', () => {
  it('reads the current host process identity without exposing it in test output', async () => {
    const actual = await inspectProcess(process.pid);
    expect(actual?.pid).toBe(process.pid);
    expect(actual?.startTime).toBeTruthy();
    expect(actual?.commandLine).toBeTruthy();
  });

  it('recognizes repository ownership from command line or working directory', () => {
    expect(processBelongsToRepo(identity(), 'C:/repo/trademind', 'win32')).toBe(true);
    expect(
      processBelongsToRepo(
        identity({ commandLine: 'python unrelated.py', workingDirectory: 'C:/other' }),
        'C:/repo/trademind',
        'win32',
      ),
    ).toBe(false);
  });

  it('refuses to terminate a reused PID before invoking taskkill', async () => {
    const expected = identity();
    const reused = identity({
      startTime: '2026-08-09T01:00:00.000Z',
      commandLine: 'python unrelated.py',
      workingDirectory: 'C:/other',
    });
    const capture = vi.fn(async () => ({ ok: true, out: '', err: '' }));

    const result = await terminateProcessTree(
      expected,
      { repoRoot: 'C:/repo/trademind', provenance: 'manifest' },
      { platform: 'win32', inspect: async () => reused, capture },
    );

    expect(result.status).toBe('identity-changed');
    expect(capture).not.toHaveBeenCalled();
  });

  it('refuses repository-provenance termination when the process is unrelated', async () => {
    const unrelated = identity({ commandLine: 'python unrelated.py', workingDirectory: 'C:/other' });
    const capture = vi.fn(async () => ({ ok: true, out: '', err: '' }));

    const result = await terminateProcessTree(
      unrelated,
      { repoRoot: 'C:/repo/trademind', provenance: 'repository' },
      { platform: 'win32', inspect: async () => unrelated, capture },
    );

    expect(result.status).toBe('refused');
    expect(capture).not.toHaveBeenCalled();
  });

  it('terminates a validated Windows process tree with taskkill /T /F', async () => {
    const expected = identity();
    let running = true;
    const capture = vi.fn(async (command: string) => {
      if (command === 'taskkill') running = false;
      return { ok: true, out: '', err: '' };
    });

    const result = await terminateProcessTree(
      expected,
      { repoRoot: 'C:/repo/trademind', provenance: 'manifest' },
      {
        platform: 'win32',
        inspect: async () => (running ? expected : undefined),
        capture,
        wait: async () => undefined,
      },
    );

    expect(result.status).toBe('terminated');
    expect(capture).toHaveBeenCalledWith('taskkill', ['/PID', '101', '/T', '/F']);
  });

  it('uses a POSIX process group and falls back to SIGKILL after the graceful timeout', async () => {
    const expected = identity();
    let running = true;
    const signal = vi.fn((pid: number, name: NodeJS.Signals) => {
      if (name === 'SIGKILL') running = false;
    });

    const result = await terminateProcessTree(
      expected,
      { repoRoot: '/repo/trademind', provenance: 'manifest' },
      {
        platform: 'linux',
        inspect: async () => (running ? expected : undefined),
        signal,
        wait: async () => undefined,
        timeoutMs: 0,
      },
    );

    expect(result.status).toBe('terminated');
    expect(signal.mock.calls).toEqual([
      [-101, 'SIGTERM'],
      [-101, 'SIGKILL'],
    ]);
  });
});

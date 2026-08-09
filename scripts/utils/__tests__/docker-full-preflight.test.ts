import { describe, expect, it, vi } from 'vitest';

import { checkDockerFullPortConflicts } from '../docker-full-preflight.js';
import type { ProcessController, ProcessIdentity } from '../process-lifecycle.js';

function owner(pid: number, commandLine: string, workingDirectory: string): ProcessIdentity {
  return {
    pid,
    parentPid: 10,
    startTime: `start-${pid}`,
    commandLine,
    executablePath: 'C:/Program Files/nodejs/node.exe',
    workingDirectory,
  };
}

describe('Docker full-stack port preflight', () => {
  it('distinguishes TradeMind local listeners from unrelated listeners without killing either', async () => {
    const local = owner(401, 'pnpm run dev:admin C:/repo/trademind', 'C:/repo/trademind');
    const unrelated = owner(402, 'python C:/other/api.py', 'C:/other');
    const terminate = vi.fn();
    const controller: ProcessController = {
      platform: 'win32',
      inspect: async (pid) => (pid === 401 ? local : pid === 402 ? unrelated : undefined),
      terminate,
    };

    const result = await checkDockerFullPortConflicts('C:/repo/trademind', undefined, {
      controller,
      runningServices: async () => [],
      listPids: async (port) => (port === 8000 ? [401] : port === 8080 ? [402] : []),
    });

    expect(result.map(({ service, relation }) => ({ service, relation }))).toEqual([
      { service: 'admin', relation: 'repository' },
      { service: 'backend', relation: 'other' },
    ]);
    expect(terminate).not.toHaveBeenCalled();
  });
});

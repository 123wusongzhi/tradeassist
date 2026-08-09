import { EventEmitter } from 'node:events';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  prepareLocalDevPorts,
  writeRuntimeManifest,
  type DevRuntimeManifest,
} from '../dev-runtime.js';
import { createShutdownCoordinator, shutdownManagedDevRun } from '../dev-supervisor.js';
import type { ProcessController, ProcessIdentity } from '../process-lifecycle.js';

const tempDirs: string[] = [];

afterEach(() => {
  for (const dir of tempDirs.splice(0)) fs.rmSync(dir, { recursive: true, force: true });
});

function makeIdentity(pid: number, tag: string, parentPid = 10): ProcessIdentity {
  return {
    pid,
    parentPid,
    startTime: `start-${pid}`,
    commandLine: `node C:/repo/trademind/${tag}.js`,
    executablePath: 'C:/Program Files/nodejs/node.exe',
    workingDirectory: 'C:/repo/trademind',
  };
}

function manifestPath(): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trademind-dev-runtime-test-'));
  tempDirs.push(dir);
  return path.join(dir, 'runtime.json');
}

describe('development runtime cleanup', () => {
  it('keeps core services running when the optional OpenCLI bridge exits', async () => {
    const messages: string[] = [];
    const optional = new EventEmitter();
    const backend = new EventEmitter();
    const coordinator = createShutdownCoordinator((message) => messages.push(message));
    coordinator.watch({ tag: 'opencli-bridge', required: false, on: optional.on.bind(optional) });
    coordinator.watch({ tag: 'backend', required: true, on: backend.on.bind(backend) });

    optional.emit('exit', 1, null);

    expect(coordinator.isRequested()).toBe(false);
    expect(messages).toEqual([
      'Optional process "opencli-bridge" exited (code=1); core services remain running.',
    ]);

    backend.emit('exit', 1, null);
    await expect(coordinator.wait).resolves.toMatchObject({ exitCode: 1 });
  });

  it('cleans backend, admin, collector, and optional OpenCLI trees after a required failure', async () => {
    const repoRoot = 'C:/repo/trademind';
    const filePath = manifestPath();
    const identities = new Map<number, ProcessIdentity>([
      [201, makeIdentity(201, 'launcher')],
      [211, makeIdentity(211, 'backend', 201)],
      [212, makeIdentity(212, 'admin', 201)],
      [213, makeIdentity(213, 'collector', 201)],
      [214, makeIdentity(214, 'opencli', 201)],
    ]);
    const ports = new Map<number, number>([
      [8080, 211],
      [8000, 212],
      [3001, 213],
      [3100, 214],
    ]);
    const terminate = vi.fn(async (expected: ProcessIdentity) => {
      identities.delete(expected.pid);
      for (const [port, pid] of ports) if (pid === expected.pid) ports.delete(port);
      return { pid: expected.pid, status: 'terminated' as const };
    });
    const controller: ProcessController = {
      platform: 'win32',
      inspect: async (pid) => identities.get(pid),
      terminate,
    };
    const runtime: DevRuntimeManifest = {
      version: 1,
      repoRoot,
      createdAt: '2026-08-09T00:00:00.000Z',
      launcher: identities.get(201)!,
      services: [
        { tag: 'backend', required: true, ports: [8080], identity: identities.get(211)! },
        { tag: 'admin', required: true, ports: [8000], identity: identities.get(212)! },
        { tag: 'collector', required: true, ports: [3001], identity: identities.get(213)! },
        { tag: 'opencli-bridge', required: false, ports: [3100], identity: identities.get(214)! },
      ],
    };
    writeRuntimeManifest(runtime, { manifestPath: filePath });

    const backend = new EventEmitter();
    const coordinator = createShutdownCoordinator();
    coordinator.watch({ tag: 'backend', required: true, on: backend.on.bind(backend) });
    backend.emit('exit', 1, null);
    const request = await coordinator.wait;
    const result = await shutdownManagedDevRun(repoRoot, runtime, [...ports.keys()], {
      controller,
      manifestPath: filePath,
      listPids: async (port) => (ports.has(port) ? [ports.get(port)!] : []),
    });

    expect(request.exitCode).toBe(1);
    expect(result.ok).toBe(true);
    expect(terminate.mock.calls.map(([item]) => (item as ProcessIdentity).pid).sort()).toEqual([
      211, 212, 213, 214,
    ]);
    expect(ports.size).toBe(0);
    expect(fs.existsSync(filePath)).toBe(false);
  });

  it('reports an unrelated port owner and never terminates it', async () => {
    const unrelated = makeIdentity(301, 'unrelated');
    unrelated.commandLine = 'python C:/other/server.py';
    unrelated.workingDirectory = 'C:/other';
    const terminate = vi.fn();
    const controller: ProcessController = {
      platform: 'win32',
      inspect: async (pid) => (pid === unrelated.pid ? unrelated : undefined),
      terminate,
    };

    const result = await prepareLocalDevPorts('C:/repo/trademind', [8000], {
      controller,
      manifestPath: manifestPath(),
      listPids: async () => [unrelated.pid],
    });

    expect(result.ok).toBe(false);
    expect(result.conflicts).toHaveLength(1);
    expect(result.conflicts[0]?.relation).toBe('other');
    expect(terminate).not.toHaveBeenCalled();
  });

  it('recognizes a Windows listener through a repository-owned parent when cwd is unavailable', async () => {
    const repoRoot = 'C:/repo/trademind';
    const launcher = makeIdentity(310, 'scripts/dev-all.ts');
    launcher.commandLine = 'node C:/repo/trademind/node_modules/tsx/dist/cli.mjs scripts/dev-all.ts';
    launcher.workingDirectory = '';
    const listener = makeIdentity(311, 'compiled-server', launcher.pid);
    listener.commandLine = 'C:/Users/test/AppData/Local/Temp/go-build/server.exe';
    listener.executablePath = listener.commandLine;
    listener.workingDirectory = '';
    const identities = new Map<number, ProcessIdentity>([
      [launcher.pid, launcher],
      [listener.pid, listener],
    ]);
    const terminate = vi.fn(async (expected: ProcessIdentity) => {
      identities.delete(expected.pid);
      return { pid: expected.pid, status: 'terminated' as const };
    });
    const controller: ProcessController = {
      platform: 'win32',
      inspect: async (pid) => identities.get(pid),
      terminate,
    };

    const result = await prepareLocalDevPorts(repoRoot, [8080], {
      controller,
      manifestPath: manifestPath(),
      listPids: async () => [listener.pid],
    });

    expect(result.ok).toBe(true);
    expect(result.conflicts).toEqual([]);
    expect(terminate).toHaveBeenCalledWith(listener, {
      repoRoot,
      provenance: 'repository',
    });
  });
});

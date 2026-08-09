import { spawn, type ChildProcess } from 'node:child_process';
import process from 'node:process';

import { describe, expect, it } from 'vitest';

import {
  inspectProcess,
  systemProcessController,
  terminateProcessTree,
  waitForProcessIdentity,
} from '../process-lifecycle.js';
import { repoRoot } from '../paths.js';

function firstOutputLine(child: ChildProcess): Promise<string> {
  return new Promise((resolve, reject) => {
    let output = '';
    const timer = setTimeout(() => reject(new Error('controlled child did not report its descendant PID')), 5_000);
    child.once('error', (error) => {
      clearTimeout(timer);
      reject(error);
    });
    child.stdout?.on('data', (chunk: Buffer | string) => {
      output += chunk.toString();
      const newline = output.indexOf('\n');
      if (newline < 0) return;
      clearTimeout(timer);
      resolve(output.slice(0, newline).trim());
    });
  });
}

async function waitUntilGone(pid: number): Promise<boolean> {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    if (!(await inspectProcess(pid))) return true;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  return !(await inspectProcess(pid));
}

describe('controlled host process-tree lifecycle', () => {
  it(
    'terminates only the spawned parent and descendant after identity verification',
    async () => {
      const parentScript = [
        "const { spawn } = require('node:child_process');",
        "const child = spawn(process.execPath, ['-e', 'setInterval(() => {}, 1000)'], { stdio: 'ignore' });",
        "process.stdout.write(String(child.pid) + '\\n');",
        'setInterval(() => {}, 1000);',
      ].join('\n');
      const parent = spawn(process.execPath, ['-e', parentScript], {
        cwd: repoRoot,
        detached: process.platform !== 'win32',
        windowsHide: true,
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      const parentPid = parent.pid;
      if (!parentPid) throw new Error('controlled parent process did not start');

      let parentIdentity = await waitForProcessIdentity(parentPid, systemProcessController, {
        attempts: 50,
        delayMs: 100,
      });
      let descendantPid = 0;
      try {
        descendantPid = Number.parseInt(await firstOutputLine(parent), 10);
        expect(Number.isSafeInteger(descendantPid) && descendantPid > 4).toBe(true);
        expect(await waitForProcessIdentity(descendantPid, systemProcessController)).toBeDefined();
        expect(parentIdentity).toBeDefined();
        if (!parentIdentity) throw new Error('controlled parent identity was unavailable');

        const result = await terminateProcessTree(parentIdentity, {
          repoRoot,
          provenance: 'manifest',
        });

        expect(result.status).toBe('terminated');
        expect(await waitUntilGone(parentPid)).toBe(true);
        expect(await waitUntilGone(descendantPid)).toBe(true);
        parentIdentity = undefined;
      } finally {
        if (parentIdentity) {
          await terminateProcessTree(parentIdentity, { repoRoot, provenance: 'manifest' });
        }
        if (descendantPid > 4 && (await inspectProcess(descendantPid))) {
          try {
            process.kill(descendantPid, 'SIGKILL');
          } catch {
            // The controlled descendant already exited between inspection and cleanup.
          }
        }
      }
    },
    20_000,
  );
});

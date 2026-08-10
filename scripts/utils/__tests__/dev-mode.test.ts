import { describe, expect, it, vi } from 'vitest';

import {
  checkLocalDevMode,
  findDockerOnlyEnvIssues,
  runningFullStackCoreServices,
} from '../dev-mode.js';

describe('local development mode guard', () => {
  it('detects Docker-only service addresses without returning their original values', () => {
    const issues = findDockerOnlyEnvIssues({
      DB_HOST: 'postgres',
      REDIS_ADDR: 'redis:6379',
      COLLECTOR_PLAYWRIGHT_BASE_URL: 'http://collector:3001',
      COLLECTOR_MAIN_SERVICE_URL: 'http://backend:8080',
      DB_PASSWORD: 'must-not-appear',
    });

    expect(issues.map((issue) => issue.key)).toEqual([
      'DB_HOST',
      'REDIS_ADDR',
      'COLLECTOR_PLAYWRIGHT_BASE_URL',
      'COLLECTOR_MAIN_SERVICE_URL',
    ]);
    expect(JSON.stringify(issues)).not.toContain('must-not-appear');
  });

  it('detects a running trademind-full stack using read-only Docker commands', async () => {
    const commands: string[][] = [];
    const capture = vi.fn(async (command: string, args: string[]) => {
      commands.push([command, ...args]);
      return { ok: true, out: 'admin\nbackend\ncollector', err: '' };
    });

    const services = await runningFullStackCoreServices('C:/repo/trademind', capture);

    expect(services).toEqual(['admin', 'backend', 'collector']);
    expect(commands).toHaveLength(1);
    expect(commands[0]).toContain('ps');
    expect(commands.flat().join(' ')).not.toMatch(/\b(stop|down|kill|rm)\b/);
  });

  it('fails the local-mode check while leaving Docker untouched', async () => {
    const capture = vi.fn(async () => ({ ok: true, out: 'backend\nadmin', err: '' }));
    const result = await checkLocalDevMode('C:/repo/trademind', undefined, capture);

    expect(result.ok).toBe(false);
    expect(result.runningFullStackServices).toEqual(['backend', 'admin']);
    expect(capture).toHaveBeenCalledTimes(1);
  });
});

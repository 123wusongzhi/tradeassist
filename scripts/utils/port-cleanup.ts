import process from 'node:process';

import { runCapture } from './command.js';
import { readEnvFile } from './env-file.js';

export type CaptureCommand = (
  cmd: string,
  args: string[],
) => Promise<{ ok: boolean; out: string; err: string }>;

export type DevServicePortMap = {
  backend: number;
  admin: number;
  collector: number;
  openCliBridge?: number;
};

export type DockerPublishPortMap = {
  backend: number;
  admin: number;
  collector: number;
};

export function parsePortFromAddr(addr: string | undefined, defaultPort: number): number {
  if (!addr?.trim()) return defaultPort;
  const a = addr.trim();
  if (a.startsWith(':')) {
    const p = Number.parseInt(a.slice(1), 10);
    return validPort(p) ? p : defaultPort;
  }
  const lastColon = a.lastIndexOf(':');
  if (lastColon > 0) {
    const p = Number.parseInt(a.slice(lastColon + 1), 10);
    if (validPort(p)) return p;
  }
  if (/^\d+$/.test(a)) {
    const p = Number.parseInt(a, 10);
    return validPort(p) ? p : defaultPort;
  }
  return defaultPort;
}

export function parsePort(value: string | undefined, defaultPort: number): number {
  if (!value?.trim()) return defaultPort;
  const parsed = Number.parseInt(value.trim(), 10);
  return validPort(parsed) ? parsed : defaultPort;
}

function validPort(port: number): boolean {
  return Number.isSafeInteger(port) && port > 0 && port < 65_536;
}

function lineHasListeningPort(line: string, port: number): boolean {
  if (!/LISTENING/i.test(line)) return false;
  return new RegExp(`:${port}(\\s|$)`).test(line);
}

/** Read-only listener discovery. This function never terminates a process. */
export async function listListeningPids(
  port: number,
  options: { platform?: NodeJS.Platform; capture?: CaptureCommand } = {},
): Promise<number[]> {
  if (!validPort(port)) return [];
  const platform = options.platform ?? process.platform;
  const capture = options.capture ?? runCapture;

  if (platform === 'win32') {
    const result = await capture('netstat', ['-ano', '-p', 'tcp']);
    if (!result.ok) return [];
    const pids = new Set<number>();
    for (const line of result.out.split(/\r?\n/)) {
      if (!lineHasListeningPort(line, port)) continue;
      const parts = line.trim().split(/\s+/);
      const pid = Number.parseInt(parts[parts.length - 1] ?? '', 10);
      if (Number.isSafeInteger(pid) && pid > 4 && pid !== process.pid) pids.add(pid);
    }
    return [...pids];
  }

  const result = await capture('lsof', ['-nP', `-iTCP:${port}`, '-sTCP:LISTEN', '-t']);
  if (!result.ok || !result.out) return [];
  return [
    ...new Set(
      result.out
        .split(/\r?\n/)
        .map((value) => Number.parseInt(value.trim(), 10))
        .filter((pid) => Number.isSafeInteger(pid) && pid > 0 && pid !== process.pid),
    ),
  ];
}

export function resolveDevServicePortMap(
  envPath: string | undefined,
  overrides: NodeJS.ProcessEnv = process.env,
): DevServicePortMap {
  const fileEnv = readEnvFile(envPath);
  const value = (key: string): string | undefined => overrides[key] ?? fileEnv[key];
  const openCliEnabled = ['1', 'true', 'yes', 'on'].includes(
    (value('OPENCLI_BRIDGE_ENABLED') ?? '').trim().toLowerCase(),
  );
  const ports: DevServicePortMap = {
    backend: parsePortFromAddr(value('APP_HTTP_ADDR'), 8080),
    admin: parsePort(value('ADMIN_DEV_PORT'), 8000),
    collector: parsePortFromAddr(value('COLLECTOR_HTTP_ADDR'), 3001),
  };
  if (openCliEnabled) {
    ports.openCliBridge = parsePortFromAddr(value('OPENCLI_BRIDGE_HTTP_ADDR'), 3100);
  }
  return ports;
}

/** Exact local service ports; Umi is pinned to ADMIN_DEV_PORT instead of scanning 8000-8010. */
export function resolveDevServicePorts(
  envPath: string | undefined,
  overrides: NodeJS.ProcessEnv = process.env,
): number[] {
  return [...new Set(Object.values(resolveDevServicePortMap(envPath, overrides)))];
}

export function resolveDockerPublishPortMap(
  envPath: string | undefined,
  overrides: NodeJS.ProcessEnv = process.env,
): DockerPublishPortMap {
  const fileEnv = readEnvFile(envPath);
  const value = (key: string): string | undefined => overrides[key] ?? fileEnv[key];
  return {
    backend: parsePort(value('BACKEND_PUBLISH_PORT'), 8080),
    admin: parsePort(value('ADMIN_PUBLISH_PORT'), 8000),
    collector: parsePort(value('COLLECTOR_PUBLISH_PORT'), 3001),
  };
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

import path from 'node:path';

import { runCapture } from './command.js';
import { readEnvFile } from './env-file.js';

export const FULL_STACK_CORE_SERVICES = ['admin', 'backend', 'collector'] as const;

export type DockerOnlyEnvIssue = {
  key: string;
  safeLocalValue: string;
};

type CaptureCommand = typeof runCapture;

function hostFromAddress(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return '';
  if (!trimmed.includes('://')) {
    return (trimmed.split('/')[0]?.split(':')[0] ?? '').toLowerCase();
  }
  try {
    return new URL(trimmed).hostname.toLowerCase();
  } catch {
    const withoutScheme = trimmed.replace(/^[a-z][a-z\d+.-]*:\/\//i, '');
    const host = withoutScheme.split('/')[0]?.split(':')[0] ?? '';
    return host.toLowerCase();
  }
}

export function findDockerOnlyEnvIssues(values: Record<string, string>): DockerOnlyEnvIssue[] {
  const issues: DockerOnlyEnvIssue[] = [];
  if ((values.DB_HOST ?? '').trim().toLowerCase() === 'postgres') {
    issues.push({ key: 'DB_HOST', safeLocalValue: '127.0.0.1' });
  }
  if (hostFromAddress(values.REDIS_ADDR ?? '') === 'redis') {
    issues.push({ key: 'REDIS_ADDR', safeLocalValue: '127.0.0.1:6379' });
  }
  for (const key of ['COLLECTOR_PLAYWRIGHT_BASE_URL', 'COLLECTOR_BASE_URL']) {
    if (hostFromAddress(values[key] ?? '') === 'collector') {
      issues.push({ key, safeLocalValue: 'http://127.0.0.1:3001' });
    }
  }
  if (hostFromAddress(values.COLLECTOR_MAIN_SERVICE_URL ?? '') === 'backend') {
    issues.push({ key: 'COLLECTOR_MAIN_SERVICE_URL', safeLocalValue: 'http://127.0.0.1:8080' });
  }
  return issues;
}

export function findDockerOnlyEnvIssuesInFile(envPath: string | undefined): DockerOnlyEnvIssue[] {
  return findDockerOnlyEnvIssues(readEnvFile(envPath));
}

function parseServices(output: string): string[] {
  const allowed = new Set<string>(FULL_STACK_CORE_SERVICES);
  return [
    ...new Set(
      output
        .split(/\r?\n/)
        .map((line) => line.trim())
        .filter((line) => allowed.has(line)),
    ),
  ];
}

/** Read-only full-stack detection. It never invokes docker stop/down/kill. */
export async function runningFullStackCoreServices(
  repoRoot: string,
  capture: CaptureCommand = runCapture,
): Promise<string[]> {
  const composeFile = path.join(repoRoot, 'docker-compose.full.yml');
  const compose = await capture('docker', [
    'compose',
    '-f',
    composeFile,
    'ps',
    '--services',
    '--status',
    'running',
  ]);
  if (compose.ok) return parseServices(compose.out);

  // Compatibility fallback for older Compose versions; still read-only and project-label scoped.
  const dockerPs = await capture('docker', [
    'ps',
    '--filter',
    'label=com.docker.compose.project=trademind-full',
    '--format',
    '{{.Label "com.docker.compose.service"}}',
  ]);
  return dockerPs.ok ? parseServices(dockerPs.out) : [];
}

export type LocalDevModeCheck = {
  ok: boolean;
  runningFullStackServices: string[];
  dockerOnlyEnvIssues: DockerOnlyEnvIssue[];
};

export async function checkLocalDevMode(
  repoRoot: string,
  envPath: string | undefined,
  capture: CaptureCommand = runCapture,
): Promise<LocalDevModeCheck> {
  const [runningFullStackServices, dockerOnlyEnvIssues] = await Promise.all([
    runningFullStackCoreServices(repoRoot, capture),
    Promise.resolve(findDockerOnlyEnvIssuesInFile(envPath)),
  ]);
  return {
    ok: runningFullStackServices.length === 0 && dockerOnlyEnvIssues.length === 0,
    runningFullStackServices,
    dockerOnlyEnvIssues,
  };
}

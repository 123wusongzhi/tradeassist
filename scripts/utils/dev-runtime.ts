import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import process from 'node:process';

import { listListeningPids, sleep } from './port-cleanup.js';
import {
  processBelongsToRepo,
  sameProcessIdentity,
  systemProcessController,
  waitForProcessIdentity,
  type ProcessController,
  type ProcessIdentity,
  type TerminateProcessResult,
} from './process-lifecycle.js';

const RUNTIME_MANIFEST_VERSION = 1;

export type ManagedServiceRecord = {
  tag: string;
  required: boolean;
  ports: number[];
  identity: ProcessIdentity;
};

export type DevRuntimeManifest = {
  version: typeof RUNTIME_MANIFEST_VERSION;
  repoRoot: string;
  createdAt: string;
  launcher: ProcessIdentity;
  services: ManagedServiceRecord[];
};

export type PortOwnerRelation = 'manifest' | 'repository' | 'other';

export type PortOwner = {
  port: number;
  identity: ProcessIdentity;
  relation: PortOwnerRelation;
};

export type RuntimeDependencies = {
  controller?: ProcessController;
  listPids?: (port: number) => Promise<number[]>;
  manifestPath?: string;
};

export type RuntimeCleanupResult = {
  ok: boolean;
  terminated: TerminateProcessResult[];
  lingering: PortOwner[];
  manifestFound: boolean;
};

function normalizedRepoRoot(repoRoot: string): string {
  const resolved = path.resolve(repoRoot).replace(/[\\/]+$/, '');
  return process.platform === 'win32' ? resolved.toLowerCase() : resolved;
}

export function runtimeManifestPath(repoRoot: string): string {
  const repoKey = crypto.createHash('sha256').update(normalizedRepoRoot(repoRoot)).digest('hex').slice(0, 16);
  return path.join(os.tmpdir(), 'trademind-dev-runtime', `${repoKey}.json`);
}

function manifestPathFor(repoRoot: string, dependencies: RuntimeDependencies): string {
  return dependencies.manifestPath ?? runtimeManifestPath(repoRoot);
}

function isProcessIdentity(value: unknown): value is ProcessIdentity {
  if (!value || typeof value !== 'object') return false;
  const record = value as Record<string, unknown>;
  return (
    Number.isSafeInteger(record.pid) &&
    typeof record.parentPid === 'number' &&
    typeof record.startTime === 'string' &&
    Boolean(record.startTime) &&
    typeof record.commandLine === 'string' &&
    Boolean(record.commandLine) &&
    typeof record.executablePath === 'string' &&
    typeof record.workingDirectory === 'string'
  );
}

function isManifest(value: unknown, repoRoot: string): value is DevRuntimeManifest {
  if (!value || typeof value !== 'object') return false;
  const record = value as Record<string, unknown>;
  if (
    record.version !== RUNTIME_MANIFEST_VERSION ||
    typeof record.repoRoot !== 'string' ||
    normalizedRepoRoot(record.repoRoot) !== normalizedRepoRoot(repoRoot) ||
    typeof record.createdAt !== 'string' ||
    !isProcessIdentity(record.launcher) ||
    !Array.isArray(record.services)
  ) {
    return false;
  }
  return record.services.every((service) => {
    if (!service || typeof service !== 'object') return false;
    const item = service as Record<string, unknown>;
    return (
      typeof item.tag === 'string' &&
      typeof item.required === 'boolean' &&
      Array.isArray(item.ports) &&
      item.ports.every((port) => Number.isSafeInteger(port) && port > 0 && port < 65_536) &&
      isProcessIdentity(item.identity)
    );
  });
}

export function readRuntimeManifest(
  repoRoot: string,
  dependencies: RuntimeDependencies = {},
): DevRuntimeManifest | undefined {
  const filePath = manifestPathFor(repoRoot, dependencies);
  if (!fs.existsSync(filePath)) return undefined;
  try {
    const value = JSON.parse(fs.readFileSync(filePath, 'utf8')) as unknown;
    return isManifest(value, repoRoot) ? value : undefined;
  } catch {
    return undefined;
  }
}

export function writeRuntimeManifest(
  manifest: DevRuntimeManifest,
  dependencies: RuntimeDependencies = {},
): void {
  const filePath = manifestPathFor(manifest.repoRoot, dependencies);
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  const tempPath = `${filePath}.${process.pid}.tmp`;
  fs.writeFileSync(tempPath, `${JSON.stringify(manifest, null, 2)}\n`, { encoding: 'utf8', mode: 0o600 });
  fs.renameSync(tempPath, filePath);
}

export function removeRuntimeManifest(repoRoot: string, dependencies: RuntimeDependencies = {}): void {
  const filePath = manifestPathFor(repoRoot, dependencies);
  try {
    fs.unlinkSync(filePath);
  } catch (error: unknown) {
    const code = error && typeof error === 'object' && 'code' in error ? String(error.code) : '';
    if (code !== 'ENOENT') throw error;
  }
}

export async function createRuntimeManifest(
  repoRoot: string,
  dependencies: RuntimeDependencies = {},
): Promise<DevRuntimeManifest> {
  const controller = dependencies.controller ?? systemProcessController;
  const launcher = await waitForProcessIdentity(process.pid, controller);
  if (!launcher) {
    throw new Error('无法读取本地开发启动器的进程身份；为避免留下不可验证的进程，已中止启动。');
  }
  return {
    version: RUNTIME_MANIFEST_VERSION,
    repoRoot: path.resolve(repoRoot),
    createdAt: new Date().toISOString(),
    launcher,
    services: [],
  };
}

export async function addServiceToManifest(
  manifest: DevRuntimeManifest,
  service: { tag: string; required: boolean; ports: number[]; pid: number },
  dependencies: RuntimeDependencies = {},
): Promise<ManagedServiceRecord> {
  const controller = dependencies.controller ?? systemProcessController;
  const identity = await waitForProcessIdentity(service.pid, controller);
  if (!identity) {
    throw new Error(`无法记录 ${service.tag} 的 PID ${service.pid} 进程身份。`);
  }
  const record = {
    tag: service.tag,
    required: service.required,
    ports: [...new Set(service.ports)],
    identity,
  };
  manifest.services.push(record);
  writeRuntimeManifest(manifest, dependencies);
  return record;
}

async function isDescendantOfManifest(
  identity: ProcessIdentity,
  manifest: DevRuntimeManifest | undefined,
  controller: ProcessController,
): Promise<boolean> {
  if (!manifest) return false;
  const roots = [manifest.launcher, ...manifest.services.map((service) => service.identity)];
  if (roots.some((root) => sameProcessIdentity(root, identity))) return true;

  let parentPid = identity.parentPid;
  const visited = new Set<number>([identity.pid]);
  for (let depth = 0; depth < 32 && parentPid > 4 && !visited.has(parentPid); depth += 1) {
    visited.add(parentPid);
    const parent = await controller.inspect(parentPid);
    if (!parent) return false;
    if (roots.some((root) => sameProcessIdentity(root, parent))) return true;
    parentPid = parent.parentPid;
  }
  return false;
}

async function belongsToRepositoryProcessTree(
  identity: ProcessIdentity,
  repoRoot: string,
  controller: ProcessController,
): Promise<boolean> {
  if (processBelongsToRepo(identity, repoRoot, controller.platform)) return true;

  // Win32_Process does not expose cwd. A listener such as a compiled `go run`
  // child can therefore look unrelated even though a verified pnpm/tsx parent
  // command contains the repository path. Walk the live ancestry without ever
  // weakening the repository-path + known-dev-entrypoint signature.
  let parentPid = identity.parentPid;
  const visited = new Set<number>([identity.pid]);
  for (let depth = 0; depth < 32 && parentPid > 4 && !visited.has(parentPid); depth += 1) {
    visited.add(parentPid);
    const parent = await controller.inspect(parentPid);
    if (!parent) return false;
    if (processBelongsToRepo(parent, repoRoot, controller.platform)) return true;
    parentPid = parent.parentPid;
  }
  return false;
}

export async function inspectPortOwners(
  repoRoot: string,
  ports: number[],
  dependencies: RuntimeDependencies = {},
): Promise<PortOwner[]> {
  const controller = dependencies.controller ?? systemProcessController;
  const listPids = dependencies.listPids ?? ((port) => listListeningPids(port));
  const manifest = readRuntimeManifest(repoRoot, dependencies);
  const owners: PortOwner[] = [];
  for (const port of [...new Set(ports)]) {
    for (const pid of await listPids(port)) {
      const identity = await controller.inspect(pid);
      if (!identity) continue;
      const manifestOwned = await isDescendantOfManifest(identity, manifest, controller);
      const repositoryOwned =
        !manifestOwned && (await belongsToRepositoryProcessTree(identity, repoRoot, controller));
      owners.push({
        port,
        identity,
        relation: manifestOwned
          ? 'manifest'
          : repositoryOwned
            ? 'repository'
            : 'other',
      });
    }
  }
  return owners;
}

function identityKey(identity: ProcessIdentity): string {
  return `${identity.pid}|${identity.startTime}|${identity.commandLine}`;
}

async function terminateRecords(
  repoRoot: string,
  identities: Array<{ identity: ProcessIdentity; provenance: 'manifest' | 'repository' }>,
  controller: ProcessController,
): Promise<TerminateProcessResult[]> {
  const unique = new Map<string, { identity: ProcessIdentity; provenance: 'manifest' | 'repository' }>();
  for (const item of identities) unique.set(identityKey(item.identity), item);
  const results: TerminateProcessResult[] = [];
  for (const item of unique.values()) {
    results.push(await controller.terminate(item.identity, { repoRoot, provenance: item.provenance }));
  }
  return results;
}

function cleanupFailed(results: TerminateProcessResult[]): boolean {
  return results.some((result) => result.status === 'failed' || result.status === 'refused');
}

export async function cleanupRecordedDevRun(
  repoRoot: string,
  ports: number[],
  options: { includeLauncher?: boolean; cleanRepoOwnedListeners?: boolean } = {},
  dependencies: RuntimeDependencies = {},
): Promise<RuntimeCleanupResult> {
  const controller = dependencies.controller ?? systemProcessController;
  const manifest = readRuntimeManifest(repoRoot, dependencies);
  const beforeOwners = await inspectPortOwners(repoRoot, ports, dependencies);
  const targets: Array<{ identity: ProcessIdentity; provenance: 'manifest' | 'repository' }> = [];

  if (manifest) {
    if (options.includeLauncher && manifest.launcher.pid !== process.pid) {
      targets.push({ identity: manifest.launcher, provenance: 'manifest' });
    }
    for (const service of [...manifest.services].reverse()) {
      targets.push({ identity: service.identity, provenance: 'manifest' });
    }
  }

  if (options.cleanRepoOwnedListeners) {
    for (const owner of beforeOwners) {
      if (owner.relation === 'manifest' || owner.relation === 'repository') {
        targets.push({
          identity: owner.identity,
          provenance: owner.relation === 'manifest' ? 'manifest' : 'repository',
        });
      }
    }
  }

  const terminated = await terminateRecords(repoRoot, targets, controller);
  if (targets.length > 0) await sleep(controller.platform === 'win32' ? 500 : 250);

  const trackedKeys = new Set(
    beforeOwners
      .filter((owner) => owner.relation !== 'other')
      .map((owner) => identityKey(owner.identity)),
  );
  const afterOwners = await inspectPortOwners(repoRoot, ports, dependencies);
  const lingering = afterOwners.filter(
    (owner) => owner.relation !== 'other' || trackedKeys.has(identityKey(owner.identity)),
  );
  const ok = !cleanupFailed(terminated) && lingering.length === 0;
  if (manifest && ok) removeRuntimeManifest(repoRoot, dependencies);
  return { ok, terminated, lingering, manifestFound: Boolean(manifest) };
}

export type PrepareLocalPortsResult = RuntimeCleanupResult & {
  conflicts: PortOwner[];
};

export async function prepareLocalDevPorts(
  repoRoot: string,
  ports: number[],
  dependencies: RuntimeDependencies = {},
): Promise<PrepareLocalPortsResult> {
  const cleanup = await cleanupRecordedDevRun(
    repoRoot,
    ports,
    { includeLauncher: true, cleanRepoOwnedListeners: true },
    dependencies,
  );
  const conflicts = await inspectPortOwners(repoRoot, ports, dependencies);
  return {
    ...cleanup,
    ok: cleanup.ok && conflicts.length === 0,
    conflicts,
  };
}

export function portOwnerLabel(owner: PortOwner): string {
  const executable = owner.identity.executablePath
    ? path.basename(owner.identity.executablePath)
    : owner.identity.commandLine.trim().split(/\s+/)[0] || 'unknown';
  const kind = owner.relation === 'other' ? 'unrelated' : 'TradeMind local';
  return `port ${owner.port}: PID ${owner.identity.pid} (${executable}, ${kind})`;
}

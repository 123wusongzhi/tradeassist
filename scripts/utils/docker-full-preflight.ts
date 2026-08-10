import {
  inspectPortOwners,
  type PortOwner,
  type RuntimeDependencies,
} from './dev-runtime.js';
import { runningFullStackCoreServices } from './dev-mode.js';
import { resolveDockerPublishPortMap } from './port-cleanup.js';

export type DockerFullPortConflict = PortOwner & {
  service: 'admin' | 'backend' | 'collector';
};

export async function checkDockerFullPortConflicts(
  repoRoot: string,
  envPath: string | undefined,
  dependencies: RuntimeDependencies & {
    runningServices?: () => Promise<string[]>;
  } = {},
): Promise<DockerFullPortConflict[]> {
  const runningServices = new Set(
    await (dependencies.runningServices?.() ?? runningFullStackCoreServices(repoRoot)),
  );
  const portMap = resolveDockerPublishPortMap(envPath);
  const conflicts: DockerFullPortConflict[] = [];
  const services: Array<'admin' | 'backend' | 'collector'> = ['admin', 'backend'];
  if (portMap.collector !== undefined) services.push('collector');
  for (const service of services) {
    // Existing compose services already own their published port; `compose up` may update them safely.
    if (runningServices.has(service)) continue;
    const port = portMap[service];
    if (port === undefined) continue;
    const owners = await inspectPortOwners(repoRoot, [port], dependencies);
    for (const owner of owners) conflicts.push({ ...owner, service });
  }
  return conflicts;
}

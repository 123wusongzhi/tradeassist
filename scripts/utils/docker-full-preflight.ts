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
  for (const service of ['admin', 'backend', 'collector'] as const) {
    // Existing compose services already own their published port; `compose up` may update them safely.
    if (runningServices.has(service)) continue;
    const owners = await inspectPortOwners(repoRoot, [portMap[service]], dependencies);
    for (const owner of owners) conflicts.push({ ...owner, service });
  }
  return conflicts;
}

import process from 'node:process';

import {
  cleanupRecordedDevRun,
  type DevRuntimeManifest,
  type RuntimeCleanupResult,
  type RuntimeDependencies,
} from './dev-runtime.js';

export type SupervisedProcess = {
  tag: string;
  required: boolean;
  on: (
    event: 'exit',
    listener: (code: number | null, signal: NodeJS.Signals | null) => void,
  ) => unknown;
};

export type ShutdownRequest = {
  reason: string;
  exitCode: number;
};

export type ShutdownCoordinator = {
  watch: (service: SupervisedProcess) => void;
  request: (request: ShutdownRequest) => void;
  wait: Promise<ShutdownRequest>;
  isRequested: () => boolean;
};

export function createShutdownCoordinator(
  onOptionalExit: (message: string) => void = () => undefined,
): ShutdownCoordinator {
  let requested = false;
  let resolveRequest: (request: ShutdownRequest) => void = () => undefined;
  const wait = new Promise<ShutdownRequest>((resolve) => {
    resolveRequest = resolve;
  });

  const request = (value: ShutdownRequest): void => {
    if (requested) return;
    requested = true;
    resolveRequest(value);
  };

  return {
    watch(service) {
      service.on('exit', (code, signal) => {
        if (requested) return;
        if (!service.required) {
          onOptionalExit(
            `Optional process "${service.tag}" exited (${signal ? `signal=${signal}` : `code=${code ?? 'unknown'}`}); core services remain running.`,
          );
          return;
        }
        const exitCode = code && code > 0 ? code : 1;
        request({
          reason: `必需服务「${service.tag}」已退出（${signal ? `signal=${signal}` : `code=${code ?? 'unknown'}`}），正在清理全部本地进程树。`,
          exitCode,
        });
      });
    },
    request,
    wait,
    isRequested: () => requested,
  };
}

export async function shutdownManagedDevRun(
  repoRoot: string,
  manifest: DevRuntimeManifest,
  ports: number[],
  dependencies: RuntimeDependencies = {},
): Promise<RuntimeCleanupResult> {
  // The active launcher must never terminate itself. Every service root is identity-bound in the manifest.
  const result = await cleanupRecordedDevRun(
    repoRoot,
    ports,
    { includeLauncher: false, cleanRepoOwnedListeners: false },
    dependencies,
  );
  if (result.ok && manifest.launcher.pid === process.pid) {
    // cleanupRecordedDevRun removes the manifest only after verified service-port cleanup.
    return result;
  }
  return result;
}

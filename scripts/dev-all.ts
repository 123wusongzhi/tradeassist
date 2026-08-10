import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import readline from 'node:readline';

import { execa } from 'execa';
import pc from 'picocolors';

import { runDevEnvChecks } from './check-dev-env.js';
import {
  addServiceToManifest,
  createRuntimeManifest,
  portOwnerLabel,
  prepareLocalDevPorts,
  writeRuntimeManifest,
  type DevRuntimeManifest,
} from './utils/dev-runtime.js';
import { createShutdownCoordinator, shutdownManagedDevRun } from './utils/dev-supervisor.js';
import type { InfraMode } from './utils/infra.js';
import { addrToHttpUrl, readEnvKey, resolveEffectiveEnvPath } from './utils/env-file.js';
import { banner, tagLine } from './utils/log.js';
import { resolveDevServicePortMap, resolveDevServicePorts } from './utils/port-cleanup.js';
import { repoRoot } from './utils/paths.js';

type ManagedProc = {
  tag: string;
  required: boolean;
  ports: number[];
  subprocess: ReturnType<typeof execa>;
};

function envEnabled(value: string | undefined): boolean {
  return ['1', 'true', 'yes', 'on'].includes((value ?? '').trim().toLowerCase());
}

function ensureRootEnvFromExample(): void {
  const envPath = path.join(repoRoot, '.env');
  const example = path.join(repoRoot, '.env.example');
  if (fs.existsSync(envPath)) {
    tagLine('env', '.env exists', pc.green);
    return;
  }
  if (!fs.existsSync(example)) {
    throw new Error('根目录缺少 .env，且找不到 .env.example，无法自动生成。请手动创建 .env。');
  }
  fs.copyFileSync(example, envPath);
  console.log(
    pc.yellow('[env]'),
    '已从 .env.example 复制生成根目录 .env。首次启动可使用示例中的本地默认配置；生产密钥请勿写入仓库。',
  );
}

function attachPrefixedLines(stream: NodeJS.ReadableStream | null, tag: string, kind: 'out' | 'err'): void {
  if (!stream) return;
  const lines = readline.createInterface({ input: stream, crlfDelay: Infinity });
  const color = kind === 'err' ? pc.red : pc.cyan;
  const prefix = color(`[${tag}]`);
  lines.on('line', (line) => console.log(prefix, line));
}

async function startInfra(mode: InfraMode): Promise<void> {
  if (mode === 'local') {
    tagLine('infra', 'Using local PostgreSQL / Redis (skipping Docker Compose)', pc.green);
    return;
  }

  tagLine('infra', 'Starting PostgreSQL + Redis via Docker Compose...', pc.magenta);
  try {
    await execa('docker', ['compose', 'up', '-d', '--wait', 'postgres', 'redis'], {
      cwd: repoRoot,
      stdio: 'inherit',
    });
  } catch {
    await execa('docker', ['compose', 'up', '-d', 'postgres', 'redis'], {
      cwd: repoRoot,
      stdio: 'inherit',
    });
    console.log(
      pc.yellow('[infra]'),
      '提示：当前 Docker Compose 可能不支持 `--wait`，容器已启动；若后端暂时连不上数据库，请等待数秒后重试。',
    );
  }
  tagLine('infra', 'PostgreSQL / Redis started', pc.green);
}

function printServiceHints(envPath: string | undefined): void {
  const backendAddr = envPath ? readEnvKey(envPath, 'APP_HTTP_ADDR') : undefined;
  const collectorAddr = envPath ? readEnvKey(envPath, 'COLLECTOR_HTTP_ADDR') : undefined;
  const bridgeEnabled = envEnabled(
    process.env.OPENCLI_BRIDGE_ENABLED ?? (envPath ? readEnvKey(envPath, 'OPENCLI_BRIDGE_ENABLED') : undefined),
  );
  const bridgeAddr = envPath ? readEnvKey(envPath, 'OPENCLI_BRIDGE_HTTP_ADDR') : undefined;
  const ports = resolveDevServicePortMap(envPath);
  const backendUrl = addrToHttpUrl(backendAddr ?? ':8080');
  const collectorUrl = addrToHttpUrl(collectorAddr ?? '127.0.0.1:3001');
  const bridgeUrl = addrToHttpUrl(bridgeAddr ?? '127.0.0.1:3100');

  if (backendUrl) console.log(pc.bold(pc.green(`[backend] ${backendUrl}`)));
  if (ports.collector !== undefined && collectorUrl) {
    console.log(pc.bold(pc.green(`[playwright-collector] ${collectorUrl}`)));
  }
  if (bridgeEnabled && bridgeUrl) console.log(pc.bold(pc.green(`[opencli-bridge] ${bridgeUrl}`)));
  console.log(pc.bold(pc.green('[admin]')), pc.green(`http://127.0.0.1:${ports.admin}`));
}

async function preparePorts(envPath: string | undefined): Promise<number[]> {
  const ports = resolveDevServicePorts(envPath);
  tagLine('dev', 'Checking registered local processes and configured ports…', pc.yellow);
  const result = await prepareLocalDevPorts(repoRoot, ports);
  for (const terminated of result.terminated) {
    if (terminated.status === 'terminated') {
      tagLine('dev', `Stopped verified previous process tree PID ${terminated.pid}`, pc.yellow);
    }
  }
  if (!result.ok) {
    if (result.lingering.length > 0) {
      console.error(pc.red('[dev] 已验证的 TradeMind 本地进程仍在监听：'));
      for (const owner of result.lingering) console.error(`  - ${portOwnerLabel(owner)}`);
    }
    if (result.conflicts.length > 0) {
      console.error(pc.red('[dev] 配置端口已被占用；不会按端口终止未知或无关进程：'));
      for (const owner of result.conflicts) console.error(`  - ${portOwnerLabel(owner)}`);
    }
    throw new Error('本地开发端口预检失败。请按上方 PID/端口信息处理，或修改对应端口配置后重试。');
  }
  tagLine('dev', 'Configured enabled service ports are available', pc.green);
  return ports;
}

function spawnManagedProcess(
  tag: string,
  required: boolean,
  ports: number[],
): ManagedProc {
  const subprocess = execa('pnpm', ['run', `dev:${tag}`], {
    cwd: repoRoot,
    reject: false,
    stdout: 'pipe',
    stderr: 'pipe',
    detached: process.platform !== 'win32',
    windowsHide: true,
    env: { ...process.env, TRADEMIND_REPO_ROOT: repoRoot },
  });
  attachPrefixedLines(subprocess.stdout, tag, 'out');
  attachPrefixedLines(subprocess.stderr, tag, 'err');
  return { tag, required, ports, subprocess };
}

async function main(): Promise<number> {
  banner('TradeMind Dev Launcher');
  ensureRootEnvFromExample();

  const check = await runDevEnvChecks({ quietBanner: true, skipContainerStatus: true });
  if (!check.ok || !check.infraMode) return 1;

  const envPath = resolveEffectiveEnvPath(repoRoot);
  const ports = await preparePorts(envPath);
  await startInfra(check.infraMode);

  const portMap = resolveDevServicePortMap(envPath);
  const openCliBridgeEnabled = envEnabled(
    process.env.OPENCLI_BRIDGE_ENABLED ??
      (envPath ? readEnvKey(envPath, 'OPENCLI_BRIDGE_ENABLED') : undefined),
  );
  const manifest = await createRuntimeManifest(repoRoot);
  writeRuntimeManifest(manifest);
  const managed: ManagedProc[] = [];
  const coordinator = createShutdownCoordinator((message) => console.warn(pc.yellow(`[dev] ${message}`)));
  const onSignal = (signal: NodeJS.Signals) => {
    coordinator.request({ reason: `收到 ${signal}，正在清理全部本地进程树。`, exitCode: 0 });
  };
  const onSigint = () => onSignal('SIGINT');
  const onSigterm = () => onSignal('SIGTERM');
  process.on('SIGINT', onSigint);
  process.on('SIGTERM', onSigterm);

  let cleanupAttempted = false;
  const cleanup = async (reason: string): Promise<boolean> => {
    if (cleanupAttempted) return true;
    cleanupAttempted = true;
    console.log(pc.yellow('\n[dev]'), reason);
    const result = await shutdownManagedDevRun(repoRoot, manifest, ports);
    if (!result.ok) {
      console.error(pc.red('[dev] 进程树清理未完成；运行清单已保留，可执行 `pnpm dev:stop` 重试。'));
      for (const owner of result.lingering) console.error(`  - ${portOwnerLabel(owner)}`);
      return false;
    }
    tagLine('dev', 'All verified local process trees stopped and service ports released', pc.green);
    return true;
  };

  try {
    const specs: Array<{ tag: string; required: boolean; ports: number[] }> = [
      { tag: 'backend', required: true, ports: [portMap.backend] },
      { tag: 'admin', required: true, ports: [portMap.admin] },
    ];
    if (portMap.collector !== undefined) {
      specs.push({ tag: 'collector', required: true, ports: [portMap.collector] });
    } else {
      tagLine('collector', 'Playwright collector disabled; use browser extension or OpenCLI', pc.yellow);
    }
    if (openCliBridgeEnabled && portMap.openCliBridge) {
      specs.push({ tag: 'opencli-bridge', required: false, ports: [portMap.openCliBridge] });
    }

    for (const spec of specs) {
      if (coordinator.isRequested()) break;
      tagLine(spec.tag, 'starting...', pc.blue);
      const service = spawnManagedProcess(spec.tag, spec.required, spec.ports);
      managed.push(service);
      coordinator.watch({
        tag: service.tag,
        required: service.required,
        on: (event, listener) => service.subprocess.on(event, listener),
      });
      const pid = service.subprocess.pid;
      if (!pid) {
        if (!service.required) {
          tagLine(service.tag, 'optional process did not start; core services remain running', pc.yellow);
          continue;
        }
        throw new Error(`未取得 ${service.tag} 的 PID，已中止启动。`);
      }
      try {
        await addServiceToManifest(manifest, { ...spec, pid });
      } catch (error: unknown) {
        if (!service.required && service.subprocess.exitCode !== null) {
          tagLine(
            service.tag,
            'optional process exited before runtime registration; core services remain running',
            pc.yellow,
          );
          continue;
        }
        throw error;
      }
    }

    printServiceHints(envPath);
    const request = await coordinator.wait;
    const cleaned = await cleanup(request.reason);
    return cleaned ? request.exitCode : 1;
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error);
    const cleaned = await cleanup(`启动失败（${message}），正在清理已登记的进程树。`);
    if (!cleaned) throw error;
    throw error;
  } finally {
    process.removeListener('SIGINT', onSigint);
    process.removeListener('SIGTERM', onSigterm);
    // Keep subprocess promises observed without blocking forever if a verified cleanup reports failure.
    for (const service of managed) void service.subprocess.catch(() => undefined);
  }
}

main()
  .then((exitCode) => {
    process.exitCode = exitCode;
  })
  .catch((error: unknown) => {
    console.error(pc.red('[dev]'), error instanceof Error ? error.message : error);
    process.exitCode = 1;
  });

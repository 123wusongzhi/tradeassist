import process from 'node:process';

import { execa } from 'execa';
import pc from 'picocolors';

import {
  cleanupRecordedDevRun,
  inspectPortOwners,
  portOwnerLabel,
} from './utils/dev-runtime.js';
import { runningFullStackCoreServices } from './utils/dev-mode.js';
import { resolveEffectiveEnvPath } from './utils/env-file.js';
import { resolveDevServicePorts } from './utils/port-cleanup.js';
import { repoRoot } from './utils/paths.js';

async function main(): Promise<number> {
  const envPath = resolveEffectiveEnvPath(repoRoot);
  const ports = resolveDevServicePorts(envPath);
  const fullStackServices = await runningFullStackCoreServices(repoRoot);

  console.log(pc.yellow('[dev:stop]'), '正在读取当前仓库的本地开发运行清单并验证进程身份…');
  const cleanup = await cleanupRecordedDevRun(
    repoRoot,
    ports,
    { includeLauncher: true, cleanRepoOwnedListeners: true },
  );
  for (const result of cleanup.terminated) {
    if (result.status === 'terminated') {
      console.log(pc.green('[dev:stop]'), `已停止验证通过的进程树 PID ${result.pid}`);
    } else if (result.status === 'identity-changed') {
      console.log(pc.yellow('[dev:stop]'), `PID ${result.pid} 已复用或身份变化，未终止。`);
    }
  }
  if (!cleanup.ok) {
    console.error(pc.red('[dev:stop] 本地进程清理未完成：'));
    for (const owner of cleanup.lingering) console.error(`  - ${portOwnerLabel(owner)}`);
  }

  const remainingOwners = await inspectPortOwners(repoRoot, ports);
  for (const owner of remainingOwners.filter((item) => item.relation === 'other')) {
    console.warn(pc.yellow(`[dev:stop] 保留无关进程：${portOwnerLabel(owner)}`));
  }

  if (fullStackServices.length > 0) {
    console.log(
      pc.yellow('[dev:stop]'),
      `检测到 trademind-full 正在运行（${fullStackServices.join('、')}）；本命令不会停止完整 Docker 栈。`,
    );
  }

  const compose = await execa('docker', ['compose', 'stop'], {
    cwd: repoRoot,
    stdio: 'inherit',
    reject: false,
  });
  if (compose.exitCode !== 0) {
    console.error(pc.red('[dev:stop]'), '停止本地基础设施 Compose 失败。');
  }
  return cleanup.ok && compose.exitCode === 0 ? 0 : 1;
}

main()
  .then((exitCode) => {
    process.exitCode = exitCode;
  })
  .catch((error: unknown) => {
    console.error(pc.red('[dev:stop]'), error instanceof Error ? error.message : error);
    process.exitCode = 1;
  });

import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

import { execa } from 'execa';
import pc from 'picocolors';

import { checkDockerFullPortConflicts } from './utils/docker-full-preflight.js';
import { portOwnerLabel } from './utils/dev-runtime.js';
import { repoRoot } from './utils/paths.js';

async function main(): Promise<number> {
  const envPath = path.join(repoRoot, '.env');
  if (!fs.existsSync(envPath)) {
    console.error(
      pc.red('[docker:full:up]'),
      '根目录缺少 .env。请先复制 `.env.docker.example` 为 `.env` 并配置必需密钥。',
    );
    return 1;
  }

  const conflicts = await checkDockerFullPortConflicts(repoRoot, envPath);
  if (conflicts.length > 0) {
    console.error(pc.red('[docker:full:up] 完整栈发布端口预检失败；不会终止任何进程：'));
    for (const conflict of conflicts) {
      console.error(`  - ${conflict.service}: ${portOwnerLabel(conflict)}`);
    }
    if (conflicts.some((conflict) => conflict.relation !== 'other')) {
      console.error('检测到当前仓库的本地开发进程，请先运行 `pnpm dev:stop`。');
    }
    if (conflicts.some((conflict) => conflict.relation === 'other')) {
      console.error('检测到无关占用者；请自行停止该程序，或修改对应 *_PUBLISH_PORT 后重试。');
    }
    return 1;
  }

  const result = await execa(
    'docker',
    ['compose', '-f', 'docker-compose.full.yml', 'up', '-d', '--build'],
    { cwd: repoRoot, stdio: 'inherit', reject: false },
  );
  return result.exitCode ?? 1;
}

main()
  .then((exitCode) => {
    process.exitCode = exitCode;
  })
  .catch((error: unknown) => {
    console.error(pc.red('[docker:full:up]'), error instanceof Error ? error.message : error);
    process.exitCode = 1;
  });

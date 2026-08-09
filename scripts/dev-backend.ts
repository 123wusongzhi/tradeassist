import process from 'node:process';

import { execa } from 'execa';

import { inspectPortOwners, portOwnerLabel } from './utils/dev-runtime.js';
import { addrToHttpUrl, readEnvKey, resolveEffectiveEnvPath } from './utils/env-file.js';
import { parsePortFromAddr } from './utils/port-cleanup.js';
import { backendDir, repoRoot } from './utils/paths.js';

async function main(): Promise<void> {
  const envFile = resolveEffectiveEnvPath(repoRoot);
  const httpAddr = envFile ? (readEnvKey(envFile, 'APP_HTTP_ADDR') ?? ':8080') : ':8080';
  const url = addrToHttpUrl(httpAddr);
  if (url) {
    console.log(`[backend] ${url}`);
  }

  const port = parsePortFromAddr(httpAddr, 8080);
  const owners = await inspectPortOwners(repoRoot, [port]);
  if (owners.length > 0) {
    console.error(`[backend] 端口 ${port} 已被占用；不会按端口终止任何进程。`);
    for (const owner of owners) console.error(`  - ${portOwnerLabel(owner)}`);
    console.error('请先运行 `pnpm dev:stop` 清理已登记的 TradeMind 本地进程，或停止/改端口后重试。');
    process.exitCode = 1;
    return;
  }

  const r = await execa('go', ['run', './cmd/server'], {
    cwd: backendDir,
    stdio: 'inherit',
    reject: false,
    env: {
      ...process.env,
      TRADEMIND_REPO_ROOT: repoRoot,
    },
  });

  if (r.exitCode !== 0) {
    console.error('\n[backend] 启动失败。请依次排查：');
    console.error('  1) Go 是否已安装且在 PATH 中（go version）');
    console.error('  2) 根目录 .env 或 backend/.env 是否存在，数据库与 Redis 配置是否正确（勿提交密钥）');
    console.error('  3) PostgreSQL / Redis 是否已启动（pnpm dev:infra 或 Docker）');
    console.error('  4) 端口是否被占用（见 APP_HTTP_ADDR）\n');
    process.exit(r.exitCode ?? 1);
  }
}

main().catch((e: unknown) => {
  console.error('[backend] 未预期的错误:', e);
  process.exit(1);
});

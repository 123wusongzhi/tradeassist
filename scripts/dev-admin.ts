import process from 'node:process';

import { execa } from 'execa';

import { readEnvFile, resolveEffectiveEnvPath } from './utils/env-file.js';
import { parsePort } from './utils/port-cleanup.js';
import { repoRoot } from './utils/paths.js';

function forwardedDevArgs(): string[] {
  const args = process.argv.slice(2);
  return args[0] === '--' ? args.slice(1) : args;
}

function explicitPort(args: string[]): string | undefined {
  const inline = args.find((arg) => arg.startsWith('--port='));
  if (inline) return inline.slice('--port='.length);
  const index = args.indexOf('--port');
  return index >= 0 ? args[index + 1] : undefined;
}

async function main(): Promise<void> {
  const fileEnv = readEnvFile(resolveEffectiveEnvPath(repoRoot));
  const devArgs = forwardedDevArgs();
  const adminPort = parsePort(
    explicitPort(devArgs) ?? process.env.ADMIN_DEV_PORT ?? fileEnv.ADMIN_DEV_PORT,
    8000,
  );
  const commandArgs = ['--filter', '@trademind/admin', 'dev'];
  if (devArgs.length > 0) commandArgs.push('--', ...devArgs);
  const result = await execa('pnpm', commandArgs, {
    cwd: repoRoot,
    stdio: 'inherit',
    reject: false,
    env: {
      ...fileEnv,
      ...process.env,
      ADMIN_DEV_PORT: String(adminPort),
      PORT: String(adminPort),
      TRADEMIND_REPO_ROOT: repoRoot,
    },
  });
  process.exitCode = result.exitCode ?? 1;
}

main().catch((error: unknown) => {
  console.error('[admin]', error);
  process.exitCode = 1;
});

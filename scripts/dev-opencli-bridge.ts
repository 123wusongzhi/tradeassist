import process from 'node:process';
import { join } from 'node:path';
import { execa } from 'execa';
import { readEnvFile, resolveEffectiveEnvPath } from './utils/env-file.js';
import { repoRoot } from './utils/paths.js';

async function main(): Promise<void> {
  const envPath = resolveEffectiveEnvPath(repoRoot);
  const fileEnv = readEnvFile(envPath);
  const openCliEnv = Object.fromEntries(Object.entries(fileEnv).filter(([key]) => key.startsWith('OPENCLI_')));
  const result = await execa('tsx', ['watch', 'src/opencli-bridge/index.ts'], {
    cwd: join(repoRoot, 'collector'),
    stdio: 'inherit',
    reject: false,
    env: { ...openCliEnv, ...process.env },
  });
  process.exit(result.exitCode ?? 1);
}

main().catch((error: unknown) => {
  console.error('[opencli-bridge]', error);
  process.exit(1);
});

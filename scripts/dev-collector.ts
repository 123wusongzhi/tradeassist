import process from 'node:process';
import { join } from 'node:path';
import { execa } from 'execa';
import { readEnvFile, resolveEffectiveEnvPath } from './utils/env-file.js';
import { repoRoot } from './utils/paths.js';

async function main(): Promise<void> {
  const envPath = resolveEffectiveEnvPath(repoRoot);
  const fileEnv = readEnvFile(envPath);
  const collectorEnv = Object.fromEntries(
    Object.entries(fileEnv).filter(([key]) => key.startsWith('COLLECTOR_') || key === 'BROWSER_PROFILE_ROOT'),
  );
  const result = await execa('tsx', ['watch', 'src/index.ts'], {
    cwd: join(repoRoot, 'collector'),
    stdio: 'inherit',
    reject: false,
    env: { ...collectorEnv, ...process.env },
  });
  process.exit(result.exitCode ?? 1);
}

main().catch((error: unknown) => {
  console.error('[collector]', error);
  process.exit(1);
});

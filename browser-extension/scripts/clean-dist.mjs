import { rmSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const packageRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const output = resolve(packageRoot, 'dist');
if (dirname(output) !== packageRoot) {
  throw new Error(`refusing to clean unexpected path: ${output}`);
}
rmSync(output, { recursive: true, force: true });

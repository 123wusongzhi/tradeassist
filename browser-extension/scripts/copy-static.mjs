import { copyFileSync, mkdirSync, readdirSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const packageRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const source = resolve(packageRoot, 'public');
const output = resolve(packageRoot, 'dist');
mkdirSync(output, { recursive: true });
for (const entry of readdirSync(source, { withFileTypes: true })) {
  if (!entry.isFile()) {
    throw new Error(`unexpected non-file public entry: ${entry.name}`);
  }
  copyFileSync(join(source, entry.name), join(output, entry.name));
}

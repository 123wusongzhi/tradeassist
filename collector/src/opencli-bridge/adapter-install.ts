import { copyFileSync, existsSync, mkdirSync, readFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const MANAGED_MARKER = '@trademind-managed-opencli-adapter';
const ADAPTER_FILES = ['product.js', 'shared.js'] as const;

export type OpenCliAdapterSyncOptions = {
  sourceRoot?: string;
  targetRoot?: string;
};

export type OpenCliAdapterSyncResult = {
  targetRoot: string;
  updated: boolean;
};

function defaultSourceRoot(): string {
  return fileURLToPath(new URL('../../opencli-adapters/tmall/', import.meta.url));
}

function defaultTargetRoot(): string {
  return join(homedir(), '.opencli', 'clis', 'tmall');
}

function readUTF8(path: string): string {
  return readFileSync(path, 'utf8');
}

export function syncTradeMindOpenCliAdapter(options: OpenCliAdapterSyncOptions = {}): OpenCliAdapterSyncResult {
  const sourceRoot = options.sourceRoot ?? defaultSourceRoot();
  const targetRoot = options.targetRoot ?? defaultTargetRoot();
  const sourceProduct = join(sourceRoot, 'product.js');

  if (!existsSync(sourceProduct) || !readUTF8(sourceProduct).includes(MANAGED_MARKER)) {
    throw new Error(`TradeMind OpenCLI adapter assets are missing or invalid at ${sourceRoot}`);
  }

  const existingFiles = ADAPTER_FILES.filter((file) => existsSync(join(targetRoot, file)));
  if (
    existingFiles.length > 0 &&
    (!existsSync(join(targetRoot, 'product.js')) || !readUTF8(join(targetRoot, 'product.js')).includes(MANAGED_MARKER))
  ) {
    throw new Error(
      'A non-TradeMind tmall adapter already exists in ~/.opencli/clis/tmall. ' +
        'Move or back it up before starting the TradeMind OpenCLI Bridge.',
    );
  }

  mkdirSync(targetRoot, { recursive: true });
  let updated = false;
  for (const file of ADAPTER_FILES) {
    const source = join(sourceRoot, file);
    const target = join(targetRoot, file);
    if (!existsSync(source)) {
      throw new Error(`TradeMind OpenCLI adapter asset is missing: ${source}`);
    }
    if (!existsSync(target) || readUTF8(source) !== readUTF8(target)) {
      copyFileSync(source, target);
      updated = true;
    }
  }

  return { targetRoot, updated };
}

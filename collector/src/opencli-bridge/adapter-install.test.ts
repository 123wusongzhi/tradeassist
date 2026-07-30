import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { afterEach, describe, expect, it } from 'vitest';
import { syncTradeMindOpenCliAdapter } from './adapter-install.js';

const tempRoots: string[] = [];

function tempRoot(): string {
  const root = mkdtempSync(join(tmpdir(), 'trademind-opencli-adapter-'));
  tempRoots.push(root);
  return root;
}

function writeManagedSource(root: string, version: string): string {
  const source = join(root, 'source');
  mkdirSync(source, { recursive: true });
  writeFileSync(
    join(source, 'product.js'),
    `// @trademind-managed-opencli-adapter ${version}\nexport const version = '${version}';\n`,
  );
  writeFileSync(join(source, 'shared.js'), `export const shared = '${version}';\n`);
  return source;
}

afterEach(() => {
  for (const root of tempRoots.splice(0)) {
    rmSync(root, { recursive: true, force: true });
  }
});

describe('TradeMind OpenCLI adapter sync', () => {
  it('installs and idempotently updates the managed adapter', () => {
    const root = tempRoot();
    const source = writeManagedSource(root, 'v1');
    const target = join(root, 'target');

    expect(syncTradeMindOpenCliAdapter({ sourceRoot: source, targetRoot: target }).updated).toBe(true);
    expect(syncTradeMindOpenCliAdapter({ sourceRoot: source, targetRoot: target }).updated).toBe(false);

    writeManagedSource(root, 'v2');
    expect(syncTradeMindOpenCliAdapter({ sourceRoot: source, targetRoot: target }).updated).toBe(true);
    expect(readFileSync(join(target, 'product.js'), 'utf8')).toContain('v2');
  });

  it('does not overwrite an unrelated user adapter', () => {
    const root = tempRoot();
    const source = writeManagedSource(root, 'v1');
    const target = join(root, 'target');
    mkdirSync(target, { recursive: true });
    writeFileSync(join(target, 'product.js'), '// user-owned adapter\n');

    expect(() => syncTradeMindOpenCliAdapter({ sourceRoot: source, targetRoot: target })).toThrow(
      /non-TradeMind tmall adapter/,
    );
  });
});

describe('TradeMind tmall adapter access detection', () => {
  async function resolveAccessStatus(signals: Record<string, unknown>) {
    const adapterSource = readFileSync(
      fileURLToPath(new URL('../../opencli-adapters/tmall/shared.js', import.meta.url)),
      'utf8',
    );
    const adapterURL = `data:text/javascript;base64,${Buffer.from(adapterSource).toString('base64')}`;
    const adapter = (await import(/* @vite-ignore */ adapterURL)) as {
      resolveAccessStatus: (value: Record<string, unknown>) => {
        status: string;
        errorCode?: string;
      };
    };
    return adapter.resolveAccessStatus(signals);
  }

  it('does not turn incidental 404 text on a real product page into a removed item', async () => {
    await expect(
      resolveAccessStatus({
        bodySnippet: 'RXG-404 型号 商品详情 立即购买',
        pageTitle: '继电器商品详情',
        productCoreHit: true,
        verifyRequiredHit: false,
        loginRequiredHit: false,
      }),
    ).resolves.toEqual({ status: 'public' });
  });

  it('prioritizes verification evidence over a soft not-found response', async () => {
    await expect(
      resolveAccessStatus({
        bodySnippet: '商品不存在',
        pageTitle: '安全验证',
        productCoreHit: false,
        verifyRequiredHit: true,
        loginRequiredHit: false,
      }),
    ).resolves.toEqual({
      status: 'verify_required',
      errorCode: 'VERIFY_REQUIRED',
    });
  });

  it('keeps an explicit missing-item page without product structure as not found', async () => {
    await expect(
      resolveAccessStatus({
        bodySnippet: '该商品已下架',
        pageTitle: '页面提示',
        productCoreHit: false,
        verifyRequiredHit: false,
        loginRequiredHit: false,
      }),
    ).resolves.toEqual({ status: 'not_found', errorCode: 'ITEM_NOT_FOUND' });
  });
});

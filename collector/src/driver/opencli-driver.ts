import { execFile } from 'node:child_process';
import { existsSync } from 'node:fs';
import type { NormalizedProduct, ProductSku } from '../types/product.js';

export type OpenCliDriverConfig = {
  bin: string;
  timeoutMs: number;
  skuClickMax: number;
  skuPriceMax: number;
};

export type OpenCliCommandResult = {
  stdout: string;
  stderr: string;
};

export type OpenCliRunner = (args: string[], timeoutMs: number) => Promise<OpenCliCommandResult>;

export type OpenCliRuntimeStatus = {
  ready: boolean;
  binaryAvailable: boolean;
  daemonRunning: boolean;
  extensionConnected: boolean;
  profileAvailable: boolean;
  message: string;
};

export function getOpenCliDriverConfig(): OpenCliDriverConfig {
  const timeout = Number(process.env.OPENCLI_TIMEOUT_MS ?? '120000');
  const skuClick = Number(process.env.OPENCLI_SKU_CLICK_MAX ?? '24');
  const skuPrice = Number(process.env.OPENCLI_SKU_PRICE_MAX ?? '24');
  return {
    bin: process.env.OPENCLI_BIN ?? 'opencli',
    timeoutMs: Number.isFinite(timeout) && timeout > 0 ? timeout : 120000,
    skuClickMax: Number.isFinite(skuClick) && skuClick >= 0 ? Math.min(48, Math.floor(skuClick)) : 24,
    skuPriceMax: Number.isFinite(skuPrice) && skuPrice >= 0 ? Math.min(48, Math.floor(skuPrice)) : 24,
  };
}

function resolveSpawnTarget(bin: string): {
  cmd: string;
  prefixArgs: string[];
} {
  if (bin.endsWith('.js')) {
    return { cmd: process.execPath, prefixArgs: [bin] };
  }
  const globalRoot = process.env.OPENCLI_NPM_ROOT;
  const candidates = [
    globalRoot ? `${globalRoot}/@jackwener/opencli/dist/src/main.js` : '',
    `${process.env.APPDATA ?? ''}/npm/node_modules/@jackwener/opencli/dist/src/main.js`,
    '/usr/local/lib/node_modules/@jackwener/opencli/dist/src/main.js',
    '/usr/lib/node_modules/@jackwener/opencli/dist/src/main.js',
  ].filter(Boolean);
  for (const candidate of candidates) {
    try {
      if (existsSync(candidate)) {
        return { cmd: process.execPath, prefixArgs: [candidate] };
      }
    } catch {
      // Continue to the next candidate.
    }
  }
  return { cmd: bin, prefixArgs: [] };
}

export class OpenCliCollectError extends Error {
  constructor(
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = 'OpenCliCollectError';
  }
}

function readStructuredErrorCode(output: string): string {
  return output.match(/^\s*code:\s*['"]?([A-Z][A-Z0-9_]*)['"]?\s*$/im)?.[1]?.toUpperCase() ?? '';
}

export function classifyOpenCliCommandError(
  error: { code?: string | number | null; message: string },
  stderr: string,
  stdout: string,
): OpenCliCollectError {
  const commandOutput = `${stderr}\n${stdout}`.trim();
  const combined = `${commandOutput}\n${error.message}`.trim();
  const reportedCode = readStructuredErrorCode(commandOutput);
  if (error.code === 'ENOENT' || /not recognized|command not found/i.test(combined)) {
    return new OpenCliCollectError(
      'PROVIDER_NOT_AVAILABLE',
      'OpenCLI command was not found. Install @jackwener/opencli on the host.',
    );
  }
  if (/安全验证|滑块|验证|captcha|punish|x5secdata/i.test(combined)) {
    return new OpenCliCollectError('VERIFY_REQUIRED', combined.slice(0, 500));
  }
  if (/需要登录|登录淘宝|登录天猫|AuthRequired|Unauthorized/i.test(combined)) {
    return new OpenCliCollectError('LOGIN_REQUIRED', combined.slice(0, 500));
  }
  if (reportedCode === 'AUTH_REQUIRED' || reportedCode === 'LOGIN_WALL') {
    return new OpenCliCollectError('LOGIN_REQUIRED', combined.slice(0, 500));
  }
  if (reportedCode === 'ITEM_NOT_FOUND') {
    return new OpenCliCollectError('ITEM_NOT_FOUND', combined.slice(0, 500));
  }
  if (reportedCode === 'EMPTY_RESULT') {
    if (/TITLE_NOT_FOUND/i.test(commandOutput)) {
      return new OpenCliCollectError(
        'PARSE_FAILED_TITLE_MISSING',
        'OpenCLI opened the page but could not extract the product title.',
      );
    }
    if (/MAIN_IMAGES_EMPTY/i.test(commandOutput)) {
      return new OpenCliCollectError(
        'MAIN_IMAGES_EMPTY',
        'OpenCLI opened the page but could not extract product images.',
      );
    }
    return new OpenCliCollectError(
      'PARSE_FAILED',
      'OpenCLI returned no product data. The page may require login or verification, be temporarily limited, or the adapter may need an update.',
    );
  }
  if (['SELECTOR', 'PAGE_CHANGED', 'API_ERROR', 'NETWORK', 'COMMAND_EXEC'].includes(reportedCode)) {
    return new OpenCliCollectError('PARSE_FAILED', combined.slice(0, 500));
  }
  if (/timeout|超时|timed out/i.test(combined)) {
    return new OpenCliCollectError('TIMEOUT', combined.slice(0, 500));
  }
  if (
    reportedCode === 'BROWSER_CONNECT' ||
    reportedCode === 'ADAPTER_LOAD' ||
    /extension not connected|Browser Bridge|not connected/i.test(combined)
  ) {
    return new OpenCliCollectError(
      'PROVIDER_NOT_AVAILABLE',
      'OpenCLI browser extension is not connected. Start the daemon and connect the Chrome extension.',
    );
  }
  return new OpenCliCollectError('COLLECT_FAILED', combined.slice(0, 500) || 'OpenCLI failed');
}

export const runOpenCliCommand: OpenCliRunner = async (args, timeoutMs) => {
  const target = resolveSpawnTarget(getOpenCliDriverConfig().bin);
  return new Promise<OpenCliCommandResult>((resolve, reject) => {
    execFile(
      target.cmd,
      [...target.prefixArgs, ...args],
      { timeout: timeoutMs, maxBuffer: 32 * 1024 * 1024, windowsHide: true },
      (error, stdout, stderr) => {
        const result = {
          stdout: String(stdout ?? ''),
          stderr: String(stderr ?? ''),
        };
        if (error) {
          reject(classifyOpenCliCommandError(error, result.stderr, result.stdout));
          return;
        }
        resolve(result);
      },
    );
  });
};

const SOURCE_ADAPTER_MAP: Record<string, string> = {
  taobao_tmall: 'tmall',
  taobao: 'tmall',
  tmall: 'tmall',
};

export function resolveOpenCliAdapter(source: string): string | undefined {
  return SOURCE_ADAPTER_MAP[source.trim().toLowerCase()];
}

type OpenCliRow = {
  title?: string;
  priceText?: string;
  price?: number;
  priceMin?: number;
  priceMax?: number;
  shopName?: string;
  currency?: string;
  mainImages?: string[];
  descriptionImages?: string[];
  attributes?: Record<string, string | number | boolean>;
  skuGroups?: unknown;
  skus?: ProductSku[];
  qualityStatus?: string;
  qualityScore?: number;
  warnings?: string[];
  sourceUrl?: string;
  debug?: Record<string, unknown>;
};

function toNormalizedProduct(source: string, sourceUrl: string, row: OpenCliRow): NormalizedProduct {
  const title = String(row.title ?? '').trim();
  if (!title) {
    throw new OpenCliCollectError('PARSE_FAILED_TITLE_MISSING', 'OpenCLI result has no title');
  }
  const mainImages = Array.isArray(row.mainImages) ? row.mainImages.filter(Boolean) : [];
  if (mainImages.length === 0) {
    throw new OpenCliCollectError('MAIN_IMAGES_EMPTY', 'OpenCLI result has no main images');
  }
  const skus: ProductSku[] = Array.isArray(row.skus)
    ? row.skus.map((sku) => ({
        id: sku.id ?? sku.skuCode,
        properties: sku.properties ?? {},
        price: typeof sku.price === 'number' && sku.price > 0 ? sku.price : undefined,
        stock: typeof sku.stock === 'number' ? sku.stock : undefined,
        skuCode: sku.skuCode,
        image: sku.image,
        raw: sku.raw ?? {},
      }))
    : [];

  return {
    source,
    sourceUrl: String(row.sourceUrl ?? sourceUrl),
    title,
    currency: String(row.currency ?? 'CNY'),
    mainDescription: '',
    mainImages,
    descriptionImages: Array.isArray(row.descriptionImages) ? row.descriptionImages.filter(Boolean) : [],
    attributes: row.attributes ?? {},
    skus,
    raw: {
      engine: 'opencli',
      adapter: resolveOpenCliAdapter(source) ?? source,
      price: row.price,
      priceMin: row.priceMin,
      priceMax: row.priceMax,
      priceText: row.priceText,
      shopName: row.shopName,
      skuGroups: row.skuGroups,
      qualityStatus: row.qualityStatus,
      qualityScore: row.qualityScore,
      warnings: row.warnings ?? [],
      debug: row.debug ?? {},
    },
  };
}

export async function collectViaOpenCli(
  source: string,
  url: string,
  options: Record<string, unknown> = {},
  runner: OpenCliRunner = runOpenCliCommand,
): Promise<NormalizedProduct> {
  const adapter = resolveOpenCliAdapter(source);
  if (!adapter) {
    throw new OpenCliCollectError('PROVIDER_NOT_IMPLEMENTED', `OpenCLI does not support source "${source}"`);
  }
  const config = getOpenCliDriverConfig();
  const rawSkuClick =
    options.skuClickMax ??
    options.skuClickMaxCount ??
    options.skuMaxClicks ??
    options['sku-click'] ??
    config.skuClickMax;
  const parsedSkuClick = Number(rawSkuClick);
  const skuClick =
    Number.isFinite(parsedSkuClick) && parsedSkuClick >= 0
      ? Math.min(48, Math.floor(parsedSkuClick))
      : config.skuClickMax;
  const rawSkuPriceMax =
    options.skuPriceMax ??
    options.skuPriceMaxCount ??
    options.skuMaxPriceProbes ??
    options['sku-price-max'] ??
    config.skuPriceMax;
  const parsedSkuPriceMax = Number(rawSkuPriceMax);
  const skuPriceMax =
    Number.isFinite(parsedSkuPriceMax) && parsedSkuPriceMax >= 0
      ? Math.min(48, Math.floor(parsedSkuPriceMax))
      : config.skuPriceMax;
  const result = await runner(
    [
      adapter,
      'product',
      url,
      '--sku-click',
      String(skuClick),
      '--sku-price-max',
      String(skuPriceMax),
      '-f',
      'json',
    ],
    config.timeoutMs,
  );

  let rows: OpenCliRow[];
  try {
    const parsed = JSON.parse(result.stdout) as unknown;
    rows = Array.isArray(parsed) ? (parsed as OpenCliRow[]) : [];
  } catch {
    throw new OpenCliCollectError('PARSE_FAILED', `OpenCLI output is not valid JSON: ${result.stdout.slice(0, 200)}`);
  }
  if (rows.length === 0) {
    throw new OpenCliCollectError('COLLECT_FAILED', 'OpenCLI returned an empty result');
  }
  return toNormalizedProduct(source, url, rows[0]!);
}

export function parseOpenCliDaemonStatusOutput(output: string): Omit<OpenCliRuntimeStatus, 'message'> {
  const text = output.trim();
  const binaryAvailable = text.length > 0;
  const daemonRunning = /^\s*Daemon:\s*(running|stale)\b/im.test(text);
  const extensionConnected = /^\s*Extension:\s*connected\b/im.test(text);
  const profiles = text.match(/^\s*Profiles:\s*(.+)$/im)?.[1]?.trim() ?? '';
  const profileAvailable = Boolean(profiles) && !/\b(none|not connected)\b/i.test(profiles);
  return {
    ready: binaryAvailable && daemonRunning && extensionConnected && profileAvailable,
    binaryAvailable,
    daemonRunning,
    extensionConnected,
    profileAvailable,
  };
}

export async function probeOpenCliStatus(runner: OpenCliRunner = runOpenCliCommand): Promise<OpenCliRuntimeStatus> {
  try {
    // `opencli doctor` performs a live BrowserBridge.connect() probe, which leases
    // a Chrome window. Status reads are triggered by ordinary Admin navigation, so
    // they must use the passive daemon command and never touch a browser window.
    const result = await runner(['daemon', 'status'], 10000);
    const parsed = parseOpenCliDaemonStatusOutput(`${result.stdout}\n${result.stderr}`);
    const message = parsed.ready
      ? 'OpenCLI daemon, browser extension, and profile are ready'
      : !parsed.daemonRunning
        ? 'OpenCLI daemon is not running'
        : !parsed.extensionConnected
          ? 'OpenCLI browser extension is not connected or no profile is selected'
          : 'OpenCLI browser profile is not available';
    return {
      ...parsed,
      message,
    };
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return {
      ready: false,
      binaryAvailable: !(error instanceof OpenCliCollectError && /command was not found/i.test(message)),
      daemonRunning: false,
      extensionConnected: false,
      profileAvailable: false,
      message,
    };
  }
}

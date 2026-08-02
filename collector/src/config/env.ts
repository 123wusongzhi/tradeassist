/**
 * 环境变量（由 docker / systemd / .env 注入，不写入代码默认值中的密钥）。
 */
import { getBrowserProfileRoot, get1688UserDataDir, getStorageStateRoot } from '../browser/browser-paths.js';

export { getBrowserProfileRoot, get1688UserDataDir, getStorageStateRoot };

export type CollectorHttpConfig = {
  host: string;
  port: number;
  token: string;
};

function parseHttpAddress(raw: string): { host: string; port: number } {
  const value = raw.trim() || '127.0.0.1:3001';
  if (/^:\d+$/.test(value)) {
    return { host: '0.0.0.0', port: Number(value.slice(1)) };
  }
  if (/^\d+$/.test(value)) {
    const port = Number(value);
    if (port > 0 && port < 65536) return { host: '127.0.0.1', port };
  }
  const index = value.lastIndexOf(':');
  if (index > 0) {
    const host = value.slice(0, index).replace(/^\[|\]$/g, '');
    const port = Number(value.slice(index + 1));
    if (host && Number.isInteger(port) && port > 0 && port < 65536) {
      return { host, port };
    }
  }
  throw new Error(`Invalid COLLECTOR_HTTP_ADDR: ${raw}`);
}

export function isLoopbackHost(host: string): boolean {
  const value = host.trim().toLowerCase();
  return value === '127.0.0.1' || value === 'localhost' || value === '::1';
}

export function getCollectorHttpConfig(): CollectorHttpConfig {
  const address = parseHttpAddress(
    process.env.COLLECTOR_HTTP_ADDR ?? '127.0.0.1:3001',
  );
  const token = String(process.env.COLLECTOR_INTERNAL_TOKEN ?? '').trim();
  if (!isLoopbackHost(address.host) && !token) {
    throw new Error(
      'COLLECTOR_INTERNAL_TOKEN is required when COLLECTOR_HTTP_ADDR is not loopback',
    );
  }
  return { ...address, token };
}

export function getHttpPort(): number {
  try {
    return getCollectorHttpConfig().port;
  } catch {
    return 3001;
  }
}

export function getDefaultNavigationTimeoutMs(): number {
  const n = Number(process.env.COLLECTOR_GOTO_TIMEOUT_MS ?? '45000');
  return Number.isFinite(n) && n > 0 ? n : 45000;
}

export function getBrowserHeadless(): boolean {
  const v = process.env.COLLECTOR_HEADLESS;
  if (v === '0' || v === 'false') return false;
  return true;
}

/** @deprecated 使用 getBrowserProfileRoot() */
export function getBrowserProfileBaseDir(): string {
  return getBrowserProfileRoot();
}

/** @deprecated 使用 getStorageStateRoot() */
export function getStorageStateBaseDir(): string {
  return getStorageStateRoot();
}

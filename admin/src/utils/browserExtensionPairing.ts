/** 浏览器扩展配对连接信息（与 browser-extension/src/pairing.ts 保持同一协议）。 */

export const PAIRING_PREFIX = 'TM-PAIR:';

export type BrowserExtensionPairingPayload = {
  v: 1;
  apiBase: string;
  code: string;
};

function isLoopbackHost(hostname: string): boolean {
  const value = hostname.trim().toLowerCase();
  return (
    value === 'localhost' ||
    value.endsWith('.localhost') ||
    value === '127.0.0.1' ||
    value === '[::1]' ||
    value === '::1'
  );
}

/** RFC 4648 base64url 编码（无补齐符）。 */
function base64UrlEncode(raw: string): string {
  const bytes = new TextEncoder().encode(raw);
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_';
  let out = '';
  for (let i = 0; i < bytes.length; i += 3) {
    const a = bytes[i] ?? 0;
    const b = bytes[i + 1] ?? 0;
    const c = bytes[i + 2] ?? 0;
    const combined = (a << 16) | (b << 8) | c;
    out += alphabet[(combined >>> 18) & 63];
    out += alphabet[(combined >>> 12) & 63];
    if (i + 1 < bytes.length) out += alphabet[(combined >>> 6) & 63];
    if (i + 2 < bytes.length) out += alphabet[combined & 63];
  }
  return out;
}

/** 判断当前页面是否为远程 HTTP 地址（远程部署必须 HTTPS 才能连接扩展）。 */
export function isRemoteHttpOrigin(origin: string): boolean {
  let url: URL;
  try {
    url = new URL(origin);
  } catch {
    return false;
  }
  return url.protocol === 'http:' && !isLoopbackHost(url.hostname);
}

/** 生成粘贴到扩展侧边栏的一次性连接信息（地址 + 配对码）。 */
export function buildBrowserExtensionConnectionInfo(apiBase: string, code: string): string {
  const url = new URL(apiBase.trim());
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    throw new Error('TradeMind 地址必须使用 http 或 https');
  }
  if (url.protocol === 'http:' && !isLoopbackHost(url.hostname)) {
    throw new Error('远程 TradeMind 地址必须使用 HTTPS');
  }
  const payload: BrowserExtensionPairingPayload = {
    v: 1,
    apiBase: url.origin,
    code: code.trim().toUpperCase(),
  };
  return `${PAIRING_PREFIX}${base64UrlEncode(JSON.stringify(payload))}`;
}

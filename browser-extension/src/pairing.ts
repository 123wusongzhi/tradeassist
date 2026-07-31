export const PAIRING_PREFIX = 'TM-PAIR:';

export type PairingPayload = {
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

export function normalizeAPIBase(raw: string): string {
  const parsed = new URL(raw.trim());
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    throw new Error('TradeMind 地址必须使用 http 或 https');
  }
  if (parsed.protocol === 'http:' && !isLoopbackHost(parsed.hostname)) {
    throw new Error('远程 TradeMind 地址必须使用 HTTPS');
  }
  parsed.hash = '';
  parsed.search = '';
  return parsed.toString().replace(/\/+$/, '');
}

function fromBase64URL(raw: string): string {
  const normalized = raw.replace(/-/g, '+').replace(/_/g, '/');
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=');
  return decodeURIComponent(
    Array.from(atob(padded))
      .map((char) => `%${char.charCodeAt(0).toString(16).padStart(2, '0')}`)
      .join(''),
  );
}

export function parsePairingInput(raw: string, fallbackAPIBase: string): PairingPayload {
  const value = raw.trim();
  if (!value) {
    throw new Error('请输入配对码');
  }
  if (!value.startsWith(PAIRING_PREFIX)) {
    return {
      v: 1,
      apiBase: normalizeAPIBase(fallbackAPIBase),
      code: value.toUpperCase(),
    };
  }
  let decoded: unknown;
  try {
    decoded = JSON.parse(fromBase64URL(value.slice(PAIRING_PREFIX.length)));
  } catch {
    throw new Error('连接信息格式无效，请在管理端重新生成');
  }
  if (!decoded || typeof decoded !== 'object') {
    throw new Error('连接信息格式无效');
  }
  const payload = decoded as Partial<PairingPayload>;
  if (payload.v !== 1 || typeof payload.apiBase !== 'string' || typeof payload.code !== 'string') {
    throw new Error('连接信息版本不受支持');
  }
  return {
    v: 1,
    apiBase: normalizeAPIBase(payload.apiBase),
    code: payload.code.trim().toUpperCase(),
  };
}

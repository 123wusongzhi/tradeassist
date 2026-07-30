export type OpenCliBridgeConfig = {
  host: string;
  port: number;
  token: string;
};

function parseAddress(raw: string): { host: string; port: number } {
  const value = raw.trim() || '127.0.0.1:3100';
  if (/^:\d+$/.test(value)) {
    return { host: '0.0.0.0', port: Number(value.slice(1)) };
  }
  const index = value.lastIndexOf(':');
  if (index > 0) {
    const host = value.slice(0, index).replace(/^\[|\]$/g, '');
    const port = Number(value.slice(index + 1));
    if (host && Number.isInteger(port) && port > 0 && port < 65536) {
      return { host, port };
    }
  }
  throw new Error(`Invalid OPENCLI_BRIDGE_HTTP_ADDR: ${raw}`);
}

export function isLoopbackHost(host: string): boolean {
  const value = host.trim().toLowerCase();
  return value === '127.0.0.1' || value === 'localhost' || value === '::1';
}

export function getOpenCliBridgeConfig(): OpenCliBridgeConfig {
  const address = parseAddress(
    process.env.OPENCLI_BRIDGE_HTTP_ADDR ?? '127.0.0.1:3100',
  );
  const token = String(process.env.OPENCLI_BRIDGE_TOKEN ?? '').trim();
  if (!isLoopbackHost(address.host) && !token) {
    throw new Error(
      'OPENCLI_BRIDGE_TOKEN is required when OPENCLI_BRIDGE_HTTP_ADDR is not loopback',
    );
  }
  return { ...address, token };
}

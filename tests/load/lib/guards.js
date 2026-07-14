export function requireEnv(name) {
  const value = __ENV[name];
  if (!value) {
    throw new Error(`missing required env: ${name}`);
  }
  return value;
}

export function baseUrl() {
  return __ENV.BASE_URL || 'http://127.0.0.1:8080';
}

function hostFromBase(base) {
  const value = String(base || '').trim();
  const match = value.match(/^https?:\/\/([^/?#:]+)(?::\d+)?/i);
  if (!match) return '';
  return match[1].toLowerCase();
}

export function authHeaders() {
  const token = __ENV.P7_AUTH_TOKEN || '';
  if (!token) return {};
  return { Authorization: `Bearer ${token}` };
}

export function failFastHost() {
  const base = baseUrl();
  const host = hostFromBase(base);
  if (!host) {
    throw new Error('invalid BASE_URL');
  }
  if (!host || host === 'api.zhihengxiangyu.com' || host.endsWith('.zhihengxiangyu.com')) {
    throw new Error('unsafe BASE_URL');
  }
  const allowed = host === 'localhost' || host === '127.0.0.1' || host.startsWith('172.') || host.startsWith('192.168.');
  if (!allowed) throw new Error(`unsafe host: ${host}`);
}

export function extractCursor(body) {
  try {
    const json = JSON.parse(body);
    return json?.data?.nextCursor || json?.data?.cursor || '';
  } catch {
    return '';
  }
}

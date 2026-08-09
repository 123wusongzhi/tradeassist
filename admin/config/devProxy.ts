export const DEFAULT_ADMIN_DEV_API_PROXY_TARGET = 'http://127.0.0.1:8080';

const LOOPBACK_HOSTS = new Set(['127.0.0.1', 'localhost', '::1', '[::1]']);

export function resolveAdminDevApiProxyTarget(
  env: Record<string, string | undefined> = process.env,
): string {
  const raw = (env.ADMIN_DEV_API_PROXY_TARGET ?? DEFAULT_ADMIN_DEV_API_PROXY_TARGET).trim();
  let target: URL;
  try {
    target = new URL(raw);
  } catch {
    throw new Error(
      'ADMIN_DEV_API_PROXY_TARGET 必须是完整的本机 HTTP(S) URL，例如 http://127.0.0.1:8081。',
    );
  }
  if (!['http:', 'https:'].includes(target.protocol)) {
    throw new Error('ADMIN_DEV_API_PROXY_TARGET 仅允许 http:// 或 https://。');
  }
  if (!LOOPBACK_HOSTS.has(target.hostname.toLowerCase())) {
    throw new Error('ADMIN_DEV_API_PROXY_TARGET 仅允许 localhost、127.0.0.1 或 ::1。');
  }
  if (!target.port) {
    throw new Error('ADMIN_DEV_API_PROXY_TARGET 必须显式包含端口。');
  }
  if (target.username || target.password || target.search || target.hash) {
    throw new Error('ADMIN_DEV_API_PROXY_TARGET 不得包含凭据、查询参数或片段。');
  }
  if (target.pathname !== '/' && target.pathname !== '') {
    throw new Error('ADMIN_DEV_API_PROXY_TARGET 只能配置源地址，不能包含路径。');
  }
  return target.origin;
}

export function createAdminDevProxy(env: Record<string, string | undefined> = process.env) {
  const target = resolveAdminDevApiProxyTarget(env);
  return {
    '/api': { target, changeOrigin: true },
    '/static': { target, changeOrigin: true },
  };
}

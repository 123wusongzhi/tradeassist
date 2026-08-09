import fs from 'node:fs';
import path from 'node:path';

import { describe, expect, it } from 'vitest';

import {
  DEFAULT_ADMIN_DEV_API_PROXY_TARGET,
  createAdminDevProxy,
  resolveAdminDevApiProxyTarget,
} from './devProxy.js';

describe('Admin development API proxy', () => {
  it('defaults to the local backend on port 8080', () => {
    expect(resolveAdminDevApiProxyTarget({})).toBe(DEFAULT_ADMIN_DEV_API_PROXY_TARGET);
    expect(createAdminDevProxy({})['/api'].target).toBe('http://127.0.0.1:8080');
  });

  it('accepts an explicit loopback override', () => {
    const proxy = createAdminDevProxy({ ADMIN_DEV_API_PROXY_TARGET: 'http://localhost:8081/' });
    expect(proxy['/api'].target).toBe('http://localhost:8081');
    expect(proxy['/static'].target).toBe('http://localhost:8081');
  });

  it('keeps the Docker nginx proxy on the container backend address', () => {
    const nginxConfig = fs.readFileSync(path.join(process.cwd(), 'admin', 'nginx.conf'), 'utf8');
    expect(nginxConfig).toContain('proxy_pass http://backend:8080/api/;');
    expect(nginxConfig).toContain('proxy_pass http://backend:8080/health;');
    expect(nginxConfig).not.toContain('ADMIN_DEV_API_PROXY_TARGET');
  });

  it.each([
    'http://backend:8080',
    'ftp://127.0.0.1:8080',
    'http://user:secret@127.0.0.1:8080',
    'http://127.0.0.1:8080/api',
    'http://127.0.0.1',
  ])('rejects unsafe or ambiguous target %s', (target) => {
    expect(() => resolveAdminDevApiProxyTarget({ ADMIN_DEV_API_PROXY_TARGET: target })).toThrow();
  });
});

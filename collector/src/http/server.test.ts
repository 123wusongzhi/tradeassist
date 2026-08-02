import type { AddressInfo } from 'node:net';
import type { Server } from 'node:http';
import { afterEach, describe, expect, it } from 'vitest';
import type { BrowserManager } from '../browser/manager.js';
import { createCollectorServer } from './server.js';

const servers: Server[] = [];

afterEach(async () => {
  await Promise.all(
    servers.splice(0).map(
      (server) =>
        new Promise<void>((resolve) => {
          server.close(() => resolve());
        }),
    ),
  );
});

async function startServer(options: { token?: string; maxBodyBytes?: number } = {}) {
  const browser = {} as BrowserManager;
  const server = createCollectorServer(browser, options);
  servers.push(server);
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => resolve());
  });
  const { port } = server.address() as AddressInfo;
  return `http://127.0.0.1:${port}`;
}

describe('collector HTTP boundary', () => {
  it('keeps health public but requires the configured token for every v1 route', async () => {
    const baseURL = await startServer({ token: 'collector-test-token' });

    expect((await fetch(`${baseURL}/health`)).status).toBe(200);
    expect((await fetch(`${baseURL}/v1/providers`)).status).toBe(401);
    expect(
      (
        await fetch(`${baseURL}/v1/providers`, {
          headers: { Authorization: 'Bearer wrong-token' },
        })
      ).status,
    ).toBe(401);
    expect(
      (
        await fetch(`${baseURL}/v1/providers`, {
          headers: { Authorization: 'Bearer collector-test-token' },
        })
      ).status,
    ).toBe(200);
  });

  it('rejects oversized request bodies before dispatching a collection task', async () => {
    const baseURL = await startServer({ token: 'collector-test-token', maxBodyBytes: 64 });
    const response = await fetch(`${baseURL}/v1/collect`, {
      method: 'POST',
      headers: {
        Authorization: 'Bearer collector-test-token',
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ source: 'custom', url: `https://example.com/${'a'.repeat(200)}` }),
    });
    expect(response.status).toBe(413);
    await expect(response.json()).resolves.toMatchObject({
      error: { code: 'PAYLOAD_TOO_LARGE' },
    });
  });

  it('rejects invalid JSON with a stable client error', async () => {
    const baseURL = await startServer({ token: 'collector-test-token' });
    const response = await fetch(`${baseURL}/v1/collect`, {
      method: 'POST',
      headers: {
        Authorization: 'Bearer collector-test-token',
        'Content-Type': 'application/json',
      },
      body: '{',
    });
    expect(response.status).toBe(400);
    await expect(response.json()).resolves.toMatchObject({
      error: { code: 'INVALID_REQUEST' },
    });
  });

  it('rejects unsafe targets without echoing internal URL details', async () => {
    const baseURL = await startServer({ token: 'collector-test-token' });
    const response = await fetch(`${baseURL}/v1/collect`, {
      method: 'POST',
      headers: {
        Authorization: 'Bearer collector-test-token',
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ source: 'custom', url: 'http://169.254.169.254/latest/meta-data/' }),
    });
    expect(response.status).toBe(422);
    await expect(response.json()).resolves.toEqual({
      ok: false,
      error: { code: 'UNSAFE_TARGET_URL', message: 'target URL is not publicly routable' },
    });
  });
});

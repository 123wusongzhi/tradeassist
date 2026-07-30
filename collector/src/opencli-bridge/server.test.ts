import type { AddressInfo } from 'node:net';
import { afterEach, describe, expect, it } from 'vitest';
import type { NormalizedProduct } from '../types/product.js';
import { createOpenCliBridgeServer } from './server.js';

const servers: ReturnType<typeof createOpenCliBridgeServer>[] = [];

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

async function startServer(token = 'bridge-token') {
  let collectCalls = 0;
  const product: NormalizedProduct = {
    source: 'taobao_tmall',
    sourceUrl: 'https://item.taobao.com/item.htm?id=1',
    title: 'Bridge product',
    currency: 'CNY',
    mainImages: ['https://img.example/1.jpg'],
    descriptionImages: [],
    attributes: {},
    skus: [],
    raw: { engine: 'opencli' },
  };
  const server = createOpenCliBridgeServer(
    { host: '127.0.0.1', port: 0, token },
    {
      collect: async () => {
        collectCalls += 1;
        return product;
      },
      status: async () => ({
        ready: true,
        binaryAvailable: true,
        daemonRunning: true,
        extensionConnected: true,
        profileAvailable: true,
        message: 'ready',
      }),
    },
  );
  servers.push(server);
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address() as AddressInfo;
  return {
    baseURL: `http://127.0.0.1:${address.port}`,
    collectCalls: () => collectCalls,
  };
}

describe('OpenCLI Bridge HTTP isolation', () => {
  it('keeps liveness public but protects status and collect', async () => {
    const app = await startServer();
    const health = await fetch(`${app.baseURL}/health`);
    expect(health.status).toBe(200);

    const unauthorized = await fetch(`${app.baseURL}/v1/opencli/status`);
    expect(unauthorized.status).toBe(401);

    const status = await fetch(`${app.baseURL}/v1/opencli/status`, {
      headers: { Authorization: 'Bearer bridge-token' },
    });
    expect(status.status).toBe(200);
    expect(await status.json()).toMatchObject({ ok: true, data: { ready: true } });
  });

  it('accepts only taobao_tmall product URLs', async () => {
    const app = await startServer();
    const headers = {
      Authorization: 'Bearer bridge-token',
      'Content-Type': 'application/json',
    };
    const invalidShape = await fetch(`${app.baseURL}/v1/collect`, {
      method: 'POST',
      headers,
      body: 'null',
    });
    expect(invalidShape.status).toBe(400);

    const invalid = await fetch(`${app.baseURL}/v1/collect`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ source: '1688', url: 'https://detail.1688.com/offer/1.html' }),
    });
    expect(invalid.status).toBe(400);
    expect(app.collectCalls()).toBe(0);

    const valid = await fetch(`${app.baseURL}/v1/collect`, {
      method: 'POST',
      headers,
      body: JSON.stringify({
        source: 'taobao_tmall',
        url: 'https://item.taobao.com/item.htm?id=1',
      }),
    });
    expect(valid.status).toBe(200);
    expect(await valid.json()).toMatchObject({
      ok: true,
      data: { product: { title: 'Bridge product' } },
    });
    expect(app.collectCalls()).toBe(1);
  });
});

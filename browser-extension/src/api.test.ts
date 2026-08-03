import { afterEach, describe, expect, it, vi } from 'vitest';
import { TradeMindAPI } from './api.js';
import type { NormalizedProduct } from './types.js';

describe('TradeMind browser extension API', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('omits browser credentials from pairing and device requests', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) =>
      new Response(JSON.stringify({ code: 0, message: 'ok', data: {} }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);

    const pairingAPI = new TradeMindAPI('http://127.0.0.1:8000');
    await pairingAPI.exchangePairing('ABCDE-FGHJK', 'Test Chrome');

    const deviceAPI = new TradeMindAPI('http://127.0.0.1:8000', 'tmx_test_device_token');
    await deviceAPI.session();
    await deviceAPI.createTask('https://detail.tmall.com/item.htm?id=1');
    const product: NormalizedProduct = {
      source: 'taobao_tmall',
      sourceUrl: 'https://detail.tmall.com/item.htm?id=1',
      title: 'Test product',
      currency: 'CNY',
      mainImages: [],
      descriptionImages: [],
      attributes: {},
      skus: [],
      raw: {},
    };
    await deviceAPI.submitResult('task-id', product);
    await deviceAPI.submitFailure('task-id', 'TEST', 'test failure');

    expect(fetchMock).toHaveBeenCalledTimes(5);
    for (const [, init] of fetchMock.mock.calls) {
      expect(init).toEqual(expect.objectContaining({ credentials: 'omit' }));
      expect(new Headers(init?.headers).has('Cookie')).toBe(false);
    }
  });
});

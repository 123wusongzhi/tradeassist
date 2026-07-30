import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { batchCheckProductReadiness, getProductReadiness } from '../productReadiness';

const requestMock = vi.mocked(request);

describe('product readiness service', () => {
  it('defaults readiness mode to draft and encodes product IDs', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { productId: 'p/1', status: 'ready', score: 100, canPublish: true, errorCount: 0, warningCount: 0, checks: [] } });

    await getProductReadiness('p/1', { platform: 'douyin_shop', shopId: 'shop-1' });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/p%2F1/readiness', {
      method: 'GET',
      params: { platform: 'douyin_shop', shopId: 'shop-1', mode: 'draft' },
    });
  });

  it('posts batch readiness payloads to the shared contract endpoint', async () => {
    const payload = { productIds: ['p1', 'p2'], platform: 'douyin_shop', shopId: 'shop-1' };
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { list: [] } });

    await batchCheckProductReadiness(payload);

    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/readiness/batch', { method: 'POST', data: payload });
  });
});

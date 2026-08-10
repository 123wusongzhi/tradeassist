import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { requireSuccessfulShopConnection, testShopConnection } from '../shops';

const requestMock = vi.mocked(request);

describe('shop connection services', () => {
  it('rejects an HTTP-success response when the provider result is not OK', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { ok: false, message: 'API Key 已停用' },
    });

    await expect(testShopConnection('shop-1')).rejects.toThrow('API Key 已停用');
    expect(requestMock).toHaveBeenCalledWith('/api/v1/shops/shop-1/test-connection', {
      method: 'POST',
      data: {},
    });
  });

  it('preserves a successful provider result for the success message', async () => {
    const result = {
      ok: true,
      message: 'ozon connection ok',
      shopName: 'Verified shop',
      externalShopId: 'client-1',
    };

    expect(requireSuccessfulShopConnection(result)).toBe(result);
  });
});

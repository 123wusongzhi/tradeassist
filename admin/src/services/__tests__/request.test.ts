import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { ApiRequestError, getJSON, getWithParams, postJSON } from '../request';

const requestMock = vi.mocked(request);

describe('request helpers', () => {
  it('unwraps successful GET envelopes', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'p1' } });

    await expect(getJSON('/api/v1/products/p1')).resolves.toEqual({ id: 'p1' });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/p1', { method: 'GET' });
  });

  it('sends POST data through the backend envelope', async () => {
    const body = { title: '测试商品' };
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'created' } });

    await expect(postJSON('/api/v1/products', body)).resolves.toEqual({ id: 'created' });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/products', { method: 'POST', data: body });
  });

  it('passes query params without dropping undefined boundary keys', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { list: [] } });

    await getWithParams('/api/v1/products/p1/readiness', { platform: 'douyin_shop', shopId: undefined, mode: 'draft' });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/p1/readiness', {
      method: 'GET',
      params: { platform: 'douyin_shop', shopId: undefined, mode: 'draft' },
    });
  });

  it('throws backend business errors with message fallback', async () => {
    requestMock.mockResolvedValueOnce({ code: 40001, message: '商品不存在', data: null });

    await expect(getJSON('/api/v1/products/missing')).rejects.toThrow('商品不存在');
  });

  it('keeps the backend envelope when an HTTP error rejects before unwrap', async () => {
    requestMock.mockRejectedValueOnce({
      response: {
        data: {
          code: 40001,
          message: 'Ozon 店铺授权已失效或 API Key 已停用，请更新凭证后重试',
          data: { errorCode: 'OZON_CATEGORY_ATTR_SYNC_FAILED' },
          traceId: 'trace-ozon-1',
        },
      },
    });

    const failure = await getJSON(
      '/api/v1/platform/ozon/categories/100%3A200/attributes',
    ).catch((error) => error);
    expect(failure).toBeInstanceOf(ApiRequestError);
    expect(failure).toMatchObject({
      code: 40001,
      message: 'Ozon 店铺授权已失效或 API Key 已停用，请更新凭证后重试',
      traceId: 'trace-ozon-1',
      data: { errorCode: 'OZON_CATEGORY_ATTR_SYNC_FAILED' },
    });
  });
});

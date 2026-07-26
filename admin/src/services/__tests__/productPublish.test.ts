import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { createDouyinProductDraft, publishProduct } from '../productPublish';

const requestMock = vi.mocked(request);

describe('product publish services', () => {
  it('creates Douyin draft with safe draft publish mode by default', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { id: 'task-1', productId: 'p/1', shopId: 'shop-1', platform: 'douyin_shop', taskType: 'publish', status: 'pending', mode: 'draft', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z' },
    });

    await createDouyinProductDraft('p/1', { shopId: 'shop-1' });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/p%2F1/platform-configs/douyin_shop/create-draft', {
      method: 'POST',
      data: { publishMode: 'save_as_platform_draft', shopId: 'shop-1' },
    });
  });

  it('preserves explicit force flags for traditional publish requests', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { id: 'task-2', productId: 'p1', shopId: 'shop-1', platform: 'douyin_shop', taskType: 'publish', status: 'pending', mode: 'publish', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z' },
    });

    await publishProduct('p1', { shopId: 'shop-1', force: true, options: { dryRun: false } });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/p1/publish', {
      method: 'POST',
      data: { shopId: 'shop-1', force: true, options: { dryRun: false } },
    });
  });

  it('exposes backend business code and data on publish errors', async () => {
    requestMock.mockResolvedValueOnce({ code: 40901, message: '发布检查未通过', data: { blocked: true } });

    await expect(publishProduct('p1', { shopId: 'shop-1' })).rejects.toMatchObject({
      message: '发布检查未通过',
      businessCode: 40901,
      data: { blocked: true },
    });
  });
});

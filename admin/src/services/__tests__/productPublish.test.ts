import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  canReconcileOzonPublishTask,
  createDouyinProductDraft,
  extractOzonOfferIds,
  extractOzonWarnings,
  productPublishBusinessStatus,
  publishProduct,
  reconcileOzonPublishTask,
  type ProductPublishTaskDTO,
} from '../productPublish';

const requestMock = vi.mocked(request);

describe('product publish services', () => {
  const task = (overrides: Partial<ProductPublishTaskDTO> = {}): ProductPublishTaskDTO => ({
    id: 'task-1',
    productId: 'product-1',
    shopId: 'shop-1',
    platform: 'ozon',
    taskType: 'product_publish',
    status: 'succeeded',
    mode: 'publish',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  });

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

  it('only labels a verified sellable/published business state as successful', () => {
    expect(productPublishBusinessStatus(task({ publishStatus: 'imported' }))).toMatchObject({
      text: 'Ozon 已接收，待确认可售',
      successful: false,
    });
    expect(productPublishBusinessStatus(task({ publishStatus: 'needs_action' }))).toMatchObject({
      text: '需要修改',
      successful: false,
    });
    expect(
      productPublishBusinessStatus(
        task({
          status: 'failed',
          publishStatus: 'result_unknown',
          retryable: false,
        }),
      ),
    ).toMatchObject({
      requiresReconciliation: true,
      successful: false,
    });
    expect(productPublishBusinessStatus(task({ publishStatus: 'published' }))).toMatchObject({
      text: '历史上架状态待核对',
      requiresReconciliation: true,
      successful: false,
    });
    expect(canReconcileOzonPublishTask(task({ status: 'success', publishStatus: 'published' }))).toBe(false);
    expect(canReconcileOzonPublishTask(task({ status: 'failed', publishStatus: 'result_unknown' }))).toBe(true);
    expect(
      productPublishBusinessStatus(
        task({
          publishStatus: 'published',
          platformResult: { providerSummary: { sellableVerified: true } },
        }),
      ),
    ).toMatchObject({
      text: '成功上架',
      successful: true,
    });
    expect(productPublishBusinessStatus(task({ platform: 'douyin_shop', publishStatus: 'published' }))).toMatchObject({
      text: '成功上架',
      successful: true,
    });
    expect(productPublishBusinessStatus(task({ publishStatus: 'draft_created' }))).toMatchObject({
      text: '仅创建本地草稿',
      successful: false,
    });
  });

  it('extracts copyable offer ids and explicit Ozon warnings from nested provider data', () => {
    const row = task({
      platformPayload: {
        items: [{ offer_id: 'OFFER-1' }, { offerId: 'OFFER-2' }],
      },
      platformResult: {
        warnings: [{ message: '尺寸需要修正' }],
        items: [{ severity: 'warning', message: '重量需要修正' }],
      },
    });
    expect(extractOzonOfferIds(row)).toEqual(['OFFER-1', 'OFFER-2']);
    expect(extractOzonWarnings(row)).toEqual(['尺寸需要修正', '重量需要修正']);
  });

  it('posts an operator-confirmed reconciliation without automatically retrying', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: task({ recoveryState: 'confirmed_not_created', retryable: true }),
    });
    await reconcileOzonPublishTask('task/1', {
      outcome: 'platform_not_created',
      evidence: '已在 Ozon 后台按 offer_id 核对，未找到商品',
    });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/product-publish/tasks/task%2F1/reconcile-ozon', {
      method: 'POST',
      data: {
        outcome: 'platform_not_created',
        evidence: '已在 Ozon 后台按 offer_id 核对，未找到商品',
      },
    });
  });
});

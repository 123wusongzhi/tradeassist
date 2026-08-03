import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  buildOzonPlatformAttributes,
  confirmOzonCategoryGroup,
  publishOzonProduct,
  saveOzonProductConfig,
  syncOzonCategoryFlow,
  toOzonAttributeFormValues,
  validateOzonReadiness,
} from '../ozonPublish';

const requestMock = vi.mocked(request);

describe('ozon publish services', () => {
  it('keeps dictionary text together with its Ozon dictionary value id', () => {
    const attributes = [
      {
        attrId: 'brand',
        dictionaryId: 'dict-1',
        options: [{ id: '42', value: 'Acme' }],
      },
      { attrId: 'model' },
    ];
    expect(
      buildOzonPlatformAttributes(attributes, { brand: '42', model: 'A-1' }),
    ).toEqual({
      brand: { value: 'Acme', dictionaryValueId: '42' },
      model: { value: 'A-1' },
    });
    expect(
      toOzonAttributeFormValues(attributes, {
        brand: { value: 'Acme', dictionaryValueId: '42' },
        model: { value: 'A-1' },
      }),
    ).toEqual({ brand: '42', model: 'A-1' });
    expect(
      buildOzonPlatformAttributes(attributes, { brand: 'Legacy brand' }),
    ).toEqual({ brand: { value: 'Legacy brand' } });
  });

  it('saves product-level Ozon configuration without publishing', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: {
        id: 'config-1',
        productId: 'p/1',
        shopId: 'shop-1',
        categoryId: 'cat-1',
      },
    });
    const saved = await saveOzonProductConfig('p/1', {
      shopId: 'shop-1',
      categoryId: 'cat-1',
    });
    expect(saved.id).toBe('config-1');
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/products/p%2F1/platform-configs/ozon',
      {
        method: 'PUT',
        data: { shopId: 'shop-1', categoryId: 'cat-1' },
      },
    );
  });

  it('uses an explicit Ozon readiness request', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { canPublish: true },
    });
    await validateOzonReadiness('p1', 'shop-1');
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/products/p1/readiness/validate',
      {
        method: 'POST',
        data: { platform: 'ozon', shopId: 'shop-1' },
      },
    );
  });

  it('confirms category groups as local configuration only', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { groups: [] },
    });
    await confirmOzonCategoryGroup({
      shopId: 'shop-1',
      saveMappings: true,
      groups: [
        {
          sourceCategoryKey: 'desk',
          productIds: ['p1'],
          categoryId: '100:200',
          categoryPath: '桌子',
        },
      ],
    });
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/product-publish/ozon/category-groups/confirm',
      {
        method: 'POST',
        data: {
          shopId: 'shop-1',
          saveMappings: true,
          groups: [
            {
              sourceCategoryKey: 'desk',
              productIds: ['p1'],
              categoryId: '100:200',
              categoryPath: '桌子',
            },
          ],
        },
      },
    );
  });

  it('receives sync stats together with its asynchronous run identity', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: {
        stats: { activeCount: 1 },
        run: { id: 'run-1', status: 'pending' },
        runId: 'run-1',
      },
    });
    const result = await syncOzonCategoryFlow('shop-1');
    expect(result.runId).toBe('run-1');
    expect(result.run?.status).toBe('pending');
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/platform/ozon/categories/sync',
      { method: 'POST', data: { shopId: 'shop-1' } },
    );
  });

  it('sends a stable idempotency header for a real Ozon submit', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { id: 'task-1', status: 'pending' },
    });
    await publishOzonProduct('p1', 'shop-1', 'ozon-submit:123:abc');
    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/p1/publish', {
      method: 'POST',
      data: { shopId: 'shop-1', options: { platform: 'ozon' } },
      headers: { 'Idempotency-Key': 'ozon-submit:123:abc' },
    });
  });
});

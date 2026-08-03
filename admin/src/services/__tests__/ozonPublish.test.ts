import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  buildOzonPlatformAttributes,
  buildOzonSKUImagePreview,
  confirmOzonCategoryGroup,
  publishOzonProduct,
  saveOzonProductConfig,
  syncOzonCategoryFlow,
  toOzonImageConfigInput,
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
      data: { productId: 'p/1', shopId: 'shop-1', categoryId: 'cat-1' },
    });
    await saveOzonProductConfig('p/1', {
      shopId: 'shop-1',
      categoryId: 'cat-1',
    });
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/products/p%2F1/platform-configs/ozon',
      {
        method: 'PUT',
        data: { shopId: 'shop-1', categoryId: 'cat-1' },
      },
    );
  });

  it('builds stable per-SKU image order and serializes explicit selections', () => {
    const sharedImages = [
      {
        id: 'shared-1',
        url: 'https://example.test/shared.jpg',
        imageType: 'main',
        sortOrder: 1,
      },
      {
        id: 'duplicate-original',
        url: 'https://example.test/red.jpg',
        imageType: 'detail',
        sortOrder: 2,
      },
    ];
    const red = buildOzonSKUImagePreview(
      {
        skuId: 'sku-red',
        skuCode: 'RED',
        skuName: '红色',
        originalMainImageUrl: 'https://example.test/red.jpg',
        additionalImageIds: [
          'shared-1',
          'duplicate-original',
          'shared-1',
        ],
        finalImages: [],
        canPublish: false,
        issues: [],
      },
      sharedImages,
    );
    expect(red.finalImages.map((image) => image.url)).toEqual([
      'https://example.test/red.jpg',
      'https://example.test/shared.jpg',
    ]);
    expect(red.finalImages.map((image) => image.position)).toEqual([1, 2]);
    expect(red.canPublish).toBe(true);

    const missing = buildOzonSKUImagePreview(
      {
        skuId: 'sku-missing',
        skuName: '蓝色 / L',
        additionalImageIds: ['shared-1'],
        finalImages: [],
        canPublish: false,
        issues: [],
      },
      sharedImages,
    );
    expect(missing.canPublish).toBe(false);
    expect(missing.issues[0]).toMatchObject({
      code: 'OZON_SKU_MAIN_IMAGE_MISSING',
      skuId: 'sku-missing',
    });

    expect(toOzonImageConfigInput([red, missing])).toEqual({
      version: 1,
      skuSelections: [
        {
          skuId: 'sku-red',
          fallbackMainImageId: undefined,
          additionalImageIds: ['shared-1', 'duplicate-original'],
        },
        {
          skuId: 'sku-missing',
          fallbackMainImageId: undefined,
          additionalImageIds: ['shared-1'],
        },
      ],
    });
  });

  it('sends the versioned Ozon SKU image config without invoking publish', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { productId: 'p1' },
    });
    await saveOzonProductConfig('p1', {
      shopId: 'shop-1',
      categoryId: '100:200',
      ozonImages: {
        version: 1,
        skuSelections: [
          {
            skuId: 'sku-1',
            fallbackMainImageId: 'image-1',
            additionalImageIds: ['image-2'],
          },
        ],
      },
    });
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/products/p1/platform-configs/ozon',
      {
        method: 'PUT',
        data: {
          shopId: 'shop-1',
          categoryId: '100:200',
          ozonImages: {
            version: 1,
            skuSelections: [
              {
                skuId: 'sku-1',
                fallbackMainImageId: 'image-1',
                additionalImageIds: ['image-2'],
              },
            ],
          },
        },
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

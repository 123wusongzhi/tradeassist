import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  getOzonAttributeMappings,
  putOzonAttributeMappings,
  queryOzonCategories,
  syncOzonCategories,
  syncOzonCategoryAttributes,
} from '../ozonCategories';

const requestMock = vi.mocked(request);

function envelope<T>(data: T) {
  return { code: 0, message: 'ok', data };
}

describe('ozon category services', () => {
  it('queries leaf categories with query params', async () => {
    requestMock.mockResolvedValueOnce(
      envelope({ list: [{ id: 'cat-1', categoryId: '100:200', name: 'Стол', level: 2, isLeaf: true }], total: 1, leafCount: 1 }),
    );
    const res = await queryOzonCategories({ keyword: 'Стол', onlyLeaf: true, limit: 200 });
    expect(res.leafCount).toBe(1);
    expect(requestMock).toHaveBeenCalledWith('/api/v1/platform/ozon/categories', {
      method: 'GET',
      params: { keyword: 'Стол', onlyLeaf: '1', limit: '200' },
    });
  });

  it('syncs category tree with optional shop id', async () => {
    requestMock.mockResolvedValueOnce(envelope({ count: 2, leafCount: 1 }));
    await syncOzonCategories('shop-1');
    expect(requestMock).toHaveBeenCalledWith('/api/v1/platform/ozon/categories/sync', {
      method: 'POST',
      data: { shopId: 'shop-1' },
    });
    requestMock.mockResolvedValueOnce(envelope({ count: 2, leafCount: 1 }));
    await syncOzonCategories();
    expect(requestMock).toHaveBeenLastCalledWith('/api/v1/platform/ozon/categories/sync', {
      method: 'POST',
      data: {},
    });
  });

  it('syncs attributes for a leaf category', async () => {
    requestMock.mockResolvedValueOnce(envelope({ count: 3 }));
    await syncOzonCategoryAttributes('cat-1');
    expect(requestMock).toHaveBeenCalledWith('/api/v1/platform/ozon/categories/cat-1/attributes/sync', {
      method: 'POST',
      data: {},
    });
  });

  it('round-trips attribute mappings via PUT', async () => {
    requestMock.mockResolvedValueOnce(envelope({ list: [{ attributeId: '85', localField: 'brand', enabled: true }] }));
    const got = await getOzonAttributeMappings('cat-1');
    expect(got.list[0].localField).toBe('brand');
    requestMock.mockResolvedValueOnce(
      envelope({ list: [{ attributeId: '85', attributeName: 'Бренд', localField: 'brand_name', enabled: true }] }),
    );
    const saved = await putOzonAttributeMappings('cat-1', [
      { attributeId: '85', attributeName: 'Бренд', localField: 'brand_name', enabled: true },
    ]);
    expect(saved.list[0].localField).toBe('brand_name');
    expect(requestMock).toHaveBeenLastCalledWith('/api/v1/platform/ozon/categories/cat-1/attribute-mappings', {
      method: 'PUT',
      data: { items: [{ attributeId: '85', attributeName: 'Бренд', localField: 'brand_name', enabled: true }] },
    });
  });
});

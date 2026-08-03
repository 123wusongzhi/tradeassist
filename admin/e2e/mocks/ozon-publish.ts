import { ok } from './envelope';
import { E2E_PRODUCT_ID } from './product.fixture';

export const E2E_OZON_SHOP_ID = 'e2e-ozon-shop';
export const E2E_OZON_CATEGORY_ID = '100:200';

export const e2eOzonStats = {
  activeCount: 2,
  inactiveCount: 1,
  lastSyncedAt: '2026-08-03T00:00:00Z',
  lastRun: {
    id: 'e2e-ozon-sync-run',
    status: 'pending',
    statusLabel: '任务已创建，等待处理',
    summary: { added: 1, changed: 1, deactivated: 1, reactivated: 1 },
  },
  diffCounts: { added: 1, changed: 1, deactivated: 1, reactivated: 1 },
};

export const e2eOzonChanges = [
  {
    id: 'e2e-ozon-change-added',
    changeType: 'added',
    categoryName: '桌子',
    occurredAt: '2026-08-03T00:00:00Z',
    detail: '新增叶子类目',
  },
  {
    id: 'e2e-ozon-change-changed',
    changeType: 'changed',
    categoryName: '办公桌',
    occurredAt: '2026-08-03T00:00:00Z',
    detail: '属性模板已更新',
  },
  {
    id: 'e2e-ozon-change-deactivated',
    changeType: 'deactivated',
    categoryName: '旧桌子',
    occurredAt: '2026-08-03T00:00:00Z',
    detail: '已停用，保留审计记录',
  },
  {
    id: 'e2e-ozon-change-reactivated',
    changeType: 'reactivated',
    categoryName: '恢复桌子',
    occurredAt: '2026-08-03T00:00:00Z',
    detail: '已恢复使用',
  },
];

export const e2eOzonConfig = {
  productId: E2E_PRODUCT_ID,
  shopId: E2E_OZON_SHOP_ID,
  categoryId: E2E_OZON_CATEGORY_ID,
  categoryPath: '家具 / 桌子',
  sourceCategoryKey: 'e2e-source-table',
  sourceCategoryName: 'E2E 本地桌子',
  platformAttributes: {
    '85': { value: 'E2E' },
    '86': { value: '白色', dictionaryValueId: '1001' },
  },
  schemaHash: 'e2e-schema-v1',
  schemaConfirmedAt: '2026-08-03T00:00:00Z',
  ozonImages: {
    version: 1,
    configured: true,
    maxImagesPerSku: 10,
    sharedImages: [
      {
        id: 'e2e-ozon-shared-1',
        url: 'https://example.test/ozon-shared-1.jpg',
        imageType: 'main',
        sortOrder: 1,
      },
      {
        id: 'e2e-ozon-shared-2',
        url: 'https://example.test/ozon-shared-2.jpg',
        imageType: 'detail',
        sortOrder: 2,
      },
    ],
    skus: [
      {
        skuId: 'e2e-ozon-sku-red',
        skuCode: 'OZON-RED-M',
        skuName: '红色 / M',
        attrs: { 颜色: '红色', 尺码: 'M' },
        originalMainImageUrl: 'https://example.test/ozon-red.jpg',
        additionalImageIds: ['e2e-ozon-shared-1'],
        finalImages: [
          {
            url: 'https://example.test/ozon-red.jpg',
            source: 'sku_original',
            position: 1,
            imageType: 'main',
          },
          {
            imageId: 'e2e-ozon-shared-1',
            url: 'https://example.test/ozon-shared-1.jpg',
            source: 'product_shared',
            position: 2,
            imageType: 'detail',
          },
        ],
        canPublish: true,
        issues: [],
      },
      {
        skuId: 'e2e-ozon-sku-blue',
        skuCode: 'OZON-BLUE-L',
        skuName: '蓝色 / L',
        attrs: { 颜色: '蓝色', 尺码: 'L' },
        originalMainImageUrl: 'https://example.test/ozon-blue.jpg',
        additionalImageIds: [],
        finalImages: [
          {
            url: 'https://example.test/ozon-blue.jpg',
            source: 'sku_original',
            position: 1,
            imageType: 'main',
          },
        ],
        canPublish: true,
        issues: [],
      },
    ],
    issues: [],
    errorCount: 0,
  },
};

export function ozonPublishResponse(path: string) {
  const decodedPath = decodeURIComponent(path);
  if (decodedPath === '/api/v1/platform/ozon/categories/stats')
    return ok(e2eOzonStats);
  if (decodedPath === '/api/v1/platform/ozon/categories/sync-runs')
    return ok({ list: [e2eOzonStats.lastRun] });
  if (decodedPath === '/api/v1/platform/ozon/categories/changes')
    return ok({ list: e2eOzonChanges });
  if (decodedPath === '/api/v1/platform/ozon/category-mappings')
    return ok({
      list: [{ id: 'e2e-ozon-map', ...e2eOzonConfig, status: 'confirmed' }],
    });
  if (decodedPath === `/api/v1/products/${E2E_PRODUCT_ID}/platform-configs/ozon`)
    return ok(e2eOzonConfig);
  if (decodedPath === '/api/v1/platform/ozon/categories')
    return ok({
      list: [
        {
          id: E2E_OZON_CATEGORY_ID,
          name: '桌子',
          descriptionCategoryId: '100',
          typeId: '200',
          isLeaf: true,
          status: 'active',
        },
      ],
    });
  if (
    decodedPath ===
    `/api/v1/platform/ozon/categories/${E2E_OZON_CATEGORY_ID}/attributes/86/values`
  )
    return ok({
      list: [
        { id: '1001', value: '白色' },
        { id: '1002', value: '黑色' },
      ],
    });
  if (
    decodedPath ===
    `/api/v1/platform/ozon/categories/${E2E_OZON_CATEGORY_ID}/attributes`
  )
    return ok({
      list: [
        {
          id: 'e2e-attr-brand',
          categoryId: E2E_OZON_CATEGORY_ID,
          attrId: '85',
          name: '品牌',
          required: true,
          valueType: 'string',
        },
        {
          id: 'e2e-attr-color',
          categoryId: E2E_OZON_CATEGORY_ID,
          attrId: '86',
          name: '颜色',
          required: true,
          valueType: 'dictionary',
          dictionaryId: 'colors',
          options: [{ id: '1001', value: '白色' }],
        },
      ],
    });
  return null;
}

import { ok } from './envelope';
import { E2E_PRODUCT_ID, E2E_PUBLICATION_OLD, E2E_PUBLICATION_NEW, E2E_SHOP_ID, publication } from './product.fixture';

export const e2ePlatformProviders = [
  {
    platform: 'douyin_shop',
    name: '抖店',
    label: '抖店',
    status: 'available',
    authType: 'oauth',
    capabilities: ['product_publish', 'inventory_sync'],
    authSchema: [],
    appConfigSchema: {
      groupKey: 'platform_douyin_shop',
      title: '抖店接入设置',
      description: '配置抖店开放平台应用信息。',
      fields: [],
    },
    settingsGroupKey: 'platform_douyin_shop',
    capabilityStatus: { product_publish: 'available', inventory_sync: 'available' },
  },
  {
    platform: 'ozon',
    name: 'Ozon',
    label: 'Ozon',
    status: 'beta',
    authType: 'api_key',
    capabilities: ['product_publish', 'shop_info'],
    authSchema: [
      { name: 'appKey', label: 'Client-ID', type: 'text', required: true, sensitive: false },
      { name: 'accessToken', label: 'Api-Key', type: 'password', required: true, sensitive: true },
    ],
    appConfigSchema: { groupKey: '', title: '', fields: [] },
    settingsGroupKey: '',
    capabilityStatus: { product_publish: 'beta' },
  },
];

export const e2eShops = [
  {
    id: E2E_SHOP_ID,
    platform: 'douyin_shop',
    shopName: 'E2E 抖店测试店铺',
    status: 'active',
    authStatus: 'authorized',
    capabilities: { product_publish: 'available', inventory_sync: 'available' },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 'e2e-ozon-shop',
    platform: 'ozon',
    shopName: 'E2E Ozon 测试店铺',
    status: 'active',
    authStatus: 'authorized',
    capabilities: { product_publish: 'beta' },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

export const e2ePublishTargets = {
  productId: E2E_PRODUCT_ID,
  platforms: [
    {
      platform: 'douyin_shop',
      platformLabel: '抖店',
      capability: 'real_draft_create',
      capabilityLabel: '可创建平台草稿',
      settingsPath: '/settings/platforms',
      shops: [
        { shopId: E2E_SHOP_ID, shopName: 'E2E 抖店测试店铺', authStatus: 'authorized', authStatusLabel: '已授权', enabled: true },
      ],
    },
    {
      platform: 'ozon',
      platformLabel: 'Ozon',
      capability: 'local_draft_only',
      capabilityLabel: '仅生成本地草稿',
      settingsPath: '/settings/platform-publish',
      shops: [
        { shopId: 'e2e-ozon-shop', shopName: 'E2E Ozon 测试店铺', authStatus: 'authorized', authStatusLabel: '已授权', enabled: true },
      ],
    },
  ],
};

export const e2eDouyinConfig = {
  platform: 'douyin_shop',
  shopId: E2E_SHOP_ID,
  categoryId: 'e2e-douyin-category',
  categoryPath: ['服饰内衣', '女装', 'T恤'],
  attrs: { brand: 'E2E' },
  updatedAt: '2026-01-01T00:00:00Z',
};

export const e2eDouyinMapping = {
  productId: E2E_PRODUCT_ID,
  platform: 'douyin_shop',
  title: 'E2E 商品草稿长标题用于回归测试响应式和换行能力',
  categoryId: 'e2e-douyin-category',
  attrs: { brand: 'E2E' },
  mainImages: [{ imageKey: 'main-1', url: 'https://example.test/e2e-main.jpg', uploaded: true, uploadStatus: 'uploaded', platformImageId: 'e2e-platform-image' }],
  detailImages: [{ imageKey: 'detail-1', url: 'https://example.test/e2e-detail.jpg', uploaded: true, uploadStatus: 'uploaded', platformImageId: 'e2e-platform-detail' }],
  skus: [{ skuId: 'e2e-sku-1', skuCode: 'E2E-SKU-1', platformSkuId: 'e2e-platform-sku-1' }],
  errors: [],
  warnings: [],
};

export function publishResponse(path: string) {
  if (path === '/api/v1/platform/providers') return ok({ list: e2ePlatformProviders });
  if (path === '/api/v1/shops') return ok({ list: e2eShops, pagination: { page: 1, pageSize: 500, total: 1, totalPages: 1 } });
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/publish-targets`) return ok(e2ePublishTargets);
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/publications`) return ok({ list: [publication(E2E_PUBLICATION_OLD)] });
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/platform-configs/douyin_shop`) return ok(e2eDouyinConfig);
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/platform-configs/douyin_shop/mapping`) return ok(e2eDouyinMapping);
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/platform-configs/douyin_shop/publish-tasks`) return ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } });
  return null;
}

export function latestPublicationsResponse() {
  return ok({ list: [publication(E2E_PUBLICATION_NEW, 'e2e-platform-product-new')] });
}

export function skuBindingsResponse(publicationId = E2E_PUBLICATION_OLD) {
  return ok({
    publicationId,
    externalProductId: publicationId === E2E_PUBLICATION_NEW ? 'e2e-platform-product-new' : 'e2e-platform-product-old',
    total: 1,
    bound: 1,
    skipped: 0,
    unmatched: 0,
    ambiguous: 0,
    failed: 0,
    rows: [
      {
        publicationSkuId: `${publicationId}-sku`,
        productSkuId: 'e2e-sku-1',
        skuCode: 'E2E-SKU-1',
        specName: '蓝色 / M',
        status: 'bound',
        platformSkuId: 'e2e-platform-sku-1',
        price: 129.9,
        stock: 88,
      },
    ],
    platformSkus: [],
    inventorySyncReady: true,
  });
}

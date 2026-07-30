export const E2E_PRODUCT_ID = 'e2e-product-draft';
export const E2E_SHOP_ID = 'e2e-shop-douyin';
export const E2E_PUBLICATION_OLD = 'e2e-publication-old';
export const E2E_PUBLICATION_NEW = 'e2e-publication-new';

export const e2eProduct = {
  id: E2E_PRODUCT_ID,
  tenantId: 1,
  createdBy: 'e2e-user',
  source: 'custom',
  sourceUrl: 'https://example.test/products/e2e-product-draft',
  originalTitle: 'E2E 原始商品标题',
  title: 'E2E 商品草稿长标题用于回归测试响应式和换行能力',
  aiTitle: 'E2E AI 优化标题',
  description: 'E2E 商品描述，包含足够长的文本用于验证页面布局不会横向溢出。',
  aiDescription: 'E2E AI 商品描述。',
  currency: 'CNY',
  status: 'draft',
  rawData: { raw: { productPrice: 129.9, attributeCandidates: { 材质: '棉', 颜色: '蓝色' } } },
  mainImages: ['https://example.test/e2e-main.jpg'],
  descriptionImages: ['https://example.test/e2e-detail.jpg'],
  attributes: { 材质: '棉', 颜色: '蓝色' },
  costPrice: 70,
  salePrice: 129.9,
  images: [
    {
      id: 'e2e-image-main',
      productId: E2E_PRODUCT_ID,
      imageType: 'main',
      source: 'mock',
      originUrl: 'https://example.test/e2e-main.jpg',
      publicUrl: 'https://example.test/e2e-main.jpg',
      sortOrder: 1,
      score: 95,
      isBestMain: true,
    },
  ],
  skus: [
    {
      id: 'e2e-sku-1',
      productId: E2E_PRODUCT_ID,
      skuCode: 'E2E-SKU-1',
      skuName: '蓝色 / M',
      attrs: { 颜色: '蓝色', 尺码: 'M' },
      price: 129.9,
      costPrice: 70,
      stock: 88,
      warningStock: 10,
      safetyStock: 5,
      stockStatus: 'normal',
      imageUrl: 'https://example.test/e2e-main.jpg',
    },
  ],
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

export const e2eProductList = [
  {
    id: E2E_PRODUCT_ID,
    title: e2eProduct.title,
    source: 'custom',
    status: 'draft',
    currency: 'CNY',
    salePrice: 129.9,
    imageUrl: 'https://example.test/e2e-main.jpg',
    skuCount: 1,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

export const e2eProgress = {
  productId: E2E_PRODUCT_ID,
  completionPercent: 95,
  currentStep: 'publish',
  currentStepLabel: '准备刊登',
  nextActionLabel: '检查并创建草稿',
  nextActionKey: 'publish',
  nextActionUrl: `/product/drafts/${E2E_PRODUCT_ID}?tab=publish`,
  blockerCount: 0,
  warningCount: 0,
  publishReady: true,
  completedSteps: ['basic', 'ai', 'images', 'skus'],
  pendingSteps: ['publish'],
  blockers: [],
  warnings: [],
  updatedAt: '2026-01-01T00:00:00Z',
  stepStatus: { basic: 'done', ai: 'done', images: 'done', skus: 'done', publish: 'ready' },
};

export const e2eReadinessPassed = {
  productId: E2E_PRODUCT_ID,
  platform: 'douyin_shop',
  shopId: E2E_SHOP_ID,
  mode: 'draft',
  status: 'passed',
  statusLabel: '检查通过',
  result: 'passed',
  resultLabel: '通过',
  canPublish: true,
  errorCount: 0,
  warningCount: 0,
  checks: [],
};

export const e2eReadinessFailed = {
  ...e2eReadinessPassed,
  status: 'failed',
  statusLabel: '检查未通过',
  result: 'failed',
  resultLabel: '未通过',
  canPublish: false,
  errorCount: 1,
  warningCount: 0,
  checks: [
    {
      group: 'basic',
      code: 'missing_title',
      title: '标题缺失',
      level: 'error',
      message: '商品标题缺失，不能正式刊登。',
      suggestion: '补充商品标题后重新检查。',
    },
  ],
};

export function publication(id = E2E_PUBLICATION_OLD, externalProductId = 'e2e-platform-product-old') {
  return {
    id,
    productId: E2E_PRODUCT_ID,
    shopId: E2E_SHOP_ID,
    shopName: 'E2E 抖店测试店铺',
    platform: 'douyin_shop',
    publishStatus: 'draft_created',
    externalProductId,
    externalUrl: '',
    skuBindingSyncedAt: '2026-01-01T00:00:00Z',
    skuMappingsSummary: ['已绑定 1 个规格'],
  };
}

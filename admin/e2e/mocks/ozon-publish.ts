import { ok } from "./envelope";
import { E2E_PRODUCT_ID } from "./product.fixture";

export const E2E_OZON_SHOP_ID = "e2e-ozon-shop";
export const E2E_OZON_CATEGORY_ID = "100:200";

export const e2eOzonStats = {
  activeCount: 2,
  inactiveCount: 1,
  lastSyncedAt: "2026-08-03T00:00:00Z",
  lastRun: {
    id: "e2e-ozon-sync-run",
    status: "pending",
    statusLabel: "任务已创建，等待处理",
    summary: { added: 1, changed: 1, deactivated: 1, reactivated: 1 },
  },
  diffCounts: { added: 1, changed: 1, deactivated: 1, reactivated: 1 },
};

export const e2eOzonChanges = [
  {
    id: "e2e-ozon-change-added",
    changeType: "added",
    categoryName: "桌子",
    occurredAt: "2026-08-03T00:00:00Z",
    detail: "新增叶子类目",
  },
  {
    id: "e2e-ozon-change-changed",
    changeType: "changed",
    categoryName: "办公桌",
    occurredAt: "2026-08-03T00:00:00Z",
    detail: "属性模板已更新",
  },
  {
    id: "e2e-ozon-change-deactivated",
    changeType: "deactivated",
    categoryName: "旧桌子",
    occurredAt: "2026-08-03T00:00:00Z",
    detail: "已停用，保留审计记录",
  },
  {
    id: "e2e-ozon-change-reactivated",
    changeType: "reactivated",
    categoryName: "恢复桌子",
    occurredAt: "2026-08-03T00:00:00Z",
    detail: "已恢复使用",
  },
];

export const e2eOzonConfig = {
  id: "e2e-ozon-config",
  productId: E2E_PRODUCT_ID,
  shopId: E2E_OZON_SHOP_ID,
  categoryId: E2E_OZON_CATEGORY_ID,
  categoryPath: "家具 / 桌子",
  sourceCategoryKey: "e2e-source-table",
  sourceCategoryName: "E2E 本地桌子",
  platformAttributes: {
    version: 3,
    attributes: {
      "85": [{ value: "E2E" }],
      "86": [
        { value: "白色", dictionaryValueId: "1001" },
        { value: "黑色", dictionaryValueId: "1002" },
      ],
    },
    complexGroups: [
      { complexId: 501, attributes: { "87": [{ value: "棉" }] } },
    ],
    skuVariantAttributeIds: [],
    skuAttributeOverrides: {},
  },
  schemaHash: "e2e-schema-v1",
  schemaConfirmedAt: "2026-08-03T00:00:00Z",
  ozonImages: {
    version: 1,
    configured: true,
    maxImagesPerSku: 10,
    sharedImages: [
      {
        id: "e2e-ozon-shared-1",
        url: "https://example.test/ozon-shared-1.jpg",
        imageType: "main",
        sortOrder: 1,
      },
      {
        id: "e2e-ozon-shared-2",
        url: "https://example.test/ozon-shared-2.jpg",
        imageType: "detail",
        sortOrder: 2,
      },
    ],
    skus: [
      {
        skuId: "e2e-sku-1",
        skuCode: "E2E-SKU-1",
        skuName: "蓝色 / M",
        attrs: { 颜色: "蓝色", 尺码: "M" },
        originalMainImageUrl: "https://example.test/e2e-main.jpg",
        additionalImageIds: ["e2e-ozon-shared-1"],
        finalImages: [
          {
            url: "https://example.test/e2e-main.jpg",
            source: "sku_original",
            position: 1,
            imageType: "main",
          },
          {
            imageId: "e2e-ozon-shared-1",
            url: "https://example.test/ozon-shared-1.jpg",
            source: "product_shared",
            position: 2,
            imageType: "detail",
          },
        ],
        canPublish: true,
        issues: [],
      },
    ],
    issues: [],
    errorCount: 0,
  },
  ozonListing: {
    version: 1,
    titleOverride: "E2E Ozon 店铺标题",
    descriptionOverride: "E2E Ozon 店铺描述",
    currencyCode: "RUB",
    skuPriceOverrides: { "e2e-sku-1": 1990 },
    package: {
      weightG: 500,
      widthMm: 200,
      heightMm: 100,
      depthMm: 300,
      warehouseId: "9001",
      vat: "0.2",
    },
  },
  ozonPreview: {
    productId: E2E_PRODUCT_ID,
    shopId: E2E_OZON_SHOP_ID,
    categoryId: E2E_OZON_CATEGORY_ID,
    categoryPath: "家具 / 桌子",
    schemaHash: "e2e-schema-v1",
    title: { value: "E2E Ozon 店铺标题", source: "ozon_product_shop_config" },
    description: {
      value: "E2E Ozon 店铺描述",
      source: "ozon_product_shop_config",
    },
    currency: { value: "RUB", source: "ozon_product_shop_config" },
    package: {
      weightG: { value: 500, source: "ozon_product_shop_config" },
      widthMm: { value: 200, source: "ozon_product_shop_config" },
      heightMm: { value: 100, source: "ozon_product_shop_config" },
      depthMm: { value: 300, source: "ozon_product_shop_config" },
      warehouseId: { value: "9001", source: "ozon_product_shop_config" },
      vat: { value: "0.2", source: "ozon_product_shop_config" },
    },
    skus: [
      {
        skuId: "e2e-sku-1",
        skuCode: "E2E-SKU-1",
        skuName: "蓝色 / M",
        price: { value: 1990, source: "ozon_product_shop_config" },
        localStock: 88,
        stockSource: "local_inventory",
        images: [
          {
            url: "https://example.test/e2e-main.jpg",
            source: "sku_original",
            position: 1,
            imageType: "main",
          },
          {
            imageId: "e2e-ozon-shared-1",
            url: "https://example.test/ozon-shared-1.jpg",
            source: "product_shared",
            position: 2,
            imageType: "detail",
          },
        ],
        canSubmit: true,
        issues: [],
      },
    ],
    issues: [],
    errorCount: 0,
    canSubmit: true,
  },
  legacyFallback: false,
};

export function ozonPublishResponse(
  path: string,
  searchParams?: URLSearchParams,
) {
  const decodedPath = decodeURIComponent(path);
  if (decodedPath === "/api/v1/platform/ozon/categories/stats")
    return ok(e2eOzonStats);
  if (decodedPath === "/api/v1/platform/ozon/categories/sync-runs")
    return ok({ list: [e2eOzonStats.lastRun] });
  if (decodedPath === "/api/v1/platform/ozon/categories/changes")
    return ok({ list: e2eOzonChanges });
  if (decodedPath === "/api/v1/platform/ozon/category-mappings")
    return ok({
      list: [
        {
          id: "e2e-ozon-map",
          ...e2eOzonConfig,
          status: "active",
          descriptionCategoryId: "100",
          typeId: "200",
          scope: "shop",
          selectionMethod: "manual",
          confirmationReason: "商品用途、材质和规格与桌子叶子类目一致",
          templateSyncedAt: "2026-08-10T00:00:00Z",
          confirmedAt: "2026-08-03T00:00:00Z",
        },
      ],
    });
  if (decodedPath === "/api/v1/platform/ozon/warehouses")
    return ok({
      list: [
        {
          id: "9001",
          name: "E2E 莫斯科 FBS 仓",
          isRfbs: false,
          isKgt: false,
          economy: false,
        },
      ],
    });
  if (
    decodedPath === `/api/v1/products/${E2E_PRODUCT_ID}/platform-configs/ozon`
  )
    return ok(e2eOzonConfig);
  if (decodedPath === "/api/v1/platform/ozon/categories") {
    const rootOnly = searchParams?.get("rootOnly") === "1";
    const parentId = searchParams?.get("parentId") || "";
    const onlyLeaf = searchParams?.get("onlyLeaf") === "1";
    const root = {
      id: "e2e-ozon-root",
      categoryId: "100",
      parentId: "",
      name: "家具",
      path: "家具",
      level: 1,
      isLeaf: false,
      hasChildren: true,
      childCount: 1,
      status: "active",
      syncedAt: "2026-08-10T00:00:00Z",
    };
    const leaf = {
      id: E2E_OZON_CATEGORY_ID,
      categoryId: E2E_OZON_CATEGORY_ID,
      parentId: "100",
      name: "桌子",
      path: "家具 / 桌子",
      descriptionCategoryId: "100",
      typeId: "200",
      level: 2,
      isLeaf: true,
      hasChildren: false,
      childCount: 0,
      ancestors: [{ categoryId: "100", name: "家具", level: 1 }],
      status: "active",
      syncedAt: "2026-08-10T00:00:00Z",
    };
    const list = rootOnly
      ? [root]
      : parentId === "100" || onlyLeaf
        ? [leaf]
        : [root, leaf];
    return ok({
      list,
      total: 20000,
      leafCount: 18000,
      matchedTotal: list.length,
      offset: 0,
      limit: 50,
      lastSyncedAt: "2026-08-10T00:00:00Z",
      cacheStale: false,
    });
  }
  if (
    decodedPath ===
    `/api/v1/platform/ozon/categories/${E2E_OZON_CATEGORY_ID}/attributes/86/values`
  )
    return ok({
      list: [
        { id: "1001", value: "白色" },
        { id: "1002", value: "黑色" },
      ],
    });
  if (
    decodedPath ===
    `/api/v1/platform/ozon/categories/${E2E_OZON_CATEGORY_ID}/attributes`
  )
    return ok({
      list: [
        {
          id: "e2e-attr-brand",
          categoryId: E2E_OZON_CATEGORY_ID,
          attrId: "85",
          name: "品牌",
          description: "商品品牌，不用于区分 SKU",
          required: true,
          valueType: "string",
          skuVariantEligible: false,
          skuVariantEligibilityKnown: true,
          isCollection: false,
          maxValueCount: 1,
          attributeComplexId: 0,
          complexIsCollection: false,
        },
        {
          id: "e2e-attr-color",
          categoryId: E2E_OZON_CATEGORY_ID,
          attrId: "86",
          name: "颜色",
          description: "Ozon 明确允许区分 SKU 的颜色属性",
          required: true,
          valueType: "dictionary",
          dictionaryId: "colors",
          skuVariantEligible: true,
          skuVariantEligibilityKnown: true,
          isCollection: true,
          maxValueCount: 2,
          attributeComplexId: 0,
          complexIsCollection: false,
          options: [
            { id: "1001", value: "白色" },
            { id: "1002", value: "黑色" },
          ],
        },
        {
          id: "e2e-attr-material",
          categoryId: E2E_OZON_CATEGORY_ID,
          attrId: "87",
          name: "材质组合",
          required: true,
          valueType: "string",
          skuVariantEligible: false,
          skuVariantEligibilityKnown: true,
          isCollection: false,
          maxValueCount: 1,
          attributeComplexId: 501,
          complexIsCollection: true,
        },
        {
          id: "e2e-attr-capacity",
          categoryId: E2E_OZON_CATEGORY_ID,
          attrId: "20001",
          name: "容量",
          description: "仅接受整数",
          required: false,
          valueType: "Integer",
          skuVariantEligible: false,
          skuVariantEligibilityKnown: true,
          isCollection: false,
          maxValueCount: 1,
          attributeComplexId: 0,
          complexIsCollection: false,
        },
        {
          id: "e2e-attr-polarized",
          categoryId: E2E_OZON_CATEGORY_ID,
          attrId: "30002",
          name: "是否偏光",
          required: false,
          valueType: "Boolean",
          skuVariantEligible: false,
          skuVariantEligibilityKnown: true,
          isCollection: false,
          maxValueCount: 1,
          attributeComplexId: 0,
          complexIsCollection: false,
        },
        {
          id: "e2e-attr-url",
          categoryId: E2E_OZON_CATEGORY_ID,
          attrId: "30003",
          name: "商品链接",
          required: false,
          valueType: "URL",
          skuVariantEligible: false,
          skuVariantEligibilityKnown: true,
          isCollection: false,
          maxValueCount: 1,
          attributeComplexId: 0,
          complexIsCollection: false,
        },
        {
          id: "e2e-attr-unknown-aspect",
          categoryId: E2E_OZON_CATEGORY_ID,
          attrId: "40001",
          name: "待确认规格",
          required: false,
          valueType: "String",
          skuVariantEligible: false,
          skuVariantEligibilityKnown: false,
          isCollection: false,
          maxValueCount: 1,
          attributeComplexId: 0,
          complexIsCollection: false,
        },
        {
          id: "e2e-attr-unsupported-type",
          categoryId: E2E_OZON_CATEGORY_ID,
          attrId: "40002",
          name: "平台特殊规格",
          required: false,
          valueType: "OzonDimension",
          skuVariantEligible: true,
          skuVariantEligibilityKnown: true,
          isCollection: false,
          maxValueCount: 1,
          attributeComplexId: 0,
          complexIsCollection: false,
        },
      ],
      variantPolicy: {
        maxSkuCount: 100,
        maxVariantAttributeCount: 2,
        maxVariantCombinationCount: 100,
        eligibleAttributeCount: 2,
        variantEligibilityFullyKnown: false,
        source: "ozon_is_aspect+trademind_import_guardrail",
      },
    });
  return null;
}

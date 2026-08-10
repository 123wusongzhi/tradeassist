import type { Page } from "@playwright/test";
import { expect, test } from "../fixtures/admin.fixture";
import { e2eUser } from "../mocks/auth";
import { fail, ok } from "../mocks/envelope";
import { e2eProduct, E2E_PRODUCT_ID } from "../mocks/product.fixture";
import {
  E2E_OZON_CATEGORY_ID,
  E2E_OZON_SHOP_ID,
  e2eOzonConfig,
} from "../mocks/ozon-publish";
import { e2eShops } from "../mocks/publish";
import {
  expectHeaderContentAligned,
  expectNoRootOverflow,
} from "../utils/assertions";

const centerPath = `/product/publishing-center?productId=${E2E_PRODUCT_ID}&shopId=${E2E_OZON_SHOP_ID}`;
const configPath = `/api/v1/products/${E2E_PRODUCT_ID}/platform-configs/ozon`;
const fiveViewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

function cloneConfig() {
  return JSON.parse(JSON.stringify(e2eOzonConfig)) as typeof e2eOzonConfig;
}

async function routeConfigReads(
  page: Page,
  read: (url: URL) => unknown = () => cloneConfig(),
) {
  await page.route(new RegExp(`${configPath}(?:\\?.*)?$`), async (route) => {
    if (route.request().method() !== "GET") {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(ok(read(new URL(route.request().url())))),
    });
  });
}

async function expectCenterReady(page: Page) {
  await expect(
    page.getByText("刊登中心", { exact: true }).first(),
  ).toBeVisible();
  await expect(
    page.getByText("发布前检查与提交", { exact: true }),
  ).toBeVisible();
  await expect(page.locator(".publishing-center__wizard")).toHaveAttribute(
    "data-config-ready",
    "true",
  );
}

async function goPublishingStep(page: Page, title: string) {
  await page.getByText(title, { exact: true }).first().click();
}

const readinessPassed = {
  productId: E2E_PRODUCT_ID,
  platform: "ozon",
  shopId: E2E_OZON_SHOP_ID,
  mode: "publish",
  status: "passed",
  statusLabel: "检查通过",
  result: "passed",
  resultLabel: "通过",
  canPublish: true,
  errorCount: 0,
  warningCount: 0,
  checks: [],
  checkedAt: "2026-08-04T00:00:00Z",
  schemaHash: "e2e-schema-v1",
  schemaChanged: false,
  resolvedOzon: e2eOzonConfig.ozonPreview,
};

test.describe("@ozon-publish @publishing-center 统一刊登中心", () => {
  test("redirects the legacy deep link and preserves product/store context", async ({
    admin,
    page,
  }) => {
    const reads: string[] = [];
    await routeConfigReads(page, (url) => {
      reads.push(url.searchParams.get("shopId") || "");
      return cloneConfig();
    });

    await admin.goto(
      `/product/ozon-publish?productId=${E2E_PRODUCT_ID}&shopId=${E2E_OZON_SHOP_ID}`,
    );

    // The first local MFSU visit may still be compiling this lazy route even
    // after the login route has made Playwright's web server probe ready.
    await expect(page).toHaveURL(/\/product\/publishing-center\?/, {
      timeout: 30_000,
    });
    await expectCenterReady(page);
    expect(reads).toContain(E2E_OZON_SHOP_ID);
  });

  test("presents the six-step operator flow with verified warehouse and readable VAT labels", async ({
    admin,
    page,
  }) => {
    await admin.goto(centerPath);
    await expectCenterReady(page);
    await expect(
      page.getByText("即时阻断项已清零", { exact: true }),
    ).toBeVisible();
    for (const title of [
      "店铺与商品",
      "内容与图片",
      "Ozon 类目与属性",
      "规格、价格与库存",
      "包裹、仓库与税率",
      "发布前检查与提交",
    ]) {
      await expect(
        page.getByText(title, { exact: true }).first(),
      ).toBeVisible();
    }

    await goPublishingStep(page, "Ozon 类目与属性");
    await expect(
      page.getByText("从父类目开始逐级选择", { exact: true }),
    ).toBeVisible();
    await expect(page.getByText(/类目缓存：.*（有效）/)).toBeVisible();
    await expect(
      page.getByRole("combobox", { name: "Ozon 一级类目" }),
    ).toBeVisible();
    await expect(
      page.getByRole("combobox", { name: "Ozon 第 2 级类目" }),
    ).toBeVisible();
    await expect(
      page.getByText("当前叶子类目已经应用", { exact: true }),
    ).toBeVisible();
    await expect(page.getByText("映射证据确认", { exact: true })).toBeVisible();
    await expect(
      page.getByText("来源类目映射证据完整", { exact: true }),
    ).toBeVisible();
    await expect(page.getByRole("spinbutton", { name: "容量" })).toBeVisible();
    await expect(page.getByLabel("是否偏光")).toBeVisible();
    await expect(
      page.getByRole("textbox", { name: "商品链接" }),
    ).toHaveAttribute("type", "url");

    await goPublishingStep(page, "包裹、仓库与税率");
    await expect(page.getByText(/E2E 莫斯科 FBS 仓/)).toBeVisible();
    await expect(
      page.getByText("增值税 20%（接口值 0.2）", { exact: true }),
    ).toBeVisible();

    await goPublishingStep(page, "内容与图片");
    const skuImages = page.locator(".ozon-publish-page__sku-image-collapse");
    await expect(skuImages).toHaveCount(1);
    await expect(page.getByText("SKU 原始主图", { exact: true })).toBeHidden();
    await skuImages.locator(".ant-collapse-header").click();
    await expect(
      page.getByText("SKU 原始主图", { exact: true }).last(),
    ).toBeVisible();
  });

  test("selects an Ozon leaf category from its parent one level at a time before applying it", async ({
    admin,
    page,
  }) => {
    await routeConfigReads(page, () => ({
      ...cloneConfig(),
      categoryId: "",
      categoryPath: "",
      platformAttributes: {},
      schemaHash: "",
      schemaConfirmedAt: undefined,
    }));

    await admin.goto(centerPath);
    await expectCenterReady(page);
    await goPublishingStep(page, "Ozon 类目与属性");
    await expect(
      page.getByText("请从一级父类目开始", { exact: true }),
    ).toBeVisible();

    await page.getByRole("combobox", { name: "Ozon 一级类目" }).click();
    await page.getByText("家具（1 个子类目）", { exact: true }).last().click();
    await expect(
      page.getByRole("combobox", { name: "Ozon 第 2 级类目" }),
    ).toBeVisible();
    await page.getByRole("combobox", { name: "Ozon 第 2 级类目" }).click();
    await page.getByText("桌子（叶子类目）", { exact: true }).last().click();

    await expect(
      page.getByText("已定位叶子类目，尚未应用", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText(/description_category_id：100；type_id：200/),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "确认此叶子类目并加载模板" })
      .click();
    await expect(
      page.getByText("当前叶子类目已经应用", { exact: true }),
    ).toBeVisible();
    await expect(page.getByRole("spinbutton", { name: "容量" })).toBeVisible();
  });

  test("keeps the full-path keyword while locating a search result through its ancestors", async ({
    admin,
    page,
  }) => {
    const reads: Array<{ keyword: string; offset: number }> = [];
    await page.route("**/api/v1/platform/ozon/categories?**", async (route) => {
      if (route.request().method() !== "GET") {
        await route.fallback();
        return;
      }
      const url = new URL(route.request().url());
      const keyword = url.searchParams.get("keyword") || "";
      const offset = Number(url.searchParams.get("offset") || 0);
      reads.push({ keyword, offset });
      const rootOnly = url.searchParams.get("rootOnly") === "1";
      const parentId = url.searchParams.get("parentId") || "";
      if (rootOnly) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(
            ok({
              list: [
                {
                  id: "home",
                  categoryId: "home",
                  name: "住宅和花园",
                  path: "住宅和花园",
                  level: 1,
                  isLeaf: false,
                  hasChildren: true,
                  childCount: 1,
                  status: "active",
                },
              ],
              total: 20000,
              leafCount: 18000,
              matchedTotal: 1,
              offset: 0,
              limit: 50,
              lastSyncedAt: "2026-08-10T00:00:00Z",
              cacheStale: false,
            }),
          ),
        });
        return;
      }
      if (parentId) {
        const isStorage = parentId === "storage";
        const list = isStorage
          ? [
              {
                id: E2E_OZON_CATEGORY_ID,
                categoryId: E2E_OZON_CATEGORY_ID,
                parentId: "storage",
                name: "储物箱",
                path: "住宅和花园 / 收纳 / 储物箱",
                level: 3,
                isLeaf: true,
                hasChildren: false,
                childCount: 0,
                status: "active",
              },
              {
                id: "100:201",
                categoryId: "100:201",
                parentId: "storage",
                name: "储物箱候选",
                path: "住宅和花园 / 收纳 / 储物箱候选",
                level: 3,
                isLeaf: true,
                hasChildren: false,
                childCount: 0,
                status: "active",
              },
            ]
          : [
              {
                id: "storage",
                categoryId: "storage",
                parentId: "home",
                name: "收纳",
                path: "住宅和花园 / 收纳",
                level: 2,
                isLeaf: false,
                hasChildren: true,
                childCount: 1,
                status: "active",
              },
            ];
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(
            ok({
              list,
              total: 20000,
              leafCount: 18000,
              matchedTotal: list.length,
              offset: 0,
              limit: 200,
              lastSyncedAt: "2026-08-10T00:00:00Z",
              cacheStale: false,
            }),
          ),
        });
        return;
      }
      if (keyword === E2E_OZON_CATEGORY_ID) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(
            ok({
              list: [
                {
                  id: E2E_OZON_CATEGORY_ID,
                  categoryId: E2E_OZON_CATEGORY_ID,
                  parentId: "storage",
                  name: "储物箱",
                  path: "住宅和花园 / 收纳 / 储物箱",
                  descriptionCategoryId: "100",
                  typeId: "200",
                  level: 3,
                  isLeaf: true,
                  hasChildren: false,
                  childCount: 0,
                  ancestors: [
                    { categoryId: "home", name: "住宅和花园", level: 1 },
                    { categoryId: "storage", name: "收纳", level: 2 },
                  ],
                  status: "active",
                },
              ],
              total: 20000,
              leafCount: 18000,
              matchedTotal: 1,
              offset: 0,
              limit: 20,
              lastSyncedAt: "2026-08-10T00:00:00Z",
              cacheStale: false,
            }),
          ),
        });
        return;
      }
      if (!keyword) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(
            ok({
              list: [],
              total: 20000,
              leafCount: 18000,
              matchedTotal: 0,
              offset: 0,
              limit: 20,
              lastSyncedAt: "2026-08-10T00:00:00Z",
              cacheStale: false,
            }),
          ),
        });
        return;
      }
      const count = offset === 0 ? 50 : 9;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          ok({
            list: Array.from({ length: count }, (_, index) => ({
              id: `parent-search-${offset + index}`,
              categoryId:
                offset + index === 0
                  ? "100:201"
                  : `parent-search-${offset + index}`,
              parentId: "storage",
              name: `收纳叶子类目 ${offset + index + 1}`,
              path:
                offset + index === 0
                  ? "住宅和花园 / 收纳 / 储物箱候选"
                  : `住宅和花园 / 收纳 / 收纳叶子类目 ${offset + index + 1}`,
              level: 3,
              isLeaf: true,
              hasChildren: false,
              childCount: 0,
              ancestors: [
                { categoryId: "home", name: "住宅和花园", level: 1 },
                { categoryId: "storage", name: "收纳", level: 2 },
              ],
              status: "active",
            })),
            total: 20000,
            leafCount: 18000,
            matchedTotal: 59,
            offset,
            limit: 50,
            lastSyncedAt: "2026-08-10T00:00:00Z",
            cacheStale: false,
          }),
        ),
      });
    });

    await admin.goto(centerPath);
    await expectCenterReady(page);
    await goPublishingStep(page, "Ozon 类目与属性");
    const categoryInput = page.getByPlaceholder(
      "辅助定位：输入父级名称、完整路径或类目 ID",
    );
    await categoryInput.fill("收纳");
    await page.getByRole("button", { name: "搜索完整路径" }).click();
    await expect(
      page.getByText("搜索“收纳”：已展示 50 条，共匹配 59 个启用叶子类目", {
        exact: true,
      }),
    ).toBeVisible();
    await page.getByRole("button", { name: "加载更多搜索结果" }).click();
    await expect(
      page.getByText("搜索“收纳”：已展示 59 条，共匹配 59 个启用叶子类目", {
        exact: true,
      }),
    ).toBeVisible();
    expect(reads).toContainEqual({ keyword: "收纳", offset: 50 });

    await page.getByRole("combobox", { name: "类目搜索定位结果" }).click();
    await page
      .getByText(/住宅和花园 \/ 收纳 \/ 储物箱候选/)
      .last()
      .click();
    const locatedLevels = page.locator(".ozon-category-navigator__level");
    await expect(
      locatedLevels.nth(0).locator(".ant-select-selection-item"),
    ).toContainText("住宅和花园");
    await expect(
      locatedLevels.nth(1).locator(".ant-select-selection-item"),
    ).toContainText("收纳");
    await expect(
      locatedLevels.nth(2).locator(".ant-select-selection-item"),
    ).toContainText("储物箱候选");
    await expect(
      page.getByText("已定位叶子类目，尚未应用", { exact: true }),
    ).toBeVisible();
  });

  test("saves one product + Ozon store configuration and restores the same values", async ({
    admin,
    page,
  }) => {
    let persisted: Record<string, unknown> = cloneConfig();
    const readShops: string[] = [];
    await routeConfigReads(page, (url) => {
      readShops.push(url.searchParams.get("shopId") || "");
      return persisted;
    });
    admin.writeGuard.allow({
      operation: "save-ozon-store-config",
      method: "PUT",
      path: new RegExp(`${configPath}$`),
      response: (record) => {
        const body = record.postDataJSON as {
          ozonListing: {
            titleOverride?: string;
            skuPriceOverrides: Record<string, number>;
          };
          ozonImages: unknown;
          platformAttributes: unknown;
        } & Record<string, unknown>;
        const base = cloneConfig();
        persisted = {
          ...base,
          ...body,
          id: base.id,
          ozonImages: base.ozonImages,
          ozonPreview: {
            ...base.ozonPreview,
            title: {
              value:
                body.ozonListing.titleOverride || base.ozonPreview.title.value,
              source: "ozon_product_shop_config",
            },
            skus: base.ozonPreview.skus.map((sku) => ({
              ...sku,
              price: {
                value:
                  body.ozonListing.skuPriceOverrides[sku.skuId] ??
                  sku.price.value,
                source: "ozon_product_shop_config",
              },
            })),
          },
        };
        return ok(persisted);
      },
    });

    await admin.goto(centerPath);
    await expectCenterReady(page);
    await goPublishingStep(page, "内容与图片");
    await page
      .getByRole("textbox", { name: "Ozon 标题" })
      .fill("E2E 店铺 A 独立刊登标题");
    await goPublishingStep(page, "规格、价格与库存");
    await page.getByRole("spinbutton", { name: "Ozon 售价" }).fill("2099");
    await page.getByRole("button", { name: "保存当前编辑（不提交）" }).click();

    await admin.writeGuard.expectRequestCount("save-ozon-store-config", 1);
    const body = admin.writeGuard.calls("save-ozon-store-config")[0]
      .postDataJSON as {
      shopId: string;
      platformAttributes: {
        version: number;
        attributes: Record<string, unknown[]>;
        complexGroups: Array<{
          complexId: number;
          attributes: Record<string, unknown[]>;
        }>;
        skuVariantAttributeIds: string[];
        skuAttributeOverrides: Record<string, Record<string, unknown[]>>;
      };
      ozonListing: {
        titleOverride?: string;
        skuPriceOverrides: Record<string, number>;
      };
      ozonImages: { skuSelections: unknown[] };
    };
    expect(body.shopId).toBe(E2E_OZON_SHOP_ID);
    expect(body.ozonListing.titleOverride).toBe("E2E 店铺 A 独立刊登标题");
    expect(body.ozonListing.skuPriceOverrides).toEqual({ "e2e-sku-1": 2099 });
    expect(body.ozonListing).not.toHaveProperty("stock");
    expect(body).not.toHaveProperty("stock");
    expect(body.platformAttributes.version).toBe(3);
    expect(body.platformAttributes.skuVariantAttributeIds).toEqual([]);
    expect(body.platformAttributes.skuAttributeOverrides).toEqual({});
    expect(body.platformAttributes.attributes["86"]).toHaveLength(2);
    expect(body.platformAttributes.complexGroups).toEqual([
      {
        complexId: 501,
        attributes: { "87": [{ value: "棉" }] },
      },
    ]);
    expect(body.ozonImages.skuSelections).toHaveLength(1);
    expect(admin.writeGuard.calls("publish-ozon")).toHaveLength(0);

    await page.reload();
    await expectCenterReady(page);
    await goPublishingStep(page, "内容与图片");
    await expect(page.getByRole("textbox", { name: "Ozon 标题" })).toHaveValue(
      "E2E 店铺 A 独立刊登标题",
    );
    await goPublishingStep(page, "规格、价格与库存");
    await expect(
      page.getByRole("spinbutton", { name: "Ozon 售价" }),
    ).toHaveValue("2099.00");
    expect(readShops.length).toBeGreaterThanOrEqual(2);
    expect(readShops.every((value) => value === E2E_OZON_SHOP_ID)).toBe(true);
  });

  test("maps local multi-SKU values to distinct Ozon variant attributes and persists them", async ({
    admin,
    page,
  }) => {
    const secondSKU = {
      ...e2eProduct.skus[0],
      id: "e2e-sku-2",
      skuCode: "E2E-SKU-2",
      skuName: "黑色 / L",
      attrs: { 颜色: "黑色", 尺码: "L" },
      price: 139.9,
      stock: 33,
      imageUrl: "https://example.test/e2e-black.jpg",
    };
    const multiProduct = {
      ...e2eProduct,
      skus: [
        { ...e2eProduct.skus[0], attrs: { 颜色: "白色", 尺码: "M" } },
        secondSKU,
      ],
    };
    const multiConfig = cloneConfig();
    multiConfig.platformAttributes = {
      ...multiConfig.platformAttributes,
      skuVariantAttributeIds: [],
      skuAttributeOverrides: {},
    };
    multiConfig.ozonImages.skus.push({
      ...multiConfig.ozonImages.skus[0],
      skuId: secondSKU.id,
      skuCode: secondSKU.skuCode,
      skuName: secondSKU.skuName,
      attrs: secondSKU.attrs,
      originalMainImageUrl: secondSKU.imageUrl,
      additionalImageIds: [],
      finalImages: [
        {
          url: secondSKU.imageUrl,
          source: "sku_original",
          position: 1,
          imageType: "main",
        },
      ],
    });
    multiConfig.ozonPreview.skus.push({
      ...multiConfig.ozonPreview.skus[0],
      skuId: secondSKU.id,
      skuCode: secondSKU.skuCode,
      skuName: secondSKU.skuName,
      price: { value: secondSKU.price, source: "product" },
      localStock: secondSKU.stock,
      images: multiConfig.ozonImages.skus[1].finalImages,
    });
    await page.route(
      new RegExp(`/api/v1/products/${E2E_PRODUCT_ID}$`),
      async (route) => {
        if (route.request().method() !== "GET") {
          await route.fallback();
          return;
        }
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(ok(multiProduct)),
        });
      },
    );
    await routeConfigReads(page, () => multiConfig);
    admin.writeGuard.allow({
      operation: "save-ozon-sku-variants",
      method: "PUT",
      path: new RegExp(`${configPath}$`),
      response: (record) => ok({ ...multiConfig, ...record.postDataJSON }),
    });

    await admin.goto(centerPath);
    await expectCenterReady(page);
    await goPublishingStep(page, "规格、价格与库存");
    await expect(
      page.getByText("当前配置可以保存，但不能提交 Ozon", {
        exact: true,
      }),
    ).toBeVisible();
    const variantInput = page.getByRole("combobox", {
      name: "用于区分 SKU 的 Ozon 属性",
    });
    await variantInput.click();
    const variantDropdown = page.locator(".ant-select-dropdown:visible");
    const disabledVariantOptions = [
      ["品牌（必填）", "Ozon is_aspect=false"],
      ["待确认规格", "缺少 Ozon is_aspect 资格证据"],
      ["材质组合（必填）", "组合属性不能直接作为 SKU 维度"],
      ["平台特殊规格", "系统暂不支持 valueType=OzonDimension"],
    ];
    for (const [name, reason] of disabledVariantOptions) {
      await variantInput.fill(name);
      const option = variantDropdown
        .locator(".ant-select-item-option")
        .filter({ hasText: `${name} — 禁用：${reason}` });
      await expect(option).toBeVisible();
      await expect(option).toHaveClass(/ant-select-item-option-disabled/);
    }
    await variantInput.fill("颜色");
    const colorOption = variantDropdown.getByText("颜色（必填）", {
      exact: true,
    });
    await expect(colorOption).toBeVisible();
    await colorOption.click();
    await page.getByRole("button", { name: "从本地 SKU 属性自动匹配" }).click();
    await expect(page.getByText("本地属性候选").first()).toBeVisible();
    await page.getByRole("button", { name: "保存当前编辑（不提交）" }).click();

    await admin.writeGuard.expectRequestCount("save-ozon-sku-variants", 1);
    const body = admin.writeGuard.calls("save-ozon-sku-variants")[0]
      .postDataJSON as {
      platformAttributes: {
        version: number;
        attributes: Record<string, unknown>;
        skuVariantAttributeIds: string[];
        skuAttributeOverrides: Record<
          string,
          Record<string, Array<{ value: string; dictionaryValueId: string }>>
        >;
      };
    };
    expect(body.platformAttributes.version).toBe(3);
    expect(body.platformAttributes.skuVariantAttributeIds).toEqual(["86"]);
    expect(body.platformAttributes.attributes).not.toHaveProperty("86");
    expect(body.platformAttributes.skuAttributeOverrides).toEqual({
      "e2e-sku-1": {
        "86": [{ value: "白色", dictionaryValueId: "1001" }],
      },
      "e2e-sku-2": {
        "86": [{ value: "黑色", dictionaryValueId: "1002" }],
      },
    });
    expect(admin.writeGuard.calls("publish-ozon")).toHaveLength(0);
  });

  test("requires confirmation before switching away from unsaved store edits", async ({
    admin,
    page,
  }) => {
    const secondShopId = "e2e-ozon-shop-second";
    await routeConfigReads(page);
    await page.route("**/api/v1/shops?**", async (route) => {
      if (route.request().method() !== "GET") {
        await route.fallback();
        return;
      }
      const secondShop = {
        ...e2eShops.find((shop) => shop.id === E2E_OZON_SHOP_ID)!,
        id: secondShopId,
        shopName: "E2E Ozon 第二店铺",
      };
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          ok({
            list: [...e2eShops, secondShop],
            pagination: { page: 1, pageSize: 500, total: 3, totalPages: 1 },
          }),
        ),
      });
    });

    await admin.goto(centerPath);
    await expectCenterReady(page);
    await goPublishingStep(page, "内容与图片");
    const title = page.getByRole("textbox", { name: "Ozon 标题" });
    await title.fill("尚未保存的店铺编辑");
    await goPublishingStep(page, "店铺与商品");
    const shopControl = page
      .locator(".publishing-center__context-grid > div")
      .filter({ hasText: "Ozon 店铺" });
    await shopControl.locator(".ant-select-selector").click();
    await page.getByText("E2E Ozon 第二店铺", { exact: true }).click();
    const dialog = page.getByRole("dialog", {
      name: "放弃未保存的刊登编辑？",
    });
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "继续编辑" }).click();
    await goPublishingStep(page, "内容与图片");
    await expect(title).toHaveValue("尚未保存的店铺编辑");
    await expect(page).toHaveURL(new RegExp(`shopId=${E2E_OZON_SHOP_ID}`));

    await goPublishingStep(page, "店铺与商品");
    await shopControl.locator(".ant-select-selector").click();
    await page.getByText("E2E Ozon 第二店铺", { exact: true }).click();
    await page
      .getByRole("dialog", { name: "放弃未保存的刊登编辑？" })
      .getByRole("button", { name: "放弃并切换" })
      .click();
    await expect(page).toHaveURL(new RegExp(`shopId=${secondShopId}`));
  });

  test("uses read-only backend preflight, requires a second confirmation, and submits once", async ({
    admin,
    page,
  }) => {
    const task = {
      id: "e2e-ozon-publish-task",
      productId: E2E_PRODUCT_ID,
      shopId: E2E_OZON_SHOP_ID,
      shopName: "E2E Ozon 测试店铺",
      productTitle: "E2E Ozon 店铺标题",
      platform: "ozon",
      taskType: "product_publish",
      status: "pending",
      mode: "publish",
      createdAt: "2026-08-04T00:00:00Z",
      updatedAt: "2026-08-04T00:00:00Z",
    };
    admin.writeGuard.allow({
      operation: "ozon-readonly-preflight",
      method: "POST",
      path: new RegExp(
        `/api/v1/products/${E2E_PRODUCT_ID}/readiness/validate$`,
      ),
      response: ok(readinessPassed),
    });
    admin.writeGuard.allow({
      operation: "publish-ozon",
      method: "POST",
      path: new RegExp(`/api/v1/products/${E2E_PRODUCT_ID}/publish$`),
      response: ok(task),
    });
    await page.route("**/api/v1/product-publish/tasks**", async (route) => {
      if (route.request().method() !== "GET") {
        await route.fallback();
        return;
      }
      const path = new URL(route.request().url()).pathname;
      const data = path.endsWith(`/${task.id}`)
        ? task
        : {
            list: [task],
            pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 },
          };
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(ok(data)),
      });
    });

    await admin.goto(centerPath);
    await expectCenterReady(page);
    await goPublishingStep(page, "发布前检查与提交");
    const enterConfirmation = page.getByRole("button", {
      name: "进入提交确认",
    });
    await expect(enterConfirmation).toBeDisabled();
    await page.getByRole("button", { name: "运行发布前检查" }).click();

    await admin.writeGuard.expectRequestCount("ozon-readonly-preflight", 1);
    expect(
      admin.writeGuard.calls("ozon-readonly-preflight")[0].postDataJSON,
    ).toEqual({ platform: "ozon", shopId: E2E_OZON_SHOP_ID });
    await expect(page.getByText("只读检查通过", { exact: true })).toBeVisible();
    await expect(page.getByText(/库存：88/)).toBeVisible();
    await expect(page.getByText(/本地库存/).last()).toBeVisible();
    expect(admin.writeGuard.calls("publish-ozon")).toHaveLength(0);

    const submit = page
      .locator(".publishing-center__actions")
      .getByRole("button", { name: "进入提交确认" });
    await expect(submit).toBeEnabled();
    await submit.click();
    const confirmation = page.getByRole("dialog", {
      name: "确认提交到 Ozon？",
    });
    await expect(confirmation).toBeVisible();
    await expect(confirmation.getByText("E2E Ozon 测试店铺")).toBeVisible();
    await expect(
      confirmation.getByText(/家具 \/ 桌子（100:200）/),
    ).toBeVisible();
    await expect(confirmation.getByText("SKU 数")).toBeVisible();
    await expect(confirmation.getByText(/合计 88/)).toBeVisible();
    await expect(
      confirmation.getByText(/500 g；200 × 100 × 300 mm/),
    ).toBeVisible();
    await confirmation.getByRole("button", { name: "返回继续检查" }).click();
    expect(admin.writeGuard.calls("publish-ozon")).toHaveLength(0);

    await submit.click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "确认提交到 Ozon" })
      .click();
    await admin.writeGuard.expectRequestCount("publish-ozon", 1);
    const publishCall = admin.writeGuard.calls("publish-ozon")[0];
    expect(publishCall.postDataJSON).toEqual({
      shopId: E2E_OZON_SHOP_ID,
      options: { platform: "ozon" },
    });
    expect(publishCall.headers["idempotency-key"]).toMatch(/^ozon-submit:/);
    await expect(page).toHaveURL(/\/product\/publish-tasks\?/);
  });

  test("blocks a conflicting source-category mapping until the operator explicitly confirms it", async ({
    admin,
    page,
  }) => {
    await page.route(
      "**/api/v1/platform/ozon/category-mappings**",
      async (route) => {
        if (route.request().method() !== "GET") {
          await route.fallback();
          return;
        }
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(
            ok({
              list: [
                {
                  id: "conflicting-map",
                  shopId: E2E_OZON_SHOP_ID,
                  sourceCategoryKey: e2eOzonConfig.sourceCategoryKey,
                  sourceCategoryName: e2eOzonConfig.sourceCategoryName,
                  categoryId: "300:400",
                  categoryPath: "电子产品 / 3D 眼镜",
                  status: "active",
                  schemaHash: "e2e-schema-v1",
                  confirmedAt: "2026-08-03T00:00:00Z",
                },
              ],
            }),
          ),
        });
      },
    );
    admin.writeGuard.allow({
      operation: "confirm-ozon-category-mapping",
      method: "PUT",
      path: /\/api\/v1\/platform\/ozon\/category-mappings$/,
      response: (record) =>
        ok({
          id: "confirmed-map",
          ...(record.postDataJSON as Record<string, unknown>),
          status: "active",
          schemaHash: "e2e-schema-v1",
          confirmedAt: "2026-08-04T00:00:00Z",
        }),
    });
    admin.writeGuard.allow({
      operation: "ozon-readonly-preflight-after-mapping",
      method: "POST",
      path: new RegExp(
        `/api/v1/products/${E2E_PRODUCT_ID}/readiness/validate$`,
      ),
      response: ok(readinessPassed),
    });

    await admin.goto(centerPath);
    await expectCenterReady(page);
    await goPublishingStep(page, "Ozon 类目与属性");
    await expect(
      page.getByText("当前类目与已确认映射冲突", { exact: true }),
    ).toBeVisible();
    await goPublishingStep(page, "发布前检查与提交");
    await expect(
      page.getByRole("button", { name: "进入提交确认" }),
    ).toBeDisabled();

    await goPublishingStep(page, "Ozon 类目与属性");
    await page.getByRole("button", { name: "确认当前类目映射" }).click();
    const mappingDialog = page.getByRole("dialog", {
      name: "确认来源类目与 Ozon 类目映射？",
    });
    await expect(mappingDialog.getByText(/E2E 本地桌子/)).toBeVisible();
    await expect(
      mappingDialog.getByText(/家具 \/ 桌子（100:200）/),
    ).toBeVisible();
    await expect(
      mappingDialog.getByRole("button", { name: "确认当前映射" }),
    ).toBeDisabled();
    expect(
      admin.writeGuard.calls("confirm-ozon-category-mapping"),
    ).toHaveLength(0);
    await mappingDialog
      .getByPlaceholder("例如：商品用途、材质和规格与该 Ozon 叶子类目一致")
      .fill("商品用途、材质和规格与桌子叶子类目一致");
    await expect(
      mappingDialog.getByRole("button", { name: "确认当前映射" }),
    ).toBeEnabled();
    await mappingDialog.getByRole("button", { name: "确认当前映射" }).click();
    await admin.writeGuard.expectRequestCount(
      "confirm-ozon-category-mapping",
      1,
    );
    expect(
      admin.writeGuard.calls("confirm-ozon-category-mapping")[0].postDataJSON,
    ).toMatchObject({
      shopId: E2E_OZON_SHOP_ID,
      sourceCategoryKey: e2eOzonConfig.sourceCategoryKey,
      categoryId: E2E_OZON_CATEGORY_ID,
      categoryPath: "家具 / 桌子",
      status: "active",
      selectionMethod: "manual",
      confirmationReason: "商品用途、材质和规格与桌子叶子类目一致",
    });
    await expect(
      page.getByText("来源类目映射证据完整", { exact: true }),
    ).toBeVisible();

    await goPublishingStep(page, "发布前检查与提交");
    await page.getByRole("button", { name: "运行发布前检查" }).click();
    await admin.writeGuard.expectRequestCount(
      "ozon-readonly-preflight-after-mapping",
      1,
    );
    await expect(
      page.getByRole("button", { name: "进入提交确认" }),
    ).toBeEnabled();
  });

  test("keeps category synchronization in advanced maintenance and intercepts it", async ({
    admin,
    page,
  }) => {
    admin.writeGuard.allow({
      operation: "sync-ozon-category-cache",
      method: "POST",
      path: /\/api\/v1\/platform\/ozon\/categories\/sync$/,
      response: ok({ runId: "e2e-ozon-sync-run" }),
    });
    await admin.goto(centerPath);
    await expectCenterReady(page);
    await goPublishingStep(page, "Ozon 类目与属性");
    await page.getByText("高级类目维护", { exact: true }).click();
    await page.getByRole("button", { name: "同步类目缓存" }).click();
    await admin.writeGuard.expectRequestCount("sync-ozon-category-cache", 1);
    expect(
      admin.writeGuard.calls("sync-ozon-category-cache")[0].postDataJSON,
    ).toEqual({ shopId: E2E_OZON_SHOP_ID });
  });

  test("shows loading, empty, error, and readonly states without issuing writes", async ({
    page,
  }) => {
    let releaseConfig: (() => void) | undefined;
    const pendingConfig = new Promise<void>((resolve) => {
      releaseConfig = resolve;
    });
    await page.route(new RegExp(`${configPath}(?:\\?.*)?$`), async (route) => {
      if (route.request().method() !== "GET") {
        await route.fallback();
        return;
      }
      await pendingConfig;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(ok(cloneConfig())),
      });
    });
    await page.goto(centerPath);
    await expect(page.getByText("正在读取当前店铺配置…")).toBeVisible();
    releaseConfig?.();
    await expectCenterReady(page);

    await page.route("**/api/v1/auth/profile", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          ok({
            ...e2eUser,
            permissions: ["product.view"],
            storePermissions: [],
          }),
        ),
      });
    });
    await page.reload();
    await expect(
      page.getByRole("button", { name: "保存当前编辑（不提交）" }),
    ).toBeDisabled();

    await page.route(new RegExp(`${configPath}(?:\\?.*)?$`), async (route) => {
      if (route.request().method() === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(fail("店铺级 Ozon 配置读取失败", 4404)),
        });
        return;
      }
      await route.fallback();
    });
    await page.reload();
    await expect(page.getByText("刊登中心加载失败")).toBeVisible();

    await page.route("**/api/v1/products?**", async (route) => {
      if (route.request().method() === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(
            ok({
              list: [],
              pagination: {
                page: 1,
                pageSize: 200,
                total: 0,
                totalPages: 1,
              },
            }),
          ),
        });
        return;
      }
      await route.fallback();
    });
    await page.goto("/product/publishing-center");
    await expect(page.getByText("请选择商品和 Ozon 店铺")).toBeVisible();
  });

  for (const viewport of fiveViewports) {
    test(`has no root overflow and keeps the check panel first at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await admin.goto(centerPath);
      await expectCenterReady(page);
      await goPublishingStep(page, "发布前检查与提交");
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      if (viewport.width <= 1024) {
        const positions = await page.evaluate(() => {
          const editor = document
            .querySelector(".publishing-center__editor")
            ?.getBoundingClientRect();
          const check = document
            .querySelector(".publishing-center__check-panel")
            ?.getBoundingClientRect();
          return { editorTop: editor?.top, checkTop: check?.top };
        });
        expect(positions.checkTop).toBeLessThanOrEqual(
          positions.editorTop ?? Number.POSITIVE_INFINITY,
        );
      }
    });
  }
});

test.describe("@publishing-center 刊登进度", () => {
  test("automatically refreshes a processing task every three seconds until it reaches a fact status", async ({
    admin,
    page,
  }) => {
    const taskId = "e2e-ozon-polling";
    let detailReads = 0;
    const running = {
      id: taskId,
      productId: E2E_PRODUCT_ID,
      shopId: E2E_OZON_SHOP_ID,
      shopName: "E2E Ozon 测试店铺",
      productTitle: "E2E Ozon 店铺标题",
      platform: "ozon",
      taskType: "product_publish",
      status: "running",
      publishStatus: "publishing",
      mode: "publish",
      retryable: false,
      createdAt: "2026-08-04T00:00:00Z",
      updatedAt: "2026-08-04T00:00:01Z",
    };
    const imported = {
      ...running,
      status: "succeeded",
      publishStatus: "imported",
      platformProductId: "ozon-product-polling",
      platformResult: {
        platformStatus: "imported",
        sellableVerified: false,
      },
      updatedAt: "2026-08-04T00:00:04Z",
    };
    await page.route("**/api/v1/product-publish/tasks**", async (route) => {
      if (route.request().method() !== "GET") {
        await route.fallback();
        return;
      }
      const path = new URL(route.request().url()).pathname;
      const data = path.endsWith(`/${taskId}`)
        ? ++detailReads === 1
          ? running
          : imported
        : {
            list: [detailReads > 1 ? imported : running],
            pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 },
          };
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(ok(data)),
      });
    });

    await admin.goto(`/product/publish-tasks?tab=tasks&id=${taskId}`);
    await expect(
      page.getByText("每 3 秒自动刷新", { exact: true }),
    ).toBeVisible();
    await expect
      .poll(() => detailReads, { timeout: 8_000 })
      .toBeGreaterThanOrEqual(2);
    await expect(
      page.getByText("Ozon 已接收，待确认可售", { exact: true }).first(),
    ).toBeVisible();
    await expect(
      page.getByText("每 3 秒自动刷新", { exact: true }),
    ).toHaveCount(0);
  });

  test("defaults to single submissions and gates Ozon retry by retryable=true", async ({
    admin,
    page,
  }) => {
    const nonRetryable = {
      id: "e2e-ozon-uncertain",
      productId: E2E_PRODUCT_ID,
      shopId: E2E_OZON_SHOP_ID,
      shopName: "E2E Ozon 测试店铺",
      productTitle: "E2E Ozon 店铺标题",
      platform: "ozon",
      taskType: "product_publish",
      status: "failed",
      publishStatus: "result_unknown",
      recoveryState: "result_unknown",
      mode: "publish",
      retryable: false,
      errorMessage: "Ozon 返回结果不确定",
      platformProductId: "ozon-product-10001",
      platformPayload: { offer_id: "E2E-SKU-1", price: "1990" },
      platformResult: {
        result: "uncertain",
        request_id: "ozon-request-1",
        warnings: [{ message: "商品尺寸需要修正" }],
      },
      createdAt: "2026-08-04T00:00:00Z",
      updatedAt: "2026-08-04T00:01:00Z",
    };
    const retryable = {
      ...nonRetryable,
      id: "e2e-ozon-retryable",
      retryable: true,
      errorMessage: "Ozon 明确返回限流错误",
      platformProductId: undefined,
    };
    admin.writeGuard.allow({
      operation: "reconcile-ozon-not-created",
      method: "POST",
      path: new RegExp(
        `/api/v1/product-publish/tasks/${nonRetryable.id}/reconcile-ozon$`,
      ),
      response: ok({
        ...nonRetryable,
        publishStatus: "failed",
        recoveryState: "confirmed_not_created",
        retryable: true,
        errorMessage: "已人工确认 Ozon 未创建商品",
      }),
    });
    await page.route("**/api/v1/product-publish/tasks**", async (route) => {
      if (route.request().method() !== "GET") {
        await route.fallback();
        return;
      }
      const path = new URL(route.request().url()).pathname;
      const data = path.endsWith(`/${nonRetryable.id}`)
        ? nonRetryable
        : path.endsWith(`/${retryable.id}`)
          ? retryable
          : {
              list: [nonRetryable, retryable],
              pagination: {
                page: 1,
                pageSize: 20,
                total: 2,
                totalPages: 1,
              },
            };
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(ok(data)),
      });
    });

    await page.setViewportSize({ width: 390, height: 844 });
    await admin.goto("/product/publish-tasks");
    await expect(page.getByRole("tab", { name: "单品提交" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(
      page.getByRole("tab", { name: "批次（高级）" }),
    ).toHaveAttribute("aria-selected", "false");
    await expect(page.getByText("请先安全对账", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "重试" })).toHaveCount(1);

    await page.getByText("查看", { exact: true }).first().click();
    await expect(page.getByText(/刊登进度 e2e-ozon-uncertain/)).toBeVisible();
    await expect(page.getByText(/业务状态：/)).toBeVisible();
    const taskDrawer = page.getByLabel("刊登进度 e2e-ozon-uncertain");
    await expect(
      taskDrawer.getByText("平台结果待核对", { exact: true }),
    ).toBeVisible();
    await expect(page.getByText(/ozon-product-10001/)).toBeVisible();
    await expect(page.getByText("E2E-SKU-1", { exact: true })).toBeVisible();
    await expect(
      page.getByText("商品尺寸需要修正", { exact: true }),
    ).toBeVisible();
    await expect(page.getByText("返回商品刊登配置")).toBeVisible();
    await expect(page.getByText("该结果不可自动重试")).toBeVisible();
    const drawerWrapper = page.locator(".ant-drawer-content-wrapper");
    await expect(drawerWrapper).toHaveCSS("width", "390px");
    await expect
      .poll(async () => {
        const box = await drawerWrapper.boundingBox();
        return box ? Math.round(box.x + box.width) : Number.POSITIVE_INFINITY;
      })
      .toBeLessThanOrEqual(page.viewportSize()?.width ?? 0);
    const drawerBox = await drawerWrapper.boundingBox();
    expect(drawerBox?.x ?? -1).toBeGreaterThanOrEqual(0);
    await page.getByRole("button", { name: "安全对账" }).click();
    const reconcileDialog = page.getByRole("dialog", { name: "Ozon 安全对账" });
    await reconcileDialog
      .getByRole("radio", { name: "已确认 Ozon 未创建" })
      .check();
    await reconcileDialog
      .getByRole("textbox", { name: "核对依据" })
      .fill("已在 Ozon 后台按 E2E-SKU-1 核对，未找到对应商品");
    await reconcileDialog
      .getByRole("button", { name: "保存人工核对结果" })
      .click();
    await admin.writeGuard.expectRequestCount("reconcile-ozon-not-created", 1);
    expect(
      admin.writeGuard.calls("reconcile-ozon-not-created")[0].postDataJSON,
    ).toEqual({
      outcome: "platform_not_created",
      evidence: "已在 Ozon 后台按 E2E-SKU-1 核对，未找到对应商品",
    });
    await expect(
      taskDrawer.getByText("已确认 Ozon 未创建", { exact: true }),
    ).toBeVisible();
    await page.getByText("技术详情", { exact: true }).click();
    await expect(page.getByText("最终提交快照", { exact: true })).toBeVisible();
    await expect(page.getByText("平台返回结果", { exact: true })).toBeVisible();
  });
});

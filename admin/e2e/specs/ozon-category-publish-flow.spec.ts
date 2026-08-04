import type { Page } from "@playwright/test";
import { expect, test } from "../fixtures/admin.fixture";
import { e2eUser } from "../mocks/auth";
import { fail, ok } from "../mocks/envelope";
import { E2E_PRODUCT_ID } from "../mocks/product.fixture";
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
  await expect(page.getByRole("textbox", { name: "Ozon 标题" })).toHaveValue(
    "E2E Ozon 店铺标题",
  );
  await expect(page.getByLabel("即时检查与最终提交预览")).toBeVisible();
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
    await page
      .getByRole("textbox", { name: "Ozon 标题" })
      .fill("E2E 店铺 A 独立刊登标题");
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
    expect(body.platformAttributes.version).toBe(2);
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
    await expect(page.getByRole("textbox", { name: "Ozon 标题" })).toHaveValue(
      "E2E 店铺 A 独立刊登标题",
    );
    await expect(
      page.getByRole("spinbutton", { name: "Ozon 售价" }),
    ).toHaveValue("2099.00");
    expect(readShops.length).toBeGreaterThanOrEqual(2);
    expect(readShops.every((value) => value === E2E_OZON_SHOP_ID)).toBe(true);
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
    const title = page.getByRole("textbox", { name: "Ozon 标题" });
    await title.fill("尚未保存的店铺编辑");
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
    await expect(title).toHaveValue("尚未保存的店铺编辑");
    await expect(page).toHaveURL(new RegExp(`shopId=${E2E_OZON_SHOP_ID}`));

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
    await page.getByRole("button", { name: "检查并进入提交确认" }).click();

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
      .getByRole("button", { name: "确认提交到 Ozon" });
    await submit.click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.getByRole("button", { name: "返回继续检查" }).click();
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
      publishStatus: "result_uncertain",
      mode: "publish",
      retryable: false,
      errorMessage: "Ozon 返回结果不确定",
      platformProductId: "ozon-product-10001",
      platformPayload: { offer_id: "E2E-SKU-1", price: "1990" },
      platformResult: { result: "uncertain", request_id: "ozon-request-1" },
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

    await admin.goto("/product/publish-tasks");
    await expect(page.getByRole("tab", { name: "单品提交" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(
      page.getByRole("tab", { name: "批次（高级）" }),
    ).toHaveAttribute("aria-selected", "false");
    await expect(page.getByText("请人工核对", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "重试" })).toHaveCount(1);

    await page.getByText("查看", { exact: true }).first().click();
    await expect(page.getByText(/刊登进度 e2e-ozon-uncertain/)).toBeVisible();
    await expect(page.getByText(/业务状态：/)).toBeVisible();
    await expect(page.getByText(/ozon-product-10001/)).toBeVisible();
    await expect(page.getByText("返回商品刊登配置")).toBeVisible();
    await expect(
      page.getByText("该失败或不确定结果不可自动重试"),
    ).toBeVisible();
    await page.getByText("技术详情", { exact: true }).click();
    await expect(page.getByText("最终提交快照", { exact: true })).toBeVisible();
    await expect(page.getByText("平台返回结果", { exact: true })).toBeVisible();
  });
});

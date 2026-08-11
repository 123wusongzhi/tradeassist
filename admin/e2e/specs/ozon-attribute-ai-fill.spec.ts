import type { Page } from "@playwright/test";
import { expect, test } from "../fixtures/admin.fixture";
import { e2eUser } from "../mocks/auth";
import { fail, ok } from "../mocks/envelope";
import { E2E_PRODUCT_ID } from "../mocks/product.fixture";
import {
  E2E_OZON_CATEGORY_ID,
  E2E_OZON_SCHEMA_HASH,
  E2E_OZON_SHOP_ID,
  e2eOzonAttributeSuggestions,
} from "../mocks/ozon-publish";
import type { AdminPage } from "../pages/admin.page";
import { expectNoRootOverflow } from "../utils/assertions";

const centerPath = `/product/publishing-center?productId=${E2E_PRODUCT_ID}&shopId=${E2E_OZON_SHOP_ID}`;
const suggestionPath = new RegExp(
  `/api/v1/products/${E2E_PRODUCT_ID}/ai/ozon-attribute-suggestions$`,
);
const configPath = new RegExp(
  `/api/v1/products/${E2E_PRODUCT_ID}/platform-configs/ozon$`,
);
const preflightPath = new RegExp(
  `/api/v1/products/${E2E_PRODUCT_ID}/readiness/validate$`,
);
const publishPath = new RegExp(`/api/v1/products/${E2E_PRODUCT_ID}/publish$`);

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
  checkedAt: "2026-08-11T00:00:00Z",
  schemaHash: E2E_OZON_SCHEMA_HASH,
  schemaChanged: false,
};

function cloneSuggestions() {
  return JSON.parse(
    JSON.stringify(e2eOzonAttributeSuggestions),
  ) as typeof e2eOzonAttributeSuggestions;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });
  return { promise, resolve };
}

function watchPersistenceWrites(admin: AdminPage) {
  admin.writeGuard.allow({
    operation: "save-ozon-after-ai",
    method: "PUT",
    path: configPath,
    response: ok({}),
  });
  admin.writeGuard.allow({
    operation: "preflight-ozon-after-ai",
    method: "POST",
    path: preflightPath,
    response: ok(readinessPassed),
  });
  admin.writeGuard.allow({
    operation: "publish-ozon-after-ai",
    method: "POST",
    path: publishPath,
    response: ok({}),
  });
}

function expectNoSaveOrPublish(admin: AdminPage) {
  expect(admin.writeGuard.calls("save-ozon-after-ai")).toHaveLength(0);
  expect(admin.writeGuard.calls("publish-ozon-after-ai")).toHaveLength(0);
}

async function openAttributeStep(admin: AdminPage, page: Page) {
  await admin.goto(centerPath);
  await expect(page.getByText("刊登中心", { exact: true }).first()).toBeVisible(
    {
      timeout: 30_000,
    },
  );
  await expect(page.locator(".publishing-center__wizard")).toHaveAttribute(
    "data-config-ready",
    "true",
  );
  await page.getByText("Ozon 类目与属性", { exact: true }).first().click();
  await expect(
    page.getByRole("button", { name: "AI 填写空白项" }),
  ).toBeVisible();
}

test.describe("@smoke @ozon-publish AI 填写 Ozon 属性空白项", () => {
  test("fills only blanks once, marks medium confidence, clears preflight, and restores the form on undo", async ({
    admin,
    page,
  }) => {
    test.setTimeout(120_000);
    const responseGate = deferred<ReturnType<typeof ok>>();
    admin.writeGuard.allow({
      operation: "suggest-ozon-attributes",
      method: "POST",
      path: suggestionPath,
      response: () => responseGate.promise,
    });
    watchPersistenceWrites(admin);

    await admin.goto(centerPath);
    await expect(
      page.getByText("刊登中心", { exact: true }).first(),
    ).toBeVisible({
      timeout: 90_000,
    });
    await expect(page.locator(".publishing-center__wizard")).toHaveAttribute(
      "data-config-ready",
      "true",
      { timeout: 30_000 },
    );
    await page.getByText("发布前检查与提交", { exact: true }).first().click();
    await page.getByRole("button", { name: "运行发布前检查" }).click();
    await admin.writeGuard.expectRequestCount("preflight-ozon-after-ai", 1);
    await expect(page.getByText("只读检查通过", { exact: true })).toBeVisible();

    await page.getByText("Ozon 类目与属性", { exact: true }).first().click();
    const brand = page.getByRole("textbox", { name: "品牌" });
    const capacity = page.getByRole("spinbutton", { name: "容量" });
    const polarized = page.getByLabel("是否偏光");
    const polarizedItem = page
      .locator(".ant-form-item")
      .filter({ has: polarized })
      .first();
    await expect(brand).toHaveValue("E2E");
    await expect(capacity).toHaveValue("");
    await expect(
      polarizedItem.getByText("是（true）", { exact: true }),
    ).toHaveCount(0);

    const fill = page.getByRole("button", { name: "AI 填写空白项" });
    await fill.click();
    await expect(fill).toBeDisabled();
    await fill.click({ force: true });
    await admin.writeGuard.expectRequestCount("suggest-ozon-attributes", 1);
    responseGate.resolve(ok(cloneSuggestions()));

    const feedback = page.getByLabel("AI 属性填写结果");
    await expect(feedback).toBeVisible();
    await expect(
      page.getByText("AI 已部分填写空白项", { exact: true }),
    ).toBeVisible();
    await expect(feedback.getByText("已填写 2", { exact: true })).toBeVisible();
    await expect(
      feedback.getByText("建议核对 1", { exact: true }),
    ).toBeVisible();
    await expect(feedback.getByText("未找到 2", { exact: true })).toBeVisible();
    await expect(brand).toHaveValue("E2E");
    await expect(capacity).toHaveValue("24");
    await expect(
      polarizedItem.getByText("是（true）", { exact: true }),
    ).toBeVisible();
    await expect(page.getByText("AI 建议", { exact: true })).toHaveCount(2);
    await expect(page.getByText("建议核对", { exact: true })).toHaveCount(1);

    const request = admin.writeGuard.calls("suggest-ozon-attributes")[0]
      .postDataJSON as {
      shopId: string;
      categoryId: string;
      templateFingerprint: string;
      currentValues: {
        attributes: Record<string, unknown>;
        skuVariantAttributeIds?: string[];
      };
    };
    expect(request).toMatchObject({
      shopId: E2E_OZON_SHOP_ID,
      categoryId: E2E_OZON_CATEGORY_ID,
      templateFingerprint: E2E_OZON_SCHEMA_HASH,
    });
    expect(request.currentValues.attributes["85"]).toBeTruthy();
    expect(request.currentValues.attributes["20001"]).toBeFalsy();
    expectNoSaveOrPublish(admin);
    await admin.writeGuard.expectRequestCount("preflight-ozon-after-ai", 1);

    await page.getByText("发布前检查与提交", { exact: true }).first().click();
    await expect(page.getByText("只读检查通过", { exact: true })).toBeHidden();
    await page.getByText("Ozon 类目与属性", { exact: true }).first().click();

    await page.getByRole("button", { name: "撤销本次 AI 填写" }).click();
    await expect(brand).toHaveValue("E2E");
    await expect(capacity).toHaveValue("");
    await expect(
      polarizedItem.getByText("是（true）", { exact: true }),
    ).toHaveCount(0);
    await expect(page.getByText("AI 建议", { exact: true })).toHaveCount(0);
    expectNoSaveOrPublish(admin);
  });

  test("discards a stale template response without changing the form", async ({
    admin,
    page,
  }) => {
    const stale = cloneSuggestions();
    stale.context.templateFingerprint = "stale-template-fingerprint";
    stale.context.fingerprint = "stale-context-fingerprint";
    admin.writeGuard.allow({
      operation: "suggest-stale-ozon-attributes",
      method: "POST",
      path: suggestionPath,
      response: ok(stale),
    });
    watchPersistenceWrites(admin);

    await openAttributeStep(admin, page);
    const capacity = page.getByRole("spinbutton", { name: "容量" });
    await page.getByRole("button", { name: "AI 填写空白项" }).click();
    await admin.writeGuard.expectRequestCount(
      "suggest-stale-ozon-attributes",
      1,
    );
    await expect(capacity).toHaveValue("");
    await expect(page.getByLabel("AI 属性填写结果")).toHaveCount(0);
    await expect(page.getByText("AI 建议", { exact: true })).toHaveCount(0);
    expectNoSaveOrPublish(admin);
    await admin.writeGuard.expectRequestCount("preflight-ozon-after-ai", 0);
  });

  test("keeps manual values after a failed request and does not retry automatically", async ({
    admin,
    page,
  }) => {
    admin.consoleGuard.allow(
      /^Failed to load resource: the server responded with a status of 502 \(Bad Gateway\)$/,
    );
    admin.writeGuard.allow({
      operation: "suggest-failed-ozon-attributes",
      method: "POST",
      path: suggestionPath,
      status: 502,
      response: fail("AI provider timeout", 50201, {
        errorCode: "OZON_ATTRIBUTE_SUGGESTION_AI_FAILED",
      }),
    });
    watchPersistenceWrites(admin);

    await openAttributeStep(admin, page);
    const capacity = page.getByRole("spinbutton", { name: "容量" });
    await capacity.fill("17");
    const fill = page.getByRole("button", { name: "AI 填写空白项" });
    await fill.click();
    await expect(
      page.getByText("AI 填写失败，现有输入未变更", { exact: true }),
    ).toBeVisible();
    await expect(capacity).toHaveValue("17");
    await expect(fill).toBeEnabled();
    await page.waitForTimeout(750);
    await admin.writeGuard.expectRequestCount(
      "suggest-failed-ozon-attributes",
      1,
    );
    expectNoSaveOrPublish(admin);
    await admin.writeGuard.expectRequestCount("preflight-ozon-after-ai", 0);
  });

  test("readonly users cannot start or apply AI suggestions", async ({
    admin,
    page,
  }) => {
    admin.writeGuard.allow({
      operation: "readonly-suggest-ozon-attributes",
      method: "POST",
      path: suggestionPath,
      response: ok(cloneSuggestions()),
    });
    watchPersistenceWrites(admin);
    await page.route("**/api/v1/auth/profile", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          ok({
            ...e2eUser,
            role: "readonly",
            permissions: ["product.view"],
            storePermissions: [],
          }),
        ),
      });
    });

    await openAttributeStep(admin, page);
    await expect(
      page.getByRole("button", { name: "AI 填写空白项" }),
    ).toBeDisabled();
    await admin.writeGuard.expectRequestCount(
      "readonly-suggest-ozon-attributes",
      0,
    );
    expectNoSaveOrPublish(admin);
    await admin.writeGuard.expectRequestCount("preflight-ozon-after-ai", 0);
  });

  for (const viewport of [
    { width: 1440, height: 900 },
    { width: 768, height: 900 },
    { width: 375, height: 812 },
  ]) {
    test(`keeps AI feedback within the viewport at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      admin.writeGuard.allow({
        operation: `responsive-suggest-ozon-attributes-${viewport.width}`,
        method: "POST",
        path: suggestionPath,
        response: ok(cloneSuggestions()),
      });
      watchPersistenceWrites(admin);
      await page.setViewportSize(viewport);
      await openAttributeStep(admin, page);
      await page.getByRole("button", { name: "AI 填写空白项" }).click();
      const feedback = page.getByLabel("AI 属性填写结果");
      await expect(feedback).toBeVisible();
      await expectNoRootOverflow(page);
      const bounds = await feedback.boundingBox();
      expect(bounds).not.toBeNull();
      expect(bounds?.x ?? -1).toBeGreaterThanOrEqual(0);
      expect((bounds?.x ?? 0) + (bounds?.width ?? 0)).toBeLessThanOrEqual(
        viewport.width + 1,
      );
      await admin.writeGuard.expectRequestCount(
        `responsive-suggest-ozon-attributes-${viewport.width}`,
        1,
      );
      expectNoSaveOrPublish(admin);
      await admin.writeGuard.expectRequestCount("preflight-ozon-after-ai", 0);
    });
  }
});

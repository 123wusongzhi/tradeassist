import { test, expect } from "../fixtures/admin.fixture";
import type { Page } from "@playwright/test";
import { ok } from "../mocks/envelope";
import { e2eUser } from "../mocks/auth";

const order = {
  id: "e2e-order-sku",
  tenantId: 1,
  platform: "manual",
  orderNo: "E2E-SKU-ORDER",
  customerName: "E2E Buyer",
  status: "paid",
  paymentStatus: "paid",
  fulfillmentStatus: "pending",
  currency: "CNY",
  totalAmount: 100,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
  items: [
    {
      id: "e2e-item-sku",
      productTitle: "待人工绑定的明细",
      quantity: 1,
      unitPrice: 100,
    },
  ],
  shipments: [],
};

const securityViewports = [
  { name: "mobile-small", width: 375, height: 667 },
  { name: "mobile", width: 390, height: 844 },
  { name: "tablet", width: 768, height: 1024 },
  { name: "desktop", width: 1280, height: 800 },
  { name: "desktop-wide", width: 1536, height: 864 },
] as const;

async function expectNoRootOverflow(page: Page) {
  await expect
    .poll(() =>
      page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth + 1,
      ),
    )
    .toBe(true);
}

test.describe("@security secure cookie session and SKU confirmation", () => {
  for (const viewport of securityViewports) {
    test(`recovers via refresh and confirms one binding at ${viewport.name}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      // This runs after the shared auth seeding script, so the page starts with no persisted access token.
      await page.addInitScript(() => {
        window.localStorage.removeItem("trademind_admin_token");
        window.localStorage.removeItem("trademind_auth_session_mode");
      });
      let refreshCalls = 0;
      await page.route("**/api/v1/auth/refresh", async (route) => {
        refreshCalls += 1;
        await route.fulfill({
          json: ok({
            token: "refreshed-e2e-token",
            sessionMode: "secure_session",
          }),
        });
      });
      await page.route("**/api/v1/auth/profile", async (route) =>
        route.fulfill({ json: ok(e2eUser) }),
      );
      await page.route("**/api/v1/orders/e2e-order-sku", async (route) =>
        route.fulfill({ json: ok(order) }),
      );
      await page.route(
        "**/api/v1/orders/e2e-order-sku/sku-matches",
        async (route) =>
          route.fulfill({
            json: ok({
              items: [
                {
                  id: "e2e-match-sku",
                  orderItemId: "e2e-item-sku",
                  productTitle: "待人工绑定的明细",
                  matchStatus: "unmatched",
                },
              ],
            }),
          }),
      );
      await page.route(
        "**/api/v1/orders/e2e-order-sku/inventory-effects**",
        async (route) =>
          route.fulfill({
            json: ok({
              list: [],
              pagination: { page: 1, pageSize: 100, total: 0, totalPages: 1 },
            }),
          }),
      );
      await page.route(
        "**/api/v1/order-items/e2e-item-sku/sku-candidates**",
        async (route) =>
          route.fulfill({
            json: ok({
              list: [
                {
                  productSkuId: "e2e-local-sku",
                  skuCode: "E2E-LOCAL-SKU",
                  confidence: 95,
                  source: "e2e",
                  reason: "fixture",
                },
              ],
            }),
          }),
      );
      admin.writeGuard.allow({
        operation: "sku-bind",
        method: "POST",
        path: /^\/api\/v1\/order-items\/e2e-item-sku\/bind-sku$/,
        response: ok({ item: order.items[0] }),
      });

      await admin.goto("/orders/e2e-order-sku?tab=sku");
      await expect.poll(() => refreshCalls).toBe(1);
      await expect(
        page.getByRole("button", { name: "绑定 SKU" }),
      ).toBeVisible();
      await expectNoRootOverflow(page);
      await page.getByRole("button", { name: "绑定 SKU" }).click();
      await page.getByRole("button", { name: "选择候选" }).click();
      await page.getByRole("button", { name: "二次确认绑定" }).click();
      await expect(page.getByText("人工绑定规格").last()).toBeVisible();
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount("sku-bind", 0);
      await page.getByRole("button", { name: "确认执行" }).click();
      await admin.writeGuard.expectRequestCount("sku-bind", 1);
      await expect(
        page.evaluate(() =>
          window.localStorage.getItem("trademind_admin_token"),
        ),
      ).resolves.toBeNull();
    });
  }
});

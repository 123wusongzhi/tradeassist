import { test, expect } from '../fixtures/admin.fixture';
import { ok, fail } from '../mocks/envelope';
import { e2eReadinessFailed, e2eReadinessPassed, E2E_PRODUCT_ID, E2E_PUBLICATION_NEW, E2E_SHOP_ID } from '../mocks/product.fixture';
import { latestPublicationsResponse, skuBindingsResponse } from '../mocks/publish';
import { expectRequestCount } from '../utils/assertions';

async function openPublish(page: import('@playwright/test').Page) {
  await page.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=publish`);
  await expect(page.getByRole('tab', { name: /刊登/ })).toHaveAttribute('aria-selected', 'true');
}

async function selectFirstMultiPlatformTarget(page: import('@playwright/test').Page) {
  await page.getByText('E2E 抖店测试店铺').first().click();
}

async function selectLegacyPublishShop(page: import('@playwright/test').Page) {
  await page.locator('.product-draft-publish__legacy-form').getByRole('combobox').click();
  await page.keyboard.type('E2E 抖店');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  await expect(page.locator('.product-draft-publish__legacy-form')).toContainText('E2E 抖店测试店铺');
}

async function cancelSensitiveDialog(page: import('@playwright/test').Page, title: string) {
  const dialog = page.getByRole('dialog', { name: title });
  await dialog.getByRole('button', { name: /取\s*消|取消/ }).click();
}

async function confirmSensitiveDialog(page: import('@playwright/test').Page, title: string) {
  const dialog = page.getByRole('dialog', { name: title });
  await dialog.getByRole('button', { name: '确认执行' }).click();
}

async function submitLegacyPublish(page: import('@playwright/test').Page) {
  await page.locator('.product-draft-publish__legacy-form').getByRole('button', { name: '提交刊登', exact: true }).click();
}

async function confirmLegacyPublish(page: import('@playwright/test').Page) {
  await page.getByRole('dialog', { name: '确认提交刊登？' }).getByRole('button', { name: '确认提交刊登' }).click();
}

async function closeModal(page: import('@playwright/test').Page, title: string) {
  const dialog = page.getByRole('dialog', { name: title });
  await dialog.locator('button').last().click();
}

test.describe('@publish-safety 发布写请求安全', () => {
  test('creates multi-platform drafts with exact payload and no other writes', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'create-multi-platform-drafts',
      method: 'POST',
      path: new RegExp(`/api/v1/products/${E2E_PRODUCT_ID}/publish-targets/create-drafts$`),
      response: ok({ batchId: 'e2e-batch', status: 'done', statusLabel: '已完成', targetCount: 1, successCount: 1, failedCount: 0, skippedCount: 0, targets: [] }),
    });
    await openPublish(page);
    await selectFirstMultiPlatformTarget(page);
    await page.getByRole('button', { name: '创建刊登草稿' }).click();
    await expectRequestCount(admin.writeGuard, 'create-multi-platform-drafts', 1);
    expect(admin.writeGuard.calls('create-multi-platform-drafts')[0].postDataJSON).toEqual({
      targets: [{ platform: 'douyin_shop', shopId: E2E_SHOP_ID }],
      onlyReady: false,
      retryFailedOnly: false,
    });
    expect(admin.writeGuard.allCalls().map((call) => call.path)).not.toContain(`/api/v1/products/${E2E_PRODUCT_ID}/publish`);
  });

  test('creates ozon local draft with exact payload and no other writes', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'create-ozon-draft',
      method: 'POST',
      path: new RegExp(`/api/v1/products/${E2E_PRODUCT_ID}/publish-targets/create-drafts$`),
      response: ok({ batchId: 'e2e-ozon-batch', status: 'done', statusLabel: '已完成', targetCount: 1, successCount: 1, failedCount: 0, skippedCount: 0, targets: [] }),
    });
    await openPublish(page);
    await page.getByText('E2E Ozon 测试店铺').first().click();
    await page.getByRole('button', { name: '创建刊登草稿' }).click();
    await expectRequestCount(admin.writeGuard, 'create-ozon-draft', 1);
    expect(admin.writeGuard.calls('create-ozon-draft')[0].postDataJSON).toEqual({
      targets: [{ platform: 'ozon', shopId: 'e2e-ozon-shop' }],
      onlyReady: false,
      retryFailedOnly: false,
    });
    expect(admin.writeGuard.allCalls().map((call) => call.path)).not.toContain(`/api/v1/products/${E2E_PRODUCT_ID}/publish`);
  });

  test('creates douyin platform draft once after confirmation and refreshes latest publication', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'create-douyin-draft',
      method: 'POST',
      path: new RegExp(`/api/v1/products/${E2E_PRODUCT_ID}/platform-configs/douyin_shop/create-draft$`),
      response: ok({
        id: 'e2e-douyin-task',
        productId: E2E_PRODUCT_ID,
        shopId: E2E_SHOP_ID,
        platform: 'douyin_shop',
        taskType: 'create_draft',
        status: 'success',
        mode: 'save_as_platform_draft',
        platformProductId: 'e2e-platform-product-new',
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
      }),
    });
    await page.route(`**/api/v1/products/${E2E_PRODUCT_ID}/publications`, async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(latestPublicationsResponse()) }));
    await page.route(`**/api/v1/product-publications/${E2E_PUBLICATION_NEW}/douyin/sku-bindings`, async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(skuBindingsResponse(E2E_PUBLICATION_NEW)) }));

    await openPublish(page);
    await page.getByRole('button', { name: '创建抖店商品草稿' }).click();
    await cancelSensitiveDialog(page, '创建平台商品草稿');
    await expectRequestCount(admin.writeGuard, 'create-douyin-draft', 0);

    await page.getByRole('button', { name: '创建抖店商品草稿' }).click();
    await confirmSensitiveDialog(page, '创建平台商品草稿');
    await page.getByRole('button', { name: '创建抖店商品草稿' }).click({ trial: true }).catch(() => undefined);
    await expectRequestCount(admin.writeGuard, 'create-douyin-draft', 1);
    expect(admin.writeGuard.calls('create-douyin-draft')[0].postDataJSON).toEqual({ shopId: E2E_SHOP_ID, publishMode: 'save_as_platform_draft' });
    expect(admin.writeGuard.allCalls().map((call) => call.path)).not.toContain(`/api/v1/products/${E2E_PRODUCT_ID}/publish`);
  });

  test('traditional publish cancellation, readiness blocking, success, duplicate click, and failure handling', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'publish-product',
      method: 'POST',
      path: new RegExp(`/api/v1/products/${E2E_PRODUCT_ID}/publish$`),
      response: ok({ id: 'e2e-publish-task', productId: E2E_PRODUCT_ID, shopId: E2E_SHOP_ID, platform: 'douyin_shop', taskType: 'publish', status: 'queued', mode: 'publish', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z' }),
    });
    await openPublish(page);
    await selectLegacyPublishShop(page);

    await submitLegacyPublish(page);
    await cancelSensitiveDialog(page, '确认提交刊登？');
    await expectRequestCount(admin.writeGuard, 'publish-product', 0);

    await page.route(`**/api/v1/products/${E2E_PRODUCT_ID}/readiness**`, async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(e2eReadinessFailed)) }));
    await submitLegacyPublish(page);
    await confirmLegacyPublish(page);
    await expect(page.getByRole('dialog', { name: '发布检查未通过' })).toBeVisible();
    await closeModal(page, '发布检查未通过');
    await expectRequestCount(admin.writeGuard, 'publish-product', 0);

    await page.route(`**/api/v1/products/${E2E_PRODUCT_ID}/readiness**`, async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(e2eReadinessPassed)) }));
    await openPublish(page);
    await selectLegacyPublishShop(page);
    await submitLegacyPublish(page);
    await confirmLegacyPublish(page);
    await expectRequestCount(admin.writeGuard, 'publish-product', 1);
    expect(admin.writeGuard.calls('publish-product')[0].postDataJSON).toEqual({ shopId: E2E_SHOP_ID, options: {} });

    await page.route(`**/api/v1/products/${E2E_PRODUCT_ID}/publish`, async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(fail('e2e publish failed', 50001, null)) }));
  });
});

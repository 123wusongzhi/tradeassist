import { test, expect } from '../fixtures/admin.fixture';
import { ProductDraftDetailPage } from '../pages/product-draft-detail.page';
import { e2eProduct, E2E_PRODUCT_ID } from '../mocks/product.fixture';
import { ok } from '../mocks/envelope';
import { expectActiveTab, expectSectionVisible } from '../utils/assertions';

const tabs = [
  ['basic', '基础信息'],
  ['ai', 'AI 文案'],
  ['images', '图片管理'],
  ['skus', '商品规格'],
  ['inventory', '库存'],
  ['readiness', '发布检查'],
  ['publish', '刊登'],
] as const;

test.describe('@product-draft 商品详情导航', () => {
  for (const [key, label] of tabs) {
    test(`opens ${key} tab by deep link`, async ({ page }) => {
      const detail = new ProductDraftDetailPage(page);
      await detail.goto(key);
      await expectActiveTab(page, label);
      await expect(page).toHaveURL(new RegExp(`/product/drafts/${E2E_PRODUCT_ID}.*tab=${key}`));
    });
  }

  test('falls back from invalid tab to basic', async ({ page }) => {
    await page.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=unknown-tab`);
    await expectActiveTab(page, '基础信息');
  });

  test('shows collected packaging rows without inferring missing measurements', async ({ page }) => {
    await page.route(`**/api/v1/products/${E2E_PRODUCT_ID}`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({
          ...e2eProduct,
          source: '1688',
          rawData: {
            raw: e2eProduct.rawData.raw,
            packaging: {
              rows: [
                { specification: '22#橡胶塞', lengthCm: 1, widthCm: 1, heightCm: 1, volumeCm3: 1, weightG: 2000 },
                { specification: '双孔8#橡胶塞', lengthCm: null, widthCm: null, heightCm: null, volumeCm3: null, weightG: 100 },
              ],
            },
          },
        })),
      });
    });
    const detail = new ProductDraftDetailPage(page);
    await detail.goto('basic');

    await expect(page.getByText('包装信息', { exact: true })).toBeVisible();
    await expect(page.getByRole('columnheader', { name: '体积(cm³)', exact: true })).toBeVisible();
    const missingRow = page.getByRole('row').filter({ hasText: '双孔8#橡胶塞' });
    await expect(missingRow).toContainText('100');
    await expect(missingRow.getByRole('cell').nth(1)).toHaveText('—');
    await expect(page.getByRole('row').filter({ hasText: '22#橡胶塞' })).toContainText('2000');
    for (const viewport of [
      { width: 1280, height: 800 },
      { width: 768, height: 900 },
      { width: 390, height: 844 },
    ]) {
      await page.setViewportSize(viewport);
      await expect(page.getByText('包装信息', { exact: true })).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
    }
  });

  test('hides packaging section for non-1688 products without packaging rows', async ({ page }) => {
    const detail = new ProductDraftDetailPage(page);
    await detail.goto('basic');
    await expect(page.getByText('包装信息', { exact: true })).toHaveCount(0);
  });

  test('restores readiness publish-check after refresh', async ({ page }) => {
    await page.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=readiness&section=publish-check`);
    await expectActiveTab(page, '发布检查');
    await expectSectionVisible(page, 'publish-check');
    await page.reload();
    await expectActiveTab(page, '发布检查');
    await expectSectionVisible(page, 'publish-check');
  });

  test('restores publish config and douyin sku binding deep links after refresh', async ({ page }) => {
    await page.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=publish&section=publish-config`);
    await expectActiveTab(page, '刊登');
    await expectSectionVisible(page, 'publish-config');
    await page.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=publish&section=douyin-sku-bindings`);
    await expectActiveTab(page, '刊登');
    await expectSectionVisible(page, 'douyin-sku-bindings');
    await page.reload();
    await expectActiveTab(page, '刊登');
    await expectSectionVisible(page, 'douyin-sku-bindings');
  });

  test('inventory manage binding navigates to publish sku binding section', async ({ page }) => {
    await page.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=inventory`);
    await expectActiveTab(page, '库存');
    const manage = page.getByRole('button', { name: '管理绑定' }).first();
    if (await manage.isVisible()) {
      await manage.click();
      await expectActiveTab(page, '刊登');
      await expect(page).toHaveURL(/tab=publish/);
      await expect(page).toHaveURL(/section=douyin-sku-bindings/);
    }
  });
});

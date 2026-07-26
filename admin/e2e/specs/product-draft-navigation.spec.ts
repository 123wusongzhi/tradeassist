import { test, expect } from '../fixtures/admin.fixture';
import { ProductDraftDetailPage } from '../pages/product-draft-detail.page';
import { E2E_PRODUCT_ID } from '../mocks/product.fixture';
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

import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eProduct, E2E_PRODUCT_ID } from '../mocks/product.fixture';

const external1688ImageUrl = 'https://cbu01.alicdn.com/img/ibank/e2e-1688-main.jpg';
const storedImageUrl = 'https://storage.example.test/products/e2e-1688-main.jpg';
const onePixelPng = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII=',
  'base64',
);

test.describe('@smoke @product-draft 1688 商品图片同步', () => {
  test('recovers a failed external preview after an explicit main-image sync', async ({ admin, page }) => {
    test.setTimeout(120_000);
    let synced = false;
    let imageRequestCount = 0;
    let storedImageRequestCount = 0;
    let imageReferer: string | undefined;
    await page.route(external1688ImageUrl, async (route) => {
      imageRequestCount += 1;
      imageReferer = route.request().headers().referer;
      await route.fulfill({ status: 200, contentType: 'image/jpeg', body: 'invalid image bytes' });
    });
    await page.route(storedImageUrl, async (route) => {
      storedImageRequestCount += 1;
      await route.fulfill({ status: 200, contentType: 'image/png', body: onePixelPng });
    });
    await page.route(`**/api/v1/products/${E2E_PRODUCT_ID}`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({
          ...e2eProduct,
          source: '1688',
          sourceUrl: 'https://detail.1688.com/offer/123456789.html',
          mainImages: [external1688ImageUrl],
          images: [
            {
              ...e2eProduct.images[0],
              source: '1688',
              originUrl: external1688ImageUrl,
              publicUrl: synced ? storedImageUrl : external1688ImageUrl,
              objectKey: synced ? 'products/e2e-1688-main.jpg' : undefined,
              storageKey: undefined,
            },
          ],
        })),
      });
    });
    admin.writeGuard.allow({
      operation: 'sync-1688-main-images',
      method: 'POST',
      path: new RegExp(`/api/v1/products/${E2E_PRODUCT_ID}/sync-images$`),
      response: () => {
        synced = true;
        return ok({ synced: 1, skipped: 0, failed: 0 });
      },
    });

    await page.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=images`);

    await expect(page.getByText('1688 采集图片默认为外链，发布前建议同步到平台存储，避免防盗链或外链失效。')).toBeVisible({ timeout: 90_000 });
    await expect.poll(() => imageRequestCount).toBe(1);
    expect(imageReferer).toBeUndefined();
    await expect(page.getByText('加载失败')).toBeVisible();
    await admin.writeGuard.expectRequestCount('sync-1688-main-images', 0);

    await page.getByRole('button', { name: '批量同步主图' }).click();

    await admin.writeGuard.expectRequestCount('sync-1688-main-images', 1);
    expect(admin.writeGuard.calls('sync-1688-main-images')[0]?.postDataJSON).toEqual({ scope: 'main' });
    await expect(page.getByText('已同步 1 张主图')).toBeVisible();
    await expect.poll(() => storedImageRequestCount).toBe(1);
    const recoveredPreview = page.locator('img.product-draft-images__thumb-image').first();
    await expect(recoveredPreview).toHaveAttribute('src', storedImageUrl);
    await expect(recoveredPreview).toBeVisible();
    await expect.poll(() => recoveredPreview.evaluate((image: HTMLImageElement) => ({
      complete: image.complete,
      naturalWidth: image.naturalWidth,
      naturalHeight: image.naturalHeight,
    }))).toEqual({ complete: true, naturalWidth: 1, naturalHeight: 1 });
    await expect(page.getByText('加载失败')).toHaveCount(0);
    await page.waitForTimeout(750);
    await expect(page.getByText('加载失败')).toHaveCount(0);
  });
});

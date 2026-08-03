import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import {
  expectActiveTab,
  expectHeaderContentAligned,
  expectNoRootOverflow,
} from '../utils/assertions';
import { ok } from '../mocks/envelope';

const viewports = [
  { name: 'desktop-large', width: 1440, height: 900 },
  { name: 'desktop', width: 1280, height: 800 },
  { name: 'tablet-landscape', width: 1024, height: 768 },
  { name: 'tablet', width: 768, height: 900 },
  { name: 'mobile', width: 375, height: 812 },
];

async function waitForPlatformSettingsBundle(page: Page) {
  await expect(page.getByText('平台接入设置', { exact: true }).first()).toBeVisible({
    timeout: 30_000,
  });
}

test.describe('@platform-settings-ozon Ozon 平台接入入口', () => {
  for (const viewport of viewports) {
    test(`shows truthful shop-level guidance at ${viewport.name}`, async ({ admin, page }) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      await admin.goto('/settings/platforms?platform=ozon');

      await waitForPlatformSettingsBundle(page);
      await expectActiveTab(page, 'Ozon');
      await expect(page.getByText('Ozon 无需平台级应用配置')).toBeVisible();
      await expect(page.getByText('店铺级授权', { exact: true })).toBeVisible();
      await expect(page.getByText('无需应用配置', { exact: true })).toBeVisible();
      await expect(page.getByText(/Client-ID、Api-Key/)).toBeVisible();
      await expect(page.getByText(/重量、尺寸、增值税率、仓库和币种/)).toBeVisible();
      await expect(page.getByText(/选择类目和动态属性，执行发布前检查/)).toBeVisible();

      await expect(page.getByRole('link', { name: '前往店铺授权' })).toHaveAttribute(
        'href',
        '/shops/manage',
      );
      await expect(page.getByRole('link', { name: '前往 Ozon 刊登预设' })).toHaveAttribute(
        'href',
        '/settings/platform-publish',
      );
      await expect(page.getByRole('link', { name: '进入商品级 Ozon 流程' })).toHaveAttribute(
        'href',
        '/product/ozon-publish',
      );
      await expect(page.getByRole('button', { name: '保存设置' })).toHaveCount(0);
      await expect(page.getByRole('button', { name: '测试连接' })).toHaveCount(0);

      await expectHeaderContentAligned(page);
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('falls back from an unknown platform query without hiding the platform list', async ({
    admin,
    page,
  }) => {
    await admin.goto('/settings/platforms?platform=unknown-platform');

    await waitForPlatformSettingsBundle(page);
    await expectActiveTab(page, '抖店');
    await expect(page.getByRole('tab', { name: /Ozon/ })).toBeVisible();
    await expect
      .poll(() => new URL(page.url()).searchParams.get('platform'))
      .toBe('douyin_shop');
    await expectNoRootOverflow(page);
  });

  test('keeps the empty state readable and free of writes', async ({ admin, page }) => {
    await page.route('**/api/v1/platform/providers', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: [] })),
      });
    });

    await admin.goto('/settings/platforms?platform=ozon');

    await waitForPlatformSettingsBundle(page);
    await expect(page.getByText('暂无可展示的平台接入信息，请刷新页面后重试。')).toBeVisible();
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('shows Ozon in the integration overview without a fake configured state', async ({
    admin,
    page,
  }) => {
    await admin.goto('/settings/integrations');

    const ozonCard = page.getByRole('group', { name: 'Ozon 接入状态' });
    await expect(ozonCard.getByText('Ozon', { exact: true })).toBeVisible();
    await expect(ozonCard.getByText('店铺级授权', { exact: true })).toBeVisible();
    await expect(ozonCard.getByText('无需应用配置', { exact: true })).toBeVisible();
    await expect(ozonCard.getByText('已配置', { exact: true })).toHaveCount(0);

    const entry = ozonCard.getByRole('link', { name: '查看授权入口' });
    await expect(entry).toHaveAttribute('href', '/settings/platforms?platform=ozon');
    await entry.focus();
    await expect(entry).toBeFocused();
    await page.keyboard.press('Enter');

    await waitForPlatformSettingsBundle(page);
    await expectActiveTab(page, 'Ozon');
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});

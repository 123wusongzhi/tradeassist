import { expect, test } from '../fixtures/admin.fixture';
import type { Page } from '@playwright/test';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { expectNoRootOverflow } from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

const e2eDevices = [
  {
    id: 'e2e-device-active',
    name: 'E2E Chrome 侧边栏',
    status: 'active',
    expiresAt: '2026-11-01T00:00:00Z',
    lastUsedAt: '2026-07-31T12:00:00Z',
    createdAt: '2026-07-31T00:00:00Z',
  },
  {
    id: 'e2e-device-expired',
    name: 'E2E 旧设备',
    status: 'expired',
    expiresAt: '2026-07-01T00:00:00Z',
    lastUsedAt: null,
    createdAt: '2026-06-01T00:00:00Z',
  },
];

/**
 * 覆盖 fixture 的 `['*']` 权限：真实后端返回显式权限列表，前端按精确匹配判断。
 * 只给本页需要的 product.view / product.write，避免全权限渲染完整侧边栏菜单
 * （完整菜单会触发既有 `/inventory` 重复 key 的 antd console.error）。
 */
async function routeAdminProfileWithPermissions(page: Page) {
  await page.route('**/api/v1/auth/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          ...e2eUser,
          permissions: ['product.view', 'product.write'],
        }),
      ),
    });
  });
}

async function routeBrowserExtensionReads(page: Page, devices: unknown[] = []) {
  await page.route('**/api/v1/collect/browser-extension/**', async (route) => {
    const request = route.request();
    if (request.method() !== 'GET') {
      await route.fallback();
      return;
    }
    const path = new URL(request.url()).pathname;
    if (path === '/api/v1/collect/browser-extension/devices') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: devices })),
      });
      return;
    }
    await route.fallback();
  });
}

test.describe('@collect browser-extension pairing page', () => {
  test.beforeEach(async ({ page }) => {
    await routeAdminProfileWithPermissions(page);
  });

  for (const viewport of viewports) {
    test(`renders empty state without overflow at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await routeBrowserExtensionReads(page);

      await admin.goto('/collect/browser-extension');

      // 首次导航会触发 dev server 的 MFSU 编译，首屏断言需留足时间
      await expect(page.getByText('浏览器扩展采集')).toBeVisible({ timeout: 40_000 });
      await expect(page.getByText('安装与首次连接')).toBeVisible();
      await expect(page.getByText('为什么更省心')).toBeVisible();
      await expect(page.getByText('还没有连接浏览器扩展')).toBeVisible();
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('generates a pairing code and shows it for copying', async ({ admin, page }) => {
    await routeBrowserExtensionReads(page);
    admin.writeGuard.allow({
      operation: 'create-pairing',
      method: 'POST',
      path: /\/api\/v1\/collect\/browser-extension\/pairings$/,
      response: ok({ code: 'ABCDE-FGHJK', expiresAt: '2026-08-01T00:10:00Z' }),
    });

    await admin.goto('/collect/browser-extension');
    await page.getByRole('button', { name: '生成连接信息' }).click();

    await expect(page.getByText('一次性连接信息已生成')).toBeVisible();
    await expect(page.getByText('ABCDE-FGHJK')).toBeVisible();
    await expect(page.getByRole('button', { name: '复制连接信息' })).toBeVisible();
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('create-pairing', 1);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('lists devices and revokes one with a single confirmed request', async ({ admin, page }) => {
    await routeBrowserExtensionReads(page, e2eDevices);
    admin.writeGuard.allow({
      operation: 'revoke-device',
      method: 'DELETE',
      path: /\/api\/v1\/collect\/browser-extension\/devices\/[^/]+$/,
      response: ok({ revoked: true }),
    });

    await admin.goto('/collect/browser-extension');

    await expect(page.getByText('E2E Chrome 侧边栏')).toBeVisible();
    await expect(page.getByText('E2E 旧设备')).toBeVisible();
    await expect(page.getByText('当前 1 个有效设备')).toBeVisible();

    const revokeButton = page.getByRole('button', { name: '撤销' }).first();
    await revokeButton.click();
    await expect(page.getByText('撤销这个扩展设备？')).toBeVisible();
    await page.getByRole('button', { name: /确\s*定/ }).click();

    await expect(page.getByText('扩展设备已撤销')).toBeVisible();
    await admin.writeGuard.expectRequestCount('revoke-device', 1);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});

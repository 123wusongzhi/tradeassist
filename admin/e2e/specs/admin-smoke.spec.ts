import { test, expect } from '../fixtures/admin.fixture';
import { expectNoRootOverflow } from '../utils/assertions';

const smokeRoutes = [
  { path: '/dashboard/product-operations', name: /运营总览|工作台/ },
  { path: '/collect/hub', name: /采集中心/ },
  { path: '/ai/operation-workbench', name: /商品运营工作台/ },
  { path: '/product/drafts', name: /商品草稿|E2E 商品草稿/ },
  { path: '/ops/task-center/alerts', name: /告警中心/ },
  { path: '/files', name: /文件管理/ },
];

test.describe('@smoke Admin route smoke', () => {
  for (const route of smokeRoutes) {
    test(`renders ${route.path} without login, fatal error, or writes`, async ({ admin, page }) => {
      await admin.goto(route.path);
      // A cold local Umi/MFSU route can remain on its Bundling page beyond the
      // global assertion timeout; wait for the real app root without weakening
      // any of the route, console, overflow, or write-safety assertions below.
      await expect(page.locator('#root')).toBeVisible({ timeout: 30_000 });
      await expect(page.getByText(route.name).first()).toBeVisible();
      await expect(page).not.toHaveURL(/\/user\/login/);
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }
});

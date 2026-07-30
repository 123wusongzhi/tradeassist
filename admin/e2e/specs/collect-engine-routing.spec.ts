import { test, expect } from '../fixtures/admin.fixture';
import type { Page } from '@playwright/test';
import { ok } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

type EngineRouteOptions = {
  defaultEngine?: 'opencli' | 'playwright';
  openCliReady?: boolean;
};

async function routeCollectEngineReads(page: Page, options: EngineRouteOptions = {}) {
  const defaultEngine = options.defaultEngine ?? 'playwright';
  const openCliReady = options.openCliReady ?? false;
  await page.route('**/api/v1/**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/collect/engines/status') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            defaultEngine,
            engines: [
              {
                engine: 'opencli',
                enabled: openCliReady,
                configured: true,
                reachable: openCliReady,
                ready: openCliReady,
                status: openCliReady ? 'ready' : 'disabled',
                message: openCliReady
                  ? 'OpenCLI daemon, extension, and profile are ready'
                  : 'opencli bridge is disabled',
                supportedSources: ['taobao_tmall'],
              },
              {
                engine: 'playwright',
                enabled: true,
                configured: true,
                reachable: true,
                ready: true,
                status: 'ready',
                message: 'ok',
                supportedSources: ['taobao_tmall'],
              },
            ],
          }),
        ),
      });
      return;
    }
    if (path === '/api/v1/settings') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ items: [] })),
      });
      return;
    }
    if (path === '/api/v1/collect/providers') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok([
            {
              source: 'taobao_tmall',
              name: '淘宝/天猫',
              description: 'E2E',
              status: 'available',
              batchSupported: true,
              urlPatterns: [],
              features: [],
              notes: '',
            },
          ]),
        ),
      });
      return;
    }
    await route.fallback();
  });
}

async function guardCollectorApi(page: Page) {
  const writes: string[] = [];
  await page.route('**/api/collector/**', async (route) => {
    const request = route.request();
    if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method().toUpperCase())) {
      writes.push(`${request.method()} ${new URL(request.url()).pathname}`);
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          loggedIn: true,
          status: 'logged_in',
          message: 'intercepted by E2E',
        }),
      ),
    });
  });
  return writes;
}

test.describe('@collect-engine engine status and responsive safety', () => {
  for (const viewport of viewports) {
    test(`disables unavailable OpenCLI at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await routeCollectEngineReads(page);
      await admin.goto('/collect/tasks?sourcePlatform=taobao_tmall');

      const openCliOption = page.locator('.ant-segmented-item-disabled').filter({
        hasText: 'OpenCLI',
      });
      await expect(openCliOption).toBeVisible();
      await expect(page.getByText('Playwright（备用）').first()).toBeVisible();
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  for (const viewport of viewports) {
    test(`uses ready OpenCLI without probing Playwright at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await routeCollectEngineReads(page, {
        defaultEngine: 'opencli',
        openCliReady: true,
      });
      const collectorWrites = await guardCollectorApi(page);

      await admin.goto('/collect/hub');
      await expect(page.getByText('淘宝/天猫主引擎 OpenCLI 已就绪')).toBeVisible();

      const taobaoCard = page.locator('article').filter({ hasText: 'taobao_tmall' });
      await taobaoCard.getByRole('button', { name: '开始采集' }).click();
      await expect(page.getByText('OpenCLI Bridge 已就绪').last()).toBeVisible();
      await expect(page.locator('.ant-segmented-item-selected').filter({ hasText: 'OpenCLI' })).toBeVisible();
      await expect.poll(() => collectorWrites.length).toBe(0);
      await expectNoRootOverflow(page);
      await page.getByRole('button', { name: /取\s*消/ }).click();

      await admin.goto('/settings/collector?provider=taobao_tmall');
      await expect(page.getByText('淘宝/天猫双引擎配置')).toBeVisible();
      await expect(page.getByText('OpenCLI Bridge 已就绪')).toBeVisible();
      await expect(page.locator('.ant-segmented-item-selected').filter({ hasText: 'OpenCLI' })).toBeVisible();
      await expect.poll(() => collectorWrites.length).toBe(0);
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }
});

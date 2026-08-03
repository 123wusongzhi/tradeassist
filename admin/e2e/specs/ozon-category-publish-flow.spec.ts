import { expect, test } from '../fixtures/admin.fixture';
import { E2E_PRODUCT_ID, e2eProductList } from '../mocks/product.fixture';
import {
  E2E_OZON_CATEGORY_ID,
  E2E_OZON_SHOP_ID,
  e2eOzonConfig,
} from '../mocks/ozon-publish';

const pagePath = `/product/ozon-publish?productId=${E2E_PRODUCT_ID}`;

async function openStage(page: import('@playwright/test').Page, stage: string) {
  await page.goto(`${pagePath}&stage=${stage}`);
  await expect(
    page.getByRole('tab', {
      name:
        stage === 'sync'
          ? '同步状态'
          : stage === 'mapping'
            ? '类目映射库'
            : stage === 'config'
              ? '商品配置'
              : stage === 'preflight'
                ? '发布前检查'
                : '草稿与提交',
    }),
  ).toHaveAttribute('aria-selected', 'true', { timeout: 30_000 });
}

async function expectNoRootOverflow(page: import('@playwright/test').Page) {
  const dimensions = await page.evaluate(() => ({
    html: [
      document.documentElement.scrollWidth,
      document.documentElement.clientWidth,
    ],
    body: [document.body.scrollWidth, document.body.clientWidth],
  }));
  expect(
    dimensions.html[0],
    `html overflow ${dimensions.html.join(' > ')}`,
  ).toBeLessThanOrEqual(dimensions.html[1]);
  expect(
    dimensions.body[0],
    `body overflow ${dimensions.body.join(' > ')}`,
  ).toBeLessThanOrEqual(dimensions.body[1]);
}

test.describe('@ozon-publish Ozon 类目与刊登流程', () => {
  test('shows async sync task processing, four change states, and preserves deep-link stage', async ({
    page,
  }) => {
    await openStage(page, 'sync');
    for (const change of ['added', 'changed', 'deactivated', 'reactivated'])
      await expect(page.getByText(change, { exact: true })).toBeVisible();
    await expect(
      page.getByText('任务已创建，等待处理', { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText(/全量同步只更新 TradeMind 的 Ozon 类目树缓存/),
    ).toBeVisible();
    await page.getByRole('tab', { name: '类目映射库' }).click();
    await expect(page).toHaveURL(/stage=mapping/);
    await page.reload();
    await expect(page.getByRole('tab', { name: '类目映射库' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
  });

  test('keeps recommendation unconfirmed and confirms a category group with one guarded write', async ({
    admin,
    page,
  }) => {
    admin.writeGuard.allow({
      operation: 'group-check',
      method: 'POST',
      path: /\/api\/v1\/product-publish\/ozon\/category-groups\/check$/,
      response: {
        code: 0,
        message: 'ok',
        data: {
          groups: [
            {
              key: 'e2e-source-table',
              sourceCategoryKey: 'e2e-source-table',
              sourceCategoryName: 'E2E 本地桌子',
              productIds: [E2E_PRODUCT_ID],
              recommendedCategoryId: E2E_OZON_CATEGORY_ID,
              recommendedCategoryPath: '家具 / 桌子',
              status: 'ready',
              statusLabel: '可确认',
            },
          ],
        },
      },
    });
    admin.writeGuard.allow({
      operation: 'group-confirm',
      method: 'POST',
      path: /\/api\/v1\/product-publish\/ozon\/category-groups\/confirm$/,
      response: { code: 0, message: 'ok', data: {} },
    });
    await openStage(page, 'mapping');
    await page
      .getByRole('button', { name: '检查批量类目分组' })
      .locator('..')
      .locator('.ant-select')
      .click();
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Enter');
    await page.getByRole('button', { name: '检查批量类目分组' }).click();
    await expect(page.getByText(/推荐候选，尚未确认/)).toBeVisible();
    await page.getByRole('button', { name: '确认本组类目' }).click();
    await admin.writeGuard.expectRequestCount('group-confirm', 1);
    expect(admin.writeGuard.calls('group-confirm')[0].postDataJSON).toEqual({
      shopId: E2E_OZON_SHOP_ID,
      groups: [
        {
          sourceCategoryKey: 'e2e-source-table',
          sourceCategoryName: 'E2E 本地桌子',
          productIds: [E2E_PRODUCT_ID],
          categoryId: E2E_OZON_CATEGORY_ID,
          categoryPath: '家具 / 桌子',
        },
      ],
      saveMappings: false,
    });
  });

  test('blocks preflight and submit while product configuration has unsaved changes', async ({
    page,
  }) => {
    await openStage(page, 'config');
    await page
      .getByRole('textbox', { name: '本地类目说明' })
      .fill('尚未保存的类目说明');
    await expect(
      page.getByText('商品级 Ozon 配置有未保存的修改'),
    ).toBeVisible();

    await page.getByRole('tab', { name: '发布前检查' }).click();
    await expect(
      page.getByRole('button', { name: '运行发布前检查' }),
    ).toBeDisabled();
    await page.getByRole('tab', { name: '草稿与提交' }).click();
    await expect(
      page.getByRole('button', { name: '创建本地草稿' }),
    ).toBeDisabled();
    await expect(
      page.getByRole('button', { name: '提交到 Ozon' }),
    ).toBeDisabled();
  });

  test('updates an existing product config and keeps it after refresh', async ({
    admin,
    page,
  }) => {
    let persisted = { ...e2eOzonConfig };
    await page.route(
      new RegExp(
        `/api/v1/products/${E2E_PRODUCT_ID}/platform-configs/ozon(?:\\?.*)?$`,
      ),
      async (route) => {
        if (route.request().method() !== 'GET') {
          await route.fallback();
          return;
        }
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ code: 0, message: 'ok', data: persisted }),
        });
      },
    );
    admin.writeGuard.allow({
      operation: 'ozon-config-update',
      method: 'PUT',
      path: new RegExp(
        `/api/v1/products/${E2E_PRODUCT_ID}/platform-configs/ozon$`,
      ),
      response: (record) => {
        persisted = {
          ...persisted,
          ...(record.postDataJSON as Partial<typeof persisted>),
          id: e2eOzonConfig.id,
        };
        return { code: 0, message: 'ok', data: persisted };
      },
    });

    await openStage(page, 'config');
    const sourceCategory = page.getByRole('textbox', {
      name: '本地类目说明',
    });
    await sourceCategory.fill('E2E 已更新类目');
    await page.getByRole('button', { name: '保存商品级 Ozon 配置' }).click();
    await admin.writeGuard.expectRequestCount('ozon-config-update', 1);
    await expect(
      page.getByText('商品级 Ozon 配置已保存，尚未提交到 Ozon。'),
    ).toBeVisible();
    expect(persisted.id).toBe(e2eOzonConfig.id);
    expect(persisted.sourceCategoryName).toBe('E2E 已更新类目');

    await page.reload();
    await expect(sourceCategory).toHaveValue('E2E 已更新类目');
  });

  test('shows an actionable permission reason instead of internal error', async ({
    admin,
    page,
  }) => {
    admin.consoleGuard.allow(
      /^Failed to load resource: the server responded with a status of 403 \(Forbidden\)$/,
    );
    admin.writeGuard.allow({
      operation: 'ozon-preflight-forbidden',
      method: 'POST',
      path: new RegExp(
        `/api/v1/products/${E2E_PRODUCT_ID}/readiness/validate$`,
      ),
      status: 403,
      response: {
        code: 40302,
        message:
          '全局管理员仅可跨租户查看，不能代表目标租户执行写操作；请使用目标租户管理员账号',
        data: { errorCode: 'CROSS_TENANT_OPERATION_FORBIDDEN' },
        traceId: 'e2e-ozon-forbidden',
      },
    });

    await openStage(page, 'preflight');
    await page.getByRole('button', { name: '运行发布前检查' }).click();
    await admin.writeGuard.expectRequestCount('ozon-preflight-forbidden', 1);
    const errorMessage = page.getByText(/当前为跨租户只读查看/);
    await expect(errorMessage).toBeVisible();
    await expect(page.getByText(/internal error/i)).toHaveCount(0);
  });

  test('disables tenant writes during a global-admin cross-tenant view', async ({
    page,
  }) => {
    await page.route(/\/api\/v1\/products(?:\?.*)?$/, (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          code: 0,
          message: 'ok',
          data: {
            list: [{ ...e2eProductList[0], tenantId: 1 }],
            pagination: {
              page: 1,
              pageSize: 100,
              total: 1,
              totalPages: 1,
            },
          },
        }),
      }),
    );
    await openStage(page, 'config');
    await expect(
      page.getByText('当前为跨租户只读查看', { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole('button', { name: '保存商品级 Ozon 配置' }),
    ).toBeDisabled();
    await page.getByRole('tab', { name: '发布前检查' }).click();
    await expect(
      page.getByRole('button', { name: '运行发布前检查' }),
    ).toBeDisabled();
  });

  test('shows a deactivated Ozon credential and restores the last usable category', async ({
    admin,
    page,
  }) => {
    const unavailableCategoryId = '101:201';
    await page.route('**/api/v1/platform/ozon/categories?**', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          code: 0,
          message: 'ok',
          data: {
            list: [
              {
                id: 'e2e-unavailable-category',
                categoryId: unavailableCategoryId,
                name: '失效凭证测试类目',
                descriptionCategoryId: '101',
                typeId: '201',
                isLeaf: true,
                status: 'active',
              },
            ],
            total: 1,
          },
        }),
      }),
    );
    admin.writeGuard.allow({
      operation: 'category-attribute-sync-invalid-credential',
      method: 'POST',
      path: /\/api\/v1\/platform\/ozon\/categories\/101(?:%3A|:)201\/attributes\/sync$/i,
      response: {
        code: 40001,
        message:
          'Ozon 店铺授权已失效或 API Key 已停用，请前往店铺管理更新凭证后重试',
        data: { errorCode: 'OZON_CATEGORY_ATTR_SYNC_FAILED' },
        traceId: 'e2e-ozon-invalid-credential',
      },
    });

    await openStage(page, 'config');
    const category = page.getByRole('combobox', { name: 'Ozon 叶类目' });
    await category.fill('失效凭证测试类目');
    await category.click();
    const unavailableOption = page
      .locator('.ant-select-item-option-content')
      .filter({ hasText: '失效凭证测试类目（101）' });
    await expect(unavailableOption).toBeVisible();
    await unavailableOption.click();

    await admin.writeGuard.expectRequestCount(
      'category-attribute-sync-invalid-credential',
      1,
    );
    await expect(
      page.getByText('Ozon 类目属性模板同步失败', { exact: true }),
    ).toBeVisible();
    const templateError = page
      .locator('.ant-alert-error')
      .filter({ hasText: 'Ozon 类目属性模板同步失败' });
    await expect(templateError).toContainText('API Key 已停用');
    await expect(templateError).toContainText('已恢复上一个可用类目');
    await expect(
      page.locator(
        `.ant-select-selection-item[title="${E2E_OZON_CATEGORY_ID}"]`,
      ),
    ).toBeVisible();
    await expect(
      page.getByRole('button', { name: '保存商品级 Ozon 配置' }),
    ).toBeEnabled();
    expect(admin.writeGuard.allCalls()).toHaveLength(1);
  });

  test('blocks submit when the live schema has changed and performs no write', async ({
    page,
  }) => {
    await page.route(
      `**/api/v1/products/${E2E_PRODUCT_ID}/readiness/validate`,
      (route) =>
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            code: 0,
            message: 'ok',
            data: {
              canPublish: false,
              schemaChanged: true,
              schemaHash: 'e2e-schema-v2',
              checkedAt: '2026-08-03T00:00:00Z',
              checks: [
                {
                  code: 'OZON_SCHEMA_CHANGED',
                  level: 'error',
                  title: '属性模板已变化',
                  message: '请重新确认类目和属性。',
                },
              ],
            },
          }),
        }),
    );
    await openStage(page, 'preflight');
    await page.getByRole('button', { name: '运行发布前检查' }).click();
    await expect(page.getByText('检查未通过，不能提交到 Ozon')).toBeVisible();
    await page.getByRole('tab', { name: '草稿与提交' }).click();
    await expect(
      page.getByRole('button', { name: '提交到 Ozon' }),
    ).toBeDisabled();
  });

  test('distinguishes local draft from real submit and sends one idempotent request after confirmation', async ({
    admin,
    page,
  }) => {
    admin.writeGuard.allow({
      operation: 'local-draft',
      method: 'POST',
      path: new RegExp(
        `/api/v1/products/${E2E_PRODUCT_ID}/publish-targets/create-drafts$`,
      ),
      response: { code: 0, message: 'ok', data: {} },
    });
    admin.writeGuard.allow({
      operation: 'ozon-submit',
      method: 'POST',
      path: new RegExp(`/api/v1/products/${E2E_PRODUCT_ID}/publish$`),
      response: {
        code: 0,
        message: 'ok',
        data: {
          id: 'e2e-ozon-task',
          status: 'queued',
          productId: E2E_PRODUCT_ID,
          shopId: E2E_OZON_SHOP_ID,
          platform: 'ozon',
          createdAt: '2026-08-03T00:00:00Z',
          updatedAt: '2026-08-03T00:00:00Z',
        },
      },
    });
    await page.route(
      `**/api/v1/products/${E2E_PRODUCT_ID}/readiness/validate`,
      (route) =>
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            code: 0,
            message: 'ok',
            data: {
              canPublish: true,
              schemaChanged: false,
              schemaHash: 'e2e-schema-v1',
              checkedAt: '2026-08-03T00:00:00Z',
              checks: [],
            },
          }),
        }),
    );
    await openStage(page, 'submit');
    await page.getByRole('button', { name: '创建本地草稿' }).click();
    await admin.writeGuard.expectRequestCount('local-draft', 1);
    await expect(page.getByText('本地草稿已创建，未调用 Ozon。')).toBeVisible();

    await openStage(page, 'preflight');
    await page.getByRole('button', { name: '运行发布前检查' }).click();
    await page.getByRole('tab', { name: '草稿与提交' }).click();
    await expect(page.getByRole('tab', { name: '草稿与提交' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await page.getByRole('button', { name: '提交到 Ozon' }).click();
    const dialog = page.getByRole('dialog', { name: '确认提交到 Ozon？' });
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: /取\s*消/ }).click();
    await admin.writeGuard.expectRequestCount('ozon-submit', 0);
    await page.getByRole('button', { name: '提交到 Ozon' }).click();
    await dialog
      .getByRole('button', { name: '创建 Ozon 提交任务' })
      .click({ clickCount: 2 });
    await admin.writeGuard.expectRequestCount('ozon-submit', 1);
    const record = admin.writeGuard.calls('ozon-submit')[0];
    expect(record.postDataJSON).toEqual({
      shopId: E2E_OZON_SHOP_ID,
      options: { platform: 'ozon' },
    });
    expect(record.headers['idempotency-key']).toMatch(/^ozon-submit:/);
    expect(record.url).toContain(`/api/v1/products/${E2E_PRODUCT_ID}/publish`);
    await expect(
      page.getByText('已创建提交任务，等待处理', { exact: true }),
    ).toBeVisible();
    expect(admin.writeGuard.allCalls()).toHaveLength(2);
  });

  test('has no root horizontal overflow at required viewports', async ({
    page,
  }) => {
    for (const viewport of [
      [1440, 900],
      [1280, 800],
      [1024, 768],
      [768, 900],
      [375, 812],
    ] as const) {
      await page.setViewportSize({ width: viewport[0], height: viewport[1] });
      await openStage(page, 'config');
      await expect(
        page.getByText('商品级 Ozon 配置', { exact: true }),
      ).toBeVisible();
      await expectNoRootOverflow(page);
    }
    await expect(
      page.getByRole('button', { name: '保存商品级 Ozon 配置' }),
    ).toBeVisible();
    await expect(page.getByText(e2eOzonConfig.sourceCategoryName)).toHaveCount(
      0,
    );
  });
});

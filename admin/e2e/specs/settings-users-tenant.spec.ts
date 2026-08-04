import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { e2eAdminTenants, e2eAdminUsers } from '../mocks/admin-users';
import { ok, fail } from '../mocks/envelope';
import {
  expectHeaderContentAligned,
  expectModalWithinViewport,
  expectNoRootOverflow,
} from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

async function routeAdminUsers(page: Page, tenantFailure = false) {
  await page.route('**/api/v1/admin/tenants', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        tenantFailure ? fail('租户目录暂时不可用', 50001, null) : ok({ list: e2eAdminTenants }),
      ),
    });
  });
  await page.route('**/api/v1/admin/users**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          list: e2eAdminUsers,
          pagination: { page: 1, pageSize: 20, total: e2eAdminUsers.length, totalPages: 1 },
        }),
      ),
    });
  });
}

async function selectOption(page: Page, dialogName: RegExp, fieldLabel: string, optionName: string | RegExp) {
  const dialog = page.getByRole('dialog', { name: dialogName });
  const combobox = dialog.getByRole('combobox', { name: fieldLabel, exact: true });
  await combobox.focus();
  await combobox.press('ArrowDown');
  await page.getByRole('option', { name: optionName }).click();
}

test.describe('@settings-users tenant administrator assignment', () => {
  test('requires tenant selection and sends it when creating a tenant administrator', async ({ admin, page }) => {
    await routeAdminUsers(page);
    admin.writeGuard.allow({
      operation: 'create-tenant-admin',
      method: 'POST',
      path: /^\/api\/v1\/admin\/users$/,
      response: ok({ ...e2eAdminUsers[0], id: 'e2e-created-tenant-admin' }),
    });
    await admin.goto('/settings/users');

    await page.getByRole('button', { name: '新建用户' }).click();
    const dialog = page.getByRole('dialog', { name: '新建用户' });
    await dialog.getByLabel('邮箱').fill('created-tenant-admin@example.test');
    await dialog.getByLabel('初始密码').fill('SafePassphrase42!');
    await selectOption(page, /新建用户/, '角色', '租户管理员');
    await expect(dialog.getByRole('combobox', { name: '所属租户', exact: true })).toBeVisible();

    await dialog.getByRole('button', { name: '创建用户' }).click();
    await expect(dialog.getByText('请选择所属租户')).toBeVisible();
    await admin.writeGuard.expectRequestCount('create-tenant-admin', 0);

    await selectOption(page, /新建用户/, '所属租户', /租户 1.*mery/);
    await dialog.getByRole('button', { name: '创建用户' }).click();
    await admin.writeGuard.expectRequestCount('create-tenant-admin', 1);
    expect(admin.writeGuard.calls('create-tenant-admin')[0]?.postDataJSON).toMatchObject({
      email: 'created-tenant-admin@example.test',
      role: 'tenant_admin',
      tenantId: 1,
    });
  });

  test('updates role and tenant together after explicit confirmation', async ({ admin, page }) => {
    await routeAdminUsers(page);
    admin.writeGuard.allow({
      operation: 'update-tenant-assignment',
      method: 'PATCH',
      path: /^\/api\/v1\/admin\/users\/e2e-operator$/,
      response: ok({ ...e2eAdminUsers[1], role: 'tenant_admin', tenantId: 2 }),
    });
    await admin.goto('/settings/users');

    const row = page.getByRole('row', { name: /待分配运营/ });
    await row.getByRole('button', { name: '改角色' }).click();
    await selectOption(page, /修改角色与租户/, '角色', '租户管理员');
    await selectOption(page, /修改角色与租户/, '所属租户', /第二测试租户/);
    const assignmentDialog = page.getByRole('dialog', { name: /修改角色与租户/ });
    await assignmentDialog.getByRole('button', { name: '保存角色' }).click();

    const confirmation = page.getByRole('dialog', { name: '修改用户角色' });
    await expect(confirmation.getByText(/租户管理员（租户 2）/)).toBeVisible();
    await confirmation.getByRole('button', { name: '确认执行' }).click();
    await admin.writeGuard.expectRequestCount('update-tenant-assignment', 1);
    expect(admin.writeGuard.calls('update-tenant-assignment')[0]?.postDataJSON).toEqual({
      role: 'tenant_admin',
      tenantId: 2,
    });
  });

  test('shows tenant loading errors and performs no write', async ({ admin, page }) => {
    await routeAdminUsers(page, true);
    await admin.goto('/settings/users');
    await page.getByRole('button', { name: '新建用户' }).click();
    await selectOption(page, /新建用户/, '角色', '租户管理员');

    const dialog = page.getByRole('dialog', { name: '新建用户' });
    await expect(dialog.getByText('租户列表加载失败')).toBeVisible();
    await expect(dialog.getByText('租户目录暂时不可用')).toBeVisible();
    expect(admin.writeGuard.allCalls()).toHaveLength(0);
  });

  for (const viewport of viewports) {
    test(`has no root overflow at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      await routeAdminUsers(page);
      await page.setViewportSize(viewport);
      await admin.goto('/settings/users');
      await expect(page.getByText('现有租户管理员')).toBeVisible();
      await expect(page.getByRole('row', { name: /现有租户管理员/ }).first()).toContainText('租户 1 内全部店铺');
      await expect(page.getByRole('row', { name: /待修复租户管理员/ })).toContainText('未分配（权限失效）');
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);

      if (viewport.width === 375) {
        await page.getByRole('button', { name: '新建用户' }).click();
        await expectModalWithinViewport(page);
      }
    });
  }
});

import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  createAdminUser,
  fetchAdminTenants,
  updateAdminUser,
} from '../adminUsers';

const requestMock = vi.mocked(request);

describe('admin user tenant assignment service', () => {
  it('loads assignable tenants from the global admin endpoint', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { list: [{ id: 1, name: '测试租户', shopNames: ['mery'] }] },
    });

    await expect(fetchAdminTenants()).resolves.toEqual({
      list: [{ id: 1, name: '测试租户', shopNames: ['mery'] }],
    });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/admin/tenants', { method: 'GET' });
  });

  it('preserves tenantId when creating a tenant administrator', async () => {
    const body = {
      email: 'tenant-admin@example.test',
      password: 'SafePassphrase42!',
      role: 'tenant_admin',
      tenantId: 7,
    };
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'user-7' } });

    await createAdminUser(body);

    expect(requestMock).toHaveBeenCalledWith('/api/v1/admin/users', {
      method: 'POST',
      data: body,
    });
  });

  it('sends role and tenantId together when updating an assignment', async () => {
    const body = { role: 'tenant_admin', tenantId: 9 };
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'user-9' } });

    await updateAdminUser('user-9', body);

    expect(requestMock).toHaveBeenCalledWith('/api/v1/admin/users/user-9', {
      method: 'PATCH',
      data: body,
    });
  });
});

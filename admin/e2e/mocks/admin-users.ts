export const e2eAdminTenants = [
  { id: 1, shopNames: ['mery'] },
  { id: 2, name: '第二测试租户', shopNames: ['e2e-shop-2'] },
];

export const e2eAdminUsers = [
  {
    id: 'e2e-tenant-admin',
    tenantId: 1,
    username: 'tenant-admin@example.test',
    email: 'tenant-admin@example.test',
    displayName: '现有租户管理员',
    role: 'tenant_admin',
    status: 'active',
    storePermissions: [],
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
  },
  {
    id: 'e2e-operator',
    tenantId: 0,
    username: 'operator@example.test',
    email: 'operator@example.test',
    displayName: '待分配运营',
    role: 'operator',
    status: 'active',
    storePermissions: [
      {
        id: 'e2e-store-grant',
        storeId: 'e2e-shop-mery',
        storeName: 'mery',
        platform: 'ozon',
        permissionScope: 'manage',
      },
    ],
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
  },
  {
    id: 'e2e-invalid-tenant-admin',
    tenantId: 0,
    username: 'invalid-tenant-admin@example.test',
    email: 'invalid-tenant-admin@example.test',
    displayName: '待修复租户管理员',
    role: 'tenant_admin',
    status: 'active',
    storePermissions: [],
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
  },
];

import { PERMISSIONS } from '../../src/utils/permission';

export const E2E_TOKEN = 'e2e-admin-token';

export const e2eUser = {
  id: 'e2e-user',
  username: 'e2e-user',
  email: 'e2e-user@example.test',
  displayName: 'E2E 管理员',
  role: 'admin',
  status: 'active',
  permissions: Object.values(PERMISSIONS),
  storePermissions: [],
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

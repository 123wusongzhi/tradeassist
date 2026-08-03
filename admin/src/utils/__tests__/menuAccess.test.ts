import { describe, expect, it } from 'vitest';
import { canAccessPath, filterMenuByPermission } from '@/utils/menuAccess';
import { PERMISSIONS, ROLES } from '@/utils/permission';

const GLOBAL_OPERATIONS_PATHS = [
  '/ops/workers/monitor',
  '/ops/observability',
  '/ops/backups',
  '/ops/restores',
  '/ops/releases',
  '/ops/disaster-recovery',
  '/ops/platform-runtime',
  '/collect/monitor',
];

describe('global operations route access', () => {
  it.each([ROLES.TENANT_ADMIN, ROLES.OPERATOR, ROLES.READONLY])(
    'hides and denies global operations pages for %s',
    (role) => {
      for (const path of GLOBAL_OPERATIONS_PATHS) {
        expect(canAccessPath(path, role, [PERMISSIONS.SETTINGS_MANAGE])).toBe(false);
      }

      const visibleMenus = filterMenuByPermission(
        GLOBAL_OPERATIONS_PATHS.map((path) => ({ path, name: path })),
        role,
        [PERMISSIONS.SETTINGS_MANAGE],
      );
      expect(visibleMenus).toHaveLength(0);
    },
  );

  it('allows a global administrator to view and access every global operations page', () => {
    for (const path of GLOBAL_OPERATIONS_PATHS) {
      expect(canAccessPath(path, ROLES.ADMIN)).toBe(true);
    }

    const visibleMenus = filterMenuByPermission(
      GLOBAL_OPERATIONS_PATHS.map((path) => ({ path, name: path })),
      ROLES.ADMIN,
    );
    expect(visibleMenus).toHaveLength(GLOBAL_OPERATIONS_PATHS.length);
  });
});

describe('Ozon publish route access', () => {
  it('requires product view permission while allowing read-only inspection', () => {
    expect(canAccessPath('/product/ozon-publish', ROLES.OPERATOR)).toBe(true);
    expect(canAccessPath('/product/ozon-publish', ROLES.READONLY)).toBe(true);
    expect(canAccessPath('/product/ozon-publish', ROLES.REVIEWER)).toBe(false);
  });
});

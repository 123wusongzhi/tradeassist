import { describe, expect, it } from 'vitest';
import { canAccessPath } from '@/utils/menuAccess';
import {
  hasPermission,
  normalizeRole,
  PERMISSIONS,
  permissionsForRole,
  ROLES,
} from '@/utils/permission';

describe('tenant administrator permissions', () => {
  it('allows tenant business operations', () => {
    expect(normalizeRole(ROLES.TENANT_ADMIN)).toBe(ROLES.TENANT_ADMIN);
    expect(hasPermission(ROLES.TENANT_ADMIN, PERMISSIONS.PRODUCT_WRITE)).toBe(true);
    expect(hasPermission(ROLES.TENANT_ADMIN, PERMISSIONS.AI_IMAGE_APPLY)).toBe(true);
    expect(hasPermission(ROLES.TENANT_ADMIN, PERMISSIONS.PUBLISH_CREATE_DRAFT)).toBe(true);
    expect(hasPermission(ROLES.TENANT_ADMIN, PERMISSIONS.ORDER_OPERATE)).toBe(true);
    expect(hasPermission(ROLES.TENANT_ADMIN, PERMISSIONS.SKU_BIND)).toBe(true);
    expect(hasPermission(ROLES.TENANT_ADMIN, PERMISSIONS.INVENTORY_OPERATE)).toBe(true);
    expect(hasPermission(ROLES.TENANT_ADMIN, PERMISSIONS.CUSTOMER_OPERATE)).toBe(true);
    expect(hasPermission(ROLES.TENANT_ADMIN, PERMISSIONS.TASK_RETRY)).toBe(true);
    expect(hasPermission(ROLES.TENANT_ADMIN, PERMISSIONS.STORE_OPERATE)).toBe(true);
    expect(hasPermission(ROLES.TENANT_ADMIN, PERMISSIONS.COLLECT_PROFILE_MANAGE)).toBe(true);
    expect(canAccessPath('/collect/browser-profiles', ROLES.TENANT_ADMIN)).toBe(true);
  });

  it('never grants global administration permissions, including profile extras', () => {
    const profilePermissions = [
      PERMISSIONS.PRODUCT_VIEW,
      PERMISSIONS.COLLECT_PROFILE_MANAGE,
      PERMISSIONS.SETTINGS_MANAGE,
      PERMISSIONS.USER_MANAGE,
      PERMISSIONS.OPERATIONLOG_VIEW,
      PERMISSIONS.OBSERVABILITY_READ,
      PERMISSIONS.BACKUP_CREATE,
      PERMISSIONS.RESTORE_EXECUTE,
      PERMISSIONS.RELEASE_EXECUTE,
      PERMISSIONS.DR_EXECUTE,
    ];

    expect(permissionsForRole(ROLES.TENANT_ADMIN, profilePermissions)).toEqual([
      PERMISSIONS.PRODUCT_VIEW,
      PERMISSIONS.COLLECT_PROFILE_MANAGE,
    ]);
    expect(canAccessPath('/product/drafts', ROLES.TENANT_ADMIN)).toBe(true);
    expect(canAccessPath('/shops/manage', ROLES.TENANT_ADMIN)).toBe(true);
    expect(canAccessPath('/settings/security', ROLES.TENANT_ADMIN)).toBe(false);
    expect(canAccessPath('/settings/users', ROLES.TENANT_ADMIN)).toBe(false);
    expect(canAccessPath('/system/operation-logs', ROLES.TENANT_ADMIN)).toBe(false);
    expect(canAccessPath('/ops/observability', ROLES.TENANT_ADMIN)).toBe(false);
    expect(canAccessPath('/ops/backups', ROLES.TENANT_ADMIN)).toBe(false);
    expect(canAccessPath('/ops/restores', ROLES.TENANT_ADMIN)).toBe(false);
    expect(canAccessPath('/ops/releases', ROLES.TENANT_ADMIN)).toBe(false);
    expect(canAccessPath('/ops/disaster-recovery', ROLES.TENANT_ADMIN)).toBe(false);
  });

  it('continues to fail closed for unknown roles even when a profile sends permissions', () => {
    expect(normalizeRole('tenant_admin ')).toBe(ROLES.TENANT_ADMIN);
    expect(permissionsForRole('unknown-admin', [PERMISSIONS.USER_MANAGE])).not.toContain(
      PERMISSIONS.USER_MANAGE,
    );
  });
});

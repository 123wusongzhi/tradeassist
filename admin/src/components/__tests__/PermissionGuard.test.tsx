import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import PermissionGuard from '@/components/PermissionGuard';
import { PERMISSIONS, ROLES } from '@/utils/permission';

const { usePermission } = vi.hoisted(() => ({ usePermission: vi.fn() }));

vi.mock('@/hooks/usePermission', () => ({ usePermission }));

function mockPermission(role: string, can = true) {
  usePermission.mockReturnValue({
    role,
    readonly: role === ROLES.READONLY,
    can: () => can,
  });
}

describe('PermissionGuard global administration', () => {
  it.each([ROLES.TENANT_ADMIN, ROLES.OPERATOR, ROLES.READONLY])(
    'renders a 403 before mounting protected content for %s',
    (role) => {
      mockPermission(role);
      const protectedContent = vi.fn();
      const ProtectedContent = () => {
        protectedContent();
        return <div>全局运维数据</div>;
      };

      render(
        <PermissionGuard require={PERMISSIONS.SETTINGS_MANAGE} requireGlobalAdmin showForbiddenPage>
          <ProtectedContent />
        </PermissionGuard>,
      );

      expect(screen.getByText('无权限')).toBeInTheDocument();
      expect(screen.queryByText('全局运维数据')).not.toBeInTheDocument();
      expect(protectedContent).not.toHaveBeenCalled();
    },
  );

  it('mounts protected content for the effective global admin role', () => {
    mockPermission(ROLES.ADMIN);

    render(
      <PermissionGuard require={PERMISSIONS.SETTINGS_MANAGE} requireGlobalAdmin showForbiddenPage>
        <div>全局运维数据</div>
      </PermissionGuard>,
    );

    expect(screen.getByText('全局运维数据')).toBeInTheDocument();
  });
});

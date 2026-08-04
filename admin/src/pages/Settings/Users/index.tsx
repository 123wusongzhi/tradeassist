import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import PermissionGuard from '@/components/PermissionGuard';
import { PAGE_COPY } from '@/constants/copywriting';
import {
  confirmAssignStorePermissions,
  confirmChangeUserRole,
  confirmDisableUser,
} from '@/constants/sensitiveActions';
import { formatDateTime } from '@/utils/formatTime';
import {
  createAdminUser,
  fetchAdminTenants,
  fetchAdminUsers,
  setAdminUserStorePermissions,
  updateAdminUser,
  type AdminTenantOption,
  type AdminUserRow,
  type CreateAdminUserBody,
} from '@/services/adminUsers';
import { queryShops, type ShopListRow } from '@/services/shops';
import { Alert, Button, Form, Input, Modal, Select, Space, Tag, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { usePermission } from '@/hooks/usePermission';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { PERMISSIONS } from '@/utils/permission';

const ROLE_OPTIONS = [
  { label: '管理员', value: 'admin' },
  { label: '租户管理员', value: 'tenant_admin' },
  { label: '运营', value: 'operator' },
  { label: '只读', value: 'readonly' },
];

const STATUS_OPTIONS = [
  { label: '正常', value: 'active' },
  { label: '禁用', value: 'disabled' },
];

const SCOPE_OPTIONS = [
  { label: '只读', value: 'view' },
  { label: '运营', value: 'operate' },
  { label: '管理', value: 'manage' },
];

type RoleFormValues = {
  role: string;
  tenantId?: number;
};

function roleTag(role: string) {
  const r = (role || '').toLowerCase();
  if (r === 'admin') return <Tag color="blue">管理员</Tag>;
  if (r === 'tenant_admin') return <Tag color="geekblue">租户管理员</Tag>;
  if (r === 'operator') return <Tag color="cyan">运营</Tag>;
  if (r === 'readonly') return <Tag>只读</Tag>;
  return <Tag>{role}</Tag>;
}

function adminUserLabel(row: Pick<AdminUserRow, 'displayName' | 'email' | 'username'>): string {
  return (row.displayName || '').trim() || (row.email || '').trim() || row.username || '该用户';
}

function tenantOptionLabel(option: AdminTenantOption): string {
  const base = option.name?.trim() || `租户 ${option.id}`;
  const shops = (option.shopNames || []).filter(Boolean);
  return shops.length > 0 ? `${base}（店铺：${shops.join('、')}）` : base;
}

function tenantAssignmentLabel(row: Pick<AdminUserRow, 'tenantId' | 'role'>) {
  const tenantId = Number(row.tenantId || 0);
  if (row.role === 'tenant_admin' && tenantId <= 0) {
    return <Tag color="error">未分配（权限失效）</Tag>;
  }
  if (tenantId <= 0) return <Tag>系统域</Tag>;
  return <Tag color="geekblue">租户 {tenantId}</Tag>;
}

function storePermissionLabel(row: AdminUserRow): string {
  if (row.role === 'admin') return '全部';
  if (row.role === 'tenant_admin') {
    return row.tenantId > 0 ? `租户 ${row.tenantId} 内全部店铺` : '未分配租户';
  }
  return (row.storePermissions || []).map((permission) => permission.storeName || permission.storeId).join('、') || '—';
}

export default function SettingsUsersPage() {
  const actionRef = useRef<ActionType>();
  const { canManageUsers, user: currentUser } = usePermission();
  const [createOpen, setCreateOpen] = useState(false);
  const [createSubmitting, setCreateSubmitting] = useState(false);
  const [permOpen, setPermOpen] = useState(false);
  const [editUser, setEditUser] = useState<AdminUserRow | null>(null);
  const [roleOpen, setRoleOpen] = useState(false);
  const [roleUser, setRoleUser] = useState<AdminUserRow | null>(null);
  const [roleSubmitting, setRoleSubmitting] = useState(false);
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [tenants, setTenants] = useState<AdminTenantOption[]>([]);
  const [tenantsLoading, setTenantsLoading] = useState(false);
  const [tenantLoadError, setTenantLoadError] = useState('');
  const [createForm] = Form.useForm<CreateAdminUserBody>();
  const [roleForm] = Form.useForm<RoleFormValues>();
  const [permForm] = Form.useForm();
  const createRole = Form.useWatch('role', createForm);
  const editRole = Form.useWatch('role', roleForm);

  const tenantOptions = useMemo(
    () => tenants.map((tenant) => ({ label: tenantOptionLabel(tenant), value: tenant.id })),
    [tenants],
  );

  const loadTenants = useCallback(async () => {
    setTenantsLoading(true);
    setTenantLoadError('');
    try {
      const res = await fetchAdminTenants();
      setTenants(res.list || []);
    } catch (error: unknown) {
      setTenants([]);
      setTenantLoadError((error as Error)?.message || '租户列表加载失败');
    } finally {
      setTenantsLoading(false);
    }
  }, []);

  const loadShops = useCallback(async () => {
    try {
      const res = await queryShops({ page: 1, pageSize: 200 });
      setShops(res.list || []);
    } catch {
      setShops([]);
    }
  }, []);

  const openCreateModal = useCallback(() => {
    setCreateOpen(true);
    void loadTenants();
  }, [loadTenants]);

  const emptyLocale = useListEmptyLocale('usersSettings', {
    onAction: openCreateModal,
    actionLabel: '创建用户',
  });

  const openRoleModal = useCallback(
    (row: AdminUserRow) => {
      setRoleUser(row);
      setRoleOpen(true);
      void loadTenants();
    },
    [loadTenants],
  );

  useEffect(() => {
    if (!roleOpen || !roleUser) return;
    roleForm.setFieldsValue({
      role: roleUser.role,
      tenantId: roleUser.tenantId > 0 ? roleUser.tenantId : undefined,
    });
  }, [roleForm, roleOpen, roleUser]);

  const saveRoleAssignment = useCallback(async () => {
    if (!roleUser) return;
    let values: RoleFormValues;
    try {
      values = await roleForm.validateFields();
    } catch {
      return;
    }
    const nextTenantId =
      values.role === 'admin'
        ? 0
        : values.role === 'tenant_admin'
          ? Number(values.tenantId)
          : Number(roleUser.tenantId || 0);
    if (values.role === roleUser.role && nextTenantId === Number(roleUser.tenantId || 0)) {
      setRoleOpen(false);
      return;
    }
    const roleLabel = ROLE_OPTIONS.find((option) => option.value === values.role)?.label || values.role;
    const assignmentLabel =
      values.role === 'tenant_admin' ? `${roleLabel}（租户 ${nextTenantId}）` : roleLabel;
    setRoleSubmitting(true);
    confirmChangeUserRole(adminUserLabel(roleUser), assignmentLabel, async () => {
      try {
        await updateAdminUser(roleUser.id, { role: values.role, tenantId: nextTenantId });
        message.success('角色与租户归属已更新，请该用户重新登录');
        setRoleOpen(false);
        actionRef.current?.reload();
      } catch (error: unknown) {
        message.error((error as Error)?.message || '更新失败');
        throw error;
      } finally {
        setRoleSubmitting(false);
      }
    }, () => setRoleSubmitting(false));
  }, [roleForm, roleUser]);

  const columns: ProColumns<AdminUserRow>[] = [
    { title: '显示名', dataIndex: 'displayName', width: 140, ellipsis: true },
    { title: '邮箱', dataIndex: 'email', width: 180, ellipsis: true, search: false },
    { title: '手机', dataIndex: 'phone', width: 120, search: false },
    {
      title: '角色',
      dataIndex: 'role',
      width: 100,
      valueType: 'select',
      fieldProps: { options: ROLE_OPTIONS },
      render: (_, row) => roleTag(row.role),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      valueType: 'select',
      fieldProps: { options: STATUS_OPTIONS },
      render: (_, row) =>
        row.status === 'disabled' ? <Tag color="error">禁用</Tag> : <Tag color="success">正常</Tag>,
    },
    {
      title: '所属租户',
      dataIndex: 'tenantId',
      width: 150,
      search: false,
      render: (_, row) => tenantAssignmentLabel(row),
    },
    {
      title: '授权店铺',
      dataIndex: 'storePermissions',
      search: false,
      ellipsis: true,
      render: (_, row) => storePermissionLabel(row),
    },
    {
      title: '最近操作',
      dataIndex: 'lastOperationAt',
      width: 168,
      search: false,
      render: (_, row) => formatDateTime(row.lastOperationAt),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 220,
      render: (_, row) => [
        <Button
          key="role"
          type="link"
          size="small"
          disabled={!canManageUsers}
          onClick={() => openRoleModal(row)}
        >
          改角色
        </Button>,
        row.role !== 'admin' && row.role !== 'tenant_admin' ? (
          <Button
            key="perm"
            type="link"
            size="small"
            onClick={async () => {
              setEditUser(row);
              await loadShops();
              permForm.setFieldsValue({
                items: (row.storePermissions || []).map((p) => ({
                  storeId: p.storeId,
                  permissionScope: p.permissionScope || 'operate',
                })),
              });
              setPermOpen(true);
            }}
          >
            店铺权限
          </Button>
        ) : null,
        row.id !== currentUser?.id ? (
          <Button
            key="disable"
            type="link"
            size="small"
            danger={row.status !== 'disabled'}
            onClick={() => {
              const next = row.status === 'disabled' ? 'active' : 'disabled';
              if (next === 'disabled') {
                confirmDisableUser(adminUserLabel(row), async () => {
                  await updateAdminUser(row.id, { status: next });
                  message.success('已更新');
                  actionRef.current?.reload();
                });
              } else {
                void updateAdminUser(row.id, { status: next }).then(() => {
                  message.success('已更新');
                  actionRef.current?.reload();
                });
              }
            }}
          >
            {row.status === 'disabled' ? '启用' : '禁用'}
          </Button>
        ) : null,
      ],
    },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.USER_MANAGE} showForbiddenPage>
      <TmPageContainer title={PAGE_COPY.usersSettings.title} subTitle={PAGE_COPY.usersSettings.description}>
        <ProTable<AdminUserRow>
          actionRef={actionRef}
          rowKey="id"
          columns={columns}
          search={{ labelWidth: 80 }}
          locale={emptyLocale}
          toolBarRender={() => [
            <Button key="create" type="primary" onClick={openCreateModal}>
              新建用户
            </Button>,
          ]}
          request={async (params) => {
            const res = await fetchAdminUsers({
              page: params.current,
              pageSize: params.pageSize,
              role: params.role as string | undefined,
              status: params.status as string | undefined,
              keyword: params.displayName as string | undefined,
            });
            return {
              data: res.list || [],
              total: res.pagination?.total || 0,
              success: true,
            };
          }}
        />

        <Modal
          title="新建用户"
          open={createOpen}
          okText="创建用户"
          confirmLoading={createSubmitting}
          onCancel={() => {
            if (!createSubmitting) setCreateOpen(false);
          }}
          onOk={() => createForm.submit()}
          destroyOnHidden
          afterClose={() => createForm.resetFields()}
        >
          <Form
            form={createForm}
            layout="vertical"
            onFinish={async (values) => {
              const { tenantId: selectedTenantId, ...baseValues } = values;
              const body: CreateAdminUserBody = {
                ...baseValues,
                ...(values.role === 'tenant_admin'
                  ? { tenantId: Number(selectedTenantId) }
                  : values.role === 'admin'
                    ? { tenantId: 0 }
                    : {}),
              };
              setCreateSubmitting(true);
              try {
                await createAdminUser(body);
                message.success('用户已创建');
                setCreateOpen(false);
                actionRef.current?.reload();
              } catch (error: unknown) {
                message.error((error as Error)?.message || '创建失败');
              } finally {
                setCreateSubmitting(false);
              }
            }}
          >
            <Form.Item name="email" label="邮箱" rules={[{ required: true }]}>
              <Input placeholder="demo_operator@example.com" />
            </Form.Item>
            <Form.Item name="password" label="初始密码" rules={[{ required: true, min: 6 }]}>
              <Input.Password />
            </Form.Item>
            <Form.Item name="displayName" label="显示名">
              <Input />
            </Form.Item>
            <Form.Item name="role" label="角色" initialValue="operator">
              <Select aria-label="角色" options={ROLE_OPTIONS} virtual={false} />
            </Form.Item>
            {createRole === 'tenant_admin' ? (
              <>
                {tenantLoadError ? (
                  <Alert
                    type="error"
                    showIcon
                    message="租户列表加载失败"
                    description={tenantLoadError}
                    action={<Button size="small" onClick={() => void loadTenants()}>重新加载</Button>}
                    style={{ marginBottom: 16 }}
                  />
                ) : null}
                <Form.Item
                  name="tenantId"
                  label="所属租户"
                  rules={[{ required: true, message: '请选择所属租户' }]}
                  extra="租户管理员将拥有该租户内全部店铺和商品的业务权限。"
                >
                  <Select
                    aria-label="所属租户"
                    showSearch
                    optionFilterProp="label"
                    loading={tenantsLoading}
                    placeholder="选择租户"
                    options={tenantOptions}
                    notFoundContent={tenantsLoading ? '正在加载租户' : '暂无可分配租户'}
                    virtual={false}
                  />
                </Form.Item>
              </>
            ) : null}
          </Form>
        </Modal>

        <Modal
          title={`修改角色与租户 — ${roleUser ? adminUserLabel(roleUser) : ''}`}
          open={roleOpen}
          okText="保存角色"
          confirmLoading={roleSubmitting}
          onCancel={() => {
            if (!roleSubmitting) setRoleOpen(false);
          }}
          onOk={saveRoleAssignment}
          destroyOnHidden
          afterClose={() => {
            roleForm.resetFields();
            setRoleUser(null);
          }}
        >
          <Form form={roleForm} layout="vertical">
            <Form.Item name="role" label="角色" rules={[{ required: true, message: '请选择角色' }]}>
              <Select aria-label="角色" options={ROLE_OPTIONS} virtual={false} />
            </Form.Item>
            {editRole === 'tenant_admin' ? (
              <>
                {tenantLoadError ? (
                  <Alert
                    type="error"
                    showIcon
                    message="租户列表加载失败"
                    description={tenantLoadError}
                    action={<Button size="small" onClick={() => void loadTenants()}>重新加载</Button>}
                    style={{ marginBottom: 16 }}
                  />
                ) : null}
                <Form.Item
                  name="tenantId"
                  label="所属租户"
                  rules={[{ required: true, message: '请选择所属租户' }]}
                  extra="保存后旧会话立即失效，用户需要重新登录。"
                >
                  <Select
                    aria-label="所属租户"
                    showSearch
                    optionFilterProp="label"
                    loading={tenantsLoading}
                    placeholder="选择租户"
                    options={tenantOptions}
                    notFoundContent={tenantsLoading ? '正在加载租户' : '暂无可分配租户'}
                    virtual={false}
                  />
                </Form.Item>
              </>
            ) : null}
          </Form>
        </Modal>

        <Modal
          title={`分配店铺权限 — ${editUser?.displayName || ''}`}
          open={permOpen}
          width={640}
          onCancel={() => setPermOpen(false)}
          onOk={() => {
            if (!editUser) return;
            confirmAssignStorePermissions(adminUserLabel(editUser), () => permForm.submit());
          }}
          destroyOnHidden
        >
          <Form
            form={permForm}
            layout="vertical"
            onFinish={async (v) => {
              if (!editUser) return;
              await setAdminUserStorePermissions(editUser.id, v.items || []);
              message.success('店铺权限已保存');
              setPermOpen(false);
              actionRef.current?.reload();
            }}
          >
            <Form.List name="items">
              {(fields, { add, remove }) => (
                <>
                  {fields.map((field) => (
                    <Space key={field.key} align="baseline" style={{ display: 'flex', marginBottom: 8 }}>
                      <Form.Item
                        {...field}
                        name={[field.name, 'storeId']}
                        rules={[{ required: true, message: '选择店铺' }]}
                      >
                        <Select
                          style={{ width: 260 }}
                          placeholder="选择店铺"
                          options={shops.map((s) => ({
                            label: `${s.shopName || s.id} (${s.platform})`,
                            value: s.id,
                          }))}
                        />
                      </Form.Item>
                      <Form.Item {...field} name={[field.name, 'permissionScope']} initialValue="operate">
                        <Select style={{ width: 120 }} options={SCOPE_OPTIONS} />
                      </Form.Item>
                      <Button type="link" onClick={() => remove(field.name)}>
                        移除
                      </Button>
                    </Space>
                  ))}
                  <Button type="dashed" onClick={() => add({ permissionScope: 'operate' })} block>
                    添加店铺
                  </Button>
                </>
              )}
            </Form.List>
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}

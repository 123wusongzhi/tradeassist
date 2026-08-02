import { TmPageContainer } from '@/components/ui';
import PermissionGuard from '@/components/PermissionGuard';
import { PERMISSIONS } from '@/utils/permission';
import {
  createBackup,
  fetchBackups,
  holdBackup,
  verifyBackup,
  type BackupJob,
} from '@/services/opsP6';
import { DatabaseOutlined, ReloadOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { Alert, Button, Modal, Space, Table, Tag, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';

function statusColor(status?: string) {
  if (status === 'completed' || status === 'passed') return 'green';
  if (status === 'failed') return 'red';
  if (status === 'manual_review' || status === 'pending') return 'gold';
  return 'blue';
}

function BackupsPageContent() {
  const [items, setItems] = useState<BackupJob[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchBackups({ page: 1, pageSize: 50 });
      setItems(res.data?.items ?? []);
    } catch {
      message.error('加载备份记录失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const createDryRun = () => {
    Modal.confirm({
      title: '创建备份记录',
      content: '当前操作会通过后端安全门执行；未启用备份时仅生成待复核记录。',
      onOk: async () => {
        await createBackup({ reason: 'operator requested from admin', dryRun: false });
        message.success('备份任务已创建');
        await load();
      },
    });
  };

  return (
    <TmPageContainer
      title="备份管理"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>
            刷新
          </Button>
          <Button type="primary" icon={<DatabaseOutlined />} onClick={createDryRun}>
            创建备份
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <Alert
          showIcon
          type="info"
          message="真实对象存储和真实生产备份验证保持待接入；页面不提供完整备份下载。"
        />
        <Table<BackupJob>
          rowKey="backupId"
          loading={loading}
          dataSource={items}
          columns={[
            { title: '备份编号', dataIndex: 'backupId', width: 220 },
            { title: '环境', dataIndex: 'environment', width: 120 },
            { title: '类型', dataIndex: 'backupType', width: 160 },
            {
              title: '状态',
              dataIndex: 'status',
              width: 130,
              render: (v) => <Tag color={statusColor(v)}>{v}</Tag>,
            },
            {
              title: '校验',
              dataIndex: 'verificationStatus',
              width: 130,
              render: (v) => <Tag color={statusColor(v)}>{v}</Tag>,
            },
            {
              title: '加密',
              dataIndex: 'encrypted',
              width: 100,
              render: (v) => <Tag color={v ? 'green' : 'gold'}>{v ? '已启用' : '未启用'}</Tag>,
            },
            { title: '存储', dataIndex: 'storageProvider', width: 120 },
            { title: '大小', dataIndex: 'artifactSize', width: 120, align: 'right' },
            { title: '创建时间', dataIndex: 'createdAt', width: 180 },
            {
              title: '操作',
              width: 220,
              render: (_, row) => (
                <Space>
                  <Button
                    size="small"
                    icon={<SafetyCertificateOutlined />}
                    onClick={() => void verifyBackup(row.backupId).then(load)}
                  >
                    校验备份
                  </Button>
                  <Button
                    size="small"
                    onClick={() => void holdBackup(row.backupId, 'operator manual hold').then(load)}
                  >
                    保留
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Space>
    </TmPageContainer>
  );
}

export default function BackupsPage() {
  return (
    <PermissionGuard require={PERMISSIONS.SETTINGS_MANAGE} requireGlobalAdmin showForbiddenPage>
      <BackupsPageContent />
    </PermissionGuard>
  );
}

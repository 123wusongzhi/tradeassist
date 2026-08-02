import { TmPageContainer } from '@/components/ui';
import PermissionGuard from '@/components/PermissionGuard';
import { PERMISSIONS } from '@/utils/permission';
import { createDRDrill, fetchDRStatus, type DRStatus } from '@/services/opsP6';
import { DeploymentUnitOutlined, ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Descriptions, Form, Input, Modal, Space, Switch, Tag, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';

function DisasterRecoveryPageContent() {
  const [status, setStatus] = useState<DRStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchDRStatus();
      setStatus(res.data ?? null);
    } catch {
      message.error('加载灾备状态失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <TmPageContainer
      title="灾备演练"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>
            刷新
          </Button>
          <Button type="primary" icon={<DeploymentUnitOutlined />} onClick={() => setOpen(true)}>
            记录演练
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <Alert
          showIcon
          type="warning"
          message="当前只记录隔离演练；真实生产灾难恢复验收、真实 PITR 演练和真实流量切换保持待接入。"
        />
        <Card title="当前状态">
          <Descriptions column={2}>
            <Descriptions.Item label="演练状态">
              <Tag>{status?.status ?? 'deferred'}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="RPO 目标">{status?.rpoTarget ?? 'draft'}</Descriptions.Item>
            <Descriptions.Item label="RTO 目标">{status?.rtoTarget ?? 'draft'}</Descriptions.Item>
            <Descriptions.Item label="真实生产备份">
              {status?.realProductionBackupVerification ?? 'deferred'}
            </Descriptions.Item>
            <Descriptions.Item label="真实生产灾备">
              {status?.realProductionDRVerification ?? 'deferred'}
            </Descriptions.Item>
            <Descriptions.Item label="真实 PITR">{status?.realPITRDrill ?? 'deferred'}</Descriptions.Item>
          </Descriptions>
        </Card>
      </Space>
      <Modal
        title="记录隔离演练"
        open={open}
        onCancel={() => setOpen(false)}
        onOk={async () => {
          const values = await form.validateFields();
          await createDRDrill(values);
          message.success('演练记录已保存');
          setOpen(false);
          form.resetFields();
          await load();
        }}
      >
        <Form form={form} layout="vertical" initialValues={{ drillType: 'isolated_restore', confirmedIsolated: true }}>
          <Form.Item name="drillType" label="演练类型" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="backupId" label="备份编号">
            <Input />
          </Form.Item>
          <Form.Item name="restoreId" label="恢复编号">
            <Input />
          </Form.Item>
          <Form.Item name="releaseId" label="发布编号">
            <Input />
          </Form.Item>
          <Form.Item name="confirmedIsolated" label="已确认隔离环境" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}

export default function DisasterRecoveryPage() {
  return (
    <PermissionGuard require={PERMISSIONS.SETTINGS_MANAGE} requireGlobalAdmin showForbiddenPage>
      <DisasterRecoveryPageContent />
    </PermissionGuard>
  );
}

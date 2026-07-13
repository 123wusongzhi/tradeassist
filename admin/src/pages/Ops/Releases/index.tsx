import { TmPageContainer } from '@/components/ui';
import {
  createRelease,
  executeRelease,
  fetchReleases,
  rollbackRelease,
  type ReleaseRun,
} from '@/services/opsP6';
import { BranchesOutlined, ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, Form, Input, Modal, Space, Table, Tag, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';

export default function ReleasesPage() {
  const [items, setItems] = useState<ReleaseRun[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchReleases({ page: 1, pageSize: 50 });
      setItems(res.data?.items ?? []);
    } catch {
      message.error('加载发布记录失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <TmPageContainer
      title="发布回滚"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>
            刷新
          </Button>
          <Button type="primary" icon={<BranchesOutlined />} onClick={() => setOpen(true)}>
            创建发布
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <Alert
          showIcon
          type="info"
          message="发布失败只允许应用层回滚；数据库恢复需进入人工高风险流程。"
        />
        <Table<ReleaseRun>
          rowKey="releaseId"
          loading={loading}
          dataSource={items}
          columns={[
            { title: '发布编号', dataIndex: 'releaseId', width: 220 },
            { title: '版本', dataIndex: 'version', width: 160 },
            { title: '环境', dataIndex: 'environment', width: 120 },
            { title: '策略', dataIndex: 'strategy', width: 120 },
            { title: '状态', dataIndex: 'state', width: 150, render: (v) => <Tag>{v}</Tag> },
            { title: '发布前备份', dataIndex: 'preBackupId', width: 220 },
            { title: '创建时间', dataIndex: 'createdAt', width: 180 },
            {
              title: '操作',
              width: 220,
              render: (_, row) => (
                <Space>
                  <Button size="small" onClick={() => void executeRelease(row.releaseId).then(load)}>
                    执行发布
                  </Button>
                  <Button
                    size="small"
                    danger
                    onClick={() => void rollbackRelease(row.releaseId, 'operator rollback').then(load)}
                  >
                    回滚应用
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Space>
      <Modal
        title="创建发布"
        open={open}
        onCancel={() => setOpen(false)}
        onOk={async () => {
          const values = await form.validateFields();
          await createRelease(values);
          message.success('发布记录已创建');
          setOpen(false);
          form.resetFields();
          await load();
        }}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="version" label="版本" rules={[{ required: true }]}>
            <Input placeholder="例如 v0.9.0-p6-dev" />
          </Form.Item>
          <Form.Item name="gitCommit" label="提交摘要">
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}

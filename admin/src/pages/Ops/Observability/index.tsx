import { TmPageContainer } from '@/components/ui';
import {
  ackAlert,
  fetchObservabilityAlerts,
  fetchObservabilityOverview,
  silenceAlert,
  type AlertEvent,
  type ObservabilityOverview,
} from '@/services/observability';
import { ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Col, Descriptions, Row, Space, Spin, Table, Tag, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';

function severityColor(sev: string) {
  switch (sev) {
    case 'critical':
      return 'red';
    case 'warning':
      return 'orange';
    default:
      return 'blue';
  }
}

export default function ObservabilityCenterPage() {
  const [overview, setOverview] = useState<ObservabilityOverview | null>(null);
  const [alerts, setAlerts] = useState<AlertEvent[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [ov, al] = await Promise.all([
        fetchObservabilityOverview(),
        fetchObservabilityAlerts({ limit: 50 }),
      ]);
      setOverview(ov?.data ?? null);
      setAlerts(al?.data?.items ?? []);
    } catch (e) {
      message.error('加载可观测性数据失败');
      setOverview(null);
      setAlerts([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <TmPageContainer
      title="可观测性中心"
      extra={
        <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>
          刷新
        </Button>
      }
    >
      <Spin spinning={loading}>
        <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message="代码级可观测性已就绪；真实生产监控平台验证保持 Deferred。"
        />
        <Card title="系统概览">
          <Descriptions column={2} size="small">
            <Descriptions.Item label="模式">{overview?.mode ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="环境">{overview?.environment ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="指标">
              {overview?.metricsEnabled ? '已启用' : '未启用'}
            </Descriptions.Item>
            <Descriptions.Item label="链路追踪">
              {overview?.tracingEnabled ? '已启用' : '未启用'}
            </Descriptions.Item>
            <Descriptions.Item label="指标路径">{overview?.metricsPath ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="内部保护">
              {overview?.metricsInternal ? '是' : '否'}
            </Descriptions.Item>
            <Descriptions.Item label="遥测导出">
              {overview?.otelExportBlocked ? '环境未配置' : '已配置'}
            </Descriptions.Item>
          </Descriptions>
        </Card>
        <Row gutter={16}>
          <Col span={8}>
            <Card title="API 状态">请求量 / 错误率 / p95 延迟（聚合接口）</Card>
          </Col>
          <Col span={8}>
            <Card title="后台任务进程">队列积压 / 死信 / 租约丢失</Card>
          </Col>
          <Col span={8}>
            <Card title="安全状态">登录异常 / Refresh Reuse / Audit Chain</Card>
          </Col>
        </Row>
        <Card title="最近告警">
          <Table<AlertEvent>
            rowKey="id"
            dataSource={alerts}
            pagination={{ pageSize: 10 }}
            columns={[
              { title: '规则', dataIndex: 'ruleId', width: 180 },
              {
                title: '级别',
                dataIndex: 'severity',
                width: 100,
                render: (v: string) => <Tag color={severityColor(v)}>{v}</Tag>,
              },
              { title: '状态', dataIndex: 'status', width: 120 },
              { title: '模块', dataIndex: 'module', width: 120 },
              { title: '摘要', dataIndex: 'summary' },
              { title: '次数', dataIndex: 'occurrenceCount', width: 80 },
              {
                title: '操作',
                width: 180,
                render: (_, row) => (
                  <Space>
                    <Button size="small" onClick={() => void ackAlert(row.id).then(load)}>
                      确认
                    </Button>
                    <Button
                      size="small"
                      onClick={() =>
                        void silenceAlert(row.id, { reason: 'operator silence', durationHours: 4 }).then(load)
                      }
                    >
                      静默
                    </Button>
                  </Space>
                ),
              },
            ]}
          />
        </Card>
        </Space>
      </Spin>
    </TmPageContainer>
  );
}

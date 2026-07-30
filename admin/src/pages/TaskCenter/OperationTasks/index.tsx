import { EyeOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Card, Col, Form, Row, Select, Space, Typography } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ErrorAlert, TmPageContainer, TmProTable } from '@/components/ui';
import { PAGE_COPY } from '@/constants/copywriting';
import {
  OPERATION_PLATFORM_LABELS,
  OPERATION_TASK_STATUS_LABELS,
  OPERATION_TASK_TYPE_LABELS,
} from '@/constants/operationTasks';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { extractOperationTaskAPIError, listTasks, type OperationTaskAPIError, type OperationTaskSummary } from '@/services/operationTasks';
import { formatDateTime } from '@/utils/formatTime';
import {
  NonProductionBoundary,
  OperationPriorityTag,
  OperationTaskStatusTag,
  copyableText,
  operationErrorMessage,
  platformLabel,
  taskTypeLabel,
} from './components/OperationTaskShared';

const { Text } = Typography;

const QUERY_KEYS = ['status', 'platform', 'taskType', 'cursor'] as const;
const LIMIT = 20;

type QueryState = Record<(typeof QUERY_KEYS)[number], string | undefined>;

function optionsFromLabels(labels: Record<string, string | { zhCN: string }>) {
  return Object.entries(labels).map(([value, label]) => ({
    value,
    label: typeof label === 'string' ? label : label.zhCN,
  }));
}

export default function OperationTasksPage() {
  const emptyLocale = useListEmptyLocale('operationTasks');
  const { state: urlState, setState: setUrlState, clearState } = useUrlQueryState<QueryState>(QUERY_KEYS);
  const [form] = Form.useForm();
  const [items, setItems] = useState<OperationTaskSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<OperationTaskAPIError | null>(null);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [hasMore, setHasMore] = useState(false);
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const requestSeq = useRef(0);

  useEffect(() => {
    form.setFieldsValue({
      status: urlState.status,
      platform: urlState.platform,
      taskType: urlState.taskType,
    });
  }, [form, urlState.platform, urlState.status, urlState.taskType]);

  const load = useCallback(async () => {
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    setLoading(true);
    setError(null);
    try {
      const page = await listTasks({
        status: urlState.status,
        platform: urlState.platform,
        taskType: urlState.taskType,
        cursor: urlState.cursor,
        limit: LIMIT,
      });
      if (requestSeq.current !== seq) return;
      setItems(page.items || []);
      setNextCursor(page.nextCursor);
      setHasMore(page.hasMore);
    } catch (e) {
      if (requestSeq.current !== seq) return;
      setError(extractOperationTaskAPIError(e));
      setItems([]);
      setNextCursor(undefined);
      setHasMore(false);
    } finally {
      if (requestSeq.current === seq) setLoading(false);
    }
  }, [urlState.cursor, urlState.platform, urlState.status, urlState.taskType]);

  useEffect(() => {
    void load();
  }, [load]);

  const updateFilters = (values: QueryState) => {
    setCursorStack([]);
    setUrlState({
      status: values.status,
      platform: values.platform,
      taskType: values.taskType,
      cursor: undefined,
    }, { replace: true });
  };

  const clearFilters = () => {
    setCursorStack([]);
    form.resetFields();
    clearState(QUERY_KEYS, { replace: true });
  };

  const goNext = () => {
    if (!nextCursor) return;
    setCursorStack((prev) => [...prev, urlState.cursor || '']);
    setUrlState({ cursor: nextCursor }, { replace: true });
  };

  const goPrev = () => {
    setCursorStack((prev) => {
      const next = [...prev];
      const prevCursor = next.pop();
      setUrlState({ cursor: prevCursor || undefined }, { replace: true });
      return next;
    });
  };

  const columns = useMemo<ProColumns<OperationTaskSummary>[]>(() => [
    {
      title: '任务标题',
      dataIndex: 'title',
      width: 260,
      render: (_, row) => (
        <Space direction="vertical" size={2}>
          <Button type="link" style={{ padding: 0 }} onClick={() => history.push(`/ops/task-center/operation-tasks/${encodeURIComponent(row.id)}`)}>
            {row.title || '未命名任务'}
          </Button>
          <Text type="secondary" ellipsis style={{ maxWidth: 240 }}>{row.summary || '—'}</Text>
        </Space>
      ),
    },
    { title: '任务类型', dataIndex: 'taskType', width: 130, render: (v) => taskTypeLabel(String(v || '')) },
    { title: '平台', dataIndex: 'platform', width: 110, render: (v) => platformLabel(String(v || '')) },
    { title: '状态', dataIndex: 'status', width: 130, render: (v) => <OperationTaskStatusTag status={String(v || '')} /> },
    { title: '优先级', dataIndex: 'priority', width: 100, render: (v) => <OperationPriorityTag priority={String(v || '')} /> },
    { title: '最新草稿', dataIndex: 'latestDraftVersion', width: 110, render: (v) => v ? `v${v}` : '—' },
    { title: '最新执行状态', dataIndex: 'latestExecutionStatus', width: 130, render: (v) => v ? <OperationTaskStatusTag status={String(v)} /> : '—' },
    { title: '创建人', dataIndex: 'createdBy', width: 130, render: (v) => copyableText(String(v || ''), 10) },
    { title: '创建时间', dataIndex: 'createdAt', width: 170, render: (v) => formatDateTime(String(v || '')) },
    { title: '更新时间', dataIndex: 'updatedAt', width: 170, render: (v) => formatDateTime(String(v || '')) },
    {
      title: '操作',
      valueType: 'option',
      width: 120,
      render: (_, row) => [
        <Button key="detail" type="link" icon={<EyeOutlined />} onClick={() => history.push(`/ops/task-center/operation-tasks/${encodeURIComponent(row.id)}`)}>
          查看详情
        </Button>,
      ],
    },
  ], []);

  const filterActive = !!(urlState.status || urlState.platform || urlState.taskType);

  return (
    <TmPageContainer
      title={PAGE_COPY.operationTasks.title}
      subTitle={PAGE_COPY.operationTasks.description}
      extra={<Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>刷新</Button>}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <NonProductionBoundary />
        {error ? <ErrorAlert title={operationErrorMessage(error)} actionHint={error.traceId ? `排查编号：${error.traceId}` : '请稍后重试或联系管理员。'} /> : null}
        <Card>
          <Form form={form} layout="vertical" onFinish={updateFilters}>
            <Row gutter={[16, 8]}>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="status" label="任务状态">
                  <Select allowClear options={optionsFromLabels(OPERATION_TASK_STATUS_LABELS)} placeholder="全部状态" />
                </Form.Item>
              </Col>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="platform" label="平台">
                  <Select allowClear options={optionsFromLabels(OPERATION_PLATFORM_LABELS)} placeholder="全部平台" />
                </Form.Item>
              </Col>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="taskType" label="任务类型">
                  <Select allowClear options={optionsFromLabels(OPERATION_TASK_TYPE_LABELS)} placeholder="全部类型" />
                </Form.Item>
              </Col>
              <Col xs={24} lg={6}>
                <Form.Item label="操作">
                  <Space wrap>
                    <Button type="primary" htmlType="submit">应用筛选</Button>
                    <Button onClick={clearFilters}>清除筛选</Button>
                  </Space>
                </Form.Item>
              </Col>
            </Row>
          </Form>
          <Text type="secondary">筛选参数与 URL 同步；当前 API 使用 keyset cursor，不提供总页数。</Text>
        </Card>

        <TmProTable<OperationTaskSummary>
          rowKey="id"
          search={false}
          loading={loading}
          columns={columns}
          dataSource={items}
          pagination={false}
          locale={emptyLocale}
          scroll={{ x: 1560 }}
          options={false}
          toolBarRender={false}
        />

        <Card size="small">
          <Space wrap>
            <Button disabled={cursorStack.length === 0 || loading} onClick={goPrev}>上一批</Button>
            <Button type="primary" disabled={!hasMore || !nextCursor || loading} onClick={goNext}>下一批</Button>
            <Text type="secondary">
              当前显示 {items.length} 条；{filterActive ? '筛选已启用' : '未启用筛选'}；不会伪造页码或总数。
            </Text>
          </Space>
        </Card>
      </Space>
    </TmPageContainer>
  );
}

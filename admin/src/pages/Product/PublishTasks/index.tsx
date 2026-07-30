import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { Alert, Button, Drawer, Popconfirm, Space, Tabs, Tag, Typography, message } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { Link, useSearchParams } from '@umijs/max';
import { useEffect, useMemo, useRef, useState } from 'react';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { publishBatchStatusTag } from '@/constants/publishLabels';
import { platformLabel } from '@/constants/userFriendly';
import { normalizeSource, parsePositiveInt, queryTimeRange } from '@/utils/urlState';
import {
  getProductPublishTask,
  queryProductPublishTasks,
  queryPublishBatches,
  retryProductPublishTask,
  type ProductPublishTaskDTO,
  type PublishBatchListItem,
} from '@/services/productPublish';

const PUBLISH_TASK_QUERY_KEYS = [
  'page',
  'pageSize',
  'keyword',
  'status',
  'platform',
  'shopId',
  'batchId',
  'tab',
  'id',
  'drawer',
  'source',
  'productId',
  'start',
  'end',
  'createdFrom',
  'createdTo',
] as const;

function tagFromStatus(raw: string) {
  const c = COLLECT_TASK_STATUS[raw as keyof typeof COLLECT_TASK_STATUS];
  if (!c) return <Tag>{raw}</Tag>;
  return <Tag color={c.color}>{c.text}</Tag>;
}

export default function ProductPublishTasksPage() {
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } =
    useUrlQueryState<Record<(typeof PUBLISH_TASK_QUERY_KEYS)[number], string | undefined>>(
      PUBLISH_TASK_QUERY_KEYS,
    );
  const navSource = normalizeSource(urlState.source);
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const batchActionRef = useRef<ActionType>();
  const batchFormRef = useRef<ProFormInstance>();
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);
  const [batchPage, setBatchPage] = useState(1);
  const [batchPageSize, setBatchPageSize] = useState(20);
  const activeTab = urlState.tab === 'tasks' ? 'tasks' : 'batches';
  const taskIdFromUrl = urlState.id;
  const statusFromUrl = urlState.status;
  const batchIdFromUrl = urlState.batchId;
  const emptyLocale = useListEmptyLocale('publishBatches', { permissionScoped: true });
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<ProductPublishTaskDTO | null>(null);

  useEffect(() => {
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
    setBatchPage(parsePositiveInt(urlState.page, 1));
    setBatchPageSize(parsePositiveInt(urlState.pageSize, 20));
    const createdRange = queryTimeRange(
      urlState.start,
      urlState.end,
      urlState.createdFrom,
      urlState.createdTo,
    );
    formRef.current?.setFieldsValue?.({
      status: statusFromUrl,
      platform: urlState.platform,
      shopId: urlState.shopId,
      productId: urlState.productId,
      createdRange,
    });
  }, [
    statusFromUrl,
    urlState.createdFrom,
    urlState.createdTo,
    urlState.end,
    urlState.page,
    urlState.pageSize,
    urlState.platform,
    urlState.productId,
    urlState.shopId,
    urlState.start,
  ]);

  useEffect(() => {
    if (!statusFromUrl && !urlState.platform && !urlState.shopId && !urlState.productId) return;
    actionRef.current?.reload?.();
  }, [statusFromUrl, urlState.platform, urlState.productId, urlState.shopId]);

  useEffect(() => {
    if (!taskIdFromUrl) return;
    void (async () => {
      try {
        const row = await getProductPublishTask(taskIdFromUrl);
        setDetail(row);
        setDetailOpen(true);
      } catch (e: unknown) {
        message.error((e as Error)?.message || '加载任务失败');
      }
    })();
  }, [taskIdFromUrl]);

  const openTaskDetail = async (id: string) => {
    const row = await getProductPublishTask(id);
    setDetail(row);
    setDetailOpen(true);
    setUrlState({ drawer: 'task', id });
  };

  const closeTaskDetail = () => {
    setDetailOpen(false);
    setDetail(null);
    setUrlState({ drawer: undefined, id: undefined }, { replace: true });
  };

  const columns: ProColumns<ProductPublishTaskDTO>[] = useMemo(
    () => [
      {
        title: '创建时间范围',
        dataIndex: 'createdRange',
        hideInTable: true,
        valueType: 'dateTimeRange',
        search: {
          transform: ([start, end]: [unknown, unknown]) => ({
            start: start ? dayjs(start as string).toISOString() : undefined,
            end: end ? dayjs(end as string).toISOString() : undefined,
          }),
        },
      },
      {
        title: '创建时间',
        dataIndex: 'createdAt',
        width: 168,
        search: false,
        render: (_, r) => formatDateTime(r.createdAt),
      },
      {
        title: '商品 ID',
        dataIndex: 'productId',
        hideInTable: true,
        valueType: 'text',
      },
      {
        title: '店铺 ID',
        dataIndex: 'shopId',
        hideInTable: true,
        valueType: 'text',
      },
      {
        title: '店铺',
        dataIndex: 'shopName',
        width: 140,
        search: false,
        ellipsis: true,
        render: (_, r) => r.shopName || '—',
      },
      {
        title: '商品',
        dataIndex: 'productTitle',
        width: 160,
        search: false,
        ellipsis: true,
        render: (_, r) => r.productTitle || '—',
      },
      {
        title: '平台',
        dataIndex: 'platform',
        width: 100,
        render: (_, r) => platformLabel(r.platform),
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 96,
        valueType: 'select',
        valueEnum: COLLECT_TASK_STATUS,
        render: (_, r) => tagFromStatus(r.status),
      },
      {
        title: '开始',
        dataIndex: 'startedAt',
        width: 156,
        search: false,
        render: (_, r) => (r.startedAt ? formatDateTime(r.startedAt) : '—'),
      },
      {
        title: '结束',
        dataIndex: 'finishedAt',
        width: 156,
        search: false,
        render: (_, r) => (r.finishedAt ? formatDateTime(r.finishedAt) : '—'),
      },
      {
        title: '错误',
        dataIndex: 'errorMessage',
        ellipsis: true,
        search: false,
        render: (_, r) => r.errorMessage || '—',
      },
      {
        title: '操作',
        valueType: 'option',
        width: 140,
        render: (_, r) => (
          <Space>
            <a onClick={() => void openTaskDetail(r.id)}>查看</a>
            {r.status === 'failed' ? (
              <Popconfirm
                title="确认重试该刊登任务？"
                onConfirm={async () => {
                  await retryProductPublishTask(r.id);
                  message.success('已提交重试');
                  actionRef.current?.reload();
                }}
              >
                <Button type="link" size="small" style={{ padding: 0 }}>
                  重试
                </Button>
              </Popconfirm>
            ) : null}
          </Space>
        ),
      },
    ],
    [],
  );

  const batchColumns: ProColumns<PublishBatchListItem>[] = useMemo(
    () => [
      {
        title: '创建时间',
        dataIndex: 'createdAt',
        width: 168,
        search: false,
        render: (_, r) => formatDateTime(r.createdAt),
      },
      {
        title: '批次名称',
        dataIndex: 'name',
        ellipsis: true,
        search: false,
        render: (_, r) => r.name || `批次 ${r.id.slice(0, 8)}`,
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 110,
        search: false,
        render: (_, r) => {
          const meta = publishBatchStatusTag(r.status, r.statusLabel);
          return <Tag color={meta.color}>{meta.text}</Tag>;
        },
      },
      { title: '商品数', dataIndex: 'productCount', width: 80, search: false },
      { title: '目标数', dataIndex: 'targetCount', width: 80, search: false },
      { title: '任务数', dataIndex: 'taskCount', width: 80, search: false },
      { title: '成功', dataIndex: 'successCount', width: 72, search: false },
      { title: '失败', dataIndex: 'failedCount', width: 72, search: false },
      {
        title: '操作',
        valueType: 'option',
        width: 100,
        render: (_, r) => {
          const detailHref = navSource
            ? `/product/publish-batches/${r.id}?source=${encodeURIComponent(navSource)}`
            : `/product/publish-batches/${r.id}`;
          return <Link to={detailHref}>查看</Link>;
        },
      },
    ],
    [navSource],
  );

  const buildListQuery = (params: Record<string, unknown>, page: number, pageSize: number) => ({
    page,
    pageSize,
    shopId: (params.shopId as string | undefined)?.trim(),
    productId: (params.productId as string | undefined)?.trim() || urlState.productId,
    platform: (params.platform as string | undefined)?.trim(),
    status: (params.status as string | undefined)?.trim() || statusFromUrl,
    start: typeof params.start === 'string' ? params.start : urlState.start,
    end: typeof params.end === 'string' ? params.end : urlState.end,
  });

  return (
    <TmPageContainer title="商品刊登任务" subTitle="查看刊登子任务与批量刊登批次进度。">
      {navSource ? (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="已从关联页面带入导航上下文（不影响权限与店铺范围）。"
        />
      ) : null}
      {batchIdFromUrl ? (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={`当前批次筛选：${batchIdFromUrl}`}
        />
      ) : null}
      <Tabs
        activeKey={activeTab}
        onChange={(key) => {
          setUrlState(
            {
              tab: key === 'tasks' ? undefined : key,
              page: undefined,
              pageSize: undefined,
              drawer: undefined,
              id: undefined,
            },
            { replace: true },
          );
          setTablePage(1);
          setBatchPage(1);
        }}
        items={[
          {
            key: 'tasks',
            label: '子任务',
            children: (
              <ProTable<ProductPublishTaskDTO>
                rowKey="id"
                actionRef={actionRef}
                formRef={formRef}
                columns={columns}
                search={{ labelWidth: 'auto', defaultCollapsed: false }}
                onReset={() => {
                  setTablePage(1);
                  setTablePageSize(20);
                  closeTaskDetail();
                  clearUrlState(PUBLISH_TASK_QUERY_KEYS, { replace: true });
                }}
                pagination={{
                  current: tablePage,
                  pageSize: tablePageSize,
                  showSizeChanger: true,
                  onChange: (page, pageSize) => {
                    setTablePage(page);
                    setTablePageSize(pageSize);
                    setUrlState({
                      page: page > 1 ? page : undefined,
                      pageSize: pageSize !== 20 ? pageSize : undefined,
                    });
                  },
                }}
                headerTitle="刊登记录"
                locale={emptyLocale}
                request={async (params) => {
                  const qp = buildListQuery(params, params.current ?? tablePage, params.pageSize ?? tablePageSize);
                  setUrlState(
                    {
                      page: Number(qp.page) > 1 ? qp.page : undefined,
                      pageSize: Number(qp.pageSize) !== 20 ? qp.pageSize : undefined,
                      shopId: qp.shopId,
                      productId: qp.productId,
                      platform: qp.platform,
                      status: qp.status,
                      start: qp.start,
                      end: qp.end,
                      tab: undefined,
                      source: urlState.source,
                      drawer: urlState.drawer,
                      id: urlState.id,
                    },
                    { replace: true },
                  );
                  const res = await queryProductPublishTasks(qp);
                  return { data: res.list, total: res.pagination.total, success: true };
                }}
              />
            ),
          },
          {
            key: 'batches',
            label: '刊登批次',
            children: (
              <ProTable<PublishBatchListItem>
                rowKey="id"
                actionRef={batchActionRef}
                formRef={batchFormRef}
                columns={batchColumns}
                search={false}
                pagination={{
                  current: batchPage,
                  pageSize: batchPageSize,
                  showSizeChanger: true,
                  onChange: (page, pageSize) => {
                    setBatchPage(page);
                    setBatchPageSize(pageSize);
                    setUrlState({
                      tab: 'batches',
                      page: page > 1 ? page : undefined,
                      pageSize: pageSize !== 20 ? pageSize : undefined,
                    });
                  },
                }}
                headerTitle="批量刊登批次"
                locale={emptyLocale}
                request={async (params) => {
                  const page = params.current ?? batchPage;
                  const pageSize = params.pageSize ?? batchPageSize;
                  setUrlState(
                    {
                      tab: 'batches',
                      page: page > 1 ? page : undefined,
                      pageSize: pageSize !== 20 ? pageSize : undefined,
                      source: urlState.source,
                    },
                    { replace: true },
                  );
                  const res = await queryPublishBatches({ page, pageSize });
                  return { data: res.list, total: res.pagination.total, success: true };
                }}
              />
            ),
          },
        ]}
      />

      <Drawer
        width={560}
        title={detail ? `刊登任务 ${detail.id}` : '详情'}
        open={detailOpen}
        destroyOnHidden
        onClose={closeTaskDetail}
      >
        {detail && (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <div>
              <Typography.Text strong>状态：</Typography.Text> {tagFromStatus(detail.status)}
            </div>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>店铺：</Typography.Text> {detail.shopName || detail.shopId}{' '}
              <Typography.Text type="secondary">({detail.platform})</Typography.Text>
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>商品：</Typography.Text>{' '}
              {detail.productTitle || detail.productId}
            </Typography.Paragraph>
            {detail.errorMessage ? (
              <Typography.Paragraph>
                <Typography.Text strong>失败原因：</Typography.Text> {detail.errorMessage}
              </Typography.Paragraph>
            ) : null}
            {detail.platformProductId ? (
              <Typography.Paragraph copyable={{ text: detail.platformProductId }}>
                <Typography.Text strong>平台商品编号：</Typography.Text> {detail.platformProductId}
              </Typography.Paragraph>
            ) : null}
            {detail.retryable != null ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Typography.Text strong>可以重试：</Typography.Text> {detail.retryable ? '是' : '否'}
              </Typography.Paragraph>
            ) : null}
            <TechnicalDetails>
              {detail.requestId ? (
                <Typography.Paragraph copyable={{ text: detail.requestId }} style={{ marginBottom: 8 }}>
                  <Typography.Text strong>请求编号：</Typography.Text> {detail.requestId}
                </Typography.Paragraph>
              ) : null}
              <Typography.Paragraph copyable={{ text: detail.id }} style={{ marginBottom: 8 }}>
                <Typography.Text strong>任务编号：</Typography.Text> {detail.id}
              </Typography.Paragraph>
              <TaskJsonBlock title="平台提交内容" value={detail.platformPayload} />
              <TaskJsonBlock title="平台返回结果" value={detail.platformResult ?? detail.output} />
              <TaskJsonBlock title="任务输入" value={detail.input} />
              <TaskJsonBlock title="任务输出" value={detail.output} last />
            </TechnicalDetails>
          </Space>
        )}
      </Drawer>
    </TmPageContainer>
  );
}

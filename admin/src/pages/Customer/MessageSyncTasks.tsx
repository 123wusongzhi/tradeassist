import { type ActionType, type ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { Alert, Button, Drawer, Popconfirm, Space, Tag, Typography, message } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { useEffect, useMemo, useRef, useState } from 'react';
import { CUSTOMER_MESSAGE_SYNC_TASK_STATUS } from '@/constants/status';
import { PLATFORM_OPTIONS, platformLabel } from '@/constants/userFriendly';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { normalizeSource, parsePositiveInt, queryTimeRange } from '@/utils/urlState';
import {
  getCustomerMessageSyncTask,
  queryCustomerMessageSyncTasks,
  retryCustomerMessageSyncTask,
  type CustomerMessageSyncTaskRow,
} from '@/services/customer';

const CUSTOMER_SYNC_QUERY_KEYS = [
  'page',
  'pageSize',
  'status',
  'platform',
  'shopId',
  'resultStatus',
  'drawer',
  'id',
  'source',
  'start',
  'end',
  'createdFrom',
  'createdTo',
] as const;

function tagFromStatus(raw: string) {
  const c = CUSTOMER_MESSAGE_SYNC_TASK_STATUS[raw as keyof typeof CUSTOMER_MESSAGE_SYNC_TASK_STATUS];
  if (!c) return <Tag>{raw}</Tag>;
  return <Tag color={c.color}>{c.text}</Tag>;
}

export default function CustomerMessageSyncTasksPage() {
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } =
    useUrlQueryState<Record<(typeof CUSTOMER_SYNC_QUERY_KEYS)[number], string | undefined>>(
      CUSTOMER_SYNC_QUERY_KEYS,
    );
  const navSource = normalizeSource(urlState.source);
  const actionRef = useRef<ActionType>();
  const formRef = useRef<import('@ant-design/pro-components').ProFormInstance>();
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);
  const taskIdFromUrl = urlState.id;
  const statusFromUrl = urlState.status || urlState.resultStatus;
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<CustomerMessageSyncTaskRow | null>(null);

  useEffect(() => {
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
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
    urlState.shopId,
    urlState.start,
  ]);

  useEffect(() => {
    if (!taskIdFromUrl) return;
    void (async () => {
      try {
        const d = await getCustomerMessageSyncTask(taskIdFromUrl);
        setDetail(d);
        setDetailOpen(true);
      } catch {
        /* ignore invalid id */
      }
    })();
  }, [taskIdFromUrl]);

  const openDetail = async (id: string) => {
    const d = await getCustomerMessageSyncTask(id);
    setDetail(d);
    setDetailOpen(true);
    setUrlState({ drawer: 'task', id });
  };

  const closeDetail = () => {
    setDetailOpen(false);
    setDetail(null);
    setUrlState({ drawer: undefined, id: undefined }, { replace: true });
  };

  const columns: ProColumns<CustomerMessageSyncTaskRow>[] = useMemo(
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
        title: '平台',
        dataIndex: 'platform',
        width: 100,
        valueType: 'select',
        fieldProps: {
          showSearch: true,
          optionFilterProp: 'label',
          options: PLATFORM_OPTIONS,
          allowClear: true,
        },
        render: (_, r) => platformLabel(r.platform),
      },
      {
        title: '模式',
        dataIndex: 'mode',
        width: 96,
        search: false,
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 96,
        valueType: 'select',
        valueEnum: CUSTOMER_MESSAGE_SYNC_TASK_STATUS,
        render: (_, r) => tagFromStatus(r.status),
      },
      {
        title: '总计',
        dataIndex: 'totalCount',
        width: 72,
        search: false,
      },
      {
        title: '成功',
        dataIndex: 'successCount',
        width: 72,
        search: false,
      },
      {
        title: '失败',
        dataIndex: 'failedCount',
        width: 72,
        search: false,
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
            <a onClick={() => void openDetail(r.id)}>查看</a>
            {r.status === 'failed' || r.status === 'partial_success' ? (
              <Popconfirm
                title="确认重试该同步任务？"
                onConfirm={async () => {
                  await retryCustomerMessageSyncTask(r.id);
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

  return (
    <TmPageContainer title="客服消息同步任务">
      {navSource ? (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="已从关联页面带入导航上下文（不影响权限与店铺范围）。"
        />
      ) : null}
      <ProTable<CustomerMessageSyncTaskRow>
        rowKey="id"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        onReset={() => {
          setTablePage(1);
          setTablePageSize(20);
          closeDetail();
          clearUrlState(CUSTOMER_SYNC_QUERY_KEYS, { replace: true });
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
        headerTitle="同步记录"
        request={async (params) => {
          const qp = {
            page: params.current ?? tablePage,
            pageSize: params.pageSize ?? tablePageSize,
            shopId: (params.shopId as string | undefined)?.trim() || urlState.shopId,
            platform: (params.platform as string | undefined)?.trim(),
            status: (params.status as string | undefined)?.trim() || statusFromUrl,
            start: typeof params.start === 'string' ? params.start : urlState.start,
            end: typeof params.end === 'string' ? params.end : urlState.end,
          };
          setUrlState(
            {
              page: Number(qp.page) > 1 ? qp.page : undefined,
              pageSize: Number(qp.pageSize) !== 20 ? qp.pageSize : undefined,
              shopId: qp.shopId,
              platform: qp.platform,
              status: qp.status,
              resultStatus: qp.status === 'partial_success' ? qp.status : undefined,
              start: qp.start,
              end: qp.end,
              source: urlState.source,
              drawer: urlState.drawer,
              id: urlState.id,
            },
            { replace: true },
          );
          const res = await queryCustomerMessageSyncTasks(qp);
          return { data: res.list, total: res.pagination.total, success: true };
        }}
      />

      <Drawer
        width={560}
        title={detail ? `同步任务 ${detail.id}` : '详情'}
        open={detailOpen}
        destroyOnHidden
        onClose={closeDetail}
      >
        {detail && (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <div>
              <Typography.Text strong>状态：</Typography.Text> {tagFromStatus(detail.status)}
            </div>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>店铺：</Typography.Text> {detail.shopName || detail.shopId}{' '}
              <Typography.Text type="secondary">({platformLabel(detail.platform)})</Typography.Text>
            </Typography.Paragraph>
            <Typography.Paragraph copyable={{ text: detail.id }}>
              <Typography.Text strong>任务编号：</Typography.Text> {detail.id}
            </Typography.Paragraph>
            {detail.errorMessage ? (
              <Typography.Paragraph type="danger">
                <Typography.Text strong>失败原因：</Typography.Text> {detail.errorMessage}
              </Typography.Paragraph>
            ) : null}
            <TechnicalDetails>
              <TaskJsonBlock title="任务输入" value={detail.input} />
              <TaskJsonBlock title="任务输出" value={detail.output} last />
            </TechnicalDetails>
          </Space>
        )}
      </Drawer>
    </TmPageContainer>
  );
}

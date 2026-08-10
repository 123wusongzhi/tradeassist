import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { Alert, Button, Checkbox, Descriptions, Drawer, Form, Input, Modal, Popconfirm, Radio, Select, Space, Tabs, Tag, Typography, message } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { Link, useSearchParams } from '@umijs/max';
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { publishBatchStatusTag } from '@/constants/publishLabels';
import { platformLabel } from '@/constants/userFriendly';
import { normalizeSource, parsePositiveInt, queryTimeRange } from '@/utils/urlState';
import { canReconcileOzonPublishTask, extractOzonOfferIds, extractOzonWarnings, getProductPublishTask, productPublishBusinessStatus, queryProductPublishTasks, queryPublishBatches, reconcileOzonPublishTask, retryProductPublishTask, type OzonReconcileOutcome, type OzonReconcilePlatformStatus, type ProductPublishTaskDTO, type PublishBatchListItem } from '@/services/productPublish';
import './index.less';

const PUBLISH_TASK_QUERY_KEYS = ['page', 'pageSize', 'keyword', 'status', 'platform', 'shopId', 'batchId', 'tab', 'id', 'drawer', 'source', 'productId', 'start', 'end', 'createdFrom', 'createdTo'] as const;

function tagFromStatus(raw: string) {
  const c = COLLECT_TASK_STATUS[raw as keyof typeof COLLECT_TASK_STATUS];
  if (!c) return <Tag>{raw}</Tag>;
  return <Tag color={c.color}>{c.text}</Tag>;
}

function businessStatusTag(task: ProductPublishTaskDTO) {
  const meta = productPublishBusinessStatus(task);
  return <Tag color={meta.color}>{meta.text}</Tag>;
}

type OzonReconcileFormValues = {
  outcome?: OzonReconcileOutcome;
  evidence?: string;
  externalProductId?: string;
  externalSpuId?: string;
  externalUrl?: string;
  platformStatus?: OzonReconcilePlatformStatus;
  sellableVerified?: boolean;
};

export default function ProductPublishTasksPage() {
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } = useUrlQueryState<Record<(typeof PUBLISH_TASK_QUERY_KEYS)[number], string | undefined>>(PUBLISH_TASK_QUERY_KEYS);
  const navSource = normalizeSource(urlState.source);
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const batchActionRef = useRef<ActionType>();
  const batchFormRef = useRef<ProFormInstance>();
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);
  const [batchPage, setBatchPage] = useState(1);
  const [batchPageSize, setBatchPageSize] = useState(20);
  const activeTab = urlState.tab === 'batches' ? 'batches' : 'tasks';
  const taskIdFromUrl = urlState.id;
  const statusFromUrl = urlState.status;
  const batchIdFromUrl = urlState.batchId;
  const emptyLocale = useListEmptyLocale('publishBatches', {
    permissionScoped: true,
  });
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<ProductPublishTaskDTO | null>(null);
  const [refreshingDetail, setRefreshingDetail] = useState(false);
  const [detailPollingError, setDetailPollingError] = useState<string>();
  const detailPollingInFlight = useRef(false);
  const [reconcileOpen, setReconcileOpen] = useState(false);
  const [reconciling, setReconciling] = useState(false);
  const [reconcileForm] = Form.useForm<OzonReconcileFormValues>();
  const reconcileOutcome = Form.useWatch('outcome', reconcileForm);
  const reconcilePlatformStatus = Form.useWatch('platformStatus', reconcileForm);
  const detailBusinessStatus = detail ? productPublishBusinessStatus(detail) : null;
  const detailIsProcessing = detail ? ['pending', 'running'].includes(String(detail.status || '').toLowerCase()) || ['pending', 'ready', 'checking', 'publishing', 'running'].includes(String(detailBusinessStatus?.code || '').toLowerCase()) : false;
  const detailCanReconcile = detail ? canReconcileOzonPublishTask(detail) : false;
  const detailOfferIds = useMemo(() => (detail ? extractOzonOfferIds(detail) : []), [detail]);
  const detailWarnings = useMemo(() => (detail ? extractOzonWarnings(detail) : []), [detail]);

  useEffect(() => {
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
    setBatchPage(parsePositiveInt(urlState.page, 1));
    setBatchPageSize(parsePositiveInt(urlState.pageSize, 20));
    const createdRange = queryTimeRange(urlState.start, urlState.end, urlState.createdFrom, urlState.createdTo);
    formRef.current?.setFieldsValue?.({
      status: statusFromUrl,
      platform: urlState.platform,
      shopId: urlState.shopId,
      productId: urlState.productId,
      createdRange,
    });
  }, [statusFromUrl, urlState.createdFrom, urlState.createdTo, urlState.end, urlState.page, urlState.pageSize, urlState.platform, urlState.productId, urlState.shopId, urlState.start]);

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

  useEffect(() => {
    if (!detailOpen || !detail?.id || !detailIsProcessing) return;
    const taskId = detail.id;
    const timer = globalThis.setInterval(() => {
      if (detailPollingInFlight.current) return;
      detailPollingInFlight.current = true;
      void getProductPublishTask(taskId)
        .then((row) => {
          setDetail(row);
          setDetailPollingError(undefined);
          actionRef.current?.reload?.();
        })
        .catch((error: unknown) => {
          setDetailPollingError((error as Error)?.message || '自动刷新暂时失败');
        })
        .finally(() => {
          detailPollingInFlight.current = false;
        });
    }, 3000);
    return () => globalThis.clearInterval(timer);
  }, [detail?.id, detailIsProcessing, detailOpen]);

  const openTaskDetail = async (id: string) => {
    const row = await getProductPublishTask(id);
    setDetail(row);
    setDetailOpen(true);
    setUrlState({ drawer: 'task', id });
  };

  const closeTaskDetail = () => {
    setDetailOpen(false);
    setDetail(null);
    setDetailPollingError(undefined);
    setUrlState({ drawer: undefined, id: undefined }, { replace: true });
  };

  const refreshTaskDetail = async () => {
    if (!detail?.id) return;
    setRefreshingDetail(true);
    try {
      const row = await getProductPublishTask(detail.id);
      setDetail(row);
      setDetailPollingError(undefined);
      actionRef.current?.reload?.();
      message.success('已刷新任务业务状态');
    } catch (error: unknown) {
      message.error((error as Error)?.message || '刷新任务失败');
    } finally {
      setRefreshingDetail(false);
    }
  };

  const openReconcile = () => {
    if (!detail || !canReconcileOzonPublishTask(detail)) return;
    reconcileForm.resetFields();
    reconcileForm.setFieldsValue({
      externalProductId: detail.platformProductId || undefined,
      externalSpuId: detailOfferIds[0],
      platformStatus: 'imported',
      sellableVerified: false,
    });
    setReconcileOpen(true);
  };

  const submitReconciliation = async () => {
    if (!detail) return;
    const values = await reconcileForm.validateFields();
    if (!values.outcome || !values.evidence?.trim()) return;
    if (values.outcome === 'platform_created' && !values.externalProductId?.trim()) {
      reconcileForm.setFields([
        {
          name: 'externalProductId',
          errors: ['确认已创建时必须填写平台商品编号'],
        },
      ]);
      return;
    }
    if (values.outcome === 'platform_created' && values.platformStatus === 'sellable' && values.sellableVerified !== true) {
      reconcileForm.setFields([
        {
          name: 'sellableVerified',
          errors: ['确认成功上架前必须勾选已验证 Ozon 前台可售'],
        },
      ]);
      return;
    }
    setReconciling(true);
    try {
      const row = await reconcileOzonPublishTask(detail.id, {
        outcome: values.outcome,
        evidence: values.evidence.trim(),
        externalProductId: values.externalProductId?.trim() || undefined,
        externalSpuId: values.externalSpuId?.trim() || undefined,
        externalUrl: values.externalUrl?.trim() || undefined,
        platformStatus: values.outcome === 'platform_created' ? values.platformStatus : undefined,
        sellableVerified: values.outcome === 'platform_created' ? values.sellableVerified === true : undefined,
      });
      setDetail(row);
      setReconcileOpen(false);
      actionRef.current?.reload?.();
      message.success(values.outcome === 'platform_not_created' ? '已记录 Ozon 未创建；任务现在可由运营手动决定是否重试' : '已记录 Ozon 人工核对结果');
    } catch (error: unknown) {
      message.error((error as Error)?.message || '保存对账结果失败');
    } finally {
      setReconciling(false);
    }
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
        title: '业务状态',
        dataIndex: 'status',
        width: 96,
        valueType: 'select',
        valueEnum: COLLECT_TASK_STATUS,
        render: (_, r) => businessStatusTag(r),
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
        render: (_, r) => {
          let retryAction: ReactNode = null;
          if (r.status === 'failed' && (r.platform !== 'ozon' || r.retryable === true)) {
            retryAction = (
              <Popconfirm
                title="确认重试该刊登提交？"
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
            );
          } else if (productPublishBusinessStatus(r).requiresReconciliation) {
            retryAction = <Typography.Text type="warning">{canReconcileOzonPublishTask(r) ? '请先安全对账' : '请人工核对'}</Typography.Text>;
          } else if (r.status === 'failed' && r.platform === 'ozon') {
            retryAction = <Typography.Text type="secondary">不可自动重试</Typography.Text>;
          }
          return (
            <Space>
              <a onClick={() => void openTaskDetail(r.id)}>查看</a>
              {retryAction}
            </Space>
          );
        },
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
      {
        title: '已建草稿',
        dataIndex: 'successCount',
        width: 88,
        search: false,
      },
      { title: '创建失败', dataIndex: 'failedCount', width: 88, search: false },
      {
        title: '操作',
        valueType: 'option',
        width: 100,
        render: (_, r) => {
          const detailHref = navSource ? `/product/publish-batches/${r.id}?source=${encodeURIComponent(navSource)}` : `/product/publish-batches/${r.id}`;
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
    <TmPageContainer title="刊登进度" subTitle="默认查看单品提交；可核对最终提交快照、平台返回结果和平台商品编号。">
      {navSource ? <Alert type="info" showIcon style={{ marginBottom: 12 }} message="已从关联页面带入导航上下文（不影响权限与店铺范围）。" /> : null}
      {batchIdFromUrl ? <Alert type="info" showIcon style={{ marginBottom: 12 }} message={`当前批次筛选：${batchIdFromUrl}`} /> : null}
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
            label: '单品提交',
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
                  return {
                    data: res.list,
                    total: res.pagination.total,
                    success: true,
                  };
                }}
              />
            ),
          },
          {
            key: 'batches',
            label: '批次（高级）',
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
                  return {
                    data: res.list,
                    total: res.pagination.total,
                    success: true,
                  };
                }}
              />
            ),
          },
        ]}
      />

      <Drawer width="min(560px, 100vw)" rootClassName="product-publish-task-drawer" title={detail ? `刊登进度 ${detail.id}` : '详情'} open={detailOpen} destroyOnHidden onClose={closeTaskDetail} styles={{ body: { overflowX: 'hidden' } }}>
        {detail && (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Space size={8} wrap>
              <Typography.Text strong>业务状态：</Typography.Text>
              {businessStatusTag(detail)}
              {detailIsProcessing ? <Tag color="processing">每 3 秒自动刷新</Tag> : null}
            </Space>
            {detailPollingError ? <Alert type="warning" showIcon message="自动刷新暂时失败" description={`${detailPollingError}；可使用下方“刷新任务结果”重试。`} /> : null}
            <Descriptions bordered size="small" column={1}>
              <Descriptions.Item label="业务状态代码">{detailBusinessStatus?.code || '—'}</Descriptions.Item>
              <Descriptions.Item label="技术任务状态">{tagFromStatus(detail.status)}</Descriptions.Item>
              <Descriptions.Item label="恢复状态">{detail.recoveryState || '—'}</Descriptions.Item>
            </Descriptions>
            {detailBusinessStatus?.code === 'imported' || detailBusinessStatus?.code === 'pending_review' ? <Alert type="info" showIcon message={detailBusinessStatus.text} description="Ozon 已受理或正在审核，但尚未确认前台可售；此时不会显示“成功上架”。" /> : null}
            {detailBusinessStatus?.code === 'needs_action' ? <Alert type="error" showIcon message="Ozon 要求修改商品" description="请先处理下方平台警告，再重新保存配置；不要把该状态当作上架成功。" /> : null}
            {detailBusinessStatus?.code === 'result_unknown' ? <Alert type="warning" showIcon message="Ozon 平台结果未知" description="禁止直接重试。请按 offer_id 或平台商品编号在 Ozon 后台只读核对，再通过“安全对账”记录事实。" /> : null}
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>店铺：</Typography.Text> {detail.shopName || detail.shopId} <Typography.Text type="secondary">({detail.platform})</Typography.Text>
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>商品：</Typography.Text> {detail.productTitle || detail.productId}
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
            {detailOfferIds.length > 0 ? (
              <div>
                <Typography.Text strong>offer_id：</Typography.Text>
                <Space direction="vertical" size={2} style={{ width: '100%', marginTop: 4 }}>
                  {detailOfferIds.map((offerId) => (
                    <Typography.Text key={offerId} code copyable={{ text: offerId }}>
                      {offerId}
                    </Typography.Text>
                  ))}
                </Space>
              </div>
            ) : null}
            {detailWarnings.length > 0 ? (
              <Alert
                type="warning"
                showIcon
                message={`Ozon 返回 ${detailWarnings.length} 项警告`}
                description={
                  <ul style={{ margin: 0, paddingInlineStart: 20 }}>
                    {detailWarnings.map((warning) => (
                      <li key={warning}>{warning}</li>
                    ))}
                  </ul>
                }
              />
            ) : null}
            {detail.retryable !== undefined && detail.retryable !== null ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Typography.Text strong>可以重试：</Typography.Text> {detail.retryable ? '是' : '否'}
              </Typography.Paragraph>
            ) : null}
            {detailBusinessStatus?.requiresReconciliation ? <Alert type="warning" showIcon message="该结果不可自动重试" description={detailCanReconcile ? '请先在 Ozon 后台按 offer_id / 平台商品编号核对是否已受理，避免重复创建。安全对账只记录人工确认事实，不会自动重试或再次调用 Ozon 写接口。' : '请在 Ozon 后台按 offer_id / 平台商品编号人工核对。该历史任务不是 failed 状态，当前不会开放一个后端必然拒绝的对账或重试操作。'} /> : null}
            <Space wrap>
              <Button loading={refreshingDetail} onClick={() => void refreshTaskDetail()}>
                刷新任务结果
              </Button>
              {detailCanReconcile ? (
                <Button type="primary" danger onClick={openReconcile}>
                  安全对账
                </Button>
              ) : null}
              <Link to={`/product/publishing-center?productId=${encodeURIComponent(detail.productId)}&shopId=${encodeURIComponent(detail.shopId)}`}>返回商品刊登配置</Link>
              <Link to={`/product/drafts/${encodeURIComponent(detail.productId)}`}>查看商品详情</Link>
            </Space>
            <TechnicalDetails>
              {detail.requestId ? (
                <Typography.Paragraph copyable={{ text: detail.requestId }} style={{ marginBottom: 8 }}>
                  <Typography.Text strong>请求编号：</Typography.Text> {detail.requestId}
                </Typography.Paragraph>
              ) : null}
              <Typography.Paragraph copyable={{ text: detail.id }} style={{ marginBottom: 8 }}>
                <Typography.Text strong>任务编号：</Typography.Text> {detail.id}
              </Typography.Paragraph>
              <TaskJsonBlock title="最终提交快照" value={detail.platformPayload} />
              <TaskJsonBlock title="平台返回结果" value={detail.platformResult ?? detail.output} />
              <TaskJsonBlock title="任务输入" value={detail.input} />
              <TaskJsonBlock title="任务输出" value={detail.output} last />
            </TechnicalDetails>
          </Space>
        )}
      </Drawer>

      <Modal title="Ozon 安全对账" open={reconcileOpen} width={680} okText="保存人工核对结果" cancelText="取消" confirmLoading={reconciling} okButtonProps={{ danger: true }} onCancel={() => setReconcileOpen(false)} onOk={() => void submitReconciliation()} destroyOnHidden>
        <Alert type="warning" showIcon message="本操作不会调用 Ozon，也不会自动重试" description="请先在 Ozon 后台按 offer_id / 平台商品编号完成只读核对。错误确认“未创建”后再重试，可能产生重复商品。" style={{ marginBottom: 16 }} />
        <Form form={reconcileForm} layout="vertical">
          <Form.Item name="outcome" label="核对结论" rules={[{ required: true, message: '请选择核对结论' }]}>
            <Radio.Group>
              <Radio value="platform_created">Ozon 已创建/已受理</Radio>
              <Radio value="platform_not_created">已确认 Ozon 未创建</Radio>
            </Radio.Group>
          </Form.Item>
          {reconcileOutcome === 'platform_created' ? (
            <>
              <Form.Item
                name="externalProductId"
                label="Ozon 平台商品编号"
                rules={[
                  {
                    required: true,
                    whitespace: true,
                    message: '请输入平台商品编号',
                  },
                ]}
              >
                <Input placeholder="只填写从 Ozon 后台核对到的编号" />
              </Form.Item>
              <Form.Item name="externalSpuId" label="offer_id / 外部 SPU">
                <Input placeholder="建议填写，便于后续幂等核对" />
              </Form.Item>
              <Form.Item name="externalUrl" label="Ozon 商品链接">
                <Input placeholder="可选；仅填写已核对的链接" />
              </Form.Item>
              <Form.Item name="platformStatus" label="Ozon 当前状态" rules={[{ required: true, message: '请选择平台状态' }]}>
                <Select
                  options={[
                    { value: 'imported', label: '已接收，尚未确认可售' },
                    { value: 'pending_review', label: '审核中' },
                    { value: 'needs_action', label: '需要修改' },
                    { value: 'sellable', label: '已验证前台可售' },
                  ]}
                />
              </Form.Item>
              {reconcilePlatformStatus === 'sellable' ? (
                <Form.Item name="sellableVerified" valuePropName="checked">
                  <Checkbox>我已在 Ozon 前台或 Seller 后台确认商品当前可售</Checkbox>
                </Form.Item>
              ) : null}
            </>
          ) : null}
          <Form.Item
            name="evidence"
            label="核对依据"
            rules={[
              { required: true, whitespace: true, message: '请填写核对依据' },
              { max: 1000, message: '核对依据不能超过 1000 字' },
            ]}
          >
            <Input.TextArea rows={4} maxLength={1000} showCount placeholder="例如：核对时间、Ozon 页面、使用的 offer_id，以及看到的状态。请勿填写密钥或 Token。" />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}

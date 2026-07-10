import { TmPageContainer } from '@/components/ui';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { aiImageBatchStatusTag } from '@/constants/aiProductImage';
import { fetchAiProductImageBatches, type AIProductImageBatchRow } from '@/services/aiProductImage';
import { AI_IMAGE_WARNING_LABEL, AI_IMAGE_WARNING_CODES } from '@/constants/aiImageWarnings';
import { formatDateTime } from '@/utils/formatTime';
import { normalizeSource, parsePositiveInt } from '@/utils/urlState';
import { Link, history } from '@umijs/max';
import { Alert, Button, Select, Space, Table, Tag } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';

const AI_IMAGE_BATCH_QUERY_KEYS = [
  'page',
  'pageSize',
  'keyword',
  'status',
  'warningCode',
  'platform',
  'shopId',
  'batchId',
  'source',
] as const;

export default function AIImageBatchListPage() {
  const emptyLocale = useListEmptyLocale('aiImageBatches');
  const { state: urlState, setState: setUrlState } =
    useUrlQueryState<Record<(typeof AI_IMAGE_BATCH_QUERY_KEYS)[number], string | undefined>>(
      AI_IMAGE_BATCH_QUERY_KEYS,
    );
  const navSource = normalizeSource(urlState.source);
  const [rows, setRows] = useState<AIProductImageBatchRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const page = parsePositiveInt(urlState.page, 1);
  const pageSize = parsePositiveInt(urlState.pageSize, 20);
  const statusFilter = urlState.status;
  const warningFilter = urlState.warningCode;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchAiProductImageBatches({ page, pageSize });
      let list = res.list;
      if (statusFilter) {
        list = list.filter((row) => row.status === statusFilter);
      }
      if (urlState.batchId) {
        list = list.filter((row) => row.id === urlState.batchId || row.batchNo === urlState.batchId);
      }
      setRows(list);
      setTotal(statusFilter || urlState.batchId ? list.length : res.pagination.total);
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, statusFilter, urlState.batchId, warningFilter]);

  useEffect(() => {
    void load();
  }, [load]);

  const warningOptions = useMemo(
    () => [
      { label: '全部 warning', value: '' },
      ...AI_IMAGE_WARNING_CODES.map((code) => ({
        label: AI_IMAGE_WARNING_LABEL[code] || code,
        value: code,
      })),
    ],
    [],
  );

  return (
    <TmPageContainer
      title="批量图片任务"
      subTitle="批量 AI 图片处理与复核"
      extra={
        <Button type="primary" onClick={() => history.push('/product/drafts')}>
          从商品列表发起
        </Button>
      }
    >
      {navSource ? (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="已从关联页面带入导航上下文（不影响权限与店铺范围）。"
        />
      ) : null}
      {warningFilter ? (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="warning 筛选已写入 URL，进入批次详情后将用于定位相关子项。"
        />
      ) : null}
      <Space wrap style={{ marginBottom: 16 }}>
        <Select
          allowClear
          placeholder="批次状态"
          style={{ minWidth: 160 }}
          value={statusFilter || undefined}
          options={[
            { label: '全部状态', value: '' },
            { label: '运行中', value: 'running' },
            { label: '待复核', value: 'pending_review' },
            { label: '部分成功', value: 'partial_success' },
            { label: '失败', value: 'failed' },
            { label: '已完成', value: 'completed' },
          ]}
          onChange={(v) =>
            setUrlState({
              status: v || undefined,
              page: undefined,
            })
          }
        />
        <Select
          allowClear
          placeholder="质量 warning"
          style={{ minWidth: 200 }}
          options={warningOptions}
          value={warningFilter || undefined}
          onChange={(v) =>
            setUrlState({
              warningCode: v || undefined,
              page: undefined,
            })
          }
        />
      </Space>
      <Table<AIProductImageBatchRow>
        rowKey="id"
        loading={loading}
        dataSource={rows}
        scroll={{ x: 960 }}
        pagination={{
          current: page,
          total,
          pageSize,
          showSizeChanger: true,
          onChange: (nextPage, nextSize) => {
            setUrlState({
              page: nextPage > 1 ? nextPage : undefined,
              pageSize: nextSize !== 20 ? nextSize : undefined,
            });
          },
        }}
        columns={[
          { title: '批次号', dataIndex: 'batchNo', width: 140 },
          {
            title: '状态',
            dataIndex: 'statusLabel',
            width: 100,
            render: (_, row) => {
              const meta = aiImageBatchStatusTag(row.status, row.statusLabel);
              return <Tag color={meta.color}>{meta.text}</Tag>;
            },
          },
          { title: '商品数', dataIndex: 'productCount', width: 80 },
          { title: '图片数', dataIndex: 'imageCount', width: 80 },
          { title: '子项数', dataIndex: 'itemCount', width: 80 },
          { title: '待复核', dataIndex: 'successCount', width: 70 },
          { title: '失败', dataIndex: 'failedCount', width: 70 },
          { title: '已应用', dataIndex: 'appliedCount', width: 80 },
          { title: '创建时间', dataIndex: 'createdAt', width: 170, render: (v) => formatDateTime(v) },
          {
            title: '操作',
            width: 120,
            render: (_, row) => {
              const qs = new URLSearchParams();
              if (navSource) qs.set('source', navSource);
              if (warningFilter) qs.set('warningCode', warningFilter);
              const suffix = qs.toString() ? `?${qs.toString()}` : '';
              return (
                <Space>
                  <Link to={`/product/ai-image-batches/${row.id}${suffix}`}>复核</Link>
                </Space>
              );
            },
          },
        ]}
        locale={emptyLocale}
      />
    </TmPageContainer>
  );
}

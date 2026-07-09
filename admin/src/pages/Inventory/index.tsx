import { type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import InventorySyncDisabledBanner from '@/components/inventory/InventorySyncDisabledBanner';
import {
  INVENTORY_BIND_STATUS,
  INVENTORY_SKU_AMBIGUOUS_MESSAGE,
  INVENTORY_SKU_NOT_BOUND_MESSAGE,
  INVENTORY_STOCK_STATUS,
  INVENTORY_SYNC_STATUS,
  inventoryTagFromMap,
} from '@/constants/inventoryLabels';
import { INVENTORY_COPY, PRODUCT_COPY } from '@/constants/copywriting';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { queryInventoryCenter, type InventoryCenterRow } from '@/services/inventory';
import { Space, Tag, Typography, message } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import { Link } from '@umijs/max';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { useKeywordSearchField } from '@/hooks/useKeywordSearchField';
import KeywordSafetyHint from '@/components/common/KeywordSafetyHint';
import { parsePositiveInt } from '@/utils/urlState';

const INVENTORY_QUERY_KEYS = [
  'page',
  'pageSize',
  'keyword',
  'stockStatus',
  'syncStatus',
  'skuBindStatus',
  'platform',
  'shopId',
  'productSkuId',
  'source',
  'skuId',
] as const;

function tagFrom(raw: string, map: Record<string, { text: string; color: string }>) {
  const cfg = inventoryTagFromMap(raw, map);
  return <Tag color={cfg.color}>{cfg.text}</Tag>;
}

export default function InventoryCenterPage() {
  const emptyLocale = useListEmptyLocale('inventoryCenter', { permissionScoped: true });
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } =
    useUrlQueryState<Record<(typeof INVENTORY_QUERY_KEYS)[number], string | undefined>>(
      INVENTORY_QUERY_KEYS,
    );
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);
  const {
    fieldProps: keywordFieldProps,
    prepareKeyword,
    showSensitiveHint,
  } = useKeywordSearchField({
    setUrlState,
    formRef,
    actionRef,
    setTablePage,
  });

  const skuIdFromUrl = useMemo(() => {
    return urlState.productSkuId || urlState.skuId;
  }, [urlState.productSkuId, urlState.skuId]);

  useEffect(() => {
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
    formRef.current?.setFieldsValue?.({
      keyword: urlState.keyword,
      stockStatus: urlState.stockStatus,
      syncStatus: urlState.syncStatus,
      skuBindStatus: urlState.skuBindStatus,
      platform: urlState.platform,
      shopId: urlState.shopId,
      productSkuId: skuIdFromUrl,
    });
  }, [
    skuIdFromUrl,
    urlState.keyword,
    urlState.page,
    urlState.pageSize,
    urlState.platform,
    urlState.shopId,
    urlState.skuBindStatus,
    urlState.stockStatus,
    urlState.syncStatus,
  ]);

  useEffect(() => {
    if (!skuIdFromUrl) return;
    actionRef.current?.reload?.();
  }, [skuIdFromUrl]);

  const columns: ProColumns<InventoryCenterRow>[] = useMemo(
    () => [
      {
        title: '关键词',
        dataIndex: 'keyword',
        hideInTable: true,
        fieldProps: { placeholder: '商品标题 / 规格编码 / 名称', ...keywordFieldProps },
      },
      { title: '规格 ID', dataIndex: 'productSkuId', hideInTable: true },
      { title: '店铺 ID', dataIndex: 'shopId', hideInTable: true },
      { title: '平台', dataIndex: 'platform', hideInTable: true },
      {
        title: '库存状态',
        dataIndex: 'stockStatus',
        hideInTable: true,
        valueType: 'select',
        valueEnum: Object.fromEntries(Object.entries(INVENTORY_STOCK_STATUS).map(([k, v]) => [k, { text: v.text }])),
      },
      {
        title: INVENTORY_COPY.skuBinding,
        dataIndex: 'skuBindStatus',
        hideInTable: true,
        valueType: 'select',
        valueEnum: Object.fromEntries(Object.entries(INVENTORY_BIND_STATUS).map(([k, v]) => [k, { text: v.text }])),
      },
      {
        title: '同步状态',
        dataIndex: 'syncStatus',
        hideInTable: true,
        valueType: 'select',
        valueEnum: Object.fromEntries(Object.entries(INVENTORY_SYNC_STATUS).map(([k, v]) => [k, { text: v.text }])),
      },
      {
        title: '仅有异常',
        dataIndex: 'hasException',
        hideInTable: true,
        valueType: 'select',
        valueEnum: { true: { text: '是' }, false: { text: '否' } },
      },
      {
        title: '商品',
        dataIndex: 'productTitle',
        width: 180,
        search: false,
        ellipsis: true,
        render: (_, r) => (
          <Link to={`/product/drafts/${r.productId}?tab=inventory`}>{r.productTitle || '—'}</Link>
        ),
      },
      {
        title: PRODUCT_COPY.sku,
        dataIndex: 'skuCode',
        width: 120,
        search: false,
        ellipsis: true,
        render: (_, r) => r.skuCode || '—',
      },
      {
        title: '规格',
        dataIndex: 'skuName',
        width: 120,
        search: false,
        ellipsis: true,
        render: (_, r) => r.skuName || '—',
      },
      { title: '本地库存', dataIndex: 'stock', width: 88, search: false },
      { title: '可用库存', dataIndex: 'availableStock', width: 88, search: false },
      { title: '预警阈值', dataIndex: 'warningStock', width: 88, search: false },
      {
        title: '库存状态',
        dataIndex: 'stockStatus',
        width: 100,
        search: false,
        render: (_, r) => tagFrom(r.stockStatus, INVENTORY_STOCK_STATUS),
      },
      {
        title: INVENTORY_COPY.skuBinding,
        dataIndex: 'skuBindStatus',
        width: 96,
        search: false,
        render: (_, r) => tagFrom(r.skuBindStatus, INVENTORY_BIND_STATUS),
      },
      {
        title: '平台同步',
        dataIndex: 'platformSyncStatus',
        width: 96,
        search: false,
        render: (_, r) => tagFrom(r.platformSyncStatus, INVENTORY_SYNC_STATUS),
      },
      {
        title: '最近扣减',
        dataIndex: 'lastDeductAt',
        width: 156,
        search: false,
        render: (_, r) => (r.lastDeductAt ? formatDateTime(r.lastDeductAt) : '—'),
      },
      {
        title: '最近同步',
        dataIndex: 'lastSyncAt',
        width: 156,
        search: false,
        render: (_, r) => (r.lastSyncAt ? formatDateTime(r.lastSyncAt) : '—'),
      },
      {
        title: '异常',
        dataIndex: 'exceptionCount',
        width: 72,
        search: false,
        render: (_, r) =>
          r.exceptionCount > 0 ? <Tag color="red">{r.exceptionCount}</Tag> : <Tag>0</Tag>,
      },
      {
        title: '操作',
        valueType: 'option',
        width: 280,
        fixed: 'right',
        render: (_, r) => (
          <Space wrap size="small">
            <Link to={`/product/drafts/${r.productId}?tab=inventory`}>查看商品</Link>
            <Link to={`/inventory/deductions?productSkuId=${encodeURIComponent(r.productSkuId)}`}>
              扣减记录
            </Link>
            <Link to={`/inventory/sync-tasks?productSkuId=${encodeURIComponent(r.productSkuId)}`}>
              同步任务
            </Link>
            {r.exceptionCount > 0 ? (
              <Link to={`/ops/task-center/failures?taskType=inventory_sync`}>失败任务</Link>
            ) : null}
          </Space>
        ),
      },
    ],
    [keywordFieldProps],
  );

  return (
    <TmPageContainer
      title="库存中心"
      subTitle="查看本地库存、SKU 绑定与平台同步状态；不自动同步、不自动补货。"
    >
      <InventorySyncDisabledBanner />
      <KeywordSafetyHint visible={showSensitiveHint} />
      <Typography.Paragraph type="secondary">
        {INVENTORY_SKU_NOT_BOUND_MESSAGE}{' '}
        {INVENTORY_SKU_AMBIGUOUS_MESSAGE}
      </Typography.Paragraph>
      <ProTable<InventoryCenterRow>
        rowKey="productSkuId"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        scroll={{ x: 1500 }}
        search={{ labelWidth: 100, defaultCollapsed: false }}
        onReset={() => {
          setTablePage(1);
          setTablePageSize(20);
          clearUrlState(INVENTORY_QUERY_KEYS, { replace: true });
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
        locale={emptyLocale}
        request={async (params) => {
          try {
            const qp = {
              keyword: prepareKeyword(params.keyword),
              productSkuId:
                (params.productSkuId as string | undefined)?.trim() || skuIdFromUrl,
              shopId: (params.shopId as string | undefined)?.trim(),
              platform: (params.platform as string | undefined)?.trim(),
              stockStatus: (params.stockStatus as string | undefined)?.trim(),
              skuBindStatus: (params.skuBindStatus as string | undefined)?.trim(),
              syncStatus: (params.syncStatus as string | undefined)?.trim(),
              page: params.current ?? tablePage,
              pageSize: params.pageSize ?? tablePageSize,
            };
            setUrlState(
              {
                page: Number(qp.page) > 1 ? qp.page : undefined,
                pageSize: Number(qp.pageSize) !== 20 ? qp.pageSize : undefined,
                keyword: qp.keyword,
                productSkuId: qp.productSkuId,
                shopId: qp.shopId,
                platform: qp.platform,
                stockStatus: qp.stockStatus,
                skuBindStatus: qp.skuBindStatus,
                syncStatus: qp.syncStatus,
                source: urlState.source,
              },
              { replace: true },
            );
            const res = await queryInventoryCenter({
              keyword: qp.keyword,
              productSkuId: qp.productSkuId,
              shopId: qp.shopId,
              platform: qp.platform,
              stockStatus: qp.stockStatus,
              skuBindStatus: qp.skuBindStatus,
              syncStatus: qp.syncStatus,
              hasException: params.hasException === 'true' || params.hasException === true,
              page: qp.page,
              pageSize: qp.pageSize,
            });
            return { data: res.list ?? [], success: true, total: res.pagination?.total ?? 0 };
          } catch (e: unknown) {
            message.error((e as Error)?.message || '加载失败');
            return { data: [], success: false, total: 0 };
          }
        }}
      />
    </TmPageContainer>
  );
}

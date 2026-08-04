import { ReloadOutlined, SaveOutlined, SyncOutlined } from '@ant-design/icons';
import { Alert, Button, Empty, Input, Select, Space, Table, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { SectionCard, EmptyState } from '@/components/ui';
import {
  getOzonAttributeMappings,
  getOzonCategoryStats,
  putOzonAttributeMappings,
  queryOzonCategories,
  queryOzonCategoryAttributes,
  syncOzonCategories,
  syncOzonCategoryAttributes,
  type OzonAttributeMapping,
  type OzonCategoryAttribute,
  type OzonCategoryNode,
  type OzonCategoryStats,
} from '@/services/ozonCategories';

const LOCAL_FIELD_OPTIONS = [
  'brand',
  'color',
  'size',
  'material',
  'weight',
  'category',
  'manufacturer',
  'country',
  'model',
  'title',
].map((v) => ({ label: v, value: v }));

function localFieldLabel(v?: string): string {
  const map: Record<string, string> = {
    brand: '品牌',
    color: '颜色',
    size: '尺码/规格',
    material: '材质',
    weight: '重量',
    category: '类目',
    manufacturer: '制造商',
    country: '原产国',
    model: '型号',
    title: '标题',
  };
  return (v && map[v]) || v || '未绑定';
}

/**
 * Ozon 类目 → 属性模板缓存 + 属性映射配置面板。
 * 属性定义不硬编码，首次同步后缓存 24h，映射只针对当前叶子类目。
 */
export default function OzonCategoryPanel() {
  const [stats, setStats] = useState<OzonCategoryStats>();
  const [syncing, setSyncing] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [categories, setCategories] = useState<OzonCategoryNode[]>([]);
  const [selected, setSelected] = useState<OzonCategoryNode | null>(null);
  const [attributes, setAttributes] = useState<OzonCategoryAttribute[]>([]);
  const [mappings, setMappings] = useState<OzonAttributeMapping[]>([]);
  const [loadingAttrs, setLoadingAttrs] = useState(false);
  const [saving, setSaving] = useState(false);

  const loadStats = useCallback(() => {
    getOzonCategoryStats()
      .then(setStats)
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    void loadStats();
  }, [loadStats]);

  const syncCategoryCache = useCallback(async () => {
    setSyncing(true);
    try {
      const next = await syncOzonCategories();
      if (next.stats) setStats(next.stats);
      const runId = next.runId || next.run?.id;
      if (next.run?.status === 'pending' || next.run?.status === 'running') {
        message.success(`Ozon 类目同步任务已创建/处理中${runId ? `（记录 ${runId}）` : ''}`);
      } else {
        message.success(`Ozon 类目同步已返回结果（叶子类目 ${next.stats?.leafCount ?? 0} 个）`);
      }
      void loadStats();
    } catch (e) {
      message.error((e as Error)?.message || '同步类目失败，请先完成 Ozon 店铺授权');
    } finally {
      setSyncing(false);
    }
  }, [loadStats]);

  const searchCategories = useCallback(async () => {
    try {
      const res = await queryOzonCategories({ keyword: keyword.trim() || undefined, onlyLeaf: true, limit: 200 });
      setCategories(res.list ?? []);
    } catch (e) {
      message.error((e as Error)?.message || '查询类目失败');
    }
  }, [keyword]);

  const selectLeaf = useCallback(async (node: OzonCategoryNode) => {
    setSelected(node);
    setAttributes([]);
    setMappings([]);
    setLoadingAttrs(true);
    try {
      const [attrRes, mapRes] = await Promise.all([
        queryOzonCategoryAttributes(node.id),
        getOzonAttributeMappings(node.id),
      ]);
      setAttributes(attrRes.list ?? []);
      const existing = mapRes.list ?? [];
      setMappings(
        (attrRes.list ?? []).map((a) => {
          const hit = existing.find((m) => m.attributeId === a.attrId);
          return hit ?? { attributeId: a.attrId, attributeName: a.name, enabled: true };
        }),
      );
    } catch (e) {
      message.error((e as Error)?.message || '加载属性模板失败');
    } finally {
      setLoadingAttrs(false);
    }
  }, []);

  const syncAttributes = useCallback(async () => {
    if (!selected) return;
    setLoadingAttrs(true);
    try {
      await syncOzonCategoryAttributes(selected.id);
      const [attrRes, mapRes] = await Promise.all([
        queryOzonCategoryAttributes(selected.id),
        getOzonAttributeMappings(selected.id),
      ]);
      setAttributes(attrRes.list ?? []);
      const existing = mapRes.list ?? [];
      setMappings((prev) =>
        (attrRes.list ?? []).map((a) => {
          const hit = existing.find((m) => m.attributeId === a.attrId) ?? prev.find((m) => m.attributeId === a.attrId);
          return hit ?? { attributeId: a.attrId, attributeName: a.name, enabled: true };
        }),
      );
      message.success('属性模板已同步（24h 缓存）');
    } catch (e) {
      message.error((e as Error)?.message || '同步属性模板失败');
    } finally {
      setLoadingAttrs(false);
    }
  }, [selected]);

  const saveMappings = useCallback(async () => {
    if (!selected) return;
    setSaving(true);
    try {
      const res = await putOzonAttributeMappings(selected.id, mappings);
      setMappings(res.list ?? mappings);
      message.success('属性映射已保存');
    } catch (e) {
      message.error((e as Error)?.message || '保存属性映射失败');
    } finally {
      setSaving(false);
    }
  }, [selected, mappings]);

  const updateMapping = useCallback((attrId: string, patch: Partial<OzonAttributeMapping>) => {
    setMappings((prev) => prev.map((m) => (m.attributeId === attrId ? { ...m, ...patch } : m)));
  }, []);

  const dictCount = useMemo(() => attributes.filter((a) => a.dictionaryId || (a.options?.length ?? 0) > 0).length, [attributes]);

  return (
    <SectionCard
      title="Ozon 类目与属性模板"
      description="属性定义不硬编码：首次同步后缓存 24h，再为每个叶子类目配置「Ozon 属性 ↔ 本地字段」映射；字典类属性上品时自动匹配 dictionary_value_id。"
      headerExtra={
        <Space wrap>
          <Button href="/product/publishing-center">进入刊登中心</Button>
          <Button icon={<SyncOutlined />} onClick={() => void syncCategoryCache()} loading={syncing}>
            同步类目树
          </Button>
          <Button icon={<ReloadOutlined />} onClick={() => void searchCategories()}>
            查询类目
          </Button>
        </Space>
      }
    >
      <Alert
        type={stats?.count ? 'success' : 'warning'}
        showIcon
        style={{ marginBottom: 12 }}
        message="Ozon 类目缓存"
        description={
          stats?.count
            ? `已缓存 ${stats.count} 个类目（叶子 ${stats.leafCount ?? 0} 个），最近同步：${stats.lastSyncedAt || '未知'}`
            : '暂无类目数据，请先完成 Ozon 店铺授权，再点击「同步类目树」。'
        }
      />
      <Space style={{ marginBottom: 12 }} wrap>
        <Input.Search
          allowClear
          placeholder="输入类目名或 ID 搜索叶子类目"
          style={{ width: 320 }}
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          onSearch={() => void searchCategories()}
        />
        {selected ? (
          <Tag color="blue">
            {selected.name}（{selected.descriptionCategoryId}:{selected.typeId}）
          </Tag>
        ) : null}
      </Space>
      <Table<OzonCategoryNode>
        rowKey="id"
        size="small"
        dataSource={categories}
        loading={false}
        pagination={{ pageSize: 10, hideOnSinglePage: true }}
        locale={{ emptyText: <Empty description="搜索叶子类目后选择" /> }}
        onRow={(record) => ({
          onClick: () => void selectLeaf(record),
          style: { cursor: 'pointer' },
        })}
        columns={[
          { title: '类目名', dataIndex: 'name', ellipsis: true },
          { title: '类目 ID', dataIndex: 'descriptionCategoryId', width: 130 },
          { title: 'type_id', dataIndex: 'typeId', width: 110 },
          {
            title: '操作',
            width: 80,
            render: (_, r) => (
              <Button type="link" size="small" onClick={(e) => { e.stopPropagation(); void selectLeaf(r); }}>
                选择
              </Button>
            ),
          },
        ]}
      />
      {selected ? (
        <div style={{ marginTop: 12 }}>
          <Space style={{ marginBottom: 8 }} wrap>
            <Button
              icon={<SyncOutlined />}
              onClick={() => void syncAttributes()}
              loading={loadingAttrs}
              disabled={!selected}
            >
              同步属性模板（24h 缓存）
            </Button>
            <Button icon={<SaveOutlined />} onClick={() => void saveMappings()} loading={saving}>
              保存属性映射
            </Button>
            {attributes.length > 0 ? (
              <Typography.Text type="secondary">
                {attributes.length} 个属性（必填 {attributes.filter((a) => a.required).length}，字典 {dictCount}）
              </Typography.Text>
            ) : null}
          </Space>
          <Table<OzonCategoryAttribute>
            rowKey="attrId"
            size="small"
            dataSource={attributes}
            loading={loadingAttrs}
            pagination={false}
            locale={{ emptyText: <EmptyState title="暂无属性模板" description="点击「同步属性模板」从 Ozon 拉取当前类目属性" /> }}
            columns={[
              { title: 'Ozon 属性', dataIndex: 'name', ellipsis: true, width: 180 },
              {
                title: '必填',
                dataIndex: 'required',
                width: 64,
                render: (v: boolean) => (v ? <Tag color="red">必填</Tag> : <Tag>可选</Tag>),
              },
              {
                title: '类型',
                dataIndex: 'valueType',
                width: 90,
                render: (v: string, r) =>
                  r.dictionaryId ? <Tag color="purple">字典</Tag> : v ? <Tag>{v}</Tag> : <Typography.Text type="secondary">—</Typography.Text>,
              },
              {
                title: '本地字段',
                key: 'localField',
                width: 220,
                render: (_, r) => {
                  const m = mappings.find((x) => x.attributeId === r.attrId);
                  return (
                    <Select
                      size="small"
                      allowClear
                      style={{ width: 200 }}
                      placeholder="选择本地字段"
                      value={m?.localField}
                      onChange={(v?: string) => updateMapping(r.attrId, { localField: v || '' })}
                      options={LOCAL_FIELD_OPTIONS}
                      showSearch
                    />
                  );
                },
              },
              {
                title: '映射状态',
                key: 'status',
                width: 120,
                render: (_, r) => {
                  const m = mappings.find((x) => x.attributeId === r.attrId);
                  if (m?.localField) return <Tag color="green">{localFieldLabel(m.localField)}</Tag>;
                  return <Typography.Text type="secondary">未绑定</Typography.Text>;
                },
              },
            ]}
          />
        </div>
      ) : null}
    </SectionCard>
  );
}

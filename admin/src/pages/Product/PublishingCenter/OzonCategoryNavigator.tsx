import { CheckCircleOutlined, SearchOutlined } from "@ant-design/icons";
import {
  Alert,
  Button,
  Collapse,
  Input,
  Select,
  Space,
  Spin,
  Steps,
  Tag,
  Typography,
} from "antd";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  queryOzonCategories,
  type OzonCategoryNode,
} from "@/services/ozonCategories";
import { formatDateTime } from "@/utils/formatTime";

const hierarchyPageSize = 200;
const searchPageSize = 50;

type CategoryLevel = {
  parentId?: string;
  list: OzonCategoryNode[];
  matchedTotal: number;
  loading: boolean;
  error?: string;
};

export type OzonCategoryFocusTarget = {
  categoryId: string;
  requestId: number;
};

type Props = {
  value?: string;
  valuePath?: string;
  disabled?: boolean;
  refreshToken?: number;
  focusTarget?: OzonCategoryFocusTarget;
  onConfirmLeaf: (category: OzonCategoryNode) => Promise<void> | void;
};

function requestError(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) return error.message;
  return fallback;
}

function breadcrumbNode(
  item: NonNullable<OzonCategoryNode["ancestors"]>[number],
  parentId: string | undefined,
  names: string[],
): OzonCategoryNode {
  return {
    id: `breadcrumb:${item.categoryId}`,
    categoryId: item.categoryId,
    parentId,
    name: item.name,
    path: [...names, item.name].join(" / "),
    level: item.level,
    isLeaf: false,
    hasChildren: true,
    childCount: 1,
    status: "active",
  };
}

export default function OzonCategoryNavigator({
  value,
  valuePath,
  disabled,
  refreshToken = 0,
  focusTarget,
  onConfirmLeaf,
}: Props) {
  const [levels, setLevels] = useState<CategoryLevel[]>([]);
  const [trail, setTrail] = useState<OzonCategoryNode[]>([]);
  const [candidate, setCandidate] = useState<OzonCategoryNode>();
  const [loadingPath, setLoadingPath] = useState(false);
  const [applying, setApplying] = useState(false);
  const [pathError, setPathError] = useState<string>();
  const [searchInput, setSearchInput] = useState("");
  const [searchKeyword, setSearchKeyword] = useState("");
  const [searchResults, setSearchResults] = useState<OzonCategoryNode[]>([]);
  const [searchMatchedTotal, setSearchMatchedTotal] = useState(0);
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState<string>();
  const [lastSyncedAt, setLastSyncedAt] = useState<string>();
  const [cacheStale, setCacheStale] = useState(false);
  const pathSequence = useRef(0);
  const searchSequence = useRef(0);

  const captureCacheMeta = useCallback(
    (result: { lastSyncedAt?: string; cacheStale: boolean }) => {
      setLastSyncedAt(result.lastSyncedAt);
      setCacheStale(result.cacheStale === true);
    },
    [],
  );

  const fetchLevel = useCallback(
    async (parentId?: string, offset = 0): Promise<CategoryLevel> => {
      const result = await queryOzonCategories({
        parentId,
        rootOnly: parentId === undefined,
        activeOnly: true,
        limit: hierarchyPageSize,
        offset,
      });
      captureCacheMeta(result);
      return {
        parentId,
        list: result.list || [],
        matchedTotal: result.matchedTotal ?? result.list?.length ?? 0,
        loading: false,
      };
    },
    [captureCacheMeta],
  );

  const loadInitialRoot = useCallback(async () => {
    const sequence = ++pathSequence.current;
    setLevels([{ list: [], matchedTotal: 0, loading: true }]);
    setPathError(undefined);
    try {
      const root = await fetchLevel();
      if (sequence !== pathSequence.current) return;
      setLevels([root]);
    } catch (error) {
      if (sequence !== pathSequence.current) return;
      const message = requestError(error, "Ozon 一级类目加载失败");
      setLevels([
        { list: [], matchedTotal: 0, loading: false, error: message },
      ]);
      setPathError(message);
    }
  }, [fetchLevel]);

  const resolveLeaf = useCallback(
    async (categoryId: string) => {
      const result = await queryOzonCategories({
        keyword: categoryId,
        onlyLeaf: true,
        limit: 20,
      });
      captureCacheMeta(result);
      return (result.list || []).find((item) => item.categoryId === categoryId);
    },
    [captureCacheMeta],
  );

  const focusLeaf = useCallback(
    async (leaf: OzonCategoryNode) => {
      const sequence = ++pathSequence.current;
      setLoadingPath(true);
      setPathError(undefined);
      try {
        const names: string[] = [];
        const chain: OzonCategoryNode[] = [];
        (leaf.ancestors || []).forEach((item) => {
          const parentId = chain.at(-1)?.categoryId;
          const node = breadcrumbNode(item, parentId, names);
          names.push(item.name);
          chain.push(node);
        });
        chain.push(leaf);
        const parentIds: Array<string | undefined> = [
          undefined,
          ...chain.slice(0, -1).map((item) => item.categoryId),
        ];
        const loaded = await Promise.all(
          parentIds.map((parentId) => fetchLevel(parentId)),
        );
        if (sequence !== pathSequence.current) return;
        setTrail(chain);
        setCandidate(leaf);
        setLevels(loaded);
      } catch (error) {
        if (sequence === pathSequence.current)
          setPathError(requestError(error, "无法还原该类目的完整父级路径"));
      } finally {
        if (sequence === pathSequence.current) setLoadingPath(false);
      }
    },
    [fetchLevel],
  );

  const focusByID = useCallback(
    async (categoryId: string) => {
      const leaf = await resolveLeaf(categoryId);
      if (!leaf) {
        setPathError("本地缓存中找不到该 Ozon 叶子类目，请先同步类目缓存");
        return;
      }
      await focusLeaf(leaf);
    },
    [focusLeaf, resolveLeaf],
  );

  useEffect(() => {
    void loadInitialRoot();
  }, [loadInitialRoot, refreshToken]);

  useEffect(() => {
    if (!value) {
      setTrail([]);
      setCandidate(undefined);
      return;
    }
    void focusByID(value);
  }, [focusByID, value]);

  useEffect(() => {
    if (!focusTarget?.categoryId) return;
    void focusByID(focusTarget.categoryId);
  }, [focusByID, focusTarget?.categoryId, focusTarget?.requestId]);

  const selectLevel = async (levelIndex: number, categoryId: string) => {
    const node = levels[levelIndex]?.list.find(
      (item) => item.categoryId === categoryId,
    );
    if (!node) return;
    pathSequence.current++;
    setTrail((current) => [...current.slice(0, levelIndex), node]);
    setLevels((current) => current.slice(0, levelIndex + 1));
    setCandidate(node.isLeaf ? node : undefined);
    setPathError(undefined);
    if (node.isLeaf) return;
    const nextIndex = levelIndex + 1;
    setLevels((current) => [
      ...current,
      {
        parentId: node.categoryId,
        list: [],
        matchedTotal: 0,
        loading: true,
      },
    ]);
    try {
      const next = await fetchLevel(node.categoryId);
      setLevels((current) => [...current.slice(0, nextIndex), next]);
      if (next.matchedTotal === 0)
        setPathError("该父类目下没有启用中的子类目，请返回上一级重新选择");
    } catch (error) {
      const message = requestError(error, "下一级 Ozon 类目加载失败");
      setLevels((current) => [
        ...current.slice(0, nextIndex),
        {
          parentId: node.categoryId,
          list: [],
          matchedTotal: 0,
          loading: false,
          error: message,
        },
      ]);
      setPathError(message);
    }
  };

  const loadMoreLevel = async (levelIndex: number) => {
    const currentLevel = levels[levelIndex];
    if (!currentLevel || currentLevel.loading) return;
    setLevels((current) =>
      current.map((item, index) =>
        index === levelIndex ? { ...item, loading: true } : item,
      ),
    );
    try {
      const next = await fetchLevel(
        currentLevel.parentId,
        currentLevel.list.length,
      );
      setLevels((current) =>
        current.map((item, index) =>
          index === levelIndex
            ? {
                ...next,
                list: [
                  ...item.list,
                  ...next.list.filter(
                    (row) =>
                      !item.list.some(
                        (existing) => existing.categoryId === row.categoryId,
                      ),
                  ),
                ],
              }
            : item,
        ),
      );
    } catch (error) {
      setLevels((current) =>
        current.map((item, index) =>
          index === levelIndex
            ? {
                ...item,
                loading: false,
                error: requestError(error, "更多子类目加载失败"),
              }
            : item,
        ),
      );
    }
  };

  const search = async (keyword: string, offset = 0) => {
    const normalized = keyword.trim();
    if (!normalized) {
      setSearchKeyword("");
      setSearchResults([]);
      setSearchMatchedTotal(0);
      setSearchError(undefined);
      return;
    }
    const sequence = ++searchSequence.current;
    setSearching(true);
    setSearchError(undefined);
    try {
      const result = await queryOzonCategories({
        keyword: normalized,
        onlyLeaf: true,
        activeOnly: true,
        limit: searchPageSize,
        offset,
      });
      if (sequence !== searchSequence.current) return;
      captureCacheMeta(result);
      setSearchKeyword(normalized);
      setSearchMatchedTotal(result.matchedTotal ?? result.list?.length ?? 0);
      setSearchResults((current) =>
        offset === 0
          ? result.list || []
          : [
              ...current,
              ...(result.list || []).filter(
                (row) =>
                  !current.some(
                    (existing) => existing.categoryId === row.categoryId,
                  ),
              ),
            ],
      );
    } catch (error) {
      if (sequence === searchSequence.current)
        setSearchError(requestError(error, "Ozon 类目搜索失败"));
    } finally {
      if (sequence === searchSequence.current) setSearching(false);
    }
  };

  const stepItems = useMemo(() => {
    const selected = trail.map((item, index) => ({
      title: `第 ${index + 1} 级`,
      description: item.name,
    }));
    if (!candidate)
      selected.push({
        title: `第 ${trail.length + 1} 级`,
        description: "请选择",
      });
    else
      selected.push({
        title: "确认叶子类目",
        description: candidate.categoryId === value ? "已应用" : "待确认",
      });
    return selected;
  }, [candidate, trail, value]);

  const confirmCandidate = async () => {
    if (!candidate || candidate.status !== "active" || !candidate.isLeaf)
      return;
    setApplying(true);
    try {
      await onConfirmLeaf(candidate);
    } finally {
      setApplying(false);
    }
  };

  return (
    <div className="ozon-category-navigator">
      <div className="ozon-category-navigator__heading">
        <div>
          <Typography.Text strong>从父类目开始逐级选择</Typography.Text>
          <Typography.Paragraph type="secondary">
            类目层级数量由 Ozon
            决定；只有到达启用中的叶子类目后才能加载属性模板。
          </Typography.Paragraph>
        </div>
        <Tag color={!lastSyncedAt ? "red" : cacheStale ? "orange" : "green"}>
          {!lastSyncedAt
            ? "类目缓存：从未同步"
            : `类目缓存：${formatDateTime(lastSyncedAt)}${cacheStale ? "（已过期）" : "（有效）"}`}
        </Tag>
      </div>

      <Steps
        size="small"
        responsive
        current={Math.max(stepItems.length - 1, 0)}
        items={stepItems}
      />

      <div className="ozon-category-navigator__search">
        <Input.Search
          value={searchInput}
          allowClear
          enterButton={
            <>
              <SearchOutlined /> 搜索完整路径
            </>
          }
          placeholder="辅助定位：输入父级名称、完整路径或类目 ID"
          loading={searching}
          disabled={disabled}
          onChange={(event) => setSearchInput(event.target.value)}
          onSearch={(keyword) => void search(keyword, 0)}
        />
        {searchKeyword || searchError ? (
          <Space direction="vertical" size={8} style={{ width: "100%" }}>
            <Typography.Text type={searchError ? "danger" : "secondary"}>
              {searchError ||
                `搜索“${searchKeyword}”：已展示 ${searchResults.length} 条，共匹配 ${searchMatchedTotal} 个启用叶子类目`}
            </Typography.Text>
            {searchResults.length ? (
              <Select
                aria-label="类目搜索定位结果"
                showSearch
                optionFilterProp="label"
                placeholder="选择搜索结果以还原父级路径（不会直接确认）"
                options={searchResults.map((item) => ({
                  value: item.categoryId,
                  label: `${item.path || item.name}（ID：${item.categoryId}）`,
                }))}
                disabled={disabled}
                onChange={(categoryId) => {
                  const item = searchResults.find(
                    (row) => row.categoryId === categoryId,
                  );
                  if (item) void focusLeaf(item);
                }}
              />
            ) : null}
            {searchResults.length < searchMatchedTotal ? (
              <Button
                size="small"
                loading={searching}
                disabled={disabled}
                onClick={() => void search(searchKeyword, searchResults.length)}
              >
                加载更多搜索结果
              </Button>
            ) : null}
          </Space>
        ) : null}
      </div>

      <Spin spinning={loadingPath} tip="正在还原完整父级路径">
        <div className="ozon-category-navigator__levels">
          {levels.map((level, index) => (
            <div
              className="ozon-category-navigator__level"
              key={`${level.parentId || "root"}:${index}`}
            >
              <Space wrap size={6}>
                <Tag color="blue">第 {index + 1} 级</Tag>
                <Typography.Text strong>
                  {index === 0
                    ? "Ozon 一级类目"
                    : `${trail[index - 1]?.name || "上一级"}的子类目`}
                </Typography.Text>
              </Space>
              <Select
                aria-label={
                  index === 0 ? "Ozon 一级类目" : `Ozon 第 ${index + 1} 级类目`
                }
                showSearch
                optionFilterProp="label"
                value={trail[index]?.categoryId}
                loading={level.loading}
                disabled={disabled || level.loading}
                placeholder={level.error || `请选择第 ${index + 1} 级类目`}
                options={level.list.map((item) => ({
                  value: item.categoryId,
                  label: `${item.name}${item.isLeaf ? "（叶子类目）" : `（${item.childCount} 个子类目）`}`,
                  disabled: item.status !== "active",
                }))}
                onChange={(categoryId) => void selectLevel(index, categoryId)}
              />
              <Typography.Text type={level.error ? "danger" : "secondary"}>
                {level.error ||
                  `已加载 ${level.list.length}/${level.matchedTotal} 个启用类目`}
              </Typography.Text>
              {level.list.length < level.matchedTotal ? (
                <Button
                  size="small"
                  loading={level.loading}
                  disabled={disabled}
                  onClick={() => void loadMoreLevel(index)}
                >
                  加载更多本级类目
                </Button>
              ) : null}
            </div>
          ))}
        </div>
      </Spin>

      {pathError ? <Alert type="error" showIcon message={pathError} /> : null}
      {candidate ? (
        <Alert
          type={candidate.categoryId === value ? "success" : "info"}
          showIcon
          icon={<CheckCircleOutlined />}
          message={
            candidate.categoryId === value
              ? "当前叶子类目已经应用"
              : "已定位叶子类目，尚未应用"
          }
          description={
            <Space direction="vertical" size={4}>
              <span>{candidate.path || candidate.name}</span>
              <Collapse
                ghost
                size="small"
                items={[
                  {
                    key: "category-technical-info",
                    label: "技术信息",
                    children: (
                      <Typography.Text type="secondary">
                        description_category_id：
                        {candidate.descriptionCategoryId || "—"}；type_id：
                        {candidate.typeId || "—"}
                      </Typography.Text>
                    ),
                  },
                ]}
              />
              {value && candidate.categoryId !== value ? (
                <span>
                  当前已应用：{valuePath || value}
                  。确认新类目后才会清空旧模板字段。
                </span>
              ) : null}
            </Space>
          }
          action={
            candidate.categoryId !== value ? (
              <Button
                type="primary"
                loading={applying}
                disabled={disabled || candidate.status !== "active"}
                onClick={() => void confirmCandidate()}
              >
                确认此叶子类目并加载模板
              </Button>
            ) : null
          }
        />
      ) : (
        <Alert
          type="info"
          showIcon
          message="请从一级父类目开始"
          description="每次只显示当前父类目下的直接子类目；未到达叶子类目时不会写入商品配置。"
        />
      )}
    </div>
  );
}

import {
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  RobotOutlined,
} from "@ant-design/icons";
import {
  Alert,
  Button,
  Card,
  Collapse,
  Empty,
  Progress,
  Space,
  Steps,
  Tag,
  Typography,
} from "antd";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  recommendOzonProductCategories,
  type OzonCategoryDifferenceDimension,
  type OzonProductCategoryRecommendation,
  type OzonProductCategoryRecommendationCandidate,
} from "@/services/ozonPublish";
import { formatDateTime } from "@/utils/formatTime";
import "./AICategoryRecommendationPanel.less";

type Props = {
  productId?: string;
  shopId?: string;
  disabled?: boolean;
  readOnly?: boolean;
  onApply: (
    candidate: OzonProductCategoryRecommendationCandidate,
  ) => void | Promise<void>;
};

const stageItems = [
  { title: "分析商品规格" },
  { title: "搜索叶子类目" },
  { title: "比较模板" },
  { title: "完成" },
];

const statusMessages: Record<
  OzonProductCategoryRecommendation["status"],
  { type: "success" | "warning" | "info" | "error"; title: string }
> = {
  ready: { type: "success", title: "已完成规格分析与类目比较" },
  partial: { type: "warning", title: "已返回部分可核对结果" },
  no_match: { type: "info", title: "暂未找到可用候选" },
  ai_unavailable: { type: "error", title: "AI 分析暂不可用" },
  category_cache_empty: { type: "warning", title: "Ozon 类目缓存为空" },
};

const strategyLabels: Record<string, string> = {
  group_all: "全部 SKU 合并刊登",
  group_subset: "仅兼容 SKU 子集可合并",
  split_single_sku: "建议拆分为单 SKU",
  manual_review: "需要人工复核",
};

function percent(value?: number) {
  return Math.round(Math.max(0, Math.min(1, Number(value) || 0)) * 100);
}

function DimensionEvidence({
  dimensions,
}: {
  dimensions: OzonCategoryDifferenceDimension[];
}) {
  if (!dimensions.length) {
    return (
      <Typography.Text type="secondary">
        当前选择范围内未识别到可证明的 SKU 区别维度。
      </Typography.Text>
    );
  }
  return (
    <div className="ai-category-recommendation__evidence-list">
      {dimensions.map((dimension) => (
        <div
          className="ai-category-recommendation__evidence-item"
          key={dimension.key}
        >
          <Space wrap size={[6, 6]}>
            <Typography.Text strong>{dimension.name}</Typography.Text>
            <Tag>{dimension.semantic}</Tag>
            <Tag color="blue">置信度 {percent(dimension.confidence)}%</Tag>
          </Space>
          <div className="ai-category-recommendation__raw-values">
            {dimension.evidence.map((evidence, index) => (
              <Tag key={`${evidence.skuId}-${evidence.sourceKey}-${index}`}>
                {evidence.skuCode || evidence.skuId}：{evidence.sourceKey} ={" "}
                {evidence.rawValue}
              </Tag>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

function CandidateCard({
  candidate,
  rank,
  readOnly,
  applying,
  onApply,
}: {
  candidate: OzonProductCategoryRecommendationCandidate;
  rank: number;
  readOnly?: boolean;
  applying: boolean;
  onApply: () => void;
}) {
  const matched = new Map(
    candidate.matchedDimensions.map((item) => [item.sourceDimensionKey, item]),
  );
  const unmatched = new Map(
    candidate.unmatchedDimensions.map((item) => [
      item.sourceDimensionKey,
      item,
    ]),
  );
  const dimensionKeys = Array.from(
    new Set([...matched.keys(), ...unmatched.keys()]),
  );
  return (
    <Card
      className="ai-category-recommendation__candidate"
      title={
        <div className="ai-category-recommendation__candidate-title">
          <Space wrap>
            <Tag color={rank === 1 ? "gold" : "default"}>候选 {rank}</Tag>
            <Tag color={candidate.approximate ? "orange" : "green"}>
              {candidate.approximate ? "近似类目" : "精准类目"}
            </Tag>
          </Space>
          <Typography.Text strong>{candidate.categoryPath}</Typography.Text>
        </div>
      }
      extra={
        <Button
          type={rank === 1 ? "primary" : "default"}
          disabled={readOnly}
          loading={applying}
          onClick={onApply}
        >
          应用此类目
        </Button>
      }
    >
      <div className="ai-category-recommendation__metrics">
        <div>
          <Typography.Text type="secondary">综合评分</Typography.Text>
          <Typography.Title level={4}>{candidate.score}</Typography.Title>
        </div>
        <div>
          <Typography.Text type="secondary">推荐置信度</Typography.Text>
          <Progress
            percent={percent(candidate.confidence)}
            size="small"
            status="normal"
          />
        </div>
        <div>
          <Typography.Text type="secondary">SKU 区别覆盖</Typography.Text>
          <Progress
            percent={percent(candidate.variantCoverage.ratio)}
            size="small"
            status={
              candidate.variantCoverage.ratio < 1 ? "exception" : "success"
            }
          />
        </div>
        <div>
          <Typography.Text type="secondary">必填属性预计完成度</Typography.Text>
          <Progress
            percent={percent(candidate.requiredCoverage.ratio)}
            size="small"
          />
        </div>
      </div>
      <Space direction="vertical" size={10} style={{ width: "100%" }}>
        <Alert
          type={
            ["split_single_sku", "manual_review"].includes(
              candidate.listingStrategy,
            )
              ? "warning"
              : "info"
          }
          showIcon
          message={
            strategyLabels[candidate.listingStrategy] ||
            candidate.listingStrategy
          }
        />
        {candidate.reasons.length ? (
          <Typography.Text>{candidate.reasons.join("；")}</Typography.Text>
        ) : null}
        {candidate.warnings.map((warning) => (
          <Alert key={warning} type="warning" message={warning} showIcon />
        ))}
        <Collapse
          size="small"
          items={[
            {
              key: "comparison",
              label: `属性对照（已映射 ${candidate.matchedDimensions.length}，未映射 ${candidate.unmatchedDimensions.length}）`,
              children: dimensionKeys.length ? (
                <div className="ai-category-recommendation__comparison">
                  {dimensionKeys.map((key) => {
                    const mapped = matched.get(key);
                    const missing = unmatched.get(key);
                    return (
                      <div key={key}>
                        <Typography.Text strong>
                          {mapped?.sourceDimensionName ||
                            missing?.sourceDimensionName ||
                            key}
                        </Typography.Text>
                        {mapped ? (
                          <Space wrap>
                            <span>→ {mapped.targetAttributeName}</span>
                            <Tag color="green">is_aspect=true</Tag>
                            <Tag color="green">资格已知</Tag>
                          </Space>
                        ) : (
                          <Typography.Text type="warning">
                            {missing?.reason || "无法安全映射"}
                          </Typography.Text>
                        )}
                      </div>
                    );
                  })}
                </div>
              ) : (
                <Typography.Text type="secondary">
                  当前没有需要对照的 SKU 区别维度。
                </Typography.Text>
              ),
            },
            {
              key: "technical",
              label: "模板与技术详情",
              children: (
                <Space direction="vertical" size={4}>
                  <Typography.Text type="secondary">
                    类目 ID：{candidate.categoryId}
                  </Typography.Text>
                  <Typography.Text type="secondary">
                    模板摘要：{candidate.schemaHash || "未提供"}
                  </Typography.Text>
                  <Typography.Text type="secondary">
                    模板时间：
                    {candidate.templateSyncedAt
                      ? formatDateTime(candidate.templateSyncedAt)
                      : "未知"}
                  </Typography.Text>
                </Space>
              ),
            },
          ]}
        />
      </Space>
    </Card>
  );
}

export default function AICategoryRecommendationPanel({
  productId,
  shopId,
  disabled,
  readOnly,
  onApply,
}: Props) {
  const [result, setResult] = useState<OzonProductCategoryRecommendation>();
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [stage, setStage] = useState(0);
  const [applyingCategoryId, setApplyingCategoryId] = useState<string>();
  const requestSequence = useRef(0);
  const analysisInFlight = useRef(false);
  const applicationSequence = useRef(0);
  const applicationInFlight = useRef(false);
  const contextKey = `${productId || ""}:${shopId || ""}`;

  useEffect(() => {
    requestSequence.current += 1;
    applicationSequence.current += 1;
    analysisInFlight.current = false;
    applicationInFlight.current = false;
    setResult(undefined);
    setError(undefined);
    setLoading(false);
    setStage(0);
    setApplyingCategoryId(undefined);
  }, [contextKey]);

  useEffect(() => {
    if (!loading) return undefined;
    const timer = globalThis.setInterval(() => {
      setStage((current) => Math.min(current + 1, 2));
    }, 700);
    return () => globalThis.clearInterval(timer);
  }, [loading]);

  const status = result ? statusMessages[result.status] : undefined;
  const canAnalyze = Boolean(productId && shopId && !disabled && !readOnly);
  const summary = useMemo(() => {
    if (!result) return "";
    const source = result.sourceSummary;
    return `已分析 ${source.selectedSkuCount}/${source.skuCount} 个 SKU；主要依据：采集 SKU 分类属性`;
  }, [result]);

  const analyze = async () => {
    if (
      !productId ||
      !shopId ||
      loading ||
      analysisInFlight.current ||
      !canAnalyze
    )
      return;
    const sequence = ++requestSequence.current;
    const requestedContext = contextKey;
    analysisInFlight.current = true;
    setLoading(true);
    setStage(0);
    setError(undefined);
    setResult(undefined);
    try {
      const next = await recommendOzonProductCategories(productId, {
        shopId,
        skuIds: [],
        refreshPolicy: "if_missing_or_stale",
      });
      if (
        sequence !== requestSequence.current ||
        requestedContext !== `${productId}:${shopId}`
      )
        return;
      setResult(next);
      setStage(3);
    } catch (requestError) {
      if (sequence !== requestSequence.current) return;
      setError(
        requestError instanceof Error
          ? requestError.message
          : "AI 类目推荐请求失败",
      );
    } finally {
      if (sequence === requestSequence.current) {
        analysisInFlight.current = false;
        setLoading(false);
      }
    }
  };

  const applyCandidate = async (
    candidate: OzonProductCategoryRecommendationCandidate,
  ) => {
    if (
      readOnly ||
      disabled ||
      applyingCategoryId ||
      applicationInFlight.current
    )
      return;
    const sequence = ++applicationSequence.current;
    applicationInFlight.current = true;
    setApplyingCategoryId(candidate.categoryId);
    try {
      await onApply(candidate);
    } finally {
      if (sequence === applicationSequence.current) {
        applicationInFlight.current = false;
        setApplyingCategoryId(undefined);
      }
    }
  };

  return (
    <section
      className="ai-category-recommendation"
      aria-label="AI 辅助选择 Ozon 类目"
    >
      <div className="ai-category-recommendation__heading">
        <div>
          <Space>
            <RobotOutlined />
            <Typography.Text strong>AI 辅助选择 Ozon 类目</Typography.Text>
          </Space>
          <Typography.Paragraph type="secondary">
            读取已入库的 SKU 分类属性，最多推荐 3
            个真实叶子类目。应用候选只修改当前未保存表单，不会保存、预检或提交到
            Ozon。
          </Typography.Paragraph>
        </div>
        <Button
          type="primary"
          icon={<RobotOutlined />}
          aria-label="AI 分析 SKU 并推荐类目"
          loading={loading}
          disabled={!canAnalyze || loading}
          onClick={() => void analyze()}
        >
          AI 分析 SKU 并推荐类目
        </Button>
      </div>
      {!productId || !shopId ? (
        <Alert type="info" showIcon message="请先选择商品和已授权 Ozon 店铺" />
      ) : readOnly ? (
        <Alert
          type="warning"
          showIcon
          message="当前账号为只读权限，不能发起 AI 推荐或应用候选"
        />
      ) : null}
      {loading ? (
        <div className="ai-category-recommendation__stages">
          <Steps size="small" current={stage} items={stageItems} responsive />
          <Typography.Text type="secondary">
            正在使用采集 SKU 属性分析区别并核对真实 Ozon 模板，请稍候…
          </Typography.Text>
        </div>
      ) : null}
      {error ? (
        <Alert
          type="error"
          showIcon
          message="AI 类目推荐请求失败"
          description={error}
          action={
            <Button
              size="small"
              aria-label="重试"
              onClick={() => void analyze()}
            >
              重试
            </Button>
          }
        />
      ) : null}
      {result && status ? (
        <Space direction="vertical" size={12} style={{ width: "100%" }}>
          <Alert
            type={status.type}
            showIcon
            icon={
              result.status === "ready" ? (
                <CheckCircleOutlined />
              ) : result.status === "partial" ? (
                <ExclamationCircleOutlined />
              ) : undefined
            }
            message={status.title}
            description={summary}
          />
          {result.warnings.map((warning) => (
            <Alert key={warning} type="warning" showIcon message={warning} />
          ))}
          <Collapse
            size="small"
            items={[
              {
                key: "source-evidence",
                label: `规格区别证据（${result.differenceDimensions.length} 个维度）`,
                children: (
                  <DimensionEvidence dimensions={result.differenceDimensions} />
                ),
              },
            ]}
          />
          {result.anomalies.map((anomaly) => (
            <Alert
              key={`${anomaly.type}-${anomaly.skuIds.join("-")}`}
              type="warning"
              showIcon
              message={anomaly.message}
              description={`异常 SKU：${anomaly.skuIds.join("、")}`}
            />
          ))}
          {result.candidates.length ? (
            <div className="ai-category-recommendation__candidates">
              {result.candidates.map((candidate, index) => (
                <CandidateCard
                  key={candidate.categoryId}
                  candidate={candidate}
                  rank={index + 1}
                  readOnly={readOnly || disabled}
                  applying={applyingCategoryId === candidate.categoryId}
                  onApply={() => void applyCandidate(candidate)}
                />
              ))}
            </div>
          ) : (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="没有可应用的 AI 候选；人工类目导航仍可继续使用"
            />
          )}
        </Space>
      ) : null}
    </section>
  );
}

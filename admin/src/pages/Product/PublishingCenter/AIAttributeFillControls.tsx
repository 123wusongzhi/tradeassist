import { RobotOutlined, UndoOutlined } from "@ant-design/icons";
import { Alert, Button, Space, Tag, Typography } from "antd";
import { useEffect, useRef, useState } from "react";
import {
  suggestOzonAttributes,
  type OzonAttributeSuggestionCurrentValues,
  type OzonAttributeSuggestionResult,
} from "@/services/ozonPublish";

export type OzonAIAttributeRequestContext = {
  productId: string;
  shopId: string;
  categoryId: string;
  templateFingerprint: string;
  generation: number;
};

export type OzonAIAttributeApplyFeedback = {
  filled: number;
  requiresReview: number;
  notFound: number;
  partial?: boolean;
  details?: string[];
};

type Props = {
  context?: OzonAIAttributeRequestContext;
  readOnly?: boolean;
  disabled?: boolean;
  disabledReason?: string;
  canUndo?: boolean;
  getCurrentValues: () => OzonAttributeSuggestionCurrentValues;
  onApplyResult: (
    result: OzonAttributeSuggestionResult,
    snapshot: OzonAttributeSuggestionCurrentValues,
    requestedContext: OzonAIAttributeRequestContext,
  ) => OzonAIAttributeApplyFeedback;
  onUndo: () => void;
};

function contextKey(context?: OzonAIAttributeRequestContext) {
  if (!context) return "";
  return [
    context.productId,
    context.shopId,
    context.categoryId,
    context.templateFingerprint,
    context.generation,
  ].join("\n");
}

function responseMatchesContext(
  result: OzonAttributeSuggestionResult,
  requested: OzonAIAttributeRequestContext,
) {
  const response = result.context;
  return Boolean(
    response &&
    response.fingerprint &&
    response.productId === requested.productId &&
    response.shopId === requested.shopId &&
    response.categoryId === requested.categoryId &&
    response.templateFingerprint === requested.templateFingerprint,
  );
}

export default function AIAttributeFillControls({
  context,
  readOnly,
  disabled,
  disabledReason,
  canUndo,
  getCurrentValues,
  onApplyResult,
  onUndo,
}: Props) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [feedback, setFeedback] = useState<OzonAIAttributeApplyFeedback>();
  const requestSequence = useRef(0);
  const requestInFlight = useRef(false);
  const currentContextKey = contextKey(context);
  const currentContextKeyRef = useRef(currentContextKey);
  const requestAllowedRef = useRef(!readOnly && !disabled);
  currentContextKeyRef.current = currentContextKey;
  requestAllowedRef.current = !readOnly && !disabled;

  useEffect(() => {
    requestSequence.current += 1;
    requestInFlight.current = false;
    setLoading(false);
    setError(undefined);
    setFeedback(undefined);
  }, [currentContextKey]);

  useEffect(() => {
    if (!readOnly && !disabled) return;
    requestSequence.current += 1;
    requestInFlight.current = false;
    setLoading(false);
  }, [disabled, readOnly]);

  const canRequest = Boolean(
    context && !readOnly && !disabled && !loading && !requestInFlight.current,
  );

  const requestSuggestions = async () => {
    if (!context || !canRequest || requestInFlight.current) return;
    const sequence = ++requestSequence.current;
    const requestedContext = { ...context };
    const requestedContextKey = contextKey(requestedContext);
    const snapshot = getCurrentValues();
    requestInFlight.current = true;
    setLoading(true);
    setError(undefined);
    setFeedback(undefined);
    try {
      const result = await suggestOzonAttributes(requestedContext.productId, {
        shopId: requestedContext.shopId,
        categoryId: requestedContext.categoryId,
        templateFingerprint: requestedContext.templateFingerprint,
        currentValues: snapshot,
      });
      if (
        sequence !== requestSequence.current ||
        requestedContextKey !== currentContextKeyRef.current ||
        !requestAllowedRef.current ||
        !responseMatchesContext(result, requestedContext)
      )
        return;
      setFeedback(onApplyResult(result, snapshot, requestedContext));
    } catch (requestError) {
      if (
        sequence !== requestSequence.current ||
        requestedContextKey !== currentContextKeyRef.current ||
        !requestAllowedRef.current
      )
        return;
      setError(
        requestError instanceof Error
          ? requestError.message
          : "AI 填写失败，现有输入未变更",
      );
    } finally {
      if (sequence === requestSequence.current) {
        requestInFlight.current = false;
        setLoading(false);
      }
    }
  };

  const feedbackType =
    feedback &&
    feedback.filled > 0 &&
    feedback.notFound === 0 &&
    !feedback.partial
      ? "success"
      : feedback && feedback.filled > 0
        ? "warning"
        : "info";

  return (
    <section
      className="publishing-center__attribute-ai"
      aria-label="Ozon AI 属性填写"
    >
      <div className="publishing-center__attribute-ai-actions">
        <div>
          <Typography.Text strong>AI 辅助填写</Typography.Text>
          <Typography.Text type="secondary">
            会结合商品文字、代表
            SKU、图片和类目主动推断；仅填写普通商品级空白属性，所有建议请审核，不会自动保存、预检或提交
            Ozon。
          </Typography.Text>
        </div>
        <Space wrap className="publishing-center__attribute-ai-buttons">
          <Button
            aria-label="AI 填写空白项"
            icon={<RobotOutlined />}
            loading={loading}
            disabled={!canRequest}
            onClick={() => void requestSuggestions()}
          >
            AI 填写空白项
          </Button>
          {canUndo ? (
            <Button
              aria-label="撤销本次 AI 填写"
              icon={<UndoOutlined />}
              disabled={loading || readOnly || disabled}
              onClick={() => {
                onUndo();
                setError(undefined);
                setFeedback(undefined);
              }}
            >
              撤销本次 AI 填写
            </Button>
          ) : null}
        </Space>
      </div>
      {readOnly ? (
        <Alert
          type="warning"
          showIcon
          message="当前账号没有商品或店铺编辑权限，不能发起或回填 AI 属性建议"
        />
      ) : disabledReason ? (
        <Alert type="info" showIcon message={disabledReason} />
      ) : null}
      {loading ? (
        <Alert
          type="info"
          showIcon
          message="正在分析商品信息并校验当前 Ozon 属性模板…"
        />
      ) : null}
      {error ? (
        <Alert
          type="error"
          showIcon
          message="AI 填写失败，现有输入未变更"
          description={`${error} 系统不会自动重试；如需重试，请再次点击“AI 填写空白项”。`}
        />
      ) : null}
      {feedback ? (
        <Alert
          type={feedbackType}
          showIcon
          message={
            feedback.filled > 0
              ? feedback.notFound > 0 || feedback.partial
                ? "AI 已部分填写空白项"
                : "AI 已填写空白项"
              : "AI 未返回可填写的空白项"
          }
          description={
            <Space direction="vertical" size={6} style={{ width: "100%" }}>
              <Space
                wrap
                size={[6, 6]}
                aria-label="AI 属性填写结果"
                aria-live="polite"
              >
                <Tag color="success">已填写 {feedback.filled}</Tag>
                <Tag color="warning">建议核对 {feedback.requiresReview}</Tag>
                <Tag>未找到 {feedback.notFound}</Tag>
              </Space>
              {feedback.details?.length ? (
                <Typography.Text type="secondary">
                  {feedback.details.slice(0, 3).join("；")}
                </Typography.Text>
              ) : null}
            </Space>
          }
        />
      ) : null}
    </section>
  );
}

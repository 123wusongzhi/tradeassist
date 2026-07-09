import { TechnicalDetails } from '@/components/ui';
import { publishCapabilityLabel } from '@/constants/publishLabels';
import { Alert } from 'antd';

type Props = {
  capability?: string;
  blockedByCredentials?: boolean;
  showRcBadge?: boolean;
};

/** 刊登 / 发布边界：local_draft_only ≠ 真实平台成功 */
export default function PublishBoundaryBanner({
  capability,
  blockedByCredentials,
  showRcBadge = true,
}: Props) {
  const cap = (capability || '').trim().toLowerCase();
  const isLocalOnly = cap === 'local_draft_only';
  const isRealDraft = cap === 'real_draft_create';

  if (!isLocalOnly && !isRealDraft && !blockedByCredentials) return null;

  let message = '创建本地草稿不等于平台真实草稿；抖店当前仍为发布候选。';
  if (isLocalOnly) {
    message = `${publishCapabilityLabel('local_draft_only')}：仅在 TradeMind 内保存草稿记录，不会调用平台写接口。`;
  } else if (isRealDraft) {
    message =
      '创建抖店草稿不等于商品上架；需人工在平台后台确认并提交，真实 E2E 未执行或被凭证阻塞时请勿视为已上线。';
  } else if (blockedByCredentials) {
    message = '真实抖店 E2E 被凭证阻塞（需真实环境凭证），当前结果仅供 Demo 与前置检查参考。';
  }

  return (
    <Alert
      type="info"
      showIcon
      style={{ marginBottom: 16 }}
      message={
        <>
          {message}
          {showRcBadge ? ' （抖店发布候选）' : null}
        </>
      }
      description={
        <TechnicalDetails label="技术详情">
          <div>
            能力标识={cap || '—'}；真实凭证阻塞={blockedByCredentials ? '是' : '否'}
          </div>
        </TechnicalDetails>
      }
    />
  );
}

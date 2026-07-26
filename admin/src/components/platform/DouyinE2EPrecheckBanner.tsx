import { history } from '@umijs/max';
import { Alert, Button, Space } from 'antd';

type Props = {
  blockedByCredentials?: boolean;
  releaseCandidate?: boolean;
  compact?: boolean;
};

/** 抖店 E2E 前置提示：不执行真实 E2E，仅说明边界与下一步 */
export default function DouyinE2EPrecheckBanner({
  blockedByCredentials = true,
  releaseCandidate = true,
  compact = false,
}: Props) {
  if (!blockedByCredentials && !releaseCandidate) return null;

  const message = blockedByCredentials
    ? '当前未配置抖店真实凭证，系统只能完成本地 Demo 与前置检查，不能执行真实抖店 E2E。'
    : '抖店仍为发布候选，真实 E2E 需人工验收，创建平台草稿不等于商品上架。';

  return (
    <Alert
      type="info"
      showIcon
      style={{ marginBottom: compact ? 8 : 16 }}
      message={
        <Space wrap>
          <span>{message}</span>
          {releaseCandidate ? (
            <span style={{ opacity: 0.85 }}>（抖店发布候选）</span>
          ) : null}
        </Space>
      }
      description={
        compact ? undefined : (
          <Space wrap style={{ marginTop: 8 }}>
            <Button size="small" onClick={() => history.push('/settings/platforms?platform=douyin_shop')}>
              去配置平台凭证
            </Button>
            <Button size="small" onClick={() => history.push('/settings/config-status')}>
              查看 E2E 准备清单
            </Button>
            <Button size="small" onClick={() => history.push('/ops/task-center/failures?platform=douyin_shop')}>
              查看失败任务
            </Button>
          </Space>
        )
      }
    />
  );
}

export function douyinCredentialStatusLabel(blocked: boolean): string {
  return blocked ? '缺少真实凭证' : '前置检查可通过';
}

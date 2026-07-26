export type LocalizedLabel = {
  zhCN: string;
  enUS: string;
  color?: 'default' | 'processing' | 'success' | 'error' | 'warning' | 'blue' | 'cyan' | 'gold' | 'purple' | 'orange' | 'red';
  description?: string;
};

export const OPERATION_TASK_STATUS_LABELS: Record<string, LocalizedLabel> = {
  suggested: { zhCN: '待处理建议', enUS: 'Suggested', color: 'default', description: '任务已创建，等待准备草稿。' },
  draft_preparing: { zhCN: '草稿准备中', enUS: 'Draft preparing', color: 'processing', description: '正在准备本地、模拟或沙箱草稿。' },
  pending_review: { zhCN: '待审核', enUS: 'Pending review', color: 'warning', description: '需要人工审核最新草稿。' },
  approved: { zhCN: '已批准', enUS: 'Approved', color: 'success', description: '最新草稿已获人工批准，可进行受控草稿生成。' },
  rejected: { zhCN: '已拒绝', enUS: 'Rejected', color: 'error', description: '审核已拒绝，任务不会继续执行。' },
  execution_queued: { zhCN: '等待执行', enUS: 'Execution queued', color: 'processing', description: '执行请求已排队，不代表真实发布。' },
  executing: { zhCN: '执行中', enUS: 'Executing', color: 'processing', description: '正在生成本地、模拟或沙箱草稿。' },
  draft_written: { zhCN: '草稿已生成', enUS: 'Draft written', color: 'success', description: '安全草稿生成完成，不代表商品发布或上架。' },
  execution_failed: { zhCN: '执行失败', enUS: 'Execution failed', color: 'error', description: '执行失败，可按后端允许状态人工重试。' },
  cancelled: { zhCN: '已取消', enUS: 'Cancelled', color: 'default', description: '任务已取消。' },
};

export const OPERATION_DRAFT_STATUS_LABELS: Record<string, LocalizedLabel> = {
  editable: { zhCN: '可编辑', enUS: 'Editable', color: 'processing' },
  pending_review: { zhCN: '待审核', enUS: 'Pending review', color: 'warning' },
  approved: { zhCN: '已批准', enUS: 'Approved', color: 'success' },
  superseded: { zhCN: '历史版本', enUS: 'Superseded', color: 'default' },
  written: { zhCN: '草稿已生成', enUS: 'Written', color: 'success' },
  failed: { zhCN: '失败', enUS: 'Failed', color: 'error' },
};

export const OPERATION_ATTEMPT_STATUS_LABELS: Record<string, LocalizedLabel> = {
  queued: { zhCN: '排队中', enUS: 'Queued', color: 'processing' },
  running: { zhCN: '运行中', enUS: 'Running', color: 'processing' },
  succeeded: { zhCN: '已完成', enUS: 'Succeeded', color: 'success' },
  failed: { zhCN: '失败', enUS: 'Failed', color: 'error' },
  cancelled: { zhCN: '已取消', enUS: 'Cancelled', color: 'default' },
};

export const OPERATION_PRIORITY_LABELS: Record<string, LocalizedLabel> = {
  low: { zhCN: '低', enUS: 'Low', color: 'default' },
  normal: { zhCN: '普通', enUS: 'Normal', color: 'blue' },
  high: { zhCN: '高', enUS: 'High', color: 'orange' },
  urgent: { zhCN: '紧急', enUS: 'Urgent', color: 'red' },
};

export const OPERATION_TASK_TYPE_LABELS: Record<string, string> = {
  product_content: '商品内容',
  order_exception: '订单异常',
  product_publish: '商品刊登',
  inventory_sync: '库存同步',
  customer_reply: '客服回复',
  ai_text: 'AI 文案',
  ai_image: 'AI 图片',
  manual_review: '人工复核',
};

export const OPERATION_SOURCE_TYPE_LABELS: Record<string, string> = {
  ai_suggestion: '智能建议',
  manual: '人工创建',
  system: '系统生成',
  import: '导入任务',
  webhook: '回调事件',
};

export const OPERATION_PLATFORM_LABELS: Record<string, string> = {
  local: '本地',
  douyin: '抖音',
  amazon: 'Amazon',
  lazada: 'Lazada',
  shopee: 'Shopee',
  tiktok: 'TikTok',
  woocommerce: 'WooCommerce',
  custom: '自定义',
};

export const OPERATION_ADAPTER_MODE_LABELS: Record<string, string> = {
  local_draft_only: '仅生成本地草稿',
  mock: '模拟草稿',
  sandbox: '沙箱草稿',
};

export const OPERATION_RESULT_TYPE_LABELS: Record<string, string> = {
  local_draft: '本地草稿',
  mock_draft: '模拟草稿',
  sandbox_fixture: '沙箱夹具',
};

export const OPERATION_EVENT_TYPE_LABELS: Record<string, LocalizedLabel> = {
  task_created: { zhCN: '任务已创建', enUS: 'Task created' },
  draft_generated: { zhCN: '草稿已生成', enUS: 'Draft generated' },
  draft_updated: { zhCN: '草稿已更新', enUS: 'Draft updated' },
  review_requested: { zhCN: '请求审核', enUS: 'Review requested' },
  approved: { zhCN: '已批准', enUS: 'Approved' },
  rejected: { zhCN: '已拒绝', enUS: 'Rejected' },
  execution_queued: { zhCN: '执行已排队', enUS: 'Execution queued' },
  execution_started: { zhCN: '执行已开始', enUS: 'Execution started' },
  draft_written: { zhCN: '草稿已生成', enUS: 'Draft written' },
  execution_failed: { zhCN: '执行失败', enUS: 'Execution failed' },
  retry_requested: { zhCN: '请求人工重试', enUS: 'Retry requested' },
  cancelled: { zhCN: '已取消', enUS: 'Cancelled' },
  permission_denied: { zhCN: '权限被拒绝', enUS: 'Permission denied' },
  production_capability_blocked: { zhCN: '生产能力已拦截', enUS: 'Production capability blocked' },
};

export const OPERATION_ERROR_LABELS: Record<string, string> = {
  validation_error: '提交内容未通过校验，请检查后重试。',
  permission_denied: '当前账号无权执行该操作。',
  not_found: '任务不存在或当前账号无权查看。',
  state_conflict: '任务状态已变化，请刷新后重新操作。',
  revision_conflict: '任务版本已变化，请查看最新内容后重新操作。',
  invalid_transition: '当前状态不允许执行该操作。',
  idempotency_payload_conflict: '该操作请求与之前提交内容不一致，请重新发起操作。',
  retry_limit_exceeded: '已达到最大重试限制。',
  production_capability_forbidden: '生产平台能力未启用，操作已被安全拦截。',
  draft_not_latest: '草稿已更新，请查看最新版本后重新审核。',
  draft_version_mismatch: '草稿版本不一致，请刷新后重试。',
  draft_hash_mismatch: '草稿内容校验不一致，请刷新后重试。',
  internal_error: '系统暂时无法完成操作，请稍后重试。',
};

export const OPERATION_METADATA_ALLOWLIST = new Set([
  'platform',
  'adapterMode',
  'taskType',
  'taskStatusBefore',
  'taskStatusAfter',
  'draftVersion',
  'payloadHash',
  'attemptNumber',
  'errorCategory',
  'retryable',
  'resultType',
  'capabilityBlocked',
]);

export const NON_PRODUCTION_BOUNDARY_COPY = {
  title: '非生产边界',
  description: '当前仅允许生成本地、模拟或沙箱草稿，不会调用真实平台发布或上架能力。',
  realWriteDisabled: '真实平台写入：未启用',
  autoPublishDisabled: '自动发布：未启用',
  autoListingDisabled: '自动上架：未启用',
};

export function operationLabel(map: Record<string, LocalizedLabel>, value?: string | null) {
  const key = (value ?? '').trim();
  return key ? map[key]?.zhCN ?? key : '—';
}

export function operationLabelColor(map: Record<string, LocalizedLabel>, value?: string | null) {
  const key = (value ?? '').trim();
  return map[key]?.color ?? 'default';
}

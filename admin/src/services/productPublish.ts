import { request } from '@umijs/max';
import type { ApiResponse } from '@/services/request';
import { getWithParams, getJSON, postJSON } from '@/services/request';
import type { ProductReadinessResult } from '@/services/productReadiness';

export type PublishTargetShop = {
  shopId: string;
  shopName: string;
  authStatus: string;
  authStatusLabel?: string;
  enabled: boolean;
};

export type PublishTargetPlatform = {
  platform: string;
  platformLabel: string;
  capability: string;
  capabilityLabel: string;
  shops: PublishTargetShop[];
  settingsGroupKey?: string;
  settingsPath?: string;
};

export type PublishTargetsResponse = {
  productId: string;
  platforms: PublishTargetPlatform[];
};

export type PublishTargetRef = {
  platform: string;
  shopId?: string | null;
};

export type PublishTargetIssue = {
  code: string;
  title: string;
  message: string;
  severity: string;
  suggestion?: string;
  technicalDetails?: Record<string, unknown>;
};

export type PublishTargetCheckResult = {
  targetKey: string;
  platform: string;
  platformLabel: string;
  shopId?: string;
  shopName?: string;
  capability: string;
  status: string;
  statusLabel: string;
  canCreateDraft: boolean;
  issues: PublishTargetIssue[];
};

export type PublishTargetsCheckResponse = {
  summary: {
    targetCount: number;
    readyCount: number;
    warningCount: number;
    blockedCount: number;
  };
  targets: PublishTargetCheckResult[];
};

export type PublishTargetTaskResult = {
  targetKey: string;
  platform: string;
  platformLabel: string;
  shopId?: string;
  shopName?: string;
  taskId?: string;
  publicationId?: string;
  status: string;
  statusLabel: string;
  capability: string;
  localDraftOnly?: boolean;
  errorCode?: string;
  errorMessage?: string;
  platformProductId?: string;
};

export type PublishTargetsCreateDraftsResponse = {
  batchId: string;
  status: string;
  statusLabel: string;
  targetCount: number;
  successCount: number;
  failedCount: number;
  skippedCount: number;
  targets: PublishTargetTaskResult[];
};

export async function fetchPublishTargets(productId: string): Promise<PublishTargetsResponse> {
  return getJSON(`/api/v1/products/${encodeURIComponent(productId)}/publish-targets`);
}

export async function checkPublishTargets(
  productId: string,
  body: {
    targets: PublishTargetRef[];
    commonConfig?: Record<string, unknown>;
    targetConfigs?: Record<string, unknown>;
  },
): Promise<PublishTargetsCheckResponse> {
  return postJSON(`/api/v1/products/${encodeURIComponent(productId)}/publish-targets/check`, body);
}

export async function createPublishTargetDrafts(
  productId: string,
  body: {
    targets: PublishTargetRef[];
    commonConfig?: Record<string, unknown>;
    targetConfigs?: Record<string, unknown>;
    onlyReady?: boolean;
    retryFailedOnly?: boolean;
    batchId?: string;
    force?: boolean;
  },
): Promise<PublishTargetsCreateDraftsResponse> {
  return postJSON(`/api/v1/products/${encodeURIComponent(productId)}/publish-targets/create-drafts`, body);
}

export type ProductPublicationRow = {
  id: string;
  productId: string;
  shopId: string;
  shopName?: string;
  platform: string;
  publishTaskId?: string;
  externalProductId?: string;
  externalUrl?: string;
  status: string;
  publishStatus: string;
  publishedAt?: string;
  lastSyncedAt?: string;
  skuBindingSyncedAt?: string;
  skuMappingsSummary?: string[];
};

export type DouyinSkuBindingRow = {
  publicationSkuId: string;
  productSkuId?: string;
  skuCode?: string;
  specName?: string;
  externalSkuId?: string;
  platformSkuName?: string;
  bindStatus?: string;
  bindConfidence?: number;
  bindMessage?: string;
  lastSyncedAt?: string;
  price?: number;
  stock?: number;
};

export type DouyinPlatformSkuCandidate = {
  platformSkuId: string;
  specName?: string;
  priceYuan?: number;
  stock?: number;
  boundToPublicationSkuId?: string;
};

export type DouyinSkuBindingSummary = {
  publicationId: string;
  externalProductId?: string;
  skuBindingSyncedAt?: string;
  total: number;
  bound: number;
  skipped: number;
  unmatched: number;
  ambiguous: number;
  failed: number;
  rows: DouyinSkuBindingRow[];
  platformSkus?: DouyinPlatformSkuCandidate[];
  inventorySyncReady?: boolean;
  inventorySyncBlockReason?: string;
  errorCode?: string;
  errorMessage?: string;
};

export type ProductPublishTaskDTO = {
  id: string;
  productId: string;
  shopId: string;
  targetStoreId?: string;
  shopName?: string;
  productTitle?: string;
  platform: string;
  targetPlatform?: string;
  taskType: string;
  status: string;
  publishStatus?: string;
  mode: string;
  publishMode?: string;
  title?: string;
  description?: string;
  images?: unknown;
  skus?: unknown;
  price?: number;
  currency?: string;
  checkResult?: unknown;
  platformPayload?: unknown;
  platformResult?: unknown;
  platformProductId?: string;
  platformRawError?: unknown;
  retryable?: boolean;
  recoveryState?: string;
  requestId?: string;
  mappingSnapshot?: unknown;
  startedAt?: string;
  finishedAt?: string;
  errorCode?: string;
  errorMessage?: string;
  input?: unknown;
  output?: unknown;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
  readiness?: ProductReadinessResult;
};

export type OzonReconcileOutcome = 'platform_created' | 'platform_not_created';
export type OzonReconcilePlatformStatus = 'imported' | 'pending_review' | 'needs_action' | 'sellable';

export type ReconcileOzonPublishTaskInput = {
  outcome: OzonReconcileOutcome;
  evidence: string;
  externalProductId?: string;
  externalSpuId?: string;
  externalUrl?: string;
  platformStatus?: OzonReconcilePlatformStatus;
  sellableVerified?: boolean;
};

export type ProductPublishBusinessStatus = {
  code: string;
  text: string;
  color?: string;
  successful: boolean;
  requiresReconciliation: boolean;
};

export function productPublishBusinessStatus(task: ProductPublishTaskDTO): ProductPublishBusinessStatus {
  const raw = String(task.publishStatus || task.status || '')
    .trim()
    .toLowerCase();
  const code = raw === 'result_uncertain' ? 'result_unknown' : raw;
  let requiresReconciliation = task.platform === 'ozon' && (code === 'result_unknown' || task.recoveryState === 'result_unknown' || (task.status === 'failed' && task.retryable !== true && task.recoveryState !== 'confirmed_not_created'));
  if (task.recoveryState === 'confirmed_not_created') {
    return {
      code: 'confirmed_not_created',
      text: '已确认 Ozon 未创建',
      color: 'blue',
      successful: false,
      requiresReconciliation: false,
    };
  }
  if (task.platform === 'ozon' && (code === 'published' || code === 'sellable') && !hasTrueJSONFlag(task.platformResult, 'sellableVerified') && !hasTrueJSONFlag(task.output, 'sellableVerified')) {
    requiresReconciliation = true;
    return {
      code: 'historical_unverified',
      text: '历史上架状态待核对',
      color: 'gold',
      successful: false,
      requiresReconciliation,
    };
  }
  const values: Record<string, Omit<ProductPublishBusinessStatus, 'code' | 'requiresReconciliation'>> = {
    published: { text: '成功上架', color: 'green', successful: true },
    sellable: { text: '成功上架', color: 'green', successful: true },
    imported: {
      text: 'Ozon 已接收，待确认可售',
      color: 'blue',
      successful: false,
    },
    pending_review: {
      text: 'Ozon 审核中',
      color: 'processing',
      successful: false,
    },
    needs_action: { text: '需要修改', color: 'orange', successful: false },
    result_unknown: {
      text: '平台结果待核对',
      color: 'gold',
      successful: false,
    },
    draft_created: { text: '仅创建本地草稿', successful: false },
    publishing: { text: '正在提交', color: 'processing', successful: false },
    checking: { text: '正在检查', color: 'processing', successful: false },
    ready: { text: '等待提交', color: 'processing', successful: false },
    pending: { text: '等待处理', color: 'processing', successful: false },
    running: { text: '正在处理', color: 'processing', successful: false },
    failed: { text: '提交失败', color: 'red', successful: false },
    cancelled: { text: '已取消', successful: false },
    canceled: { text: '已取消', successful: false },
    succeeded: {
      text: '任务已完成，平台状态待确认',
      color: 'gold',
      successful: false,
    },
    success: {
      text: '任务已完成，平台状态待确认',
      color: 'gold',
      successful: false,
    },
  };
  const meta = values[code] || { text: code || '状态未知', successful: false };
  return { code: code || 'unknown', ...meta, requiresReconciliation };
}

export function canReconcileOzonPublishTask(task: ProductPublishTaskDTO): boolean {
  return (
    String(task.platform || '')
      .trim()
      .toLowerCase() === 'ozon' &&
    String(task.status || '')
      .trim()
      .toLowerCase() === 'failed' &&
    productPublishBusinessStatus(task).requiresReconciliation
  );
}

function normalizedJSONKey(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9]/g, '');
}

function hasTrueJSONFlag(value: unknown, wantedKey: string, depth = 0): boolean {
  if (value === null || value === undefined || depth > 8) return false;
  if (Array.isArray(value)) return value.some((item) => hasTrueJSONFlag(item, wantedKey, depth + 1));
  if (typeof value !== 'object') return false;
  const normalizedWanted = normalizedJSONKey(wantedKey);
  return Object.entries(value as Record<string, unknown>).some(([key, nested]) => (normalizedJSONKey(key) === normalizedWanted && nested === true) || hasTrueJSONFlag(nested, wantedKey, depth + 1));
}

function collectStringsForKeys(value: unknown, wanted: Set<string>, result: string[], depth = 0) {
  if (value === null || value === undefined || depth > 8 || result.length >= 50) return;
  if (Array.isArray(value)) {
    value.forEach((item) => collectStringsForKeys(item, wanted, result, depth + 1));
    return;
  }
  if (typeof value !== 'object') return;
  Object.entries(value as Record<string, unknown>).forEach(([key, nested]) => {
    if (wanted.has(normalizedJSONKey(key))) {
      if (typeof nested === 'string' || typeof nested === 'number') {
        const text = String(nested).trim();
        if (text) result.push(text);
      } else if (Array.isArray(nested)) {
        nested.forEach((item) => {
          if (typeof item === 'string' || typeof item === 'number') result.push(String(item).trim());
          else if (item && typeof item === 'object') {
            const row = item as Record<string, unknown>;
            const text = String(row.message || row.text || row.value || row.description || '').trim();
            if (text) result.push(text);
          }
        });
      }
    }
    collectStringsForKeys(nested, wanted, result, depth + 1);
  });
}

export function extractOzonOfferIds(task: ProductPublishTaskDTO): string[] {
  const values: string[] = [];
  const keys = new Set(['offerid', 'externalspuid']);
  [task.platformPayload, task.input, task.mappingSnapshot, task.platformResult, task.output].forEach((value) => collectStringsForKeys(value, keys, values));
  return Array.from(new Set(values.filter(Boolean)));
}

export function extractOzonWarnings(task: ProductPublishTaskDTO): string[] {
  const values: string[] = [];
  const keys = new Set(['warning', 'warnings']);
  [task.platformResult, task.output, task.platformRawError].forEach((value) => collectStringsForKeys(value, keys, values));
  const collectWarningObjects = (value: unknown, depth = 0) => {
    if (value === null || value === undefined || depth > 8 || values.length >= 50) return;
    if (Array.isArray(value)) {
      value.forEach((item) => collectWarningObjects(item, depth + 1));
      return;
    }
    if (typeof value !== 'object') return;
    const row = value as Record<string, unknown>;
    const level = String(row.level || row.severity || row.status || '').toLowerCase();
    if (level.includes('warn')) {
      const text = String(row.message || row.text || row.description || row.code || '').trim();
      if (text) values.push(text);
    }
    Object.values(row).forEach((nested) => collectWarningObjects(nested, depth + 1));
  };
  collectWarningObjects(task.platformResult);
  collectWarningObjects(task.output);
  return Array.from(new Set(values.filter(Boolean))).slice(0, 20);
}

export async function publishProduct(
  productId: string,
  body: { shopId: string; options?: Record<string, unknown>; force?: boolean },
): Promise<ProductPublishTaskDTO> {
  const res = await request<ApiResponse<ProductPublishTaskDTO>>(`/api/v1/products/${encodeURIComponent(productId)}/publish`, {
    method: 'POST',
    data: body,
  });
  if (res.code !== 0) {
    const err = new Error(res.message || 'publish_failed') as Error & { businessCode?: number; data?: unknown };
    err.businessCode = res.code;
    err.data = res.data;
    throw err;
  }
  return res.data as ProductPublishTaskDTO;
}

export async function listProductPublications(productId: string): Promise<{ list: ProductPublicationRow[] }> {
  return getJSON(`/api/v1/products/${productId}/publications`);
}

export async function queryProductPublishTasks(params: {
  page?: number;
  pageSize?: number;
  productId?: string;
  shopId?: string;
  platform?: string;
  status?: string;
  start?: string;
  end?: string;
}): Promise<{
  list: ProductPublishTaskDTO[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}> {
  return getWithParams('/api/v1/product-publish/tasks', params);
}

export async function getProductPublishTask(id: string): Promise<ProductPublishTaskDTO> {
  return getJSON(`/api/v1/product-publish/tasks/${id}`);
}

export async function retryProductPublishTask(id: string): Promise<ProductPublishTaskDTO> {
  return postJSON(`/api/v1/product-publish/tasks/${id}/retry`, {});
}

export async function cancelProductPublishTask(id: string): Promise<ProductPublishTaskDTO> {
  return postJSON(`/api/v1/product-publish/tasks/${id}/cancel`, {});
}

export async function reconcileOzonPublishTask(id: string, body: ReconcileOzonPublishTaskInput): Promise<ProductPublishTaskDTO> {
  return postJSON(`/api/v1/product-publish/tasks/${encodeURIComponent(id)}/reconcile-ozon`, body);
}

export async function createDouyinProductDraft(
  productId: string,
  body: { shopId: string; publishMode?: string; force?: boolean },
): Promise<ProductPublishTaskDTO> {
  const res = await request<ApiResponse<ProductPublishTaskDTO>>(
    `/api/v1/products/${encodeURIComponent(productId)}/platform-configs/douyin_shop/create-draft`,
    { method: 'POST', data: { publishMode: 'save_as_platform_draft', ...body } },
  );
  if (res.code !== 0) {
    const err = new Error(res.message || 'create_draft_failed') as Error & { businessCode?: number; data?: unknown };
    err.businessCode = res.code;
    err.data = res.data;
    throw err;
  }
  return res.data as ProductPublishTaskDTO;
}

export async function getDouyinSkuBindings(publicationId: string): Promise<DouyinSkuBindingSummary> {
  return getJSON(`/api/v1/product-publications/${encodeURIComponent(publicationId)}/douyin/sku-bindings`);
}

export async function syncDouyinSkuBindings(publicationId: string): Promise<DouyinSkuBindingSummary> {
  return postJSON(`/api/v1/product-publications/${encodeURIComponent(publicationId)}/douyin/sync-sku-bindings`, {});
}

export async function bindDouyinSku(
  publicationSkuId: string,
  body: { platformSkuId: string; platformSkuName?: string; bindReason?: string },
): Promise<DouyinSkuBindingRow> {
  return postJSON(`/api/v1/product-publication-skus/${encodeURIComponent(publicationSkuId)}/douyin/bind-sku`, {
    bindReason: 'manual',
    ...body,
  });
}

export async function unbindDouyinSku(
  publicationSkuId: string,
  body?: { reason?: string },
): Promise<DouyinSkuBindingRow> {
  return postJSON(`/api/v1/product-publication-skus/${encodeURIComponent(publicationSkuId)}/douyin/unbind-sku`, {
    reason: body?.reason ?? 'manual_unbind',
  });
}

export async function listDouyinPublishTasks(
  productId: string,
  params?: { page?: number; pageSize?: number },
): Promise<{
  list: ProductPublishTaskDTO[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}> {
  return getWithParams(`/api/v1/products/${encodeURIComponent(productId)}/platform-configs/douyin_shop/publish-tasks`, params ?? {});
}

export type PublishConfigOverrides = {
  products?: Record<string, Record<string, unknown>>;
  platforms?: Record<string, Record<string, unknown>>;
  shops?: Record<string, Record<string, unknown>>;
  productTargets?: Record<string, Record<string, unknown>>;
};

export type BatchTargetCheckItem = {
  productId: string;
  productTitle: string;
  targetKey: string;
  platform: string;
  platformLabel: string;
  shopId?: string;
  shopName?: string;
  capability: string;
  status: string;
  statusLabel: string;
  canCreateDraft: boolean;
  issues: PublishTargetIssue[];
};

export type BatchTargetsCheckResponse = {
  summary: {
    productCount: number;
    targetCount: number;
    taskCount: number;
    readyCount: number;
    warningCount: number;
    blockedCount: number;
    localDraftOnlyCount: number;
  };
  items: BatchTargetCheckItem[];
};

export type BatchTargetTaskResult = {
  productId: string;
  productTitle: string;
  targetKey: string;
  platform: string;
  platformLabel: string;
  shopId?: string;
  shopName?: string;
  taskId?: string;
  publicationId?: string;
  status: string;
  statusLabel: string;
  capability: string;
  localDraftOnly?: boolean;
  errorCode?: string;
  errorMessage?: string;
  platformProductId?: string;
};

export type BatchTargetsCreateDraftsResponse = {
  batchId: string;
  status: string;
  statusLabel: string;
  productCount: number;
  targetCount: number;
  taskCount: number;
  successCount: number;
  failedCount: number;
  skippedCount: number;
  items: BatchTargetTaskResult[];
};

export type PublishBatchListItem = {
  id: string;
  batchType: string;
  name?: string;
  status: string;
  statusLabel: string;
  productCount: number;
  targetCount: number;
  taskCount: number;
  successCount: number;
  failedCount: number;
  skippedCount: number;
  createdBy?: string;
  createdAt: string;
  finishedAt?: string;
};

export type PublishBatchDetail = PublishBatchListItem & {
  items: BatchTargetTaskResult[];
  input?: Record<string, unknown>;
};

export async function fetchGlobalPublishTargets(): Promise<PublishTargetsResponse> {
  return getJSON('/api/v1/product-publish/targets');
}

export async function checkBatchPublishTargets(body: {
  productIds: string[];
  targets: PublishTargetRef[];
  commonConfig?: Record<string, unknown>;
  overrides?: PublishConfigOverrides;
}): Promise<BatchTargetsCheckResponse> {
  const res = await request<ApiResponse<BatchTargetsCheckResponse>>(`/api/v1/product-publish/batch-targets/check`, {
    method: 'POST',
    data: body,
  });
  if (res.code !== 0) {
    const err = new Error(res.message || 'check_failed') as Error & { businessCode?: number; data?: unknown };
    err.businessCode = res.code;
    err.data = res.data;
    throw err;
  }
  return res.data as BatchTargetsCheckResponse;
}

export async function createBatchPublishDrafts(body: {
  productIds: string[];
  targets: PublishTargetRef[];
  commonConfig?: Record<string, unknown>;
  overrides?: PublishConfigOverrides;
  onlyReady?: boolean;
  includeWarnings?: boolean;
  force?: boolean;
  name?: string;
}): Promise<BatchTargetsCreateDraftsResponse> {
  const res = await request<ApiResponse<BatchTargetsCreateDraftsResponse>>(
    '/api/v1/product-publish/batch-targets/create-drafts',
    { method: 'POST', data: body },
  );
  if (res.code !== 0) {
    const err = new Error(res.message || 'create_failed') as Error & { businessCode?: number; data?: unknown };
    err.businessCode = res.code;
    err.data = res.data;
    throw err;
  }
  return res.data as BatchTargetsCreateDraftsResponse;
}

export async function queryPublishBatches(params?: {
  page?: number;
  pageSize?: number;
}): Promise<{
  list: PublishBatchListItem[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}> {
  return getWithParams('/api/v1/product-publish/batches', params ?? {});
}

export async function getPublishBatch(id: string): Promise<PublishBatchDetail> {
  return getJSON(`/api/v1/product-publish/batches/${encodeURIComponent(id)}`);
}

export async function retryFailedPublishBatch(id: string): Promise<BatchTargetsCreateDraftsResponse> {
  return postJSON(`/api/v1/product-publish/batches/${encodeURIComponent(id)}/retry-failed`, {});
}

export async function cancelPendingPublishBatch(id: string): Promise<PublishBatchDetail> {
  return postJSON(`/api/v1/product-publish/batches/${encodeURIComponent(id)}/cancel-pending`, {});
}

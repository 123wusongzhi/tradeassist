import { request } from '@umijs/max';
import type { ApiResponse } from '@/services/request';
import { getJSON, getWithParams, postJSON, putJSON } from '@/services/request';
import type { ProductPublishTaskDTO } from '@/services/productPublish';

export type OzonCategoryChange = {
  id: string;
  categoryId: string;
  categoryName?: string;
  changeType: 'added' | 'changed' | 'deactivated' | 'reactivated' | string;
  occurredAt?: string;
  detail?: string;
};

export type OzonCategorySyncRun = {
  id: string;
  status:
    | 'pending'
    | 'running'
    | 'succeeded'
    | 'partial'
    | 'failed'
    | 'stale'
    | string;
  statusLabel?: string;
  startedAt?: string;
  finishedAt?: string;
  addedCount?: number;
  changedCount?: number;
  deactivatedCount?: number;
  reactivatedCount?: number;
  errorMessage?: string;
  summary?: Record<string, number>;
};

export type OzonCategorySyncStart = {
  stats?: OzonCategoryFlowStats;
  run?: OzonCategorySyncRun;
  runId?: string;
};

export type OzonCategoryFlowStats = {
  count: number;
  leafCount: number;
  activeCount?: number;
  inactiveCount?: number;
  lastSyncedAt?: string;
  lastRun?: OzonCategorySyncRun;
  diffCounts?: Partial<
    Record<'added' | 'changed' | 'deactivated' | 'reactivated', number>
  >;
};

export type OzonCategoryMapping = {
  id?: string;
  shopId?: string;
  sourceCategoryKey: string;
  sourceCategoryName?: string;
  categoryId: string;
  categoryPath?: string;
  schemaHash?: string;
  status?: string;
  updatedAt?: string;
};

export type OzonCategoryMappingInput = Pick<
  OzonCategoryMapping,
  | 'shopId'
  | 'sourceCategoryKey'
  | 'sourceCategoryName'
  | 'categoryId'
  | 'categoryPath'
  | 'status'
>;

export type OzonCategoryRecommendation = {
  sourceCategoryKey: string;
  sourceCategoryName?: string;
  candidate?: {
    categoryId: string;
    categoryPath?: string;
    score?: number;
    reason?: string;
  };
  confirmed?: boolean;
};

export type OzonProductConfig = {
  productId: string;
  shopId?: string;
  categoryId?: string;
  categoryPath?: string;
  platformAttributes?: Record<string, unknown>;
  sourceCategoryKey?: string;
  sourceCategoryName?: string;
  schemaHash?: string;
  schemaConfirmedAt?: string;
};

export type OzonProductConfigInput = Pick<
  OzonProductConfig,
  | 'shopId'
  | 'categoryId'
  | 'categoryPath'
  | 'platformAttributes'
  | 'sourceCategoryKey'
  | 'sourceCategoryName'
>;

export type OzonReadinessResult = {
  canPublish: boolean;
  checkedAt?: string;
  schemaHash?: string;
  schemaChanged?: boolean;
  checks?: Array<{
    code?: string;
    title?: string;
    message?: string;
    level?: string;
    suggestion?: string;
  }>;
  /** Backward-compatible shape used by earlier server builds. */
  items?: Array<{
    code?: string;
    title?: string;
    message?: string;
    severity?: string;
    status?: string;
  }>;
};

export type OzonCategoryGroup = {
  key: string;
  sourceCategoryKey?: string;
  sourceCategoryName?: string;
  productIds: string[];
  status: 'ready' | 'needs_work' | 'skipped' | string;
  statusLabel?: string;
  recommendedCategoryId?: string;
  recommendedCategoryPath?: string;
  issues?: Array<{ message?: string; title?: string }>;
};

type OzonAttributeShape = {
  attrId: string;
  dictionaryId?: string;
  options?: Array<{ id: string; value: string }>;
};

/** Converts UI form values into the explicit Ozon attribute payload shape. */
export function buildOzonPlatformAttributes(
  attributes: OzonAttributeShape[],
  raw?: Record<string, unknown>,
): Record<string, { value: string; dictionaryValueId?: string }> {
  return attributes.reduce<
    Record<string, { value: string; dictionaryValueId?: string }>
  >((result, attribute) => {
    const field = raw?.[attribute.attrId];
    if (field === undefined || field === null || field === '') return result;
    if (attribute.dictionaryId) {
      const option = attribute.options?.find(
        (item) => item.id === String(field),
      );
      if (option)
        result[attribute.attrId] = {
          value: option.value,
          dictionaryValueId: option.id,
        };
      else result[attribute.attrId] = { value: String(field) };
    } else result[attribute.attrId] = { value: String(field) };
    return result;
  }, {});
}

/** Restores the edit-friendly value from explicit Ozon attribute payloads. */
export function toOzonAttributeFormValues(
  attributes: OzonAttributeShape[],
  raw?: Record<string, unknown>,
): Record<string, string> {
  return attributes.reduce<Record<string, string>>((result, attribute) => {
    const value = raw?.[attribute.attrId];
    if (typeof value === 'string') result[attribute.attrId] = value;
    else if (value && typeof value === 'object') {
      const typed = value as { value?: unknown; dictionaryValueId?: unknown };
      result[attribute.attrId] = String(
        attribute.dictionaryId
          ? (typed.dictionaryValueId ?? typed.value ?? '')
          : (typed.value ?? ''),
      );
    }
    return result;
  }, {});
}

const enc = encodeURIComponent;

export function getOzonCategoryFlowStats() {
  return getJSON<OzonCategoryFlowStats>(
    '/api/v1/platform/ozon/categories/stats',
  );
}
export function syncOzonCategoryFlow(shopId?: string) {
  return postJSON<OzonCategorySyncStart>(
    '/api/v1/platform/ozon/categories/sync',
    shopId ? { shopId } : {},
  );
}
export function listOzonCategorySyncRuns() {
  return getJSON<{ list: OzonCategorySyncRun[] }>(
    '/api/v1/platform/ozon/categories/sync-runs',
  );
}
export function listOzonCategoryChanges() {
  return getJSON<{ list: OzonCategoryChange[] }>(
    '/api/v1/platform/ozon/categories/changes',
  );
}
export function listOzonCategoryMappings(shopId?: string) {
  return getWithParams<{ list: OzonCategoryMapping[] }>(
    '/api/v1/platform/ozon/category-mappings',
    { shopId },
  );
}
export function recommendOzonCategory(body: {
  shopId?: string;
  sourceCategoryKey: string;
  sourceCategoryName?: string;
}) {
  return postJSON<OzonCategoryRecommendation>(
    '/api/v1/platform/ozon/category-mappings/recommend',
    body,
  );
}
export function saveOzonCategoryMapping(body: OzonCategoryMappingInput) {
  return putJSON<OzonCategoryMapping, OzonCategoryMappingInput>(
    '/api/v1/platform/ozon/category-mappings',
    body,
  );
}
export function getOzonProductConfig(productId: string) {
  return getJSON<OzonProductConfig>(
    `/api/v1/products/${enc(productId)}/platform-configs/ozon`,
  );
}
export function saveOzonProductConfig(
  productId: string,
  body: OzonProductConfigInput,
) {
  return putJSON<OzonProductConfig, OzonProductConfigInput>(
    `/api/v1/products/${enc(productId)}/platform-configs/ozon`,
    body,
  );
}
export function validateOzonReadiness(productId: string, shopId: string) {
  return postJSON<OzonReadinessResult>(
    `/api/v1/products/${enc(productId)}/readiness/validate`,
    { platform: 'ozon', shopId },
  );
}
export function checkOzonCategoryGroups(body: {
  productIds: string[];
  shopId?: string;
}) {
  return postJSON<{ groups: OzonCategoryGroup[] }>(
    '/api/v1/product-publish/ozon/category-groups/check',
    body,
  );
}
export function confirmOzonCategoryGroup(body: {
  shopId: string;
  groups: Array<{
    sourceCategoryKey?: string;
    sourceCategoryName?: string;
    productIds: string[];
    categoryId: string;
    categoryPath?: string;
  }>;
  saveMappings?: boolean;
}) {
  return postJSON<{ groups: OzonCategoryGroup[] }>(
    '/api/v1/product-publish/ozon/category-groups/confirm',
    body,
  );
}

export async function publishOzonProduct(
  productId: string,
  shopId: string,
  idempotencyKey: string,
): Promise<ProductPublishTaskDTO> {
  const response = await request<ApiResponse<ProductPublishTaskDTO>>(
    `/api/v1/products/${enc(productId)}/publish`,
    {
      method: 'POST',
      data: { shopId, options: { platform: 'ozon' } },
      headers: { 'Idempotency-Key': idempotencyKey },
    },
  );
  if (response.code !== 0)
    throw new Error(response.message || '创建 Ozon 提交任务失败');
  return response.data as ProductPublishTaskDTO;
}

export function searchOzonLeafCategories(keyword?: string) {
  return getWithParams<{
    list: Array<{
      id: string;
      categoryId: string;
      name: string;
      descriptionCategoryId?: string;
      typeId?: string;
    }>;
    total: number;
  }>('/api/v1/platform/ozon/categories', {
    keyword,
    onlyLeaf: '1',
    activeOnly: '1',
    limit: '100',
  });
}

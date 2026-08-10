import { getJSON, getWithParams, postJSON, putJSON } from "@/services/request";

export type OzonCategoryNode = {
  id: string;
  categoryId: string;
  descriptionCategoryId?: string;
  typeId?: string;
  parentId?: string;
  name: string;
  path?: string;
  level: number;
  isLeaf: boolean;
  hasChildren: boolean;
  childCount: number;
  ancestors?: Array<{
    categoryId: string;
    name: string;
    level: number;
  }>;
  status?: string;
  syncedAt?: string;
};

export type OzonCategoryStats = {
  count: number;
  leafCount: number;
  activeCount?: number;
  inactiveCount?: number;
  lastSyncedAt?: string;
};

export type OzonCategorySyncStart = {
  stats?: OzonCategoryStats;
  run?: { id: string; status: string; statusLabel?: string };
  runId?: string;
};

export type OzonCategoryAttribute = {
  id: string;
  categoryId: string;
  attrId: string;
  name: string;
  description?: string;
  required: boolean;
  valueType?: string;
  dictionaryId?: string;
  skuVariantEligible?: boolean;
  skuVariantEligibilityKnown?: boolean;
  isCollection: boolean;
  maxValueCount?: number;
  attributeComplexId?: number;
  complexIsCollection: boolean;
  categoryDependent: boolean;
  options?: { id: string; value: string }[];
  syncedAt?: string;
  cacheStale?: boolean;
};

export type OzonVariantPolicy = {
  maxSkuCount: number;
  maxVariantAttributeCount: number;
  maxVariantCombinationCount: number;
  eligibleAttributeCount: number;
  variantEligibilityFullyKnown: boolean;
  source: "ozon_is_aspect+trademind_import_guardrail" | string;
};

export type OzonAttributeMapping = {
  attributeId: string;
  attributeName?: string;
  localField?: string;
  enabled: boolean;
  sortOrder?: number;
};

export async function queryOzonCategories(params?: {
  keyword?: string;
  parentId?: string;
  rootOnly?: boolean;
  onlyLeaf?: boolean;
  activeOnly?: boolean;
  limit?: number;
  offset?: number;
}): Promise<{
  list: OzonCategoryNode[];
  total: number;
  leafCount: number;
  matchedTotal: number;
  offset: number;
  limit: number;
  lastSyncedAt?: string;
  cacheStale: boolean;
}> {
  return getWithParams("/api/v1/platform/ozon/categories", {
    keyword: params?.keyword || undefined,
    parentId:
      typeof params?.parentId === "string" ? params.parentId : undefined,
    rootOnly: params?.rootOnly ? "1" : undefined,
    onlyLeaf: params?.onlyLeaf ? "1" : undefined,
    activeOnly: params?.activeOnly ? "1" : undefined,
    limit: params?.limit ? String(params.limit) : undefined,
    offset:
      typeof params?.offset === "number" ? String(params.offset) : undefined,
  });
}

export async function syncOzonCategories(
  shopId?: string,
): Promise<OzonCategorySyncStart> {
  return postJSON(
    "/api/v1/platform/ozon/categories/sync",
    shopId ? { shopId } : {},
  );
}

export async function getOzonCategoryStats(): Promise<OzonCategoryStats> {
  return getJSON("/api/v1/platform/ozon/categories/stats");
}

export async function queryOzonCategoryAttributes(
  categoryId: string,
  options?: { refreshKey?: string | number },
): Promise<{
  list: OzonCategoryAttribute[];
  variantPolicy: OzonVariantPolicy;
}> {
  const path = `/api/v1/platform/ozon/categories/${encodeURIComponent(categoryId)}/attributes`;
  if (options?.refreshKey !== undefined)
    return getWithParams(path, { _refresh: options.refreshKey });
  return getJSON(path);
}

export async function syncOzonCategoryAttributes(
  categoryId: string,
  shopId?: string,
): Promise<OzonCategoryStats> {
  return postJSON(
    `/api/v1/platform/ozon/categories/${encodeURIComponent(categoryId)}/attributes/sync`,
    shopId ? { shopId } : {},
  );
}

export async function searchOzonDictionaryValues(
  categoryId: string,
  attrId: string,
  shopId: string,
  keyword: string,
): Promise<{ list: Array<{ id: string; value: string }> }> {
  return getWithParams(
    `/api/v1/platform/ozon/categories/${encodeURIComponent(categoryId)}/attributes/${encodeURIComponent(attrId)}/values`,
    { shopId, keyword },
  );
}

export async function getOzonAttributeMappings(categoryId: string): Promise<{
  list: OzonAttributeMapping[];
}> {
  return getJSON(
    `/api/v1/platform/ozon/categories/${encodeURIComponent(categoryId)}/attribute-mappings`,
  );
}

export async function putOzonAttributeMappings(
  categoryId: string,
  items: OzonAttributeMapping[],
): Promise<{ list: OzonAttributeMapping[] }> {
  return putJSON(
    `/api/v1/platform/ozon/categories/${encodeURIComponent(categoryId)}/attribute-mappings`,
    { items },
  );
}

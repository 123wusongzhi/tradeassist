import { getJSON, getWithParams, postJSON, putJSON } from "@/services/request";

export type OzonCategoryNode = {
  id: string;
  categoryId: string;
  descriptionCategoryId?: string;
  typeId?: string;
  parentId?: string;
  name: string;
  level: number;
  isLeaf: boolean;
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
  required: boolean;
  valueType?: string;
  dictionaryId?: string;
  isCollection: boolean;
  maxValueCount?: number;
  attributeComplexId?: number;
  complexIsCollection: boolean;
  categoryDependent: boolean;
  options?: { id: string; value: string }[];
  syncedAt?: string;
  cacheStale?: boolean;
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
  onlyLeaf?: boolean;
  limit?: number;
}): Promise<{
  list: OzonCategoryNode[];
  total: number;
  leafCount: number;
}> {
  return getWithParams("/api/v1/platform/ozon/categories", {
    keyword: params?.keyword || undefined,
    onlyLeaf: params?.onlyLeaf ? "1" : undefined,
    limit: params?.limit ? String(params.limit) : undefined,
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

export async function queryOzonCategoryAttributes(categoryId: string): Promise<{
  list: OzonCategoryAttribute[];
}> {
  return getJSON(
    `/api/v1/platform/ozon/categories/${encodeURIComponent(categoryId)}/attributes`,
  );
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

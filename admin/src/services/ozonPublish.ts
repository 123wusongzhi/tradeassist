import { getJSON, getWithParams, postJSON, putJSON } from "@/services/request";
import type { ProductPublishTaskDTO } from "@/services/productPublish";

export type OzonCategoryChange = {
  id: string;
  categoryId: string;
  categoryName?: string;
  changeType: "added" | "changed" | "deactivated" | "reactivated" | string;
  occurredAt?: string;
  detail?: string;
};

export type OzonCategorySyncRun = {
  id: string;
  status:
    | "pending"
    | "running"
    | "succeeded"
    | "partial"
    | "failed"
    | "stale"
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
    Record<"added" | "changed" | "deactivated" | "reactivated", number>
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
  | "shopId"
  | "sourceCategoryKey"
  | "sourceCategoryName"
  | "categoryId"
  | "categoryPath"
  | "status"
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
  id?: string;
  productId: string;
  shopId?: string;
  categoryId?: string;
  categoryPath?: string;
  platformAttributes?: Record<string, unknown>;
  sourceCategoryKey?: string;
  sourceCategoryName?: string;
  schemaHash?: string;
  schemaConfirmedAt?: string;
  ozonImages?: OzonImageConfigView;
  ozonListing?: OzonListingConfigInput;
  ozonPreview?: OzonResolvedListing;
  legacyFallback?: boolean;
};

export type OzonProductConfigInput = Pick<
  OzonProductConfig,
  | "shopId"
  | "categoryId"
  | "categoryPath"
  | "platformAttributes"
  | "sourceCategoryKey"
  | "sourceCategoryName"
> & {
  ozonImages?: OzonImageConfigInput;
  ozonListing?: OzonListingConfigInput;
};

export type OzonPackageConfigInput = {
  weightG?: number;
  widthMm?: number;
  heightMm?: number;
  depthMm?: number;
  warehouseId?: string;
  vat?: string;
};

export type OzonListingConfigInput = {
  version: 1;
  titleOverride?: string;
  descriptionOverride?: string;
  currencyCode?: string;
  skuPriceOverrides: Record<string, number>;
  package: OzonPackageConfigInput;
};

export type OzonValueSource =
  | "product"
  | "ozon_product_shop_config"
  | "global_ozon_preset"
  | "local_inventory"
  | "store_contract"
  | "ozon_default"
  | "missing"
  | string;

export type OzonResolvedValue<T> = { value: T; source: OzonValueSource };

export type OzonListingIssue = {
  code: string;
  message: string;
  suggestion?: string;
  field?: string;
  skuId?: string;
};

export type OzonResolvedSKUListing = {
  skuId: string;
  skuCode?: string;
  skuName?: string;
  price: OzonResolvedValue<number>;
  localStock: number;
  stockSource: OzonValueSource;
  images: OzonResolvedImage[];
  canSubmit: boolean;
  issues: OzonListingIssue[];
};

export type OzonResolvedListing = {
  productId: string;
  shopId?: string;
  categoryId?: string;
  categoryPath?: string;
  schemaHash?: string;
  platformAttributes?: OzonPlatformAttributePayload;
  title: OzonResolvedValue<string>;
  description: OzonResolvedValue<string>;
  currency: OzonResolvedValue<string>;
  package: {
    weightG: OzonResolvedValue<number>;
    widthMm: OzonResolvedValue<number>;
    heightMm: OzonResolvedValue<number>;
    depthMm: OzonResolvedValue<number>;
    warehouseId: OzonResolvedValue<string>;
    vat: OzonResolvedValue<string>;
  };
  skus: OzonResolvedSKUListing[];
  issues: OzonListingIssue[];
  errorCount: number;
  canSubmit: boolean;
};

export type OzonAttributeSelection = {
  value: string;
  dictionaryValueId?: string;
};

export type OzonComplexAttributeGroup = {
  complexId: number;
  attributes: Record<string, OzonAttributeSelection[]>;
};

export type OzonPlatformAttributePayload = {
  version: 2;
  attributes: Record<string, OzonAttributeSelection[]>;
  complexGroups: OzonComplexAttributeGroup[];
};

export type OzonAttributeEditorValues = {
  attributes?: Record<string, string | string[]>;
  complexGroups?: Record<string, Array<Record<string, string | string[]>>>;
};

export type OzonImageSource =
  | "sku_original"
  | "manual_fallback"
  | "product_shared";

export type OzonSharedImage = {
  id: string;
  url: string;
  imageType: string;
  sortOrder: number;
};

export type OzonResolvedImage = {
  imageId?: string;
  url: string;
  source: OzonImageSource;
  position: number;
  imageType?: string;
};

export type OzonImageIssue = {
  code: string;
  message: string;
  suggestion?: string;
  skuId?: string;
};

export type OzonSKUImageConfig = {
  skuId: string;
  skuCode?: string;
  skuName?: string;
  attrs?: Record<string, unknown>;
  originalMainImageUrl?: string;
  fallbackMainImageId?: string;
  additionalImageIds: string[];
  finalImages: OzonResolvedImage[];
  canPublish: boolean;
  issues: OzonImageIssue[];
};

export type OzonImageConfigView = {
  version: number;
  configured: boolean;
  compatibilityMode?: "sku_original_only" | string;
  maxImagesPerSku: number;
  sharedImages: OzonSharedImage[];
  skus: OzonSKUImageConfig[];
  issues: OzonImageIssue[];
  errorCount: number;
};

export type OzonImageConfigInput = {
  version: number;
  skuSelections: Array<{
    skuId: string;
    fallbackMainImageId?: string;
    additionalImageIds: string[];
  }>;
};

export function buildOzonSKUImagePreview(
  sku: OzonSKUImageConfig,
  sharedImages: OzonSharedImage[],
  maxImagesPerSku = 10,
): OzonSKUImageConfig {
  const byId = new Map(sharedImages.map((image) => [image.id, image]));
  const finalImages: OzonResolvedImage[] = [];
  const issues: OzonImageIssue[] = [];
  const seenUrls = new Set<string>();
  const append = (
    url: string | undefined,
    source: OzonImageSource,
    image?: OzonSharedImage,
  ) => {
    const normalized = String(url || "").trim();
    if (
      !normalized ||
      seenUrls.has(normalized) ||
      finalImages.length >= maxImagesPerSku
    )
      return;
    seenUrls.add(normalized);
    finalImages.push({
      imageId: image?.id,
      url: normalized,
      source,
      position: finalImages.length + 1,
      imageType: source === "product_shared" ? "detail" : "main",
    });
  };

  let hasPrimary = false;
  const original = String(sku.originalMainImageUrl || "").trim();
  if (original) {
    append(original, "sku_original");
    hasPrimary = true;
    if (sku.fallbackMainImageId)
      issues.push({
        code: "OZON_SKU_FALLBACK_NOT_ALLOWED",
        message: "SKU 已有原始主图，不能同时保存替代主图",
        suggestion: "请清除该 SKU 的替代主图。",
        skuId: sku.skuId,
      });
  } else if (sku.fallbackMainImageId) {
    const fallback = byId.get(sku.fallbackMainImageId);
    if (fallback) {
      append(fallback.url, "manual_fallback", fallback);
      hasPrimary = true;
    } else {
      issues.push({
        code: "OZON_SKU_FALLBACK_IMAGE_STALE",
        message: "SKU 保存的替代主图已不存在或不可用",
        suggestion: "请重新选择替代主图。",
        skuId: sku.skuId,
      });
    }
  }

  const additionalImageIds = Array.from(
    new Set((sku.additionalImageIds || []).filter(Boolean)),
  );
  if (additionalImageIds.length > maxImagesPerSku - 1)
    issues.push({
      code: "OZON_SKU_IMAGE_LIMIT_EXCEEDED",
      message: `SKU 追加图片超过 Ozon 上限（最多 ${maxImagesPerSku - 1} 张）`,
      suggestion: "请减少商品公共图片选择。",
      skuId: sku.skuId,
    });
  additionalImageIds.forEach((imageId) => {
    const image = byId.get(imageId);
    if (!image) {
      issues.push({
        code: "OZON_SKU_SHARED_IMAGE_STALE",
        message: "SKU 选择的商品公共图片已不存在或不可用",
        suggestion: "请重新检查追加图片。",
        skuId: sku.skuId,
      });
      return;
    }
    append(image.url, "product_shared", image);
  });
  if (!hasPrimary)
    issues.push({
      code: "OZON_SKU_MAIN_IMAGE_MISSING",
      message: `SKU「${sku.skuName || sku.skuCode || sku.skuId}」缺少原始主图，且未指定替代主图`,
      suggestion: "请补齐采集原图，或明确选择替代主图。",
      skuId: sku.skuId,
    });
  return {
    ...sku,
    additionalImageIds,
    finalImages,
    issues,
    canPublish: hasPrimary && issues.length === 0,
  };
}

export function toOzonImageConfigInput(
  skus: OzonSKUImageConfig[],
): OzonImageConfigInput {
  return {
    version: 1,
    skuSelections: skus.map((sku) => ({
      skuId: sku.skuId,
      fallbackMainImageId: sku.fallbackMainImageId || undefined,
      additionalImageIds: Array.from(
        new Set((sku.additionalImageIds || []).filter(Boolean)),
      ),
    })),
  };
}

export type OzonReadinessResult = {
  canPublish: boolean;
  errorCount?: number;
  warningCount?: number;
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
  resolvedOzon?: OzonResolvedListing;
};

export type OzonCategoryGroup = {
  key: string;
  sourceCategoryKey?: string;
  sourceCategoryName?: string;
  productIds: string[];
  status: "ready" | "needs_work" | "skipped" | string;
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

export type OzonRichAttributeShape = OzonAttributeShape & {
  isCollection?: boolean;
  maxValueCount?: number;
  attributeComplexId?: number;
  complexIsCollection?: boolean;
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
    if (field === undefined || field === null || field === "") return result;
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
    if (typeof value === "string") result[attribute.attrId] = value;
    else if (value && typeof value === "object") {
      const typed = value as { value?: unknown; dictionaryValueId?: unknown };
      result[attribute.attrId] = String(
        attribute.dictionaryId
          ? (typed.dictionaryValueId ?? typed.value ?? "")
          : (typed.value ?? ""),
      );
    }
    return result;
  }, {});
}

function editorSelections(
  attribute: OzonRichAttributeShape,
  raw: string | string[] | undefined,
): OzonAttributeSelection[] {
  const values = (Array.isArray(raw) ? raw : [raw])
    .map((value) => String(value ?? "").trim())
    .filter(Boolean);
  // Preserve every editor value here. The backend validates is_collection and
  // max_value_count authoritatively; silently slicing would turn an invalid
  // required/multi-value configuration into a different platform payload.
  return values.map((value) => {
    if (!attribute.dictionaryId) return { value };
    const option = attribute.options?.find((item) => item.id === value);
    return option
      ? { value: option.value, dictionaryValueId: option.id }
      : { value };
  });
}

/** Moves historical flat complex values into the repeated-group editor shape. */
export function normalizeOzonAttributeEditorValues(
  attributes: OzonRichAttributeShape[],
  editor: OzonAttributeEditorValues = {},
): OzonAttributeEditorValues {
  const normalized: OzonAttributeEditorValues = {
    attributes: { ...(editor.attributes || {}) },
    complexGroups: Object.fromEntries(
      Object.entries(editor.complexGroups || {}).map(([key, groups]) => [
        key,
        groups.map((group) => ({ ...group })),
      ]),
    ),
  };
  const byComplex = new Map<number, OzonRichAttributeShape[]>();
  attributes.forEach((attribute) => {
    const complexId = Number(attribute.attributeComplexId || 0);
    if (complexId <= 0) return;
    byComplex.set(complexId, [...(byComplex.get(complexId) || []), attribute]);
  });
  byComplex.forEach((defs, complexId) => {
    const legacyGroup: Record<string, string | string[]> = {};
    defs.forEach((attribute) => {
      const value = normalized.attributes?.[attribute.attrId];
      if (editorSelections(attribute, value).length > 0) {
        legacyGroup[attribute.attrId] = value!;
      }
      delete normalized.attributes?.[attribute.attrId];
    });
    if (Object.keys(legacyGroup).length === 0) return;
    const key = String(complexId);
    const explicitGroups = normalized.complexGroups?.[key] || [];
    const hasExplicitValues = explicitGroups.some((group) =>
      defs.some(
        (attribute) =>
          editorSelections(attribute, group[attribute.attrId]).length > 0,
      ),
    );
    normalized.complexGroups![key] = hasExplicitValues
      ? [...explicitGroups, legacyGroup]
      : [legacyGroup];
  });
  return normalized;
}

/** Builds the canonical payload that preserves Ozon multi-value and repeated complex groups. */
export function buildOzonPlatformAttributesV2(
  attributes: OzonRichAttributeShape[],
  editor: OzonAttributeEditorValues = {},
): OzonPlatformAttributePayload {
  const normalizedEditor = normalizeOzonAttributeEditorValues(
    attributes,
    editor,
  );
  const result: OzonPlatformAttributePayload = {
    version: 2,
    attributes: {},
    complexGroups: [],
  };
  const byComplex = new Map<number, OzonRichAttributeShape[]>();
  attributes.forEach((attribute) => {
    const complexId = Number(attribute.attributeComplexId || 0);
    if (complexId > 0) {
      byComplex.set(complexId, [
        ...(byComplex.get(complexId) || []),
        attribute,
      ]);
      return;
    }
    const selections = editorSelections(
      attribute,
      normalizedEditor.attributes?.[attribute.attrId],
    );
    if (selections.length > 0) result.attributes[attribute.attrId] = selections;
  });
  Array.from(byComplex.entries())
    .sort(([left], [right]) => left - right)
    .forEach(([complexId, defs]) => {
      const groups = normalizedEditor.complexGroups?.[String(complexId)] || [];
      groups.forEach((group) => {
        const values: Record<string, OzonAttributeSelection[]> = {};
        defs.forEach((attribute) => {
          const selections = editorSelections(
            attribute,
            group[attribute.attrId],
          );
          if (selections.length > 0) values[attribute.attrId] = selections;
        });
        if (Object.keys(values).length > 0) {
          result.complexGroups.push({ complexId, attributes: values });
        }
      });
    });
  return result;
}

/** Restores canonical v2 or historical single-value payloads into editor values. */
export function toOzonAttributeEditorValues(
  raw?: Record<string, unknown>,
): OzonAttributeEditorValues {
  const editor: OzonAttributeEditorValues = {
    attributes: {},
    complexGroups: {},
  };
  if (!raw) return editor;
  const isV2 =
    raw.version === 2 &&
    ((raw.attributes && typeof raw.attributes === "object") ||
      Array.isArray(raw.complexGroups));
  if (!isV2) {
    Object.entries(raw).forEach(([attrId, value]) => {
      if (["version", "attributes", "complexGroups"].includes(attrId)) return;
      const values = (Array.isArray(value) ? value : [value])
        .map((item) => {
          if (typeof item === "string" || typeof item === "number")
            return String(item).trim();
          if (!item || typeof item !== "object") return "";
          const typed = item as {
            value?: unknown;
            dictionaryValueId?: unknown;
          };
          return String(typed.dictionaryValueId ?? typed.value ?? "").trim();
        })
        .filter(Boolean);
      if (values.length === 1) editor.attributes![attrId] = values[0];
      else if (values.length > 1) editor.attributes![attrId] = values;
    });
    return editor;
  }
  const payload = raw as unknown as OzonPlatformAttributePayload;
  Object.entries(payload.attributes || {}).forEach(([attrId, values]) => {
    const restored = (values || [])
      .map((value) =>
        String(value.dictionaryValueId ?? value.value ?? "").trim(),
      )
      .filter(Boolean);
    if (restored.length === 1) editor.attributes![attrId] = restored[0];
    else if (restored.length > 1) editor.attributes![attrId] = restored;
  });
  (payload.complexGroups || []).forEach((group) => {
    const groupKey = String(group.complexId);
    const values: Record<string, string | string[]> = {};
    Object.entries(group.attributes || {}).forEach(([attrId, selections]) => {
      const restored = (selections || [])
        .map((selection) =>
          String(selection.dictionaryValueId ?? selection.value ?? "").trim(),
        )
        .filter(Boolean);
      if (restored.length === 1) values[attrId] = restored[0];
      else if (restored.length > 1) values[attrId] = restored;
    });
    if (Object.keys(values).length === 0) return;
    editor.complexGroups![groupKey] = [
      ...(editor.complexGroups![groupKey] || []),
      values,
    ];
  });
  return editor;
}

const enc = encodeURIComponent;

export function getOzonCategoryFlowStats() {
  return getJSON<OzonCategoryFlowStats>(
    "/api/v1/platform/ozon/categories/stats",
  );
}
export function syncOzonCategoryFlow(shopId?: string) {
  return postJSON<OzonCategorySyncStart>(
    "/api/v1/platform/ozon/categories/sync",
    shopId ? { shopId } : {},
  );
}
export function listOzonCategorySyncRuns() {
  return getJSON<{ list: OzonCategorySyncRun[] }>(
    "/api/v1/platform/ozon/categories/sync-runs",
  );
}
export function listOzonCategoryChanges() {
  return getJSON<{ list: OzonCategoryChange[] }>(
    "/api/v1/platform/ozon/categories/changes",
  );
}
export function listOzonCategoryMappings(shopId?: string) {
  return getWithParams<{ list: OzonCategoryMapping[] }>(
    "/api/v1/platform/ozon/category-mappings",
    { shopId },
  );
}
export function recommendOzonCategory(body: {
  shopId?: string;
  sourceCategoryKey: string;
  sourceCategoryName?: string;
}) {
  return postJSON<OzonCategoryRecommendation>(
    "/api/v1/platform/ozon/category-mappings/recommend",
    body,
  );
}
export function saveOzonCategoryMapping(body: OzonCategoryMappingInput) {
  return putJSON<OzonCategoryMapping, OzonCategoryMappingInput>(
    "/api/v1/platform/ozon/category-mappings",
    body,
  );
}
export function getOzonProductConfig(productId: string, shopId?: string) {
  return getWithParams<OzonProductConfig>(
    `/api/v1/products/${enc(productId)}/platform-configs/ozon`,
    shopId ? { shopId } : {},
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
    { platform: "ozon", shopId },
  );
}
export function checkOzonCategoryGroups(body: {
  productIds: string[];
  shopId?: string;
}) {
  return postJSON<{ groups: OzonCategoryGroup[] }>(
    "/api/v1/product-publish/ozon/category-groups/check",
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
    "/api/v1/product-publish/ozon/category-groups/confirm",
    body,
  );
}

export async function publishOzonProduct(
  productId: string,
  shopId: string,
  idempotencyKey: string,
): Promise<ProductPublishTaskDTO> {
  return postJSON<ProductPublishTaskDTO>(
    `/api/v1/products/${enc(productId)}/publish`,
    { shopId, options: { platform: "ozon" } },
    {
      headers: { "Idempotency-Key": idempotencyKey },
    },
  );
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
  }>("/api/v1/platform/ozon/categories", {
    keyword,
    onlyLeaf: "1",
    activeOnly: "1",
    limit: "100",
  });
}

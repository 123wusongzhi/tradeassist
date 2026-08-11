import type { OzonCategoryAttribute } from "@/services/ozonCategories";
import type {
  OzonAttributeSuggestion,
  OzonAttributeSuggestionConfidenceLevel,
} from "@/services/ozonPublish";

export type OzonAIAttributeMarker = {
  attributeId: string;
  attributeName: string;
  confidenceLevel: OzonAttributeSuggestionConfidenceLevel;
  requiresReview: boolean;
  reason?: string;
  appliedValue: string | string[];
};

export type OzonAIAttributeRejected = {
  attributeId: string;
  attributeName: string;
  reason: string;
};

export type OzonAIAttributeMergeResult = {
  attributes: Record<string, string | string[]>;
  applied: Record<string, OzonAIAttributeMarker>;
  rejected: OzonAIAttributeRejected[];
  filled: number;
  requiresReview: number;
};

export type OzonAIAttributeUndoResult = {
  attributes: Record<string, string | string[]>;
  restoredAttributeIds: string[];
  preservedManualAttributeIds: string[];
};

const mediumConfidenceThreshold = 0.55;
const highConfidenceThreshold = 0.8;

export function isOzonEditorValueFilled(value: unknown) {
  if (Array.isArray(value))
    return value.some((item) => String(item ?? "").trim() !== "");
  return String(value ?? "").trim() !== "";
}

function cloneValue(value: string | string[]) {
  return Array.isArray(value) ? [...value] : value;
}

export function cloneOzonEditorAttributes(
  attributes?: Record<string, string | string[]>,
) {
  return Object.fromEntries(
    Object.entries(attributes || {}).map(([attributeId, value]) => [
      attributeId,
      cloneValue(value),
    ]),
  );
}

function normalizedValueType(attribute: OzonCategoryAttribute) {
  return String(attribute.valueType || "")
    .trim()
    .toLowerCase();
}

function validateTypedValue(
  attribute: OzonCategoryAttribute,
  value: string,
): string | undefined {
  const valueType = normalizedValueType(attribute);
  if (["string", "text"].includes(valueType)) return undefined;
  if (["integer", "int", "int64"].includes(valueType)) {
    if (!/^[+-]?\d+$/.test(value)) return "必须填写整数";
    try {
      const parsed = BigInt(value);
      if (
        parsed < BigInt("-9223372036854775808") ||
        parsed > BigInt("9223372036854775807")
      )
        return "必须在 64 位整数范围内";
    } catch {
      return "必须填写整数";
    }
    return undefined;
  }
  if (["decimal", "float", "double", "number"].includes(valueType))
    return /^[+-]?(?:\d+(?:\.\d*)?|\.\d+)$/.test(value)
      ? undefined
      : "必须填写有限十进制数，不能使用科学计数法";
  if (["boolean", "bool"].includes(valueType))
    return value === "true" || value === "false" ? undefined : "必须选择是或否";
  if (["url", "uri", "image"].includes(valueType)) {
    try {
      const parsed = new URL(value);
      return parsed.host &&
        (parsed.protocol === "http:" || parsed.protocol === "https:")
        ? undefined
        : "必须填写完整的 http/https URL";
    } catch {
      return "必须填写完整的 http/https URL";
    }
  }
  if (valueType === "date")
    return isCalendarDate(value) ? undefined : "必须使用有效的 YYYY-MM-DD 日期";
  if (["datetime", "date_time", "timestamp"].includes(valueType))
    return isRFC3339DateTime(value)
      ? undefined
      : "必须使用带时区的 RFC3339 日期时间";
  return `系统尚不支持 valueType=${attribute.valueType || "未知"}`;
}

function isCalendarDate(value: string): boolean {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return false;
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const parsed = new Date(`${value}T00:00:00Z`);
  return (
    !Number.isNaN(parsed.getTime()) &&
    parsed.getUTCFullYear() === year &&
    parsed.getUTCMonth() + 1 === month &&
    parsed.getUTCDate() === day
  );
}

function isRFC3339DateTime(value: string): boolean {
  const match =
    /^(\d{4}-\d{2}-\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(Z|([+-])(\d{2}):(\d{2}))$/.exec(
      value,
    );
  if (!match || !isCalendarDate(match[1])) return false;
  const hour = Number(match[2]);
  const minute = Number(match[3]);
  const second = Number(match[4]);
  const offsetHour = match[7] === undefined ? 0 : Number(match[7]);
  const offsetMinute = match[8] === undefined ? 0 : Number(match[8]);
  return (
    hour <= 23 &&
    minute <= 59 &&
    second <= 59 &&
    offsetHour <= 23 &&
    offsetMinute <= 59 &&
    !Number.isNaN(Date.parse(value))
  );
}

function validateSuggestion(
  attribute: OzonCategoryAttribute,
  suggestion: OzonAttributeSuggestion,
): { value?: string | string[]; issue?: string } {
  if (Number(attribute.attributeComplexId || 0) > 0)
    return { issue: "组合属性不在 AI 自动填写范围内" };
  if (
    !Number.isFinite(suggestion.confidence) ||
    suggestion.confidence < 0 ||
    suggestion.confidence > 1
  )
    return { issue: "建议可信度无效" };
  const expectedLevel =
    suggestion.confidence >= highConfidenceThreshold
      ? "high"
      : suggestion.confidence >= mediumConfidenceThreshold
        ? "medium"
        : "low";
  if (
    suggestion.confidenceLevel !== expectedLevel ||
    suggestion.requiresReview !== (expectedLevel !== "high")
  )
    return { issue: "建议可信度等级不一致" };
  if (!Array.isArray(suggestion.values) || suggestion.values.length === 0)
    return { issue: "建议没有可用值" };
  if (!attribute.isCollection && suggestion.values.length !== 1)
    return { issue: "当前属性不是多值属性" };
  if (
    attribute.isCollection &&
    (!attribute.maxValueCount || attribute.maxValueCount <= 0)
  )
    return { issue: "多值属性缺少明确上限" };
  if (
    attribute.maxValueCount &&
    suggestion.values.length > attribute.maxValueCount
  )
    return { issue: `最多允许 ${attribute.maxValueCount} 个值` };
  if (suggestion.values.length > 50) return { issue: "建议值数量超过安全上限" };

  const values: string[] = [];
  const seen = new Set<string>();
  for (const selection of suggestion.values) {
    const value = String(selection?.value || "").trim();
    if (!value) return { issue: "建议包含空值" };
    if (attribute.dictionaryId) {
      const dictionaryValueId = String(
        selection?.dictionaryValueId || "",
      ).trim();
      if (!dictionaryValueId) return { issue: "词典建议缺少官方选项 ID" };
      const option = (attribute.options || []).find(
        (item) => item.id === dictionaryValueId,
      );
      if (!option || option.value !== value)
        return { issue: "词典建议不属于当前模板选项" };
      if (seen.has(dictionaryValueId)) return { issue: "建议包含重复词典值" };
      seen.add(dictionaryValueId);
      values.push(dictionaryValueId);
      continue;
    }
    if (selection?.dictionaryValueId)
      return { issue: "普通属性不能携带词典选项 ID" };
    const typedIssue = validateTypedValue(attribute, value);
    if (typedIssue) return { issue: typedIssue };
    if (seen.has(value)) return { issue: "建议包含重复值" };
    seen.add(value);
    values.push(value);
  }
  return { value: attribute.isCollection ? values : values[0] };
}

export function mergeOzonAIAttributeSuggestions(options: {
  template: OzonCategoryAttribute[];
  currentAttributes?: Record<string, string | string[]>;
  selectedVariantAttributeIds?: string[];
  hasMultipleSKUs?: boolean;
  suggestions: OzonAttributeSuggestion[];
}): OzonAIAttributeMergeResult {
  const attributes = cloneOzonEditorAttributes(options.currentAttributes);
  const templateById = new Map(
    options.template.map((attribute) => [attribute.attrId, attribute]),
  );
  const selectedVariants = new Set(options.selectedVariantAttributeIds || []);
  const suggestionCounts = new Map<string, number>();
  (options.suggestions || []).forEach((suggestion) => {
    const id = String(suggestion.attributeId || "").trim();
    suggestionCounts.set(id, (suggestionCounts.get(id) || 0) + 1);
  });
  const applied: Record<string, OzonAIAttributeMarker> = {};
  const rejected: OzonAIAttributeRejected[] = [];

  (options.suggestions || []).forEach((suggestion) => {
    const attributeId = String(suggestion.attributeId || "").trim();
    const attributeName = String(
      suggestion.attributeName || attributeId,
    ).trim();
    const reject = (reason: string) =>
      rejected.push({ attributeId, attributeName, reason });
    if (!attributeId || suggestionCounts.get(attributeId) !== 1) {
      reject("AI 对同一属性返回了空或冲突引用");
      return;
    }
    const attribute = templateById.get(attributeId);
    if (!attribute) {
      reject("建议属性已不在当前模板中");
      return;
    }
    if (
      selectedVariants.has(attributeId) ||
      (options.hasMultipleSKUs && attribute.skuVariantEligible)
    ) {
      reject("SKU 变体属性不在普通商品级 AI 回填范围内");
      return;
    }
    if (isOzonEditorValueFilled(attributes[attributeId])) {
      reject("当前字段已有值，未覆盖");
      return;
    }
    const validated = validateSuggestion(attribute, suggestion);
    if (validated.issue || validated.value === undefined) {
      reject(validated.issue || "建议未通过当前模板校验");
      return;
    }
    attributes[attributeId] = cloneValue(validated.value);
    applied[attributeId] = {
      attributeId,
      attributeName: attribute.name || attributeName,
      confidenceLevel: suggestion.confidenceLevel,
      requiresReview: suggestion.requiresReview,
      reason: suggestion.reason,
      appliedValue: cloneValue(validated.value),
    };
  });

  const markers = Object.values(applied);
  return {
    attributes,
    applied,
    rejected,
    filled: markers.length,
    requiresReview: markers.filter((marker) => marker.requiresReview).length,
  };
}

function valuesEqual(
  left: string | string[] | undefined,
  right: string | string[] | undefined,
) {
  if (Array.isArray(left) || Array.isArray(right))
    return (
      Array.isArray(left) &&
      Array.isArray(right) &&
      left.length === right.length &&
      left.every((value, index) => value === right[index])
    );
  return left === right;
}

export function undoOzonAIAttributeSuggestions(options: {
  currentAttributes?: Record<string, string | string[]>;
  snapshotAttributes?: Record<string, string | string[]>;
  applied: Record<string, OzonAIAttributeMarker>;
}): OzonAIAttributeUndoResult {
  const attributes = cloneOzonEditorAttributes(options.currentAttributes);
  const snapshot = cloneOzonEditorAttributes(options.snapshotAttributes);
  const restoredAttributeIds: string[] = [];
  const preservedManualAttributeIds: string[] = [];
  Object.entries(options.applied).forEach(([attributeId, marker]) => {
    if (!valuesEqual(attributes[attributeId], marker.appliedValue)) {
      preservedManualAttributeIds.push(attributeId);
      return;
    }
    if (Object.prototype.hasOwnProperty.call(snapshot, attributeId))
      attributes[attributeId] = cloneValue(snapshot[attributeId]);
    else delete attributes[attributeId];
    restoredAttributeIds.push(attributeId);
  });
  return { attributes, restoredAttributeIds, preservedManualAttributeIds };
}

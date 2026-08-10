import type { OzonCategoryAttribute } from "@/services/ozonCategories";

export type OzonAttributeFilter = "all" | "required" | "unfilled";

export type OzonAttributeProgress = {
  total: number;
  requiredTotal: number;
  requiredCompleted: number;
  filled: number;
};

function withMultiplicity(attribute: OzonCategoryAttribute, label?: string) {
  if (!attribute.isCollection) return label;
  return label ? `多值${label}` : "多值";
}

export function ozonAttributeFormatHint(
  attribute: OzonCategoryAttribute,
): string | undefined {
  if (attribute.dictionaryId)
    return attribute.isCollection ? "多选" : undefined;

  const valueType = String(attribute.valueType || "")
    .trim()
    .toLowerCase();
  if (["string", "text"].includes(valueType))
    return attribute.isCollection ? "多值文本" : undefined;
  if (["integer", "int", "int64"].includes(valueType))
    return withMultiplicity(attribute, "整数");
  if (["decimal", "float", "double", "number"].includes(valueType))
    return withMultiplicity(attribute, "数值");
  if (["boolean", "bool"].includes(valueType))
    return withMultiplicity(attribute, "是/否");
  if (["url", "uri", "image"].includes(valueType))
    return withMultiplicity(attribute, "链接");
  if (valueType === "date") return withMultiplicity(attribute, "日期");
  if (["datetime", "date_time", "timestamp"].includes(valueType))
    return withMultiplicity(attribute, "日期时间");
  return attribute.isCollection ? "多值" : undefined;
}

export function matchesOzonAttributeView(
  attribute: OzonCategoryAttribute,
  options: {
    filter: OzonAttributeFilter;
    query: string;
    filled: boolean;
  },
) {
  if (options.filter === "required" && !attribute.required) return false;
  if (options.filter === "unfilled" && options.filled) return false;

  const query = options.query.trim().toLocaleLowerCase();
  if (!query) return true;
  const searchable = [
    attribute.name,
    attribute.description,
    attribute.valueType,
    ozonAttributeFormatHint(attribute),
  ]
    .filter(Boolean)
    .join(" ")
    .toLocaleLowerCase();
  return searchable.includes(query);
}

export function summarizeOzonAttributes(
  attributes: OzonCategoryAttribute[],
  filledAttributeIDs: ReadonlySet<string>,
  completedRequiredAttributeIDs: ReadonlySet<string>,
): OzonAttributeProgress {
  const requiredAttributes = attributes.filter(
    (attribute) => attribute.required,
  );
  return {
    total: attributes.length,
    requiredTotal: requiredAttributes.length,
    requiredCompleted: requiredAttributes.filter((attribute) =>
      completedRequiredAttributeIDs.has(attribute.attrId),
    ).length,
    filled: attributes.filter((attribute) =>
      filledAttributeIDs.has(attribute.attrId),
    ).length,
  };
}

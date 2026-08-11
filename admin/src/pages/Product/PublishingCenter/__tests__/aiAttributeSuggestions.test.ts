import { describe, expect, it } from "vitest";
import type { OzonCategoryAttribute } from "@/services/ozonCategories";
import type { OzonAttributeSuggestion } from "@/services/ozonPublish";
import {
  mergeOzonAIAttributeSuggestions,
  undoOzonAIAttributeSuggestions,
} from "../aiAttributeSuggestions";

function attribute(
  attrId: string,
  overrides: Partial<OzonCategoryAttribute> = {},
): OzonCategoryAttribute {
  return {
    id: `template-${attrId}`,
    categoryId: "100:200",
    attrId,
    name: `属性 ${attrId}`,
    required: false,
    valueType: "String",
    isCollection: false,
    complexIsCollection: false,
    categoryDependent: false,
    ...overrides,
  };
}

function suggestion(
  attributeId: string,
  value: string,
  overrides: Partial<OzonAttributeSuggestion> = {},
): OzonAttributeSuggestion {
  return {
    attributeId,
    attributeName: `属性 ${attributeId}`,
    values: [{ value }],
    confidence: 0.9,
    confidenceLevel: "high",
    requiresReview: false,
    ...overrides,
  };
}

describe("Ozon AI attribute suggestion merge", () => {
  it("fills only blank ordinary fields and marks medium confidence for review", () => {
    const result = mergeOzonAIAttributeSuggestions({
      template: [
        attribute("brand"),
        attribute("count", { valueType: "Integer" }),
        attribute("automatic", { valueType: "Boolean" }),
      ],
      currentAttributes: { brand: "用户品牌" },
      suggestions: [
        suggestion("brand", "AI 品牌"),
        suggestion("count", "12"),
        suggestion("automatic", "true", {
          confidence: 0.7,
          confidenceLevel: "medium",
          requiresReview: true,
        }),
      ],
    });

    expect(result.attributes).toEqual({
      brand: "用户品牌",
      count: "12",
      automatic: "true",
    });
    expect(result.filled).toBe(2);
    expect(result.requiresReview).toBe(1);
    expect(result.applied.brand).toBeUndefined();
    expect(result.rejected).toContainEqual(
      expect.objectContaining({
        attributeId: "brand",
        reason: expect.stringContaining("未覆盖"),
      }),
    );
  });

  it("rejects invalid types, collections, dictionary ids, variants, and complex fields", () => {
    const result = mergeOzonAIAttributeSuggestions({
      template: [
        attribute("integer", { valueType: "Integer" }),
        attribute("colors", {
          valueType: "String",
          dictionaryId: "colors",
          options: [{ id: "10", value: "红色" }],
        }),
        attribute("tags", {
          isCollection: true,
          maxValueCount: 1,
        }),
        attribute("variant"),
        attribute("potentialVariant", { skuVariantEligible: true }),
        attribute("complex", { attributeComplexId: 501 }),
      ],
      selectedVariantAttributeIds: ["variant"],
      hasMultipleSKUs: true,
      suggestions: [
        suggestion("integer", "12.5"),
        suggestion("colors", "红色", {
          values: [{ value: "红色", dictionaryValueId: "999" }],
        }),
        suggestion("tags", "a", {
          values: [{ value: "a" }, { value: "b" }],
        }),
        suggestion("variant", "M"),
        suggestion("potentialVariant", "L"),
        suggestion("complex", "组合值"),
      ],
    });

    expect(result.filled).toBe(0);
    expect(result.rejected).toHaveLength(6);
    expect(result.attributes).toEqual({});
  });

  it("accepts only a current official dictionary option and stores its editor id", () => {
    const result = mergeOzonAIAttributeSuggestions({
      template: [
        attribute("color", {
          dictionaryId: "colors",
          options: [{ id: "10", value: "红色" }],
        }),
      ],
      suggestions: [
        suggestion("color", "红色", {
          values: [{ value: "红色", dictionaryValueId: "10" }],
        }),
      ],
    });
    expect(result.attributes.color).toBe("10");
    expect(result.filled).toBe(1);
  });

  it("rejects impossible dates and non-RFC3339 datetimes", () => {
    const result = mergeOzonAIAttributeSuggestions({
      template: [
        attribute("date", { valueType: "Date" }),
        attribute("datetime", { valueType: "DateTime" }),
        attribute("validDate", { valueType: "Date" }),
        attribute("validDatetime", { valueType: "DateTime" }),
      ],
      suggestions: [
        suggestion("date", "2026-99-99"),
        suggestion("datetime", "2026-08-11 10:30:00"),
        suggestion("validDate", "2024-02-29"),
        suggestion("validDatetime", "2026-08-11T10:30:00+08:00"),
      ],
    });

    expect(result.attributes).toEqual({
      validDate: "2024-02-29",
      validDatetime: "2026-08-11T10:30:00+08:00",
    });
    expect(result.rejected).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ attributeId: "date" }),
        expect.objectContaining({ attributeId: "datetime" }),
      ]),
    );
  });

  it("undoes only untouched AI values and preserves later manual edits", () => {
    const merged = mergeOzonAIAttributeSuggestions({
      template: [attribute("one"), attribute("two")],
      currentAttributes: { original: "保留" },
      suggestions: [suggestion("one", "AI-1"), suggestion("two", "AI-2")],
    });
    const undone = undoOzonAIAttributeSuggestions({
      currentAttributes: {
        ...merged.attributes,
        two: "用户后来修改",
        unrelated: "请求后填写",
      },
      snapshotAttributes: { original: "保留" },
      applied: merged.applied,
    });

    expect(undone.attributes).toEqual({
      original: "保留",
      two: "用户后来修改",
      unrelated: "请求后填写",
    });
    expect(undone.restoredAttributeIds).toEqual(["one"]);
    expect(undone.preservedManualAttributeIds).toEqual(["two"]);
  });
});

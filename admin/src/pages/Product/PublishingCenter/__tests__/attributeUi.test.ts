import { describe, expect, it } from "vitest";
import type { OzonCategoryAttribute } from "@/services/ozonCategories";
import {
  matchesOzonAttributeView,
  ozonAttributeFormatHint,
  summarizeOzonAttributes,
} from "../attributeUi";

function attribute(
  attrId: string,
  overrides: Partial<OzonCategoryAttribute> = {},
): OzonCategoryAttribute {
  return {
    id: `template-${attrId}`,
    categoryId: "dynamic-leaf",
    attrId,
    name: `动态属性 ${attrId}`,
    required: false,
    valueType: "String",
    isCollection: false,
    complexIsCollection: false,
    categoryDependent: false,
    ...overrides,
  };
}

describe("publishing center Ozon attribute UI", () => {
  it("turns useful template types into Chinese hints without exposing raw String tags", () => {
    expect(ozonAttributeFormatHint(attribute("text"))).toBeUndefined();
    expect(
      ozonAttributeFormatHint(attribute("integer", { valueType: "Integer" })),
    ).toBe("整数");
    expect(
      ozonAttributeFormatHint(
        attribute("links", { valueType: "URL", isCollection: true }),
      ),
    ).toBe("多值链接");
    expect(
      ozonAttributeFormatHint(
        attribute("choice", {
          dictionaryId: "server-dictionary",
          isCollection: true,
        }),
      ),
    ).toBe("多选");
  });

  it("filters only presentation using dynamic template metadata and fill state", () => {
    const required = attribute("required", {
      name: "动态必填项",
      description: "来自当前叶子模板",
      required: true,
    });
    const optional = attribute("optional", {
      name: "动态可选项",
      valueType: "Boolean",
    });

    expect(
      matchesOzonAttributeView(required, {
        filter: "required",
        query: "叶子模板",
        filled: false,
      }),
    ).toBe(true);
    expect(
      matchesOzonAttributeView(optional, {
        filter: "required",
        query: "",
        filled: false,
      }),
    ).toBe(false);
    expect(
      matchesOzonAttributeView(optional, {
        filter: "unfilled",
        query: "是/否",
        filled: true,
      }),
    ).toBe(false);
  });

  it("summarizes arbitrary template attributes from independent fill sets", () => {
    const attributes = [
      attribute("one", { required: true }),
      attribute("two", { required: true, attributeComplexId: 900 }),
      attribute("three"),
    ];

    expect(
      summarizeOzonAttributes(
        attributes,
        new Set(["one", "three"]),
        new Set(["one"]),
      ),
    ).toEqual({
      total: 3,
      requiredTotal: 2,
      requiredCompleted: 1,
      filled: 2,
    });
  });
});

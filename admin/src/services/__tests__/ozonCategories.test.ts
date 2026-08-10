import { request } from "@umijs/max";
import { describe, expect, it, vi } from "vitest";
import {
  getOzonAttributeMappings,
  putOzonAttributeMappings,
  queryOzonCategories,
  queryOzonCategoryAttributes,
  searchOzonDictionaryValues,
  syncOzonCategories,
  syncOzonCategoryAttributes,
} from "../ozonCategories";

const requestMock = vi.mocked(request);

function envelope<T>(data: T) {
  return { code: 0, message: "ok", data };
}

describe("ozon category services", () => {
  it("queries leaf categories with query params", async () => {
    requestMock.mockResolvedValueOnce(
      envelope({
        list: [
          {
            id: "cat-1",
            categoryId: "100:200",
            name: "Стол",
            level: 2,
            isLeaf: true,
          },
        ],
        total: 1,
        leafCount: 1,
        matchedTotal: 1,
        offset: 20,
        limit: 20,
        lastSyncedAt: "2026-08-10T00:00:00Z",
        cacheStale: false,
      }),
    );
    const res = await queryOzonCategories({
      keyword: "Стол",
      onlyLeaf: true,
      activeOnly: true,
      limit: 20,
      offset: 20,
    });
    expect(res.leafCount).toBe(1);
    expect(res.matchedTotal).toBe(1);
    expect(res.cacheStale).toBe(false);
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/platform/ozon/categories",
      {
        method: "GET",
        params: {
          keyword: "Стол",
          onlyLeaf: "1",
          activeOnly: "1",
          limit: "20",
          offset: "20",
        },
      },
    );
  });

  it("queries one active hierarchy level by parent id", async () => {
    requestMock.mockResolvedValueOnce(
      envelope({
        list: [
          {
            id: "storage",
            categoryId: "storage",
            parentId: "home",
            name: "收纳",
            level: 2,
            isLeaf: false,
            hasChildren: true,
            childCount: 59,
            ancestors: [{ categoryId: "home", name: "住宅和花园", level: 1 }],
          },
        ],
        total: 7377,
        leafCount: 7000,
        matchedTotal: 1,
        offset: 0,
        limit: 200,
        cacheStale: false,
      }),
    );
    const result = await queryOzonCategories({
      parentId: "home",
      activeOnly: true,
      limit: 200,
    });
    expect(result.list[0]).toMatchObject({
      categoryId: "storage",
      hasChildren: true,
      childCount: 59,
    });
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/platform/ozon/categories",
      {
        method: "GET",
        params: {
          keyword: undefined,
          parentId: "home",
          rootOnly: undefined,
          onlyLeaf: undefined,
          activeOnly: "1",
          limit: "200",
          offset: undefined,
        },
      },
    );
  });

  it("queries active root categories explicitly", async () => {
    requestMock.mockResolvedValueOnce(
      envelope({
        list: [],
        total: 0,
        leafCount: 0,
        matchedTotal: 0,
        offset: 0,
        limit: 200,
        cacheStale: true,
      }),
    );
    await queryOzonCategories({ rootOnly: true, activeOnly: true, limit: 200 });
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/platform/ozon/categories",
      {
        method: "GET",
        params: {
          keyword: undefined,
          parentId: undefined,
          rootOnly: "1",
          onlyLeaf: undefined,
          activeOnly: "1",
          limit: "200",
          offset: undefined,
        },
      },
    );
  });

  it("syncs category tree with optional shop id", async () => {
    requestMock.mockResolvedValueOnce(
      envelope({
        stats: { count: 2, leafCount: 1 },
        run: { id: "run-1", status: "pending" },
        runId: "run-1",
      }),
    );
    const started = await syncOzonCategories("shop-1");
    expect(started.runId).toBe("run-1");
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/platform/ozon/categories/sync",
      {
        method: "POST",
        data: { shopId: "shop-1" },
      },
    );
    requestMock.mockResolvedValueOnce(
      envelope({ stats: { count: 2, leafCount: 1 } }),
    );
    await syncOzonCategories();
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/platform/ozon/categories/sync",
      {
        method: "POST",
        data: {},
      },
    );
  });

  it("searches dictionary values with category, attribute and shop scope", async () => {
    requestMock.mockResolvedValueOnce(
      envelope({ list: [{ id: "42", value: "Acme" }] }),
    );
    const result = await searchOzonDictionaryValues(
      "100:200",
      "85",
      "shop-1",
      "Acme",
    );
    expect(result.list[0]).toEqual({ id: "42", value: "Acme" });
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/platform/ozon/categories/100%3A200/attributes/85/values",
      {
        method: "GET",
        params: { shopId: "shop-1", keyword: "Acme" },
      },
    );
  });

  it("syncs attributes for a leaf category", async () => {
    requestMock.mockResolvedValueOnce(envelope({ count: 3 }));
    await syncOzonCategoryAttributes("cat-1");
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/platform/ozon/categories/cat-1/attributes/sync",
      {
        method: "POST",
        data: {},
      },
    );
  });

  it("preserves authoritative SKU eligibility and variant policy", async () => {
    requestMock.mockResolvedValueOnce(
      envelope({
        list: [
          {
            id: "brand",
            categoryId: "100:200",
            attrId: "85",
            name: "Brand",
            required: true,
            valueType: "String",
            skuVariantEligible: false,
            skuVariantEligibilityKnown: true,
            isCollection: false,
            complexIsCollection: false,
            categoryDependent: false,
          },
          {
            id: "color",
            categoryId: "100:200",
            attrId: "10096",
            name: "Color",
            required: true,
            valueType: "String",
            skuVariantEligible: true,
            skuVariantEligibilityKnown: true,
            isCollection: false,
            complexIsCollection: false,
            categoryDependent: false,
          },
        ],
        variantPolicy: {
          maxSkuCount: 100,
          maxVariantAttributeCount: 1,
          maxVariantCombinationCount: 100,
          eligibleAttributeCount: 1,
          variantEligibilityFullyKnown: true,
          source: "ozon_is_aspect+trademind_import_guardrail",
        },
      }),
    );
    const result = await queryOzonCategoryAttributes("100:200");
    expect(result.list.map((item) => item.skuVariantEligible)).toEqual([
      false,
      true,
    ]);
    expect(result.variantPolicy).toMatchObject({
      maxSkuCount: 100,
      maxVariantAttributeCount: 1,
      maxVariantCombinationCount: 100,
    });
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/platform/ozon/categories/100%3A200/attributes",
      { method: "GET" },
    );
  });

  it("round-trips attribute mappings via PUT", async () => {
    requestMock.mockResolvedValueOnce(
      envelope({
        list: [{ attributeId: "85", localField: "brand", enabled: true }],
      }),
    );
    const got = await getOzonAttributeMappings("cat-1");
    expect(got.list[0].localField).toBe("brand");
    requestMock.mockResolvedValueOnce(
      envelope({
        list: [
          {
            attributeId: "85",
            attributeName: "Бренд",
            localField: "brand_name",
            enabled: true,
          },
        ],
      }),
    );
    const saved = await putOzonAttributeMappings("cat-1", [
      {
        attributeId: "85",
        attributeName: "Бренд",
        localField: "brand_name",
        enabled: true,
      },
    ]);
    expect(saved.list[0].localField).toBe("brand_name");
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/platform/ozon/categories/cat-1/attribute-mappings",
      {
        method: "PUT",
        data: {
          items: [
            {
              attributeId: "85",
              attributeName: "Бренд",
              localField: "brand_name",
              enabled: true,
            },
          ],
        },
      },
    );
  });
});

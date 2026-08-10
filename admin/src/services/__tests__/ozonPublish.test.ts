import { request } from "@umijs/max";
import { describe, expect, it, vi } from "vitest";
import {
  buildOzonPlatformAttributes,
  autoMatchOzonSKUAttributes,
  buildOzonPlatformAttributesV3,
  buildOzonSKUImagePreview,
  confirmOzonCategoryGroup,
  getOzonProductConfig,
  listOzonWarehouses,
  normalizeOzonAttributeEditorValues,
  ozonSKUVariantTuple,
  publishOzonProduct,
  saveOzonProductConfig,
  searchOzonLeafCategories,
  syncOzonCategoryFlow,
  toOzonImageConfigInput,
  toOzonAttributeFormValues,
  toOzonAttributeEditorValues,
  validateOzonReadiness,
} from "../ozonPublish";

const requestMock = vi.mocked(request);

describe("ozon publish services", () => {
  it("searches Ozon leaf categories with pagination while preserving string callers", async () => {
    requestMock.mockResolvedValue({
      code: 0,
      message: "ok",
      data: {
        list: [],
        total: 20000,
        leafCount: 18000,
        matchedTotal: 135,
        offset: 50,
        limit: 50,
        cacheStale: false,
      },
    });
    const result = await searchOzonLeafCategories({
      keyword: "住宅和花园 / 收纳",
      limit: 50,
      offset: 50,
    });
    expect(result.matchedTotal).toBe(135);
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/platform/ozon/categories",
      {
        method: "GET",
        params: {
          keyword: "住宅和花园 / 收纳",
          onlyLeaf: "1",
          activeOnly: "1",
          limit: "50",
          offset: "50",
        },
      },
    );

    await searchOzonLeafCategories("储物箱");
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/platform/ozon/categories",
      {
        method: "GET",
        params: {
          keyword: "储物箱",
          onlyLeaf: "1",
          activeOnly: "1",
          limit: "100",
          offset: undefined,
        },
      },
    );
  });

  it("tolerates temporarily undefined complex groups while the form switches contexts", () => {
    expect(
      normalizeOzonAttributeEditorValues([], {
        complexGroups: { "501": undefined as never },
      }),
    ).toEqual({ attributes: {}, complexGroups: { "501": [] } });
  });

  it("keeps dictionary text together with its Ozon dictionary value id", () => {
    const attributes = [
      {
        attrId: "brand",
        dictionaryId: "dict-1",
        options: [{ id: "42", value: "Acme" }],
      },
      { attrId: "model" },
    ];
    expect(
      buildOzonPlatformAttributes(attributes, { brand: "42", model: "A-1" }),
    ).toEqual({
      brand: { value: "Acme", dictionaryValueId: "42" },
      model: { value: "A-1" },
    });
    expect(
      toOzonAttributeFormValues(attributes, {
        brand: { value: "Acme", dictionaryValueId: "42" },
        model: { value: "A-1" },
      }),
    ).toEqual({ brand: "42", model: "A-1" });
    expect(
      buildOzonPlatformAttributes(attributes, { brand: "Legacy brand" }),
    ).toEqual({ brand: { value: "Legacy brand" } });
  });

  it("round-trips multi-value attributes and repeated complex groups without flattening", () => {
    const attributes = [
      {
        attrId: "10",
        dictionaryId: "colors",
        isCollection: true,
        maxValueCount: 2,
        options: [
          { id: "1", value: "Red" },
          { id: "2", value: "Blue" },
        ],
      },
      { attrId: "20", attributeComplexId: 7, complexIsCollection: true },
      { attrId: "21", attributeComplexId: 7, complexIsCollection: true },
    ];
    const payload = buildOzonPlatformAttributesV3(attributes, {
      attributes: { "10": ["1", "2"] },
      complexGroups: {
        "7": [
          { "20": "100", "21": "ml" },
          { "20": "200", "21": "ml" },
        ],
      },
      skuVariantAttributeIds: [],
      skuAttributeOverrides: {},
    });
    expect(payload).toEqual({
      version: 3,
      attributes: {
        "10": [
          { value: "Red", dictionaryValueId: "1" },
          { value: "Blue", dictionaryValueId: "2" },
        ],
      },
      complexGroups: [
        {
          complexId: 7,
          attributes: { "20": [{ value: "100" }], "21": [{ value: "ml" }] },
        },
        {
          complexId: 7,
          attributes: { "20": [{ value: "200" }], "21": [{ value: "ml" }] },
        },
      ],
      skuVariantAttributeIds: [],
      skuAttributeOverrides: {},
    });
    expect(
      toOzonAttributeEditorValues(
        payload as unknown as Record<string, unknown>,
      ),
    ).toEqual({
      attributes: { "10": ["1", "2"] },
      complexGroups: {
        "7": [
          { "20": "100", "21": "ml" },
          { "20": "200", "21": "ml" },
        ],
      },
      skuVariantAttributeIds: [],
      skuAttributeOverrides: {},
    });
  });

  it("round-trips per-SKU variant values and only auto-matches exact dictionary options", () => {
    const attributes = [
      {
        attrId: "10096",
        name: "颜色",
        dictionaryId: "colors",
        options: [
          { id: "red-id", value: "红色" },
          { id: "blue-id", value: "蓝色" },
        ],
      },
      { attrId: "4180", name: "尺码" },
    ];
    const matched = autoMatchOzonSKUAttributes(
      attributes,
      [
        { id: "sku-red", attrs: { 颜色: "红色", 尺码: "M" } },
        { id: "sku-blue", attrs: { color: "蓝色", size: "L" } },
      ],
      ["10096", "4180"],
    );
    expect(matched).toMatchObject({ matchedCount: 4, unresolved: [] });
    expect(matched.values).toEqual({
      "sku-red": { "10096": "red-id", "4180": "M" },
      "sku-blue": { "10096": "blue-id", "4180": "L" },
    });

    const payload = buildOzonPlatformAttributesV3(attributes, {
      skuVariantAttributeIds: ["4180", "10096"],
      skuAttributeOverrides: matched.values,
    });
    expect(payload).toEqual({
      version: 3,
      attributes: {},
      complexGroups: [],
      skuVariantAttributeIds: ["10096", "4180"],
      skuAttributeOverrides: {
        "sku-red": {
          "10096": [{ value: "红色", dictionaryValueId: "red-id" }],
          "4180": [{ value: "M" }],
        },
        "sku-blue": {
          "10096": [{ value: "蓝色", dictionaryValueId: "blue-id" }],
          "4180": [{ value: "L" }],
        },
      },
    });
    expect(
      toOzonAttributeEditorValues(
        payload as unknown as Record<string, unknown>,
      ),
    ).toEqual({
      attributes: {},
      complexGroups: {},
      skuVariantAttributeIds: ["10096", "4180"],
      skuAttributeOverrides: matched.values,
    });
    expect(
      ozonSKUVariantTuple(
        payload.skuVariantAttributeIds,
        matched.values["sku-red"],
      ),
    ).not.toBe(
      ozonSKUVariantTuple(
        payload.skuVariantAttributeIds,
        matched.values["sku-blue"],
      ),
    );

    const unresolved = autoMatchOzonSKUAttributes(
      [{ ...attributes[0], options: [{ id: "red-id", value: "红色" }] }],
      [{ id: "sku-blue", attrs: { 颜色: "蓝色" } }],
      ["10096"],
    );
    expect(unresolved.matchedCount).toBe(0);
    expect(unresolved.unresolved).toEqual([
      { skuId: "sku-blue", attributeId: "10096" },
    ]);
  });

  it("auto-matches nested local SKU value arrays without joining them", () => {
    const matched = autoMatchOzonSKUAttributes(
      [
        {
          attrId: "10096",
          name: "颜色",
          dictionaryId: "colors",
          isCollection: true,
          options: [
            { id: "red-id", value: "红色" },
            { id: "blue-id", value: "蓝色" },
          ],
        },
      ],
      [
        {
          id: "sku-colors",
          attrs: { 颜色: { value: ["红色", "蓝色"] } },
        },
      ],
      ["10096"],
    );
    expect(matched).toEqual({
      values: { "sku-colors": { "10096": ["red-id", "blue-id"] } },
      matchedCount: 1,
      unresolved: [],
    });
  });

  it("ignores malformed array-shaped SKU override objects", () => {
    expect(
      toOzonAttributeEditorValues({
        version: 3,
        attributes: {},
        complexGroups: [],
        skuVariantAttributeIds: [],
        skuAttributeOverrides: [{ invalid: true }],
      }),
    ).toEqual({
      attributes: {},
      complexGroups: {},
      skuVariantAttributeIds: [],
      skuAttributeOverrides: {},
    });
  });

  it("preserves excess editor values so the backend can reject them explicitly", () => {
    const payload = buildOzonPlatformAttributesV3(
      [{ attrId: "single", isCollection: false, maxValueCount: 1 }],
      { attributes: { single: ["first", "second"] } },
    );
    expect(payload.attributes.single).toEqual([
      { value: "first" },
      { value: "second" },
    ]);
  });

  it("migrates historical flat complex values into a single repeated group", () => {
    const attributes = [
      { attrId: "20", attributeComplexId: 7, complexIsCollection: true },
      { attrId: "21", attributeComplexId: 7, complexIsCollection: true },
    ];
    const editor = normalizeOzonAttributeEditorValues(attributes, {
      attributes: { "20": "100", "21": "ml" },
      complexGroups: { "7": [{}] },
    });
    expect(editor).toEqual({
      attributes: {},
      complexGroups: { "7": [{ "20": "100", "21": "ml" }] },
    });
    expect(buildOzonPlatformAttributesV3(attributes, editor)).toEqual({
      version: 3,
      attributes: {},
      complexGroups: [
        {
          complexId: 7,
          attributes: {
            "20": [{ value: "100" }],
            "21": [{ value: "ml" }],
          },
        },
      ],
      skuVariantAttributeIds: [],
      skuAttributeOverrides: {},
    });
  });

  it("keeps historical complex values when explicit groups also exist", () => {
    const attributes = [
      { attrId: "20", attributeComplexId: 7, complexIsCollection: true },
      { attrId: "21", attributeComplexId: 7, complexIsCollection: true },
    ];
    expect(
      normalizeOzonAttributeEditorValues(attributes, {
        attributes: { "20": "legacy", "21": "ml" },
        complexGroups: { "7": [{ "20": "current", "21": "kg" }] },
      }),
    ).toEqual({
      attributes: {},
      complexGroups: {
        "7": [
          { "20": "current", "21": "kg" },
          { "20": "legacy", "21": "ml" },
        ],
      },
    });
  });

  it("keeps historical dictionary text as text when no option id is known", () => {
    const payload = buildOzonPlatformAttributesV3(
      [{ attrId: "brand", dictionaryId: "brands", options: [] }],
      { attributes: { brand: "Legacy brand" } },
    );
    expect(payload.attributes.brand).toEqual([{ value: "Legacy brand" }]);
  });

  it("restores historical string and array attribute values", () => {
    expect(
      toOzonAttributeEditorValues({
        brand: "Acme",
        colors: ["Red", { value: "Blue" }],
      }),
    ).toEqual({
      attributes: { brand: "Acme", colors: ["Red", "Blue"] },
      complexGroups: {},
    });
  });

  it("ignores empty complex groups while restoring v2 payloads", () => {
    expect(
      toOzonAttributeEditorValues({
        version: 2,
        complexGroups: [{ complexId: 7, attributes: {} }],
      }),
    ).toEqual({ attributes: {}, complexGroups: {} });
  });

  it("saves product-level Ozon configuration without publishing", async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: {
        id: "config-1",
        productId: "p/1",
        shopId: "shop-1",
        categoryId: "cat-1",
      },
    });
    const saved = await saveOzonProductConfig("p/1", {
      shopId: "shop-1",
      categoryId: "cat-1",
    });
    expect(saved.id).toBe("config-1");
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/products/p%2F1/platform-configs/ozon",
      {
        method: "PUT",
        data: { shopId: "shop-1", categoryId: "cat-1" },
      },
    );
  });

  it("reads Ozon configuration in the selected shop scope", async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: { productId: "p1", shopId: "shop-2" },
    });
    await getOzonProductConfig("p1", "shop-2");
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/products/p1/platform-configs/ozon",
      { method: "GET", params: { shopId: "shop-2" } },
    );
  });

  it("omits the Ozon shop query parameter for legacy unscoped reads", async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: { productId: "p1" },
    });
    await getOzonProductConfig("p1");
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/products/p1/platform-configs/ozon",
      { method: "GET", params: {} },
    );
  });

  it("builds stable per-SKU image order and serializes explicit selections", () => {
    const sharedImages = [
      {
        id: "shared-1",
        url: "https://example.test/shared.jpg",
        imageType: "main",
        sortOrder: 1,
      },
      {
        id: "duplicate-original",
        url: "https://example.test/red.jpg",
        imageType: "detail",
        sortOrder: 2,
      },
    ];
    const red = buildOzonSKUImagePreview(
      {
        skuId: "sku-red",
        skuCode: "RED",
        skuName: "红色",
        originalMainImageUrl: "https://example.test/red.jpg",
        additionalImageIds: ["shared-1", "duplicate-original", "shared-1"],
        finalImages: [],
        canPublish: false,
        issues: [],
      },
      sharedImages,
    );
    expect(red.finalImages.map((image) => image.url)).toEqual([
      "https://example.test/red.jpg",
      "https://example.test/shared.jpg",
    ]);
    expect(red.finalImages.map((image) => image.position)).toEqual([1, 2]);
    expect(red.canPublish).toBe(true);

    const missing = buildOzonSKUImagePreview(
      {
        skuId: "sku-missing",
        skuName: "蓝色 / L",
        additionalImageIds: ["shared-1"],
        finalImages: [],
        canPublish: false,
        issues: [],
      },
      sharedImages,
    );
    expect(missing.canPublish).toBe(false);
    expect(missing.issues[0]).toMatchObject({
      code: "OZON_SKU_MAIN_IMAGE_MISSING",
      skuId: "sku-missing",
    });

    expect(toOzonImageConfigInput([red, missing])).toEqual({
      version: 1,
      skuSelections: [
        {
          skuId: "sku-red",
          fallbackMainImageId: undefined,
          additionalImageIds: ["shared-1", "duplicate-original"],
        },
        {
          skuId: "sku-missing",
          fallbackMainImageId: undefined,
          additionalImageIds: ["shared-1"],
        },
      ],
    });
  });

  it("sends the versioned Ozon SKU image config without invoking publish", async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: { productId: "p1" },
    });
    await saveOzonProductConfig("p1", {
      shopId: "shop-1",
      categoryId: "100:200",
      ozonImages: {
        version: 1,
        skuSelections: [
          {
            skuId: "sku-1",
            fallbackMainImageId: "image-1",
            additionalImageIds: ["image-2"],
          },
        ],
      },
    });
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/products/p1/platform-configs/ozon",
      {
        method: "PUT",
        data: {
          shopId: "shop-1",
          categoryId: "100:200",
          ozonImages: {
            version: 1,
            skuSelections: [
              {
                skuId: "sku-1",
                fallbackMainImageId: "image-1",
                additionalImageIds: ["image-2"],
              },
            ],
          },
        },
      },
    );
  });

  it("uses an explicit Ozon readiness request", async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: { canPublish: true },
    });
    await validateOzonReadiness("p1", "shop-1");
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/products/p1/readiness/validate",
      {
        method: "POST",
        data: { platform: "ozon", shopId: "shop-1" },
      },
    );
  });

  it("loads warehouses for the selected authorized Ozon shop", async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: {
        list: [
          {
            id: "5278166",
            name: "测试 FBS 仓",
            isRfbs: false,
            isKgt: false,
            economy: false,
          },
        ],
      },
    });
    const result = await listOzonWarehouses("shop-1");
    expect(result.list[0]?.id).toBe("5278166");
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/platform/ozon/warehouses",
      { method: "GET", params: { shopId: "shop-1" } },
    );
  });

  it("confirms category groups as local configuration only", async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: { groups: [] },
    });
    await confirmOzonCategoryGroup({
      shopId: "shop-1",
      saveMappings: true,
      groups: [
        {
          sourceCategoryKey: "desk",
          productIds: ["p1"],
          categoryId: "100:200",
          categoryPath: "桌子",
        },
      ],
    });
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/product-publish/ozon/category-groups/confirm",
      {
        method: "POST",
        data: {
          shopId: "shop-1",
          saveMappings: true,
          groups: [
            {
              sourceCategoryKey: "desk",
              productIds: ["p1"],
              categoryId: "100:200",
              categoryPath: "桌子",
            },
          ],
        },
      },
    );
  });

  it("receives sync stats together with its asynchronous run identity", async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: {
        stats: { activeCount: 1 },
        run: { id: "run-1", status: "pending" },
        runId: "run-1",
      },
    });
    const result = await syncOzonCategoryFlow("shop-1");
    expect(result.runId).toBe("run-1");
    expect(result.run?.status).toBe("pending");
    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/platform/ozon/categories/sync",
      { method: "POST", data: { shopId: "shop-1" } },
    );
  });

  it("sends a stable idempotency header for a real Ozon submit", async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: { id: "task-1", status: "pending" },
    });
    await publishOzonProduct("p1", "shop-1", "ozon-submit:123:abc");
    expect(requestMock).toHaveBeenCalledWith("/api/v1/products/p1/publish", {
      method: "POST",
      data: { shopId: "shop-1", options: { platform: "ozon" } },
      headers: { "Idempotency-Key": "ozon-submit:123:abc" },
    });
  });
});

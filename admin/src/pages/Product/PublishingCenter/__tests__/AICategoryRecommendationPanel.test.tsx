import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { OzonProductCategoryRecommendation } from "@/services/ozonPublish";
import AICategoryRecommendationPanel from "../AICategoryRecommendationPanel";

const mocks = vi.hoisted(() => ({
  recommendOzonProductCategories: vi.fn(),
}));

vi.mock("@/services/ozonPublish", () => ({
  recommendOzonProductCategories: mocks.recommendOzonProductCategories,
}));

const recommendation: OzonProductCategoryRecommendation = {
  status: "partial",
  taskId: "task-1",
  sourceSummary: {
    productTitle: "SSK 固态继电器",
    skuCount: 6,
    selectedSkuCount: 6,
    skuGroupNames: ["颜色分类"],
    productAttributeCount: 2,
    primaryEvidence: "persisted_sku_classification_attributes",
  },
  productType: "固态继电器",
  differenceDimensions: [
    {
      key: "model",
      name: "型号",
      semantic: "model",
      confidence: 0.98,
      evidence: [
        {
          skuId: "sku-1",
          skuCode: "SSK3D",
          source: "sku.attrs",
          sourceKey: "颜色分类",
          rawValue: "SSK3D 直流控直流 3A 带底座 10只装",
        },
        {
          skuId: "sku-2",
          skuCode: "SSK4D",
          source: "sku.attrs",
          sourceKey: "颜色分类",
          rawValue: "SSK4D 直流控直流 4A 带底座 10只装",
        },
      ],
    },
  ],
  anomalies: [
    {
      type: "different_product_subject",
      message: "短接线是不同商品主体",
      skuIds: ["sku-wire"],
      confidence: 0.99,
    },
  ],
  warnings: ["AI 重排不可用，保留规则排序"],
  candidates: [
    {
      categoryId: "100:200",
      categoryPath: "电子产品 / 固态继电器",
      score: 72.5,
      confidence: 0.82,
      approximate: true,
      variantCoverage: { matched: 0, total: 1, ratio: 0 },
      requiredCoverage: { matched: 2, total: 3, ratio: 0.667 },
      matchedDimensions: [],
      unmatchedDimensions: [
        {
          sourceDimensionKey: "model",
          sourceDimensionName: "型号",
          reason: "模板中没有语义相符且 is_aspect=true、资格已知的属性",
        },
      ],
      listingStrategy: "split_single_sku",
      reasons: ["商品语义匹配度 92%"],
      warnings: ["模板无法承载全部 SKU 区别"],
      schemaHash: "schema-1",
      templateSyncedAt: "2026-08-10T00:00:00Z",
    },
  ],
};

function installResponsiveMocks() {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    }),
  });
}

describe("AICategoryRecommendationPanel", () => {
  beforeEach(() => {
    installResponsiveMocks();
    mocks.recommendOzonProductCategories.mockReset();
  });

  it("shows evidence and applies a candidate only through the local form callback", async () => {
    const user = userEvent.setup();
    const onApply = vi.fn().mockResolvedValue(undefined);
    mocks.recommendOzonProductCategories.mockResolvedValue(recommendation);
    render(
      <AICategoryRecommendationPanel
        productId="product-1"
        shopId="shop-1"
        onApply={onApply}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "AI 分析 SKU 并推荐类目" }),
    );
    expect(
      await screen.findByText("电子产品 / 固态继电器"),
    ).toBeInTheDocument();
    expect(screen.getByText("短接线是不同商品主体")).toBeInTheDocument();
    expect(screen.getByText("建议拆分为单 SKU")).toBeInTheDocument();
    expect(mocks.recommendOzonProductCategories).toHaveBeenCalledWith(
      "product-1",
      {
        shopId: "shop-1",
        skuIds: [],
        refreshPolicy: "if_missing_or_stale",
      },
    );
    expect(onApply).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "应用此类目" }));
    expect(onApply).toHaveBeenCalledTimes(1);
    expect(onApply).toHaveBeenCalledWith(recommendation.candidates[0]);
    expect(mocks.recommendOzonProductCategories).toHaveBeenCalledTimes(1);
  });

  it("distinguishes readonly state and never starts a request", async () => {
    render(
      <AICategoryRecommendationPanel
        productId="product-1"
        shopId="shop-1"
        readOnly
        onApply={vi.fn()}
      />,
    );
    expect(
      screen.getByText("当前账号为只读权限，不能发起 AI 推荐或应用候选"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "AI 分析 SKU 并推荐类目" }),
    ).toBeDisabled();
    expect(mocks.recommendOzonProductCategories).not.toHaveBeenCalled();
  });

  it("prevents duplicate analysis and apply requests", async () => {
    let resolveAnalysis: (
      value: OzonProductCategoryRecommendation,
    ) => void = () => undefined;
    mocks.recommendOzonProductCategories.mockReturnValueOnce(
      new Promise<OzonProductCategoryRecommendation>((resolve) => {
        resolveAnalysis = resolve;
      }),
    );
    let resolveApply: () => void = () => undefined;
    const onApply = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveApply = resolve;
        }),
    );
    render(
      <AICategoryRecommendationPanel
        productId="product-1"
        shopId="shop-1"
        onApply={onApply}
      />,
    );

    const analyzeButton = screen.getByRole("button", {
      name: "AI 分析 SKU 并推荐类目",
    });
    fireEvent.click(analyzeButton);
    fireEvent.click(analyzeButton);
    expect(mocks.recommendOzonProductCategories).toHaveBeenCalledTimes(1);
    expect(
      screen.getByText(
        "正在使用采集 SKU 属性分析区别并核对真实 Ozon 模板，请稍候…",
      ),
    ).toBeInTheDocument();
    await act(async () => {
      resolveAnalysis(recommendation);
      await Promise.resolve();
    });

    const applyButton = await screen.findByRole("button", {
      name: "应用此类目",
    });
    fireEvent.click(applyButton);
    fireEvent.click(applyButton);
    expect(onApply).toHaveBeenCalledTimes(1);
    await act(async () => {
      resolveApply();
      await Promise.resolve();
    });
  });

  it.each([
    ["no_match", "暂未找到可用候选"],
    ["ai_unavailable", "AI 分析暂不可用"],
    ["category_cache_empty", "Ozon 类目缓存为空"],
  ] as const)(
    "renders the %s business status as an empty manual fallback",
    async (status, label) => {
      const user = userEvent.setup();
      mocks.recommendOzonProductCategories.mockResolvedValue({
        ...recommendation,
        status,
        candidates: [],
      });
      render(
        <AICategoryRecommendationPanel
          productId="product-1"
          shopId="shop-1"
          onApply={vi.fn()}
        />,
      );

      await user.click(
        screen.getByRole("button", { name: "AI 分析 SKU 并推荐类目" }),
      );
      expect(await screen.findByText(label)).toBeInTheDocument();
      expect(
        screen.getByText("没有可应用的 AI 候选；人工类目导航仍可继续使用"),
      ).toBeInTheDocument();
    },
  );

  it("distinguishes a request error and keeps retry available", async () => {
    const user = userEvent.setup();
    mocks.recommendOzonProductCategories.mockRejectedValue(
      new Error("recommendation timeout"),
    );
    render(
      <AICategoryRecommendationPanel
        productId="product-1"
        shopId="shop-1"
        onApply={vi.fn()}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "AI 分析 SKU 并推荐类目" }),
    );
    expect(await screen.findByText("AI 类目推荐请求失败")).toBeInTheDocument();
    expect(screen.getByText("recommendation timeout")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "重试" })).toBeEnabled();
  });

  it("discards a superseded recommendation when product or shop changes", async () => {
    const user = userEvent.setup();
    let resolveOld: (value: OzonProductCategoryRecommendation) => void = () =>
      undefined;
    mocks.recommendOzonProductCategories.mockReturnValueOnce(
      new Promise<OzonProductCategoryRecommendation>((resolve) => {
        resolveOld = resolve;
      }),
    );
    const { rerender } = render(
      <AICategoryRecommendationPanel
        productId="product-old"
        shopId="shop-1"
        onApply={vi.fn()}
      />,
    );
    await user.click(
      screen.getByRole("button", { name: "AI 分析 SKU 并推荐类目" }),
    );
    rerender(
      <AICategoryRecommendationPanel
        productId="product-new"
        shopId="shop-1"
        onApply={vi.fn()}
      />,
    );
    await act(async () => {
      resolveOld(recommendation);
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(
        screen.queryByText("电子产品 / 固态继电器"),
      ).not.toBeInTheDocument();
    });
  });
});

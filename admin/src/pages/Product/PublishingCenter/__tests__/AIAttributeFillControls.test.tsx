import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { OzonAttributeSuggestionResult } from "@/services/ozonPublish";
import AIAttributeFillControls, {
  type OzonAIAttributeRequestContext,
} from "../AIAttributeFillControls";

const mocks = vi.hoisted(() => ({
  suggestOzonAttributes: vi.fn(),
}));

vi.mock("@/services/ozonPublish", () => ({
  suggestOzonAttributes: mocks.suggestOzonAttributes,
}));

const baseContext: OzonAIAttributeRequestContext = {
  productId: "product-1",
  shopId: "shop-1",
  categoryId: "100:200",
  templateFingerprint: "schema-1",
  generation: 1,
};

const partialResult: OzonAttributeSuggestionResult = {
  status: "partial",
  taskId: "task-1",
  context: {
    productId: "product-1",
    productUpdatedAt: "2026-08-11T00:00:00Z",
    shopId: "shop-1",
    categoryId: "100:200",
    templateFingerprint: "schema-1",
    fingerprint: "context-1",
  },
  suggestions: [
    {
      attributeId: "count",
      attributeName: "数量",
      values: [{ value: "12" }],
      confidence: 0.3,
      confidenceLevel: "low",
      inferenceBasis: "category_fallback_guess",
      requiresReview: true,
      sourceRefs: ["common_knowledge"],
    },
  ],
  skipped: [
    {
      attributeId: "url",
      attributeName: "链接",
      kind: "external",
      reason: "外部字段已跳过",
    },
  ],
  summary: {
    filled: 1,
    requiresReview: 1,
    notFound: 1,
    eligible: 1,
    high: 0,
    medium: 0,
    low: 1,
    externalSkipped: 1,
    unsupportedSkipped: 0,
    validationSkipped: 0,
  },
  warnings: [],
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

function props(
  overrides: Partial<ComponentProps<typeof AIAttributeFillControls>> = {},
) {
  return {
    context: baseContext,
    getCurrentValues: vi.fn(() => ({ attributes: { brand: "用户品牌" } })),
    onApplyResult: vi.fn(() => ({
      filled: 1,
      requiresReview: 1,
      notFound: 1,
      high: 0,
      medium: 0,
      low: 1,
      externalSkipped: 1,
      otherIncomplete: 1,
      details: ["链接：外部字段已跳过"],
    })),
    onUndo: vi.fn(),
    ...overrides,
  };
}

describe("AIAttributeFillControls", () => {
  beforeEach(() => {
    installResponsiveMocks();
    mocks.suggestOzonAttributes.mockReset();
  });

  it("shows partial counts and sends only the current editor snapshot", async () => {
    const user = userEvent.setup();
    mocks.suggestOzonAttributes.mockResolvedValue(partialResult);
    const currentProps = props({ canUndo: true });
    render(<AIAttributeFillControls {...currentProps} />);

    await user.click(screen.getByRole("button", { name: "AI 填写空白项" }));
    expect(await screen.findByText("AI 已部分填写空白项")).toBeInTheDocument();
    expect(screen.getByLabelText("AI 属性填写结果")).toHaveTextContent(
      "已填写 1",
    );
    expect(screen.getByLabelText("AI 属性填写结果")).toHaveTextContent(
      "低 1",
    );
    expect(screen.getByLabelText("AI 属性填写结果")).toHaveTextContent(
      "外部跳过 1",
    );
    expect(screen.getByLabelText("AI 属性填写结果")).toHaveTextContent(
      "其他未完成 1",
    );
    expect(mocks.suggestOzonAttributes).toHaveBeenCalledWith("product-1", {
      shopId: "shop-1",
      categoryId: "100:200",
      templateFingerprint: "schema-1",
      currentValues: { attributes: { brand: "用户品牌" } },
    });
    expect(currentProps.onApplyResult).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "撤销本次 AI 填写" }));
    expect(currentProps.onUndo).toHaveBeenCalledTimes(1);
  });

  it("distinguishes readonly state and never consumes AI", async () => {
    render(<AIAttributeFillControls {...props({ readOnly: true })} />);
    expect(
      screen.getByText(
        "当前账号没有商品或店铺编辑权限，不能发起或回填 AI 属性建议",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "AI 填写空白项" }),
    ).toBeDisabled();
    expect(mocks.suggestOzonAttributes).not.toHaveBeenCalled();
  });

  it("explains a conflicting disabled state and does not start a request", () => {
    render(
      <AIAttributeFillControls
        {...props({
          disabled: true,
          disabledReason: "正在保存当前编辑，请稍候",
        })}
      />,
    );
    const button = screen.getByRole("button", { name: "AI 填写空白项" });
    expect(button).toBeDisabled();
    expect(screen.getByText("正在保存当前编辑，请稍候")).toBeInTheDocument();
    fireEvent.click(button);
    expect(mocks.suggestOzonAttributes).not.toHaveBeenCalled();
  });

  it("prevents duplicate clicks while a request is pending", async () => {
    let resolveRequest: (value: OzonAttributeSuggestionResult) => void = () =>
      undefined;
    mocks.suggestOzonAttributes.mockReturnValueOnce(
      new Promise<OzonAttributeSuggestionResult>((resolve) => {
        resolveRequest = resolve;
      }),
    );
    render(<AIAttributeFillControls {...props()} />);
    const button = screen.getByRole("button", { name: "AI 填写空白项" });
    fireEvent.click(button);
    fireEvent.click(button);
    expect(mocks.suggestOzonAttributes).toHaveBeenCalledTimes(1);
    expect(
      screen.getByText("正在分析商品信息并校验当前 Ozon 属性模板…"),
    ).toBeInTheDocument();
    await act(async () => {
      resolveRequest(partialResult);
      await Promise.resolve();
    });
  });

  it("discards an in-flight response when the editor becomes disabled", async () => {
    let resolveRequest: (value: OzonAttributeSuggestionResult) => void = () =>
      undefined;
    mocks.suggestOzonAttributes.mockReturnValueOnce(
      new Promise<OzonAttributeSuggestionResult>((resolve) => {
        resolveRequest = resolve;
      }),
    );
    const currentProps = props();
    const { rerender } = render(<AIAttributeFillControls {...currentProps} />);
    fireEvent.click(screen.getByRole("button", { name: "AI 填写空白项" }));
    rerender(
      <AIAttributeFillControls
        {...currentProps}
        disabled
        disabledReason="正在保存当前编辑，请稍候"
      />,
    );
    await act(async () => {
      resolveRequest(partialResult);
      await Promise.resolve();
    });

    expect(currentProps.onApplyResult).not.toHaveBeenCalled();
    expect(screen.queryByLabelText("AI 属性填写结果")).not.toBeInTheDocument();
  });

  it("shows a stable failure without applying or automatically retrying", async () => {
    mocks.suggestOzonAttributes.mockRejectedValue(
      new Error("provider timeout"),
    );
    const currentProps = props();
    render(<AIAttributeFillControls {...currentProps} />);
    fireEvent.click(screen.getByRole("button", { name: "AI 填写空白项" }));
    expect(
      await screen.findByText("AI 填写失败，现有输入未变更"),
    ).toBeInTheDocument();
    expect(screen.getByText(/provider timeout/)).toBeInTheDocument();
    expect(currentProps.onApplyResult).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(mocks.suggestOzonAttributes).toHaveBeenCalledTimes(1),
    );
  });

  it.each([
    ["product", { productId: "product-2" }],
    ["shop", { shopId: "shop-2" }],
    ["category", { categoryId: "200:300" }],
    ["template", { templateFingerprint: "schema-2", generation: 2 }],
  ] as const)(
    "discards a superseded response after %s changes",
    async (_, change) => {
      let resolveOld: (value: OzonAttributeSuggestionResult) => void = () =>
        undefined;
      mocks.suggestOzonAttributes.mockReturnValueOnce(
        new Promise<OzonAttributeSuggestionResult>((resolve) => {
          resolveOld = resolve;
        }),
      );
      const currentProps = props();
      const { rerender } = render(
        <AIAttributeFillControls {...currentProps} />,
      );
      fireEvent.click(screen.getByRole("button", { name: "AI 填写空白项" }));
      rerender(
        <AIAttributeFillControls
          {...currentProps}
          context={{ ...baseContext, ...change }}
        />,
      );
      await act(async () => {
        resolveOld(partialResult);
        await Promise.resolve();
      });
      expect(currentProps.onApplyResult).not.toHaveBeenCalled();
      expect(screen.queryByText("AI 已部分填写空白项")).not.toBeInTheDocument();
    },
  );

  it.each([
    ["product id", { productId: "product-other" }],
    ["shop id", { shopId: "shop-other" }],
    ["category id", { categoryId: "900:901" }],
    ["template fingerprint", { templateFingerprint: "schema-other" }],
    ["missing context fingerprint", { fingerprint: "" }],
  ] as const)("discards a response with mismatched %s", async (_, change) => {
    mocks.suggestOzonAttributes.mockResolvedValueOnce({
      ...partialResult,
      context: { ...partialResult.context, ...change },
    });
    const currentProps = props();
    render(<AIAttributeFillControls {...currentProps} />);

    fireEvent.click(screen.getByRole("button", { name: "AI 填写空白项" }));
    await waitFor(() =>
      expect(mocks.suggestOzonAttributes).toHaveBeenCalledTimes(1),
    );
    await act(async () => {
      await Promise.resolve();
    });

    expect(currentProps.onApplyResult).not.toHaveBeenCalled();
    expect(screen.queryByLabelText("AI 属性填写结果")).not.toBeInTheDocument();
  });
});

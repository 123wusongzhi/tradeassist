import {
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  PlusOutlined,
  SaveOutlined,
  SyncOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons";
import { history, Link, useLocation } from "@umijs/max";
import {
  Alert,
  Button,
  Collapse,
  Descriptions,
  Divider,
  Form,
  Input,
  InputNumber,
  Modal,
  Segmented,
  Select,
  Space,
  Spin,
  Steps,
  Tag,
  Typography,
  message,
} from "antd";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import PermissionGuard from "@/components/PermissionGuard";
import { EmptyState, SectionCard, TmPageContainer } from "@/components/ui";
import { usePermission } from "@/hooks/usePermission";
import {
  queryOzonCategoryAttributes,
  searchOzonDictionaryValues,
  syncOzonCategoryAttributes,
  type OzonCategoryAttribute,
  type OzonVariantPolicy,
} from "@/services/ozonCategories";
import {
  autoMatchOzonSKUAttributes,
  buildOzonPlatformAttributesV3,
  buildOzonSKUImagePreview,
  getOzonProductConfig,
  listOzonWarehouses,
  listOzonCategoryMappings,
  normalizeOzonAttributeEditorValues,
  ozonSKUVariantTuple,
  publishOzonProduct,
  saveOzonCategoryMapping,
  saveOzonProductConfig,
  syncOzonCategoryFlow,
  toOzonAttributeEditorValues,
  toOzonImageConfigInput,
  validateOzonReadiness,
  type OzonAttributeEditorValues,
  type OzonAttributeSuggestionCurrentValues,
  type OzonAttributeSuggestionResult,
  type OzonImageConfigView,
  type OzonCategoryMapping,
  type OzonProductCategoryRecommendationCandidate,
  type OzonProductConfig,
  type OzonReadinessResult,
  type OzonResolvedListing,
  type OzonSKUImageConfig,
  type OzonValueSource,
  type OzonWarehouseOption,
} from "@/services/ozonPublish";
import {
  fetchProductDetail,
  fetchProducts,
  type ProductDetail,
  type ProductListRow,
  type ProductSKURow,
} from "@/services/products";
import { queryShops, type ShopListRow } from "@/services/shops";
import { formatDateTime } from "@/utils/formatTime";
import { PERMISSIONS } from "@/utils/permission";
import OzonSKUImageConfigurator, {
  type OzonSKUImageSelectionPatch,
} from "../OzonPublish/OzonSKUImageConfigurator";
import OzonCategoryNavigator, {
  type OzonCategoryFocusTarget,
} from "./OzonCategoryNavigator";
import {
  matchesOzonAttributeView,
  ozonAttributeFormatHint,
  summarizeOzonAttributes,
  type OzonAttributeFilter,
} from "./attributeUi";
import AICategoryRecommendationPanel from "./AICategoryRecommendationPanel";
import AIAttributeFillControls, {
  type OzonAIAttributeRequestContext,
} from "./AIAttributeFillControls";
import {
  cloneOzonEditorAttributes,
  mergeOzonAIAttributeSuggestions,
  undoOzonAIAttributeSuggestions,
  type OzonAIAttributeMarker,
} from "./aiAttributeSuggestions";
import "../OzonPublish/index.less";
import "./index.less";

type PublishingFormValues = OzonAttributeEditorValues & {
  categoryId?: string;
  title?: string;
  description?: string;
  currencyCode?: string;
  skuPrices?: Record<string, number | undefined>;
  package?: {
    weightG?: number;
    widthMm?: number;
    heightMm?: number;
    depthMm?: number;
    warehouseId?: string;
    vat?: string;
  };
};

type EditorIssue = {
  key: string;
  message: string;
  suggestion: string;
  field?: string;
};

type TemplateRefreshFeedback = {
  categoryId: string;
  attributeCount: number;
  requiredCount: number;
  syncedAt?: string;
};

type AIAttributeUndoSnapshot = {
  contextKey: string;
  attributes: Record<string, string | string[]>;
  applied: Record<string, OzonAIAttributeMarker>;
};

type PublishingStep = 0 | 1 | 2 | 3 | 4 | 5;

const publishingStepItems = [
  { title: "店铺与商品" },
  { title: "内容与图片" },
  { title: "Ozon 类目与属性" },
  { title: "规格、价格与库存" },
  { title: "包裹、仓库与税率" },
  { title: "发布前检查与提交" },
] as const;

const publishingStepByQuery: Record<string, PublishingStep> = {
  context: 0,
  content: 1,
  category: 2,
  sku: 3,
  package: 4,
  review: 5,
};

const ozonVATOptions = [
  { value: "0", label: "不征收增值税（接口值 0）" },
  { value: "0.1", label: "增值税 10%（接口值 0.1）" },
  { value: "0.2", label: "增值税 20%（接口值 0.2）" },
];

function ozonVATLabel(value?: string | number) {
  const normalized = String(value ?? "").trim();
  return (
    ozonVATOptions.find((item) => item.value === normalized)?.label ||
    normalized ||
    "—"
  );
}

function stepForIssue(issue: EditorIssue): PublishingStep {
  const key = issue.key;
  if (key === "product" || key === "shop") return 0;
  if (
    ["title", "description"].includes(key) ||
    key.startsWith("currency") ||
    key.startsWith("image-")
  )
    return 1;
  if (
    key.startsWith("category") ||
    key.startsWith("attr-") ||
    key.startsWith("complex-")
  )
    return 2;
  if (
    key.startsWith("price-") ||
    key.startsWith("stock-") ||
    key.startsWith("sku-variant") ||
    key.startsWith("variant-")
  )
    return 3;
  if (
    key.startsWith("package-") ||
    key.startsWith("warehouse") ||
    key === "vat"
  )
    return 4;
  return 5;
}

const sourceLabels: Record<string, string> = {
  product: "本地商品",
  ozon_product_shop_config: "当前 Ozon 店铺配置",
  global_ozon_preset: "全局 Ozon 刊登预设",
  local_inventory: "本地库存",
  store_contract: "Ozon 店铺合同",
  ozon_default: "Ozon 默认值",
  missing: "缺失",
};

function sourceLabel(source?: OzonValueSource) {
  return sourceLabels[String(source || "")] || source || "待解析";
}

function sourceTag(source?: OzonValueSource) {
  const missing = !source || source === "missing";
  let color: "red" | "blue" | undefined;
  if (missing) color = "red";
  else if (source === "ozon_product_shop_config") color = "blue";
  return <Tag color={color}>{sourceLabel(source)}</Tag>;
}

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) return error.message;
  if (error && typeof error === "object" && "message" in error) {
    const value = String((error as { message?: unknown }).message || "").trim();
    if (value) return value;
  }
  return fallback;
}

function productTitle(product?: ProductDetail | ProductListRow) {
  if (!product) return "";
  return String(
    product.title ||
      ("aiTitle" in product ? product.aiTitle : "") ||
      ("originalTitle" in product ? product.originalTitle : "") ||
      "",
  ).trim();
}

function productDescription(product?: ProductDetail) {
  return String(product?.description || product?.aiDescription || "").trim();
}

function isFilled(value: unknown) {
  if (Array.isArray(value))
    return value.some((item) => String(item ?? "").trim());
  return String(value ?? "").trim() !== "";
}

function aiAttributeContextKey(context?: OzonAIAttributeRequestContext) {
  if (!context) return "";
  return [
    context.productId,
    context.shopId,
    context.categoryId,
    context.templateFingerprint,
    context.generation,
  ].join("\n");
}

function positiveNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) && value > 0;
}

function newIdempotencyKey() {
  const random =
    globalThis.crypto?.randomUUID?.() ||
    `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `ozon-submit:${random}`;
}

function addSavedDictionaryOptions(
  attributes: OzonCategoryAttribute[],
  raw?: Record<string, unknown>,
) {
  if (!raw) return attributes;
  const selections = new Map<string, Array<{ id: string; value: string }>>();
  const add = (attrId: string, values: unknown) => {
    if (!Array.isArray(values)) return;
    values.forEach((item) => {
      if (!item || typeof item !== "object") return;
      const typed = item as { value?: unknown; dictionaryValueId?: unknown };
      const id = String(typed.dictionaryValueId ?? "").trim();
      const value = String(typed.value ?? "").trim();
      if (id && value)
        selections.set(attrId, [
          ...(selections.get(attrId) || []),
          { id, value },
        ]);
    });
  };
  if (
    (raw.version === 2 || raw.version === 3) &&
    raw.attributes &&
    typeof raw.attributes === "object"
  ) {
    Object.entries(raw.attributes as Record<string, unknown>).forEach(
      ([attrId, values]) => add(attrId, values),
    );
    const groups = Array.isArray(raw.complexGroups) ? raw.complexGroups : [];
    groups.forEach((group) => {
      if (!group || typeof group !== "object") return;
      const attrs = (group as { attributes?: Record<string, unknown> })
        .attributes;
      Object.entries(attrs || {}).forEach(([attrId, values]) =>
        add(attrId, values),
      );
    });
    const overrides =
      raw.skuAttributeOverrides && typeof raw.skuAttributeOverrides === "object"
        ? (raw.skuAttributeOverrides as Record<string, unknown>)
        : {};
    Object.values(overrides).forEach((row) => {
      if (!row || typeof row !== "object") return;
      Object.entries(row as Record<string, unknown>).forEach(
        ([attrId, values]) => add(attrId, values),
      );
    });
  } else {
    Object.entries(raw).forEach(([attrId, value]) => {
      const typed = value as
        | { value?: unknown; dictionaryValueId?: unknown }
        | undefined;
      const id = String(typed?.dictionaryValueId ?? "").trim();
      const label = String(typed?.value ?? "").trim();
      if (id && label) selections.set(attrId, [{ id, value: label }]);
    });
  }
  return attributes.map((attribute) => {
    const saved = selections.get(attribute.attrId) || [];
    if (!attribute.dictionaryId || saved.length === 0) return attribute;
    const options = [...(attribute.options || [])];
    saved.forEach((item) => {
      if (!options.some((option) => option.id === item.id))
        options.unshift(item);
    });
    return { ...attribute, options };
  });
}

function complexAttributeGroups(attributes: OzonCategoryAttribute[]) {
  const groups = new Map<number, OzonCategoryAttribute[]>();
  attributes.forEach((attribute) => {
    const complexId = Number(attribute.attributeComplexId || 0);
    if (complexId <= 0) return;
    groups.set(complexId, [...(groups.get(complexId) || []), attribute]);
  });
  return Array.from(groups.entries()).sort(([left], [right]) => left - right);
}

function effectivePackageValue(
  formValue: unknown,
  resolved?: { value: number | string; source: OzonValueSource },
) {
  return isFilled(formValue) ? formValue : resolved?.value;
}

function normalizedOzonValueType(attribute: OzonCategoryAttribute) {
  return String(attribute.valueType || "")
    .trim()
    .toLowerCase();
}

function supportsOzonAttributeInput(attribute: OzonCategoryAttribute) {
  if (attribute.dictionaryId) return true;
  return [
    "string",
    "text",
    "integer",
    "int",
    "int64",
    "decimal",
    "float",
    "double",
    "number",
    "boolean",
    "bool",
    "url",
    "uri",
    "image",
    "date",
    "datetime",
    "date_time",
    "timestamp",
  ].includes(normalizedOzonValueType(attribute));
}

function isLowFrequencyOzonAttribute(attribute: OzonCategoryAttribute) {
  const searchable = [
    attribute.name,
    attribute.description,
    attribute.valueType,
  ]
    .filter(Boolean)
    .join(" ");
  return /(?:\bjson\b|rich[\s_-]*(?:content|контент)|rich-контент|富内容|\bpdf\b|\bdebug\b|调试|诊断|отлад)/i.test(
    searchable,
  );
}

function ozonVariantAttributeDisabledReason(
  attribute: OzonCategoryAttribute,
): string | undefined {
  if (Number(attribute.attributeComplexId || 0) > 0)
    return "组合属性不能直接作为 SKU 维度";
  if (attribute.skuVariantEligibilityKnown !== true)
    return "缺少 Ozon is_aspect 资格证据，请刷新当前类目模板";
  if (attribute.skuVariantEligible !== true)
    return "Ozon is_aspect=false，当前类目明确不允许";
  if (!supportsOzonAttributeInput(attribute))
    return `系统暂不支持 valueType=${attribute.valueType || "未知"}`;
  return undefined;
}

function ozonTypedValueIssue(
  attribute: OzonCategoryAttribute,
  raw: unknown,
): string | undefined {
  if (!isFilled(raw) || attribute.dictionaryId) return undefined;
  const values = Array.isArray(raw) ? raw : [raw];
  const valueType = normalizedOzonValueType(attribute);
  for (const item of values) {
    const value = String(item ?? "").trim();
    if (["string", "text"].includes(valueType)) continue;
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
      continue;
    }
    if (["decimal", "float", "double", "number"].includes(valueType)) {
      if (!/^[+-]?(?:\d+(?:\.\d*)?|\.\d+)$/.test(value))
        return "必须填写有限十进制数，不能使用科学计数法";
      continue;
    }
    if (["boolean", "bool"].includes(valueType)) {
      if (value !== "true" && value !== "false") return "必须选择是或否";
      continue;
    }
    if (["url", "uri", "image"].includes(valueType)) {
      try {
        const parsed = new URL(value);
        if (parsed.protocol !== "http:" && parsed.protocol !== "https:")
          return "必须填写完整的 http/https URL";
      } catch {
        return "必须填写完整的 http/https URL";
      }
      continue;
    }
    if (valueType === "date") {
      if (!/^\d{4}-\d{2}-\d{2}$/.test(value))
        return "必须使用 YYYY-MM-DD 日期格式";
      continue;
    }
    if (["datetime", "date_time", "timestamp"].includes(valueType)) {
      if (
        Number.isNaN(Date.parse(value)) ||
        !/(?:Z|[+-]\d{2}:\d{2})$/.test(value)
      )
        return "必须使用带时区的 RFC3339 日期时间";
      continue;
    }
    return `系统尚不支持 valueType=${attribute.valueType || "未知"}`;
  }
  return undefined;
}

export default function PublishingCenterPage() {
  const location = useLocation();
  const { can, readonly } = usePermission();
  const [form] = Form.useForm<PublishingFormValues>();
  const initialProductId = useMemo(
    () => new URLSearchParams(location.search).get("productId") || undefined,
    [location.search],
  );
  const initialShopId = useMemo(
    () => new URLSearchParams(location.search).get("shopId") || undefined,
    [location.search],
  );
  const initialStep = useMemo(() => {
    const value = new URLSearchParams(location.search).get("step") || "";
    return publishingStepByQuery[value] ?? 0;
  }, [location.search]);
  const [products, setProducts] = useState<ProductListRow[]>([]);
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [productId, setProductId] = useState<string | undefined>(
    initialProductId,
  );
  const [shopId, setShopId] = useState<string | undefined>(initialShopId);
  const [activeStep, setActiveStep] = useState<PublishingStep>(initialStep);
  const [product, setProduct] = useState<ProductDetail>();
  const [config, setConfig] = useState<OzonProductConfig>();
  const [attributes, setAttributes] = useState<OzonCategoryAttribute[]>([]);
  const [variantPolicy, setVariantPolicy] = useState<OzonVariantPolicy>();
  const [attributeTemplateFingerprint, setAttributeTemplateFingerprint] =
    useState("");
  const [categoryPath, setCategoryPath] = useState("");
  const [categoryNavigatorRefreshToken, setCategoryNavigatorRefreshToken] =
    useState(0);
  const [categoryFocusTarget, setCategoryFocusTarget] =
    useState<OzonCategoryFocusTarget>();
  const [categoryMapping, setCategoryMapping] = useState<OzonCategoryMapping>();
  const [categoryMappingLoading, setCategoryMappingLoading] = useState(false);
  const [categoryMappingError, setCategoryMappingError] = useState<string>();
  const [confirmingCategoryMapping, setConfirmingCategoryMapping] =
    useState(false);
  const [appliedRecommendationCategoryID, setAppliedRecommendationCategoryID] =
    useState<string>();
  const [warehouseOptions, setWarehouseOptions] = useState<
    OzonWarehouseOption[]
  >([]);
  const [warehouseLoading, setWarehouseLoading] = useState(false);
  const [warehouseError, setWarehouseError] = useState<string>();
  const [skuImages, setSKUImages] = useState<OzonSKUImageConfig[]>([]);
  const [bulkImageIds, setBulkImageIds] = useState<string[]>([]);
  const [preflight, setPreflight] = useState<OzonReadinessResult>();
  const [dirty, setDirty] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingConfig, setLoadingConfig] = useState(false);
  const [saving, setSaving] = useState(false);
  const [checking, setChecking] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [templateRefreshFeedback, setTemplateRefreshFeedback] =
    useState<TemplateRefreshFeedback>();
  const [attributeTemplateError, setAttributeTemplateError] =
    useState<string>();
  const [attributeSearch, setAttributeSearch] = useState("");
  const [attributeFilter, setAttributeFilter] =
    useState<OzonAttributeFilter>("all");
  const [optionalAttributesOpen, setOptionalAttributesOpen] = useState(true);
  const [lowFrequencyAttributesOpen, setLowFrequencyAttributesOpen] =
    useState(false);
  const [formatRulesOpen, setFormatRulesOpen] = useState(false);
  const [expandedAttributeDescriptions, setExpandedAttributeDescriptions] =
    useState<Set<string>>(() => new Set());
  const [skuVariantDetailsOpen, setSKUVariantDetailsOpen] = useState(false);
  const [aiAttributeMarkers, setAIAttributeMarkers] = useState<
    Record<string, OzonAIAttributeMarker>
  >({});
  const [aiAttributeUndo, setAIAttributeUndo] =
    useState<AIAttributeUndoSnapshot>();
  const [loadError, setLoadError] = useState<string>();
  const [searchingAttribute, setSearchingAttribute] = useState<string>();
  const [searchingProducts, setSearchingProducts] = useState(false);
  const productSearchSequence = useRef(0);
  const attributeLoadSequence = useRef(0);
  const dictionarySearchSequences = useRef<Record<string, number>>({});
  // Wizard steps intentionally unmount most fields. Preserve their values in
  // the watcher so hidden steps remain part of the safety gate and save DTO.
  const watched = (Form.useWatch([], { form, preserve: true }) ||
    {}) as PublishingFormValues;

  const resetAttributeViewState = useCallback(() => {
    setAttributeTemplateError(undefined);
    setAttributeSearch("");
    setAttributeFilter("all");
    setOptionalAttributesOpen(true);
    setLowFrequencyAttributesOpen(false);
    setFormatRulesOpen(false);
    setExpandedAttributeDescriptions(new Set());
  }, []);

  const resetAIAttributeState = useCallback(() => {
    setAIAttributeMarkers({});
    setAIAttributeUndo(undefined);
  }, []);

  useEffect(() => {
    setActiveStep(initialStep);
  }, [initialStep]);

  useEffect(() => {
    setAppliedRecommendationCategoryID(undefined);
    attributeLoadSequence.current += 1;
    resetAIAttributeState();
  }, [productId, resetAIAttributeState, shopId]);

  const canEdit =
    !readonly &&
    can(PERMISSIONS.PRODUCT_WRITE) &&
    can(PERMISSIONS.STORE_OPERATE);
  const canPublish = canEdit && can(PERMISSIONS.PUBLISH_CREATE_DRAFT);
  const selectedShopCurrency = String(
    shops.find((shop) => shop.id === shopId)?.currency || "",
  )
    .trim()
    .toUpperCase();
  const selectedCategoryId = watched.categoryId;
  const selectedCategoryParts = String(selectedCategoryId || "").split(":", 2);
  const categoryTemplateSyncedAt = attributes.reduce<string | undefined>(
    (latest, attribute) => {
      if (!attribute.syncedAt) return latest;
      if (!latest) return attribute.syncedAt;
      return Date.parse(attribute.syncedAt) > Date.parse(latest)
        ? attribute.syncedAt
        : latest;
    },
    undefined,
  );
  const requiredCategoryAttributeCount = attributes.filter(
    (attribute) => attribute.required,
  ).length;

  const loadProductOptions = useCallback(
    async (keyword?: string) => {
      const sequence = ++productSearchSequence.current;
      setSearchingProducts(true);
      try {
        const result = await fetchProducts({
          page: 1,
          pageSize: 50,
          keyword: keyword?.trim() || undefined,
        });
        if (sequence !== productSearchSequence.current) return;
        setProducts((current) => {
          const selected = current.find((item) => item.id === productId);
          const next = [...(result.list || [])];
          if (selected && !next.some((item) => item.id === selected.id))
            next.unshift(selected);
          return next;
        });
      } catch (error) {
        if (sequence === productSearchSequence.current)
          message.error(errorMessage(error, "商品搜索失败"));
      } finally {
        if (sequence === productSearchSequence.current)
          setSearchingProducts(false);
      }
    },
    [productId],
  );

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      setLoading(true);
      setLoadError(undefined);
      try {
        const [productResult, shopResult] = await Promise.all([
          fetchProducts({ page: 1, pageSize: 50 }),
          queryShops({
            page: 1,
            pageSize: 500,
            platform: "ozon",
            status: "active",
            authStatus: "authorized",
          }),
        ]);
        if (cancelled) return;
        setProducts(productResult.list || []);
        const authorized = (shopResult.list || []).filter(
          (shop) =>
            shop.platform === "ozon" &&
            shop.status === "active" &&
            shop.authStatus === "authorized",
        );
        setShops(authorized);
        if (authorized.length === 1)
          setShopId((current) => current || authorized[0].id);
      } catch (error) {
        if (!cancelled)
          setLoadError(errorMessage(error, "刊登中心基础数据加载失败"));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!productId) {
      setProduct(undefined);
      setConfig(undefined);
      setAttributes([]);
      setVariantPolicy(undefined);
      setAttributeTemplateFingerprint("");
      setSKUImages([]);
      form.resetFields();
      return;
    }
    let cancelled = false;
    void fetchProductDetail(productId)
      .then((detail) => {
        if (!cancelled) {
          setProduct(detail);
          setProducts((current) =>
            current.some((item) => item.id === detail.id)
              ? current
              : [
                  {
                    id: detail.id,
                    source: detail.source,
                    title: detail.title,
                    status: detail.status,
                    currency: detail.currency,
                    createdAt: detail.createdAt,
                    updatedAt: detail.updatedAt,
                  },
                  ...current,
                ],
          );
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) setLoadError(errorMessage(error, "商品详情加载失败"));
      });
    return () => {
      cancelled = true;
    };
  }, [form, productId]);

  const hydrateConfig = useCallback(
    async (
      next: OzonProductConfig,
      detail: ProductDetail,
      isCurrent: () => boolean = () => true,
    ) => {
      const categoryId = next.categoryId || undefined;
      let nextAttributes: OzonCategoryAttribute[] = [];
      let nextVariantPolicy: OzonVariantPolicy | undefined;
      let nextTemplateFingerprint = "";
      if (categoryId) {
        const result = await queryOzonCategoryAttributes(categoryId);
        nextAttributes = addSavedDictionaryOptions(
          result.list || [],
          next.platformAttributes,
        );
        nextVariantPolicy = result.variantPolicy;
        nextTemplateFingerprint = String(result.schemaHash || "").trim();
      }
      // Store switches can finish out of order when their category templates
      // have different response times. Never hydrate a superseded shop into
      // the active form, otherwise independent store configs could appear to
      // overwrite each other in the editor.
      if (!isCurrent()) return;
      const editor = normalizeOzonAttributeEditorValues(
        nextAttributes,
        toOzonAttributeEditorValues(next.platformAttributes),
      );
      const currentSKUs = new Set(detail.skus.map((sku) => sku.id));
      const currentSKUOverrides = Object.fromEntries(
        Object.entries(editor.skuAttributeOverrides || {}).filter(([skuId]) =>
          currentSKUs.has(skuId),
        ),
      );
      const groups = complexAttributeGroups(nextAttributes);
      const complexValues = { ...(editor.complexGroups || {}) };
      groups.forEach(([complexId, defs]) => {
        const key = String(complexId);
        if (
          !complexValues[key]?.length &&
          defs.some((attribute) => attribute.required)
        )
          complexValues[key] = [{}];
      });
      const previewBySKU = new Map(
        (next.ozonPreview?.skus || []).map((sku) => [sku.skuId, sku]),
      );
      const listing = next.ozonListing;
      form.setFieldsValue({
        categoryId,
        title:
          listing?.titleOverride ||
          next.ozonPreview?.title.value ||
          productTitle(detail),
        description:
          listing?.descriptionOverride ||
          next.ozonPreview?.description.value ||
          productDescription(detail),
        currencyCode:
          listing?.currencyCode ||
          next.ozonPreview?.currency.value ||
          undefined,
        skuPrices: Object.fromEntries(
          detail.skus.map((sku) => [
            sku.id,
            listing?.skuPriceOverrides?.[sku.id] ??
              previewBySKU.get(sku.id)?.price.value ??
              sku.price,
          ]),
        ),
        package: { ...(listing?.package || {}) },
        attributes: editor.attributes || {},
        complexGroups: complexValues,
        skuVariantAttributeIds: editor.skuVariantAttributeIds || [],
        skuAttributeOverrides: currentSKUOverrides,
      });
      setAttributes(nextAttributes);
      setVariantPolicy(nextVariantPolicy);
      setAttributeTemplateFingerprint(nextTemplateFingerprint);
      setCategoryPath(next.categoryPath || categoryId || "");
      const imageView = next.ozonImages;
      setSKUImages(
        (imageView?.skus || []).map((sku) =>
          buildOzonSKUImagePreview(
            sku,
            imageView?.sharedImages || [],
            imageView?.maxImagesPerSku || 10,
          ),
        ),
      );
      setBulkImageIds([]);
      setDirty(false);
      setPreflight(undefined);
      setTemplateRefreshFeedback(undefined);
      resetAttributeViewState();
      resetAIAttributeState();
      setSKUVariantDetailsOpen(false);
    },
    [form, resetAIAttributeState, resetAttributeViewState],
  );

  useEffect(() => {
    if (!product || !shopId) {
      setConfig(undefined);
      setAttributes([]);
      setVariantPolicy(undefined);
      setAttributeTemplateFingerprint("");
      setSKUImages([]);
      setPreflight(undefined);
      resetAttributeViewState();
      return;
    }
    let cancelled = false;
    void (async () => {
      setLoadingConfig(true);
      setLoadError(undefined);
      try {
        const next = await getOzonProductConfig(product.id, shopId);
        if (cancelled) return;
        setConfig(next);
        await hydrateConfig(next, product, () => !cancelled);
      } catch (error) {
        if (!cancelled)
          setLoadError(errorMessage(error, "Ozon 商品店铺配置加载失败"));
      } finally {
        if (!cancelled) setLoadingConfig(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [hydrateConfig, product, resetAttributeViewState, shopId]);

  useEffect(() => {
    const sourceKey = String(config?.sourceCategoryKey || "").trim();
    if (!shopId || !sourceKey) {
      setCategoryMapping(undefined);
      setCategoryMappingError(undefined);
      setCategoryMappingLoading(false);
      return;
    }
    let cancelled = false;
    setCategoryMappingLoading(true);
    setCategoryMappingError(undefined);
    void listOzonCategoryMappings(shopId)
      .then((result) => {
        if (cancelled) return;
        const rows = result.list || [];
        const scoped = rows.find(
          (row) => row.sourceCategoryKey === sourceKey && row.shopId === shopId,
        );
        const tenantDefault = rows.find(
          (row) => row.sourceCategoryKey === sourceKey && !row.shopId,
        );
        setCategoryMapping(scoped || tenantDefault);
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setCategoryMapping(undefined);
          setCategoryMappingError(errorMessage(error, "类目映射证据加载失败"));
        }
      })
      .finally(() => {
        if (!cancelled) setCategoryMappingLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [config?.sourceCategoryKey, shopId]);

  useEffect(() => {
    if (!shopId) {
      setWarehouseOptions([]);
      setWarehouseError(undefined);
      setWarehouseLoading(false);
      return;
    }
    let cancelled = false;
    setWarehouseLoading(true);
    setWarehouseError(undefined);
    void listOzonWarehouses(shopId)
      .then((result) => {
        if (!cancelled) setWarehouseOptions(result.list || []);
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setWarehouseOptions([]);
          setWarehouseError(errorMessage(error, "Ozon 仓库列表读取失败"));
        }
      })
      .finally(() => {
        if (!cancelled) setWarehouseLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [shopId]);

  const markDirty = () => {
    setDirty(true);
    setPreflight(undefined);
  };

  const clearAIAttributeTracking = (attributeIDs: string[]) => {
    const changed = new Set(attributeIDs.filter(Boolean));
    if (changed.size === 0) return;
    setAIAttributeMarkers((current) =>
      Object.fromEntries(
        Object.entries(current).filter(
          ([attributeID]) => !changed.has(attributeID),
        ),
      ),
    );
    setAIAttributeUndo((current) => {
      if (!current) return current;
      const applied = Object.fromEntries(
        Object.entries(current.applied).filter(
          ([attributeID]) => !changed.has(attributeID),
        ),
      );
      return Object.keys(applied).length > 0
        ? { ...current, applied }
        : undefined;
    });
  };

  const handlePublishingValuesChange = (
    changedValues: Partial<PublishingFormValues>,
  ) => {
    const changedAttributeIDs = Object.keys(changedValues.attributes || {});
    const selectedVariantIDs = Array.isArray(
      changedValues.skuVariantAttributeIds,
    )
      ? changedValues.skuVariantAttributeIds
      : [];
    clearAIAttributeTracking([...changedAttributeIDs, ...selectedVariantIDs]);
    markDirty();
  };

  const confirmContextChange = useCallback(() => {
    if (!dirty) return Promise.resolve(true);
    return new Promise<boolean>((resolve) => {
      let settled = false;
      const finish = (confirmed: boolean) => {
        if (settled) return;
        settled = true;
        resolve(confirmed);
      };
      Modal.confirm({
        title: "放弃未保存的刊登编辑？",
        icon: <ExclamationCircleOutlined />,
        content: "切换商品或店铺会重新读取对应配置，当前未保存的修改将丢失。",
        okText: "放弃并切换",
        cancelText: "继续编辑",
        okButtonProps: { danger: true },
        onOk: () => finish(true),
        onCancel: () => finish(false),
      });
    });
  }, [dirty]);

  const selectProduct = async (value: string) => {
    if (value === productId || !(await confirmContextChange())) return;
    productSearchSequence.current += 1;
    attributeLoadSequence.current += 1;
    setDirty(false);
    setProduct(undefined);
    setConfig(undefined);
    setAttributes([]);
    setVariantPolicy(undefined);
    setAttributeTemplateFingerprint("");
    setSKUImages([]);
    form.resetFields();
    setTemplateRefreshFeedback(undefined);
    resetAttributeViewState();
    resetAIAttributeState();
    setSKUVariantDetailsOpen(false);
    setProductId(value);
    setPreflight(undefined);
    history.replace(
      `/product/publishing-center?productId=${encodeURIComponent(value)}${shopId ? `&shopId=${encodeURIComponent(shopId)}` : ""}`,
    );
  };

  const selectShop = async (value: string) => {
    if (value === shopId || !(await confirmContextChange())) return;
    attributeLoadSequence.current += 1;
    setDirty(false);
    setConfig(undefined);
    setAttributes([]);
    setVariantPolicy(undefined);
    setAttributeTemplateFingerprint("");
    setSKUImages([]);
    form.resetFields();
    setTemplateRefreshFeedback(undefined);
    resetAttributeViewState();
    resetAIAttributeState();
    setSKUVariantDetailsOpen(false);
    setShopId(value);
    setPreflight(undefined);
    if (productId)
      history.replace(
        `/product/publishing-center?productId=${encodeURIComponent(productId)}&shopId=${encodeURIComponent(value)}`,
      );
  };

  const onCategoryChange = async (
    categoryId?: string,
    canonicalPath?: string,
  ) => {
    const sequence = ++attributeLoadSequence.current;
    setAttributeTemplateFingerprint("");
    resetAIAttributeState();
    form.setFieldValue("categoryId", categoryId);
    setCategoryPath(canonicalPath || "");
    setAttributes([]);
    setVariantPolicy(undefined);
    form.setFieldsValue({
      attributes: {},
      complexGroups: {},
      skuVariantAttributeIds: [],
      skuAttributeOverrides: {},
    });
    setTemplateRefreshFeedback(undefined);
    resetAttributeViewState();
    setSKUVariantDetailsOpen(false);
    markDirty();
    if (!categoryId) return;
    try {
      const result = await queryOzonCategoryAttributes(categoryId);
      if (
        attributeLoadSequence.current !== sequence ||
        form.getFieldValue("categoryId") !== categoryId
      )
        return;
      const nextAttributes = result.list || [];
      setAttributes(nextAttributes);
      setVariantPolicy(result.variantPolicy);
      setAttributeTemplateFingerprint(String(result.schemaHash || "").trim());
      const initialGroups: Record<
        string,
        Array<Record<string, string | string[]>>
      > = {};
      complexAttributeGroups(nextAttributes).forEach(([complexId, defs]) => {
        if (defs.some((attribute) => attribute.required))
          initialGroups[String(complexId)] = [{}];
      });
      form.setFieldsValue({ complexGroups: initialGroups });
    } catch (error) {
      if (attributeLoadSequence.current !== sequence) return;
      const failure = errorMessage(error, "类目属性模板加载失败");
      setAttributeTemplateError(failure);
      message.error(failure);
    }
  };

  const searchDictionary = async (
    attribute: OzonCategoryAttribute,
    keyword: string,
  ) => {
    if (!shopId || !selectedCategoryId || keyword.trim().length < 2) return;
    const sequence =
      (dictionarySearchSequences.current[attribute.attrId] || 0) + 1;
    dictionarySearchSequences.current[attribute.attrId] = sequence;
    setSearchingAttribute(attribute.attrId);
    try {
      const result = await searchOzonDictionaryValues(
        selectedCategoryId,
        attribute.attrId,
        shopId,
        keyword.trim(),
      );
      if (dictionarySearchSequences.current[attribute.attrId] !== sequence)
        return;
      setAttributes((current) =>
        current.map((item) => {
          if (item.attrId !== attribute.attrId) return item;
          const options = [...(item.options || [])];
          (result.list || []).forEach((option) => {
            if (
              !options.some((currentOption) => currentOption.id === option.id)
            )
              options.push(option);
          });
          return { ...item, options };
        }),
      );
    } catch (error) {
      if (dictionarySearchSequences.current[attribute.attrId] === sequence)
        message.error(errorMessage(error, "Ozon 词典值搜索失败"));
    } finally {
      if (dictionarySearchSequences.current[attribute.attrId] === sequence)
        setSearchingAttribute((current) =>
          current === attribute.attrId ? undefined : current,
        );
    }
  };

  const updateSKUImage = (skuId: string, patch: OzonSKUImageSelectionPatch) => {
    const imageView = config?.ozonImages;
    setSKUImages((current) =>
      current.map((sku) =>
        sku.skuId === skuId
          ? buildOzonSKUImagePreview(
              { ...sku, ...patch },
              imageView?.sharedImages || [],
              imageView?.maxImagesPerSku || 10,
            )
          : sku,
      ),
    );
    markDirty();
  };

  const applyBulkImages = (imageIds: string[]) => {
    const imageView = config?.ozonImages;
    setSKUImages((current) =>
      current.map((sku) =>
        buildOzonSKUImagePreview(
          { ...sku, additionalImageIds: [...imageIds] },
          imageView?.sharedImages || [],
          imageView?.maxImagesPerSku || 10,
        ),
      ),
    );
    markDirty();
  };

  const resolvedPreview = config?.ozonPreview;
  const ordinaryTemplateAttributes = attributes.filter(
    (attribute) => !attribute.attributeComplexId,
  );
  const variantAttributeStates = attributes.map((attribute) => ({
    attribute,
    disabledReason: ozonVariantAttributeDisabledReason(attribute),
  }));
  const selectableVariantAttributes = variantAttributeStates
    .filter((item) => !item.disabledReason)
    .map((item) => item.attribute);
  const unavailableVariantAttributes = variantAttributeStates.filter(
    (item) => !!item.disabledReason,
  );
  const variantAttributeOptions = [
    {
      label: `可用于区分商品规格（${selectableVariantAttributes.length}）`,
      options: selectableVariantAttributes.map((attribute) => ({
        value: attribute.attrId,
        label: `${attribute.name}${attribute.required ? "（必填）" : ""}`,
      })),
    },
    ...(unavailableVariantAttributes.length > 0
      ? [
          {
            label: `不可用于区分商品规格（${unavailableVariantAttributes.length}，显示原因）`,
            options: unavailableVariantAttributes.map(
              ({ attribute, disabledReason }) => ({
                value: attribute.attrId,
                label: `${attribute.name}${attribute.required ? "（必填）" : ""} — 禁用：${disabledReason}`,
                disabled: true,
                title: disabledReason,
              }),
            ),
          },
        ]
      : []),
  ];
  const unknownVariantEligibilityCount = ordinaryTemplateAttributes.filter(
    (attribute) => attribute.skuVariantEligibilityKnown !== true,
  ).length;
  const selectedVariantAttributeIDs = watched.skuVariantAttributeIds || [];
  const variantStateByID = new Map(
    variantAttributeStates.map((item) => [item.attribute.attrId, item]),
  );
  const invalidSelectedVariantAttributes = selectedVariantAttributeIDs
    .map((attributeID) => ({
      attributeID,
      state: variantStateByID.get(attributeID),
    }))
    .filter((item) => !item.state || !!item.state.disabledReason);
  const validSelectedVariantAttributeIDs = selectedVariantAttributeIDs.filter(
    (attributeID) => {
      const state = variantStateByID.get(attributeID);
      return !!state && !state.disabledReason;
    },
  );
  const selectedVariantAttributeSet = new Set(validSelectedVariantAttributeIDs);
  const variantAttributes = selectedVariantAttributeIDs
    .map((attributeId) =>
      selectableVariantAttributes.find(
        (attribute) => attribute.attrId === attributeId,
      ),
    )
    .filter((attribute): attribute is OzonCategoryAttribute => !!attribute);
  const ordinaryAttributes = ordinaryTemplateAttributes.filter(
    (attribute) => !selectedVariantAttributeSet.has(attribute.attrId),
  );
  const complexGroups = complexAttributeGroups(attributes);

  const onVariantAttributesChange = (attributeIDs: string[]) => {
    const selected = new Set(attributeIDs);
    const values = form.getFieldsValue(true) as PublishingFormValues;
    const commonAttributes = { ...(values.attributes || {}) };
    attributeIDs.forEach((attributeID) => {
      delete commonAttributes[attributeID];
      // Ant Design merges nested objects in setFieldsValue and preserves
      // unmounted fields by default. Clear the exact path as well so an
      // attribute promoted to a SKU dimension cannot remain in the common
      // product payload and be submitted alongside per-SKU values.
      form.setFieldValue(["attributes", attributeID], undefined);
    });
    const overrides = Object.fromEntries(
      (product?.skus || []).map((sku) => [
        sku.id,
        Object.fromEntries(
          Object.entries(values.skuAttributeOverrides?.[sku.id] || {}).filter(
            ([attributeID]) => selected.has(attributeID),
          ),
        ),
      ]),
    );
    form.setFieldsValue({
      skuVariantAttributeIds: attributeIDs,
      skuAttributeOverrides: overrides,
      attributes: commonAttributes,
    });
    markDirty();
  };

  const autoMatchSKUAttributes = () => {
    if (!product || validSelectedVariantAttributeIDs.length === 0) return;
    const result = autoMatchOzonSKUAttributes(
      attributes,
      product.skus,
      validSelectedVariantAttributeIDs,
      form.getFieldValue("skuAttributeOverrides") || {},
    );
    form.setFieldValue("skuAttributeOverrides", result.values);
    markDirty();
    if (result.unresolved.length > 0) {
      message.warning(
        `已自动匹配 ${result.matchedCount} 项；另有 ${result.unresolved.length} 项需从 Ozon 词典手动选择。`,
      );
    } else {
      message.success(`已从本地 SKU 属性匹配 ${result.matchedCount} 项`);
    }
  };

  const immediateIssues = useMemo<EditorIssue[]>(() => {
    const issues: EditorIssue[] = [];
    const add = (
      key: string,
      messageText: string,
      suggestion: string,
      field?: string,
    ) => issues.push({ key, message: messageText, suggestion, field });
    if (!productId) add("product", "尚未选择商品", "请先选择要刊登的商品。");
    if (!shopId)
      add("shop", "尚未选择 Ozon 店铺", "请选择已启用且已授权的 Ozon 店铺。");
    if (!watched.categoryId)
      add(
        "category",
        "尚未选择 Ozon 类目",
        "请选择 Ozon 叶子类目。",
        "categoryId",
      );
    if (watched.categoryId && !variantPolicy)
      add(
        "variant-policy-missing",
        "当前类目的 SKU 安全策略尚未加载",
        "请重新同步并加载该叶子类目的属性模板。",
        "categoryId",
      );
    const sourceCategoryKey = String(config?.sourceCategoryKey || "").trim();
    if (sourceCategoryKey && categoryMappingError) {
      add(
        "category-mapping-load",
        "无法核对来源类目映射",
        "请刷新页面或打开类目映射维护；证据不可用时不能提交。",
        "categoryId",
      );
    } else if (sourceCategoryKey && !categoryMappingLoading) {
      if (!categoryMapping || categoryMapping.status !== "active") {
        add(
          "category-mapping-unconfirmed",
          "来源类目尚未确认对应的 Ozon 类目",
          "请核对来源类目与当前完整路径，并点击“确认当前类目映射”。",
          "categoryId",
        );
      } else if (categoryMapping.categoryId !== watched.categoryId) {
        add(
          "category-mapping-conflict",
          "当前 Ozon 类目与已确认映射冲突",
          `已确认类目为 ${categoryMapping.categoryPath || categoryMapping.categoryId}；请改回或重新人工确认。`,
          "categoryId",
        );
      } else if (!String(categoryMapping.confirmationReason || "").trim()) {
        add(
          "category-mapping-evidence-incomplete",
          "类目映射缺少人工确认理由",
          "请重新核对完整父级路径、属性模板并补充确认理由。",
          "categoryId",
        );
      }
    }
    if (!isFilled(watched.title))
      add("title", "Ozon 标题未填写", "填写标题或补齐商品标题。", "title");
    if (!isFilled(watched.description))
      add("description", "Ozon 描述未填写", "填写商品描述。", "description");
    const effectiveCurrencyCode = String(
      watched.currencyCode || preflight?.resolvedOzon?.currency.value || "",
    )
      .trim()
      .toUpperCase();
    if (!isFilled(effectiveCurrencyCode))
      add(
        "currency",
        "Ozon 币种无法确定",
        "选择币种或检查 Ozon 刊登预设。",
        "currencyCode",
      );
    else if (
      selectedShopCurrency &&
      effectiveCurrencyCode !== selectedShopCurrency
    )
      add(
        "currency-contract",
        `当前币种与 Ozon 店铺合同币种 ${selectedShopCurrency} 不一致`,
        `请改为 ${selectedShopCurrency}；Ozon 会拒绝其他币种。`,
        "currencyCode",
      );
    (product?.skus || []).forEach((sku) => {
      if (!positiveNumber(watched.skuPrices?.[sku.id])) {
        add(
          `price-${sku.id}`,
          `SKU「${sku.skuName || sku.skuCode || sku.id}」缺少有效 Ozon 售价`,
          "设置 Ozon 专属售价，或补齐本地 SKU 销售价。",
          `skuPrices.${sku.id}`,
        );
      }
      if (sku.stock === undefined || sku.stock === null)
        add(
          `stock-${sku.id}`,
          `SKU「${sku.skuName || sku.skuCode || sku.id}」尚未确认本地库存`,
          "通过现有库存调整入口确认库存；未知库存不能按 0 静默提交。",
          `skuStock.${sku.id}`,
        );
    });
    if (
      variantPolicy &&
      (product?.skus.length || 0) > variantPolicy.maxSkuCount
    )
      add(
        "sku-count-limit",
        `当前商品有 ${product?.skus.length || 0} 个 SKU，超过单次刊登安全上限 ${variantPolicy.maxSkuCount}`,
        "请拆分商品或拆成多个可独立刊登的商品配置。",
        "skuAttributeOverrides",
      );
    const pkg = watched.package || {};
    const resolvedPackage = resolvedPreview?.package;
    (
      [
        ["weightG", "重量"],
        ["widthMm", "宽度"],
        ["heightMm", "高度"],
        ["depthMm", "深度"],
      ] as const
    ).forEach(([key, label]) => {
      const value = effectivePackageValue(pkg[key], resolvedPackage?.[key]);
      if (!positiveNumber(value))
        add(
          `package-${key}`,
          `Ozon 商品${label}未设置`,
          "填写商品店铺级值，或维护全局 Ozon 刊登预设。",
          `package.${key}`,
        );
    });
    const effectiveWarehouseId = String(
      effectivePackageValue(pkg.warehouseId, resolvedPackage?.warehouseId) ??
        "",
    ).trim();
    if (shopId && warehouseError) {
      add(
        "warehouse-load",
        "无法核对当前店铺的 Ozon 仓库",
        "请确认店铺连接后重试；仓库证据不可用时不能提交。",
        "package.warehouseId",
      );
    } else if (!warehouseLoading && !effectiveWarehouseId) {
      add(
        "warehouse",
        "Ozon 仓库未设置",
        "请从当前店铺的 Ozon 仓库列表中选择。",
        "package.warehouseId",
      );
    } else if (
      !warehouseLoading &&
      effectiveWarehouseId &&
      !warehouseOptions.some(
        (warehouse) => warehouse.id === effectiveWarehouseId,
      )
    ) {
      add(
        "warehouse-invalid",
        "当前仓库不在该 Ozon 店铺的可用仓库列表中",
        "请重新选择仓库；系统不再接受未经 Ozon 核对的手填编号。",
        "package.warehouseId",
      );
    }
    if (!isFilled(effectivePackageValue(pkg.vat, resolvedPackage?.vat))) {
      add(
        "vat",
        "Ozon VAT 未设置",
        "选择适用 VAT，或维护全局 Ozon 刊登预设。",
        "package.vat",
      );
    }
    skuImages.forEach((sku) =>
      sku.issues.forEach((issue, index) => {
        add(
          `image-${sku.skuId}-${index}`,
          issue.message,
          issue.suggestion || "请检查该 SKU 图片。",
          `skuImages.${sku.skuId}`,
        );
      }),
    );
    if (
      (product?.skus.length || 0) > 1 &&
      selectedVariantAttributeIDs.length === 0
    ) {
      add(
        "sku-variant-mapping",
        "多 SKU 商品尚未选择 Ozon 变体属性",
        selectableVariantAttributes.length > 0
          ? "请选择颜色、尺码等普通类目属性，并为每个 SKU 分配唯一值。"
          : "当前模板没有可安全表达的普通变体属性；复杂组合属性不能静默作为 SKU 变体，请拆分商品或暂停提交。",
        "skuAttributeOverrides",
      );
    }
    invalidSelectedVariantAttributes.forEach(({ attributeID, state }) =>
      add(
        `variant-unavailable-${attributeID}`,
        state
          ? `已选择的 Ozon 变体属性「${state.attribute.name}」当前不可用`
          : `已选择的 Ozon 变体属性 ${attributeID} 已不在当前模板中`,
        state?.disabledReason || "请重新选择当前模板允许的变体属性。",
        "skuAttributeOverrides",
      ),
    );
    if (
      variantPolicy &&
      selectedVariantAttributeIDs.length >
        variantPolicy.maxVariantAttributeCount
    )
      add(
        "variant-dimension-limit",
        `已选择 ${selectedVariantAttributeIDs.length} 个变体维度，超过当前类目由 is_aspect 确认的上限 ${variantPolicy.maxVariantAttributeCount}`,
        "只保留 Ozon 当前模板明确允许的颜色、尺码等变体属性。",
        "skuAttributeOverrides",
      );
    const variantByID = new Map(
      selectableVariantAttributes.map((attribute) => [
        attribute.attrId,
        attribute,
      ]),
    );
    const tupleOwners = new Map<string, ProductSKURow>();
    (product?.skus || []).forEach((sku) => {
      const values = watched.skuAttributeOverrides?.[sku.id] || {};
      validSelectedVariantAttributeIDs.forEach((attributeID) => {
        const definition = variantByID.get(attributeID);
        if (!definition) return;
        if (!isFilled(values[attributeID]))
          add(
            `variant-${sku.id}-${attributeID}`,
            `SKU「${sku.skuName || sku.skuCode || sku.id}」缺少变体属性：${definition.name}`,
            definition.dictionaryId
              ? "请从 Ozon 词典中选择；本地文字不能代替 dictionaryValueId。"
              : "请填写该 SKU 对应的 Ozon 属性值。",
            `skuAttributeOverrides.${sku.id}.${attributeID}`,
          );
        const typedIssue = definition
          ? ozonTypedValueIssue(definition, values[attributeID])
          : undefined;
        if (typedIssue)
          add(
            `variant-type-${sku.id}-${attributeID}`,
            `SKU「${sku.skuName || sku.skuCode || sku.id}」的「${definition?.name || attributeID}」${typedIssue}`,
            "请按当前 Ozon valueType 修正后再提交。",
            `skuAttributeOverrides.${sku.id}.${attributeID}`,
          );
      });
      const tuple = ozonSKUVariantTuple(
        validSelectedVariantAttributeIDs,
        values,
      );
      if (!tuple || validSelectedVariantAttributeIDs.length === 0) return;
      const previous = tupleOwners.get(tuple);
      if (previous) {
        add(
          `variant-duplicate-${sku.id}`,
          `SKU「${sku.skuName || sku.skuCode || sku.id}」与「${previous.skuName || previous.skuCode || previous.id}」的变体组合重复`,
          "请为每个 SKU 分配唯一的 Ozon 变体属性组合。",
          `skuAttributeOverrides.${sku.id}`,
        );
      } else tupleOwners.set(tuple, sku);
    });
    if (
      variantPolicy &&
      tupleOwners.size > variantPolicy.maxVariantCombinationCount
    )
      add(
        "variant-combination-limit",
        `当前有 ${tupleOwners.size} 个 SKU 变体组合，超过单次刊登安全上限 ${variantPolicy.maxVariantCombinationCount}`,
        "请拆分商品后分别刊登。",
        "skuAttributeOverrides",
      );
    ordinaryAttributes.forEach((attribute) => {
      const value = watched.attributes?.[attribute.attrId];
      if (
        !supportsOzonAttributeInput(attribute) &&
        (attribute.required || isFilled(value))
      )
        add(
          `attr-type-unsupported-${attribute.attrId}`,
          `属性「${attribute.name}」的 valueType=${attribute.valueType || "未知"} 尚未受支持`,
          "请同步模板或升级系统；系统不会把未知类型当普通文本提交。",
          `attributes.${attribute.attrId}`,
        );
      if (attribute.required && !isFilled(value))
        add(
          `attr-${attribute.attrId}`,
          `Ozon 必填属性未填写：${attribute.name}`,
          "请补全当前类目属性。",
          `attributes.${attribute.attrId}`,
        );
      const typedIssue = ozonTypedValueIssue(attribute, value);
      if (typedIssue)
        add(
          `attr-type-${attribute.attrId}`,
          `属性「${attribute.name}」${typedIssue}`,
          "请按当前 Ozon valueType 修正后再提交。",
          `attributes.${attribute.attrId}`,
        );
      if (
        Array.isArray(value) &&
        attribute.maxValueCount &&
        value.length > attribute.maxValueCount
      ) {
        add(
          `attr-max-${attribute.attrId}`,
          `属性「${attribute.name}」最多允许 ${attribute.maxValueCount} 个值`,
          "减少已选择的值。",
          `attributes.${attribute.attrId}`,
        );
      }
    });
    complexGroups.forEach(([complexId, defs]) => {
      const groups = watched.complexGroups?.[String(complexId)] || [];
      if (defs.some((attribute) => attribute.required) && groups.length === 0) {
        add(
          `complex-${complexId}`,
          `Ozon 必填组合属性组 ${complexId} 尚未填写`,
          "添加一组完整的组合属性。",
          `complexGroups.${complexId}`,
        );
      }
      groups.forEach((group, index) =>
        defs.forEach((attribute) => {
          const value = group?.[attribute.attrId];
          if (
            !supportsOzonAttributeInput(attribute) &&
            (attribute.required || isFilled(value))
          )
            add(
              `complex-type-unsupported-${complexId}-${index}-${attribute.attrId}`,
              `组合属性「${attribute.name}」的 valueType=${attribute.valueType || "未知"} 尚未受支持`,
              "请同步模板或升级系统后再提交。",
              `complexGroups.${complexId}`,
            );
          if (attribute.required && !isFilled(group?.[attribute.attrId])) {
            add(
              `complex-${complexId}-${index}-${attribute.attrId}`,
              `组合属性第 ${index + 1} 组缺少：${attribute.name}`,
              "补全该字段组；系统不会按单值静默提交。",
              `complexGroups.${complexId}`,
            );
          }
          const typedIssue = ozonTypedValueIssue(attribute, value);
          if (typedIssue)
            add(
              `complex-type-${complexId}-${index}-${attribute.attrId}`,
              `组合属性「${attribute.name}」${typedIssue}`,
              "请按当前 Ozon valueType 修正后再提交。",
              `complexGroups.${complexId}`,
            );
        }),
      );
    });
    return issues;
  }, [
    complexGroups,
    categoryMapping,
    categoryMappingError,
    categoryMappingLoading,
    config?.sourceCategoryKey,
    ordinaryAttributes,
    product,
    productId,
    preflight,
    resolvedPreview,
    selectedShopCurrency,
    shopId,
    invalidSelectedVariantAttributes,
    selectedVariantAttributeIDs,
    selectableVariantAttributes,
    skuImages,
    validSelectedVariantAttributeIDs,
    variantPolicy,
    watched,
    warehouseError,
    warehouseLoading,
    warehouseOptions,
  ]);

  const filledAttributeIDs = new Set<string>();
  const completedRequiredAttributeIDs = new Set<string>();
  attributes.forEach((attribute) => {
    const complexId = Number(attribute.attributeComplexId || 0);
    let filled = false;
    let requiredComplete = false;
    if (complexId > 0) {
      const groups = watched.complexGroups?.[String(complexId)] || [];
      filled = groups.some((group) => isFilled(group?.[attribute.attrId]));
      requiredComplete =
        groups.length > 0 &&
        groups.every((group) => isFilled(group?.[attribute.attrId]));
    } else if (selectedVariantAttributeSet.has(attribute.attrId)) {
      const skuValues = (product?.skus || []).map(
        (sku) => watched.skuAttributeOverrides?.[sku.id]?.[attribute.attrId],
      );
      filled = skuValues.some(isFilled);
      requiredComplete =
        skuValues.length > 0 && skuValues.every((value) => isFilled(value));
    } else {
      filled = isFilled(watched.attributes?.[attribute.attrId]);
      requiredComplete = filled;
    }
    if (filled) filledAttributeIDs.add(attribute.attrId);
    if (attribute.required && requiredComplete)
      completedRequiredAttributeIDs.add(attribute.attrId);
  });
  const attributeProgress = summarizeOzonAttributes(
    attributes,
    filledAttributeIDs,
    completedRequiredAttributeIDs,
  );
  const attributeMatchesCurrentView = (attribute: OzonCategoryAttribute) =>
    matchesOzonAttributeView(attribute, {
      filter: attributeFilter,
      query: attributeSearch,
      filled: filledAttributeIDs.has(attribute.attrId),
    });
  const requiredAttributes = ordinaryAttributes.filter(
    (attribute) => attribute.required,
  );
  const optionalAttributes = ordinaryAttributes.filter(
    (attribute) =>
      !attribute.required && !isLowFrequencyOzonAttribute(attribute),
  );
  const lowFrequencyAttributes = ordinaryAttributes.filter(
    (attribute) =>
      !attribute.required && isLowFrequencyOzonAttribute(attribute),
  );
  const visibleRequiredAttributes = requiredAttributes.filter(
    attributeMatchesCurrentView,
  );
  const visibleOptionalAttributes = optionalAttributes.filter(
    attributeMatchesCurrentView,
  );
  const visibleLowFrequencyAttributes = lowFrequencyAttributes.filter(
    attributeMatchesCurrentView,
  );
  const visibleComplexGroups = complexGroups.filter(([, defs]) =>
    defs.some(attributeMatchesCurrentView),
  );
  const hasVisibleAttributes =
    visibleRequiredAttributes.length > 0 ||
    visibleOptionalAttributes.length > 0 ||
    visibleLowFrequencyAttributes.length > 0 ||
    visibleComplexGroups.length > 0;
  const hasActiveAttributeView =
    attributeSearch.trim().length > 0 || attributeFilter !== "all";
  const unsupportedAttributes = attributes.filter(
    (attribute) => !supportsOzonAttributeInput(attribute),
  );
  const templateCacheStale = attributes.some(
    (attribute) => attribute.cacheStale,
  );
  const lowFrequencyAttributeIDs = new Set(
    lowFrequencyAttributes.map((attribute) => attribute.attrId),
  );
  const lowFrequencyAttributeIssueCount = immediateIssues.filter((issue) => {
    if (!issue.field?.startsWith("attributes.")) return false;
    return lowFrequencyAttributeIDs.has(issue.field.split(".")[1]);
  }).length;
  const skuVariantIssueCount = immediateIssues.filter((issue) =>
    issue.field?.startsWith("skuAttributeOverrides"),
  ).length;

  const aiAttributeContext: OzonAIAttributeRequestContext | undefined =
    productId && shopId && selectedCategoryId && attributeTemplateFingerprint
      ? {
          productId,
          shopId,
          categoryId: selectedCategoryId,
          templateFingerprint: attributeTemplateFingerprint,
          generation: attributeLoadSequence.current,
        }
      : undefined;
  const aiAttributeContextRef = useRef(aiAttributeContext);
  aiAttributeContextRef.current = aiAttributeContext;
  const aiAttributeDisabledReason =
    !productId || !shopId
      ? "请先选择商品和已授权 Ozon 店铺"
      : !selectedCategoryId
        ? "请先选择 Ozon 叶子类目"
        : attributes.length === 0
          ? "当前类目属性模板尚未加载"
          : !attributeTemplateFingerprint
            ? "当前类目模板缺少指纹，请重新加载模板"
            : templateCacheStale
              ? "当前类目属性模板已过期，请刷新后再使用 AI 填写"
              : attributeTemplateError
                ? "当前类目属性模板状态异常，请刷新后再使用 AI 填写"
                : syncing
                  ? "正在刷新 Ozon 类目模板，请稍候"
                  : loadingConfig
                    ? "正在加载当前商品配置，请稍候"
                    : saving
                      ? "正在保存当前编辑，请稍候"
                      : checking
                        ? "正在运行发布前检查，请稍候"
                        : submitting
                          ? "正在提交 Ozon，请稍候"
                          : undefined;
  const aiAttributeControlsDisabled = Boolean(
    !canEdit ||
    aiAttributeDisabledReason ||
    loadingConfig ||
    syncing ||
    saving ||
    checking ||
    submitting,
  );

  const currentAIAttributeValues = () => {
    const values = form.getFieldsValue(true) as PublishingFormValues;
    return {
      attributes: cloneOzonEditorAttributes(values.attributes),
      skuVariantAttributeIds: [...(values.skuVariantAttributeIds || [])],
    };
  };

  const applyAIAttributeResult = (
    result: OzonAttributeSuggestionResult,
    snapshot: OzonAttributeSuggestionCurrentValues,
    requestedContext: OzonAIAttributeRequestContext,
  ) => {
    const requestedContextKey = aiAttributeContextKey(requestedContext);
    if (
      requestedContextKey !==
        aiAttributeContextKey(aiAttributeContextRef.current) ||
      result.context.productId !== requestedContext.productId ||
      result.context.shopId !== requestedContext.shopId ||
      result.context.categoryId !== requestedContext.categoryId ||
      result.context.templateFingerprint !==
        requestedContext.templateFingerprint
    )
      return {
        filled: 0,
        requiresReview: 0,
        notFound: 0,
        high: 0,
        medium: 0,
        low: 0,
        externalSkipped: 0,
        otherIncomplete: 0,
        details: ["编辑上下文已变化，旧建议已丢弃"],
      };

    const latest = form.getFieldsValue(true) as PublishingFormValues;
    const merged = mergeOzonAIAttributeSuggestions({
      template: attributes,
      currentAttributes: latest.attributes,
      selectedVariantAttributeIds: latest.skuVariantAttributeIds,
      hasMultipleSKUs: (product?.skus.length || 0) > 1,
      suggestions: result.suggestions || [],
    });
    if (merged.filled > 0) {
      if (Object.keys(merged.dictionaryOptions).length > 0)
        setAttributes((current) =>
          current.map((attribute) => {
            const additions = merged.dictionaryOptions[attribute.attrId] || [];
            if (additions.length === 0) return attribute;
            const optionIDs = new Set(
              (attribute.options || []).map((option) => String(option.id)),
            );
            return {
              ...attribute,
              options: [
                ...(attribute.options || []),
                ...additions.filter((option) => !optionIDs.has(option.id)),
              ],
            };
          }),
        );
      form.setFieldValue("attributes", merged.attributes);
      setAIAttributeMarkers((current) => ({
        ...current,
        ...merged.applied,
      }));
      setAIAttributeUndo({
        contextKey: requestedContextKey,
        attributes: cloneOzonEditorAttributes(snapshot.attributes),
        applied: merged.applied,
      });
      markDirty();
    }
    const details = [
      ...(result.skipped || []).map(
        (item) => `${item.attributeName || item.attributeId}：${item.reason}`,
      ),
      ...merged.rejected.map(
        (item) => `${item.attributeName || item.attributeId}：${item.reason}`,
      ),
      ...(result.warnings || []),
    ];
    const skipped = result.skipped || [];
    const serverNotFound = Math.max(
      Number(result.summary?.notFound || 0),
      skipped.length,
    );
    const skippedExternal = skipped.filter(
      (item) => item.kind === "external",
    ).length;
    const externalSkipped = Math.max(
      Number(result.summary?.externalSkipped || 0),
      skippedExternal,
    );
    const hasCategorizedSummary = [
      result.summary?.externalSkipped,
      result.summary?.unsupportedSkipped,
      result.summary?.validationSkipped,
    ].some((value) => typeof value === "number");
    const categorizedOther =
      Number(result.summary?.unsupportedSkipped || 0) +
      Number(result.summary?.validationSkipped || 0);
    const skippedOther = Math.max(0, skipped.length - skippedExternal);
    const serverOtherIncomplete = hasCategorizedSummary
      ? Math.max(categorizedOther, skippedOther)
      : Math.max(0, serverNotFound - externalSkipped, skippedOther);
    const otherIncomplete = serverOtherIncomplete + merged.rejected.length;
    return {
      filled: merged.filled,
      requiresReview: merged.requiresReview,
      notFound: Math.max(
        serverNotFound + merged.rejected.length,
        externalSkipped + otherIncomplete,
      ),
      high: merged.high,
      medium: merged.medium,
      low: merged.low,
      externalSkipped,
      otherIncomplete,
      partial:
        result.status === "partial" ||
        otherIncomplete > 0,
      details,
    };
  };

  const undoAIAttributeResult = () => {
    const current = aiAttributeUndo;
    if (
      !current ||
      current.contextKey !==
        aiAttributeContextKey(aiAttributeContextRef.current)
    ) {
      setAIAttributeUndo(undefined);
      return;
    }
    const latest = form.getFieldsValue(true) as PublishingFormValues;
    const undone = undoOzonAIAttributeSuggestions({
      currentAttributes: latest.attributes,
      snapshotAttributes: current.attributes,
      applied: current.applied,
    });
    if (undone.restoredAttributeIds.length > 0) {
      form.setFieldValue("attributes", undone.attributes);
      markDirty();
    }
    clearAIAttributeTracking(Object.keys(current.applied));
    setAIAttributeUndo(undefined);
    if (undone.preservedManualAttributeIds.length > 0)
      message.warning(
        `已撤销未修改的 AI 值；${undone.preservedManualAttributeIds.length} 个后来人工修改的字段保持不变`,
      );
    else message.success("已撤销本次 AI 填写，恢复点击前的空白值");
  };

  const submitGate = useMemo(() => {
    const reasons: string[] = [];
    const resolved = preflight?.resolvedOzon;
    if (!preflight) reasons.push("尚未运行服务端发布前检查");
    if (preflight && preflight.canPublish !== true)
      reasons.push("服务端发布前检查未通过");
    if (preflight && preflight.errorCount !== 0)
      reasons.push("服务端仍有阻断项");
    // The Go DTO omits schemaChanged when false. Only an explicit true means
    // the template changed; absence is the wire representation of "unchanged".
    if (preflight?.schemaChanged === true) reasons.push("类目模板版本尚未确认");
    if (preflight && !preflight.checkedAt) reasons.push("缺少本次检查时间");
    if (!resolved || resolved.canSubmit !== true)
      reasons.push("缺少可提交的服务端最终快照");
    if (resolved) {
      if (resolved.productId !== productId || resolved.shopId !== shopId)
        reasons.push("最终快照与当前商品或店铺不一致");
      if (!resolved.categoryId || resolved.categoryId !== selectedCategoryId)
        reasons.push("最终快照类目与当前选择不一致");
      if (
        !isFilled(resolved.title.value) ||
        !isFilled(resolved.description.value) ||
        !isFilled(resolved.currency.value)
      )
        reasons.push("最终快照标题、描述或币种不完整");
      if (
        !positiveNumber(resolved.package.weightG.value) ||
        !positiveNumber(resolved.package.widthMm.value) ||
        !positiveNumber(resolved.package.heightMm.value) ||
        !positiveNumber(resolved.package.depthMm.value) ||
        !isFilled(resolved.package.warehouseId.value) ||
        !isFilled(resolved.package.vat.value)
      )
        reasons.push("最终快照包裹、仓库或 VAT 不完整");
      if (
        !product ||
        resolved.skus.length !== product.skus.length ||
        resolved.skus.length === 0
      )
        reasons.push("最终快照 SKU 数量与当前商品不一致");
      if (
        resolved.skus.some(
          (sku) =>
            sku.canSubmit !== true ||
            !positiveNumber(sku.price.value) ||
            sku.localStock < 0 ||
            sku.images.length === 0,
        )
      )
        reasons.push("最终快照仍有 SKU 价格、库存、主图或变体阻断项");
    }
    if (immediateIssues.length > 0) reasons.push("页面即时检查仍有阻断项");
    if (dirty) reasons.push("存在未保存修改");
    return {
      ready: reasons.length === 0,
      reasons: Array.from(new Set(reasons)),
    };
  }, [
    dirty,
    immediateIssues.length,
    preflight,
    product,
    productId,
    selectedCategoryId,
    shopId,
  ]);

  const saveCurrent = useCallback(async () => {
    if (!product || !productId || !shopId)
      throw new Error("请先选择商品和 Ozon 店铺");
    const values = form.getFieldsValue(true) as PublishingFormValues;
    if (!values.categoryId) throw new Error("请先选择 Ozon 叶子类目");
    setSaving(true);
    try {
      const localTitle = productTitle(product);
      const localDescription = productDescription(product);
      const currentListing = config?.ozonListing;
      const currency = String(values.currencyCode || "")
        .trim()
        .toUpperCase();
      const skuPriceOverrides = Object.fromEntries(
        product.skus
          .map((sku) => [sku.id, values.skuPrices?.[sku.id]] as const)
          .filter(([skuId, price]) => {
            const local = product.skus.find((sku) => sku.id === skuId)?.price;
            return (
              positiveNumber(price) &&
              (local === undefined ||
                local === null ||
                Math.abs(Number(price) - local) > 0.000001)
            );
          })
          .map(([skuId, price]) => [skuId, Number(price)]),
      );
      const effectiveCurrency = config?.ozonPreview?.currency;
      const keepCurrencyOverride =
        currentListing?.currencyCode ||
        !effectiveCurrency ||
        currency !==
          String(effectiveCurrency.value || "")
            .trim()
            .toUpperCase();
      const saved = await saveOzonProductConfig(productId, {
        shopId,
        categoryId: values.categoryId,
        categoryPath,
        platformAttributes: buildOzonPlatformAttributesV3(attributes, {
          attributes: values.attributes,
          complexGroups: values.complexGroups,
          skuVariantAttributeIds: values.skuVariantAttributeIds,
          skuAttributeOverrides: values.skuAttributeOverrides,
        }),
        ozonImages: toOzonImageConfigInput(skuImages),
        ozonListing: {
          version: 1,
          titleOverride:
            String(values.title || "").trim() === localTitle
              ? undefined
              : String(values.title || "").trim(),
          descriptionOverride:
            String(values.description || "").trim() === localDescription
              ? undefined
              : String(values.description || "").trim(),
          currencyCode: keepCurrencyOverride ? currency : undefined,
          skuPriceOverrides,
          package: {
            weightG: values.package?.weightG,
            widthMm: values.package?.widthMm,
            heightMm: values.package?.heightMm,
            depthMm: values.package?.depthMm,
            warehouseId:
              String(values.package?.warehouseId || "").trim() || undefined,
            vat: String(values.package?.vat || "").trim() || undefined,
          },
        },
      });
      setConfig(saved);
      setSKUImages(
        (saved.ozonImages?.skus || []).map((sku) =>
          buildOzonSKUImagePreview(
            sku,
            saved.ozonImages?.sharedImages || [],
            saved.ozonImages?.maxImagesPerSku || 10,
          ),
        ),
      );
      setDirty(false);
      setPreflight(undefined);
      resetAIAttributeState();
      message.success("当前编辑已保存到 TradeMind，不会提交 Ozon");
      return saved;
    } finally {
      setSaving(false);
    }
  }, [
    attributes,
    categoryPath,
    config,
    form,
    product,
    productId,
    resetAIAttributeState,
    shopId,
    skuImages,
  ]);

  const runPreflight = async () => {
    if (!productId || !shopId) return;
    if (dirty) {
      Modal.warning({
        title: "有未保存修改，尚未运行检查",
        content:
          "发布前检查只读取已保存的商品店铺配置，不会自动保存当前页面。试探类目、误选属性和临时编辑均不会被写入；确认无误后，请先单独点击“保存当前编辑（不提交）”。",
        okText: "返回确认并保存",
      });
      return;
    }
    if (!config?.id) {
      Modal.warning({
        title: "尚无已保存配置，无法运行检查",
        content:
          "请先核对当前类目、属性、图片和 SKU，再单独保存。保存只写入 TradeMind 配置；发布前检查仍是下一步独立的只读操作。",
        okText: "返回核对",
      });
      return;
    }
    setChecking(true);
    try {
      const result = await validateOzonReadiness(productId, shopId);
      setPreflight(result);
      if (result.canPublish)
        message.success("只读发布前检查已通过，可单独进入提交确认");
      else
        message.warning(
          `发布前检查未通过：${result.errorCount ?? result.checks?.length ?? 0} 项错误`,
        );
    } catch (error) {
      message.error(errorMessage(error, "发布前检查失败"));
    } finally {
      setChecking(false);
    }
  };

  const confirmCurrentCategoryMapping = () => {
    const sourceCategoryKey = String(config?.sourceCategoryKey || "").trim();
    const sourceCategoryName = String(config?.sourceCategoryName || "").trim();
    const categoryId = String(selectedCategoryId || "").trim();
    if (!shopId || !sourceCategoryKey || !categoryId) return;
    const path = categoryPath || categoryId;
    const selectionMethod =
      appliedRecommendationCategoryID === categoryId
        ? "recommended_then_manual"
        : "manual";
    let confirmationReason = "";
    let confirmationModal: ReturnType<typeof Modal.confirm> | undefined;
    confirmationModal = Modal.confirm({
      title: "确认来源类目与 Ozon 类目映射？",
      icon: <ExclamationCircleOutlined />,
      width: 620,
      content: (
        <Space direction="vertical" size={8} style={{ width: "100%" }}>
          <Alert
            type="warning"
            showIcon
            message="该确认会影响同一店铺下相同来源类目的后续商品"
            description="请核对商品类型与完整叶子类目路径；系统不会根据标题关键词自动替您确认。"
          />
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label="来源类目">
              {sourceCategoryName || sourceCategoryKey}
            </Descriptions.Item>
            <Descriptions.Item label="Ozon 类目">{path}</Descriptions.Item>
            <Descriptions.Item label="选择方式">
              {selectionMethod === "recommended_then_manual"
                ? "系统推荐后人工逐级确认"
                : "人工逐级选择"}
            </Descriptions.Item>
          </Descriptions>
          <Collapse
            ghost
            size="small"
            items={[
              {
                key: "confirm-mapping-technical-info",
                label: "技术信息",
                children: (
                  <Typography.Text type="secondary">
                    description_category_id：{selectedCategoryParts[0] || "—"}
                    ；type_id：{selectedCategoryParts[1] || "—"}
                  </Typography.Text>
                ),
              },
            ]}
          />
          <div>
            <Typography.Text strong>确认理由（必填）</Typography.Text>
            <Input.TextArea
              rows={3}
              maxLength={1000}
              showCount
              placeholder="例如：商品用途、材质和规格与该 Ozon 叶子类目一致"
              onChange={(event) => {
                confirmationReason = event.target.value;
                confirmationModal?.update({
                  okButtonProps: {
                    disabled: !confirmationReason.trim(),
                  },
                });
              }}
            />
          </div>
        </Space>
      ),
      okText: "确认当前映射",
      cancelText: "返回核对",
      okButtonProps: { disabled: true },
      onOk: async () => {
        if (!confirmationReason.trim()) return;
        setConfirmingCategoryMapping(true);
        try {
          const saved = await saveOzonCategoryMapping({
            shopId,
            sourceCategoryKey,
            sourceCategoryName: sourceCategoryName || undefined,
            categoryId,
            categoryPath: path,
            status: "active",
            selectionMethod,
            confirmationReason: confirmationReason.trim(),
          });
          setCategoryMapping(saved);
          setCategoryMappingError(undefined);
          setPreflight(undefined);
          message.success("已确认当前来源类目映射；请重新运行发布前检查");
        } finally {
          setConfirmingCategoryMapping(false);
        }
      },
    });
  };

  const confirmSubmit = () => {
    if (!productId || !shopId || !preflight?.canPublish || !submitGate.ready)
      return;
    const snapshot = preflight.resolvedOzon;
    if (!snapshot) return;
    const priceValues = snapshot.skus.map((sku) => sku.price.value);
    const stockValues = snapshot.skus.map((sku) => sku.localStock);
    const priceMin = Math.min(...priceValues);
    const priceMax = Math.max(...priceValues);
    const stockTotal = stockValues.reduce((sum, value) => sum + value, 0);
    const imageTotal = snapshot.skus.reduce(
      (sum, sku) => sum + sku.images.length,
      0,
    );
    const snapshotCategoryParts = String(snapshot.categoryId || "").split(
      ":",
      2,
    );
    const shopName =
      shops.find((shop) => shop.id === shopId)?.shopName || shopId;
    const idempotencyKey = newIdempotencyKey();
    Modal.confirm({
      title: "确认提交到 Ozon？",
      icon: <ExclamationCircleOutlined />,
      width: 760,
      content: (
        <Space
          direction="vertical"
          size={12}
          style={{ width: "100%", maxHeight: "65vh", overflowY: "auto" }}
        >
          <Alert
            type="warning"
            showIcon
            message="即将调用 Ozon 写接口"
            description="提交后请在刊登进度中等待“成功上架”；“Ozon 已接收”或“审核中”都不代表已经可售。"
          />
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label="店铺">{shopName}</Descriptions.Item>
            <Descriptions.Item label="商品">
              {snapshot.title.value || productTitle(product)}
            </Descriptions.Item>
            <Descriptions.Item label="Ozon 类目">
              {snapshot.categoryPath || categoryPath}
            </Descriptions.Item>
            <Descriptions.Item label="SKU 数">
              {snapshot.skus.length}
            </Descriptions.Item>
            <Descriptions.Item label="售价摘要">
              {priceMin === priceMax ? priceMin : `${priceMin} – ${priceMax}`}{" "}
              {snapshot.currency.value}
            </Descriptions.Item>
            <Descriptions.Item label="库存摘要">
              合计 {stockTotal}；最低单 SKU {Math.min(...stockValues)}
            </Descriptions.Item>
            <Descriptions.Item label="图片摘要">
              合计 {imageTotal} 张；
              {snapshot.skus
                .map(
                  (sku) =>
                    `${sku.skuCode || sku.skuName || sku.skuId} ${sku.images.length} 张`,
                )
                .join("，")}
            </Descriptions.Item>
            <Descriptions.Item label="币种">
              {snapshot.currency.value}
            </Descriptions.Item>
            <Descriptions.Item label="仓库">
              {snapshot.package.warehouseId.value}
            </Descriptions.Item>
            <Descriptions.Item label="增值税率">
              {ozonVATLabel(snapshot.package.vat.value)}
            </Descriptions.Item>
            <Descriptions.Item label="重量与尺寸">
              {snapshot.package.weightG.value} g；
              {snapshot.package.widthMm.value} ×{" "}
              {snapshot.package.heightMm.value} ×{" "}
              {snapshot.package.depthMm.value} mm
            </Descriptions.Item>
          </Descriptions>
          <Collapse
            ghost
            size="small"
            items={[
              {
                key: "submit-technical-info",
                label: "技术信息",
                children: (
                  <Descriptions bordered size="small" column={1}>
                    <Descriptions.Item label="description_category_id">
                      {snapshotCategoryParts[0] || "—"}
                    </Descriptions.Item>
                    <Descriptions.Item label="type_id">
                      {snapshotCategoryParts[1] || "—"}
                    </Descriptions.Item>
                    <Descriptions.Item label="模板摘要">
                      {snapshot.schemaHash || "—"}
                    </Descriptions.Item>
                  </Descriptions>
                ),
              },
            ]}
          />
          <Typography.Text type="secondary">
            本次提交严格使用上方服务端最终快照；任一字段变化都必须重新运行发布前检查。
          </Typography.Text>
        </Space>
      ),
      okText: "确认提交到 Ozon",
      cancelText: "返回继续检查",
      okButtonProps: { danger: true },
      onOk: async () => {
        setSubmitting(true);
        try {
          const task = await publishOzonProduct(
            productId,
            shopId,
            idempotencyKey,
          );
          message.success("已创建 Ozon 刊登提交，正在处理");
          history.push(
            `/product/publish-tasks?tab=tasks&id=${encodeURIComponent(task.id)}`,
          );
        } catch (error) {
          message.error(
            errorMessage(
              error,
              "Ozon 提交失败，请核对刊登进度后再决定是否重试",
            ),
          );
          throw error;
        } finally {
          setSubmitting(false);
        }
      },
    });
  };

  const syncCurrentTemplate = async () => {
    if (!selectedCategoryId || !shopId) return;
    const categoryId = selectedCategoryId;
    const sequence = ++attributeLoadSequence.current;
    setAttributeTemplateFingerprint("");
    resetAIAttributeState();
    const values = form.getFieldsValue(true) as PublishingFormValues;
    const currentPayload = buildOzonPlatformAttributesV3(attributes, {
      attributes: values.attributes,
      complexGroups: values.complexGroups,
      skuVariantAttributeIds: values.skuVariantAttributeIds,
      skuAttributeOverrides: values.skuAttributeOverrides,
    });
    setSyncing(true);
    setTemplateRefreshFeedback(undefined);
    setAttributeTemplateError(undefined);
    try {
      const synced = await syncOzonCategoryAttributes(categoryId, shopId);
      const result = await queryOzonCategoryAttributes(categoryId, {
        refreshKey: Date.now(),
      });
      if (
        attributeLoadSequence.current !== sequence ||
        form.getFieldValue("categoryId") !== categoryId
      )
        return;
      const fetchedAttributes = result.list || [];
      if (fetchedAttributes.length === 0)
        throw new Error(
          "Ozon 返回的最新类目属性模板为空，页面仍保留刷新前模板；请核对店铺凭证后重试。",
        );
      if (
        Number.isFinite(synced.count) &&
        synced.count >= 0 &&
        synced.count !== fetchedAttributes.length
      )
        throw new Error(
          `模板刷新结果尚未一致（Ozon 返回 ${synced.count} 个，页面读取 ${fetchedAttributes.length} 个），未替换当前页面模板，请重试。`,
        );
      const nextAttributes = addSavedDictionaryOptions(
        fetchedAttributes,
        currentPayload,
      );
      const editor = normalizeOzonAttributeEditorValues(
        nextAttributes,
        toOzonAttributeEditorValues(currentPayload),
      );
      const nextComplexGroups = { ...(editor.complexGroups || {}) };
      complexAttributeGroups(nextAttributes).forEach(([complexId, defs]) => {
        const key = String(complexId);
        if (
          !nextComplexGroups[key]?.length &&
          defs.some((attribute) => attribute.required)
        )
          nextComplexGroups[key] = [{}];
      });
      form.setFieldValue("attributes", editor.attributes || {});
      form.setFieldValue("complexGroups", nextComplexGroups);
      form.setFieldValue(
        "skuVariantAttributeIds",
        editor.skuVariantAttributeIds || [],
      );
      form.setFieldValue(
        "skuAttributeOverrides",
        editor.skuAttributeOverrides || {},
      );
      setAttributes(nextAttributes);
      setVariantPolicy(result.variantPolicy);
      setAttributeTemplateFingerprint(String(result.schemaHash || "").trim());
      resetAttributeViewState();
      setDirty(true);
      setPreflight(undefined);
      const latestSyncedAt = nextAttributes.reduce<string | undefined>(
        (latest, attribute) => {
          if (!attribute.syncedAt) return latest;
          if (!latest) return attribute.syncedAt;
          return Date.parse(attribute.syncedAt) > Date.parse(latest)
            ? attribute.syncedAt
            : latest;
        },
        undefined,
      );
      const feedback = {
        categoryId,
        attributeCount: nextAttributes.length,
        requiredCount: nextAttributes.filter((attribute) => attribute.required)
          .length,
        syncedAt: latestSyncedAt,
      };
      setTemplateRefreshFeedback(feedback);
      message.success(
        `最新类目属性模板已重新加载，页面已生效（${feedback.attributeCount} 个属性）`,
      );
    } catch (error) {
      if (attributeLoadSequence.current !== sequence) return;
      const failure = errorMessage(error, "属性模板同步失败");
      setAttributeTemplateError(
        `${failure} 当前仍展示刷新前的模板与已填写内容。`,
      );
      message.error(failure);
    } finally {
      setSyncing(false);
    }
  };

  const syncCategoryCache = async () => {
    if (!shopId) return;
    setSyncing(true);
    try {
      await syncOzonCategoryFlow(shopId);
      setCategoryNavigatorRefreshToken((current) => current + 1);
      message.success("Ozon 类目缓存同步已启动");
    } catch (error) {
      message.error(errorMessage(error, "类目缓存同步失败"));
    } finally {
      setSyncing(false);
    }
  };

  const goToIssue = (issue: EditorIssue) => {
    setActiveStep(stepForIssue(issue));
    const field = issue.field;
    if (!field) return;
    if (field.startsWith("attributes.") || field.startsWith("complexGroups.")) {
      setAttributeSearch("");
      setAttributeFilter("all");
      setOptionalAttributesOpen(true);
      if (
        field.startsWith("attributes.") &&
        lowFrequencyAttributeIDs.has(field.split(".")[1])
      )
        setLowFrequencyAttributesOpen(true);
    }
    if (field.startsWith("skuAttributeOverrides"))
      setSKUVariantDetailsOpen(true);
    globalThis.setTimeout(() => {
      if (field.startsWith("skuImages.")) {
        document
          .getElementById("ozon-sku-images")
          ?.scrollIntoView({ behavior: "smooth", block: "start" });
        return;
      }
      if (field.startsWith("skuStock.")) {
        document
          .getElementById(`field-${field.replace(".", "-")}`)
          ?.scrollIntoView({ behavior: "smooth", block: "center" });
        return;
      }
      if (field.startsWith("skuAttributeOverrides")) {
        const skuId = field.split(".")[1];
        document
          .getElementById(
            skuId
              ? `field-skuAttributeOverrides-${skuId}`
              : "ozon-sku-variant-mapping",
          )
          ?.scrollIntoView({ behavior: "smooth", block: "center" });
        if (field.split(".").length > 2)
          form.scrollToField(field.split("."), {
            behavior: "smooth",
            block: "center",
          });
        return;
      }
      form.scrollToField(field.split("."), {
        behavior: "smooth",
        block: "center",
      });
    }, 0);
  };

  const currentImageView: OzonImageConfigView | undefined = config?.ozonImages
    ? { ...config.ozonImages, skus: skuImages }
    : undefined;
  const finalPreview: OzonResolvedListing | undefined = preflight?.resolvedOzon;

  const renderAttributeLabel = (attribute: OzonCategoryAttribute) => {
    const formatHint = ozonAttributeFormatHint(attribute);
    const aiMarker = aiAttributeMarkers[attribute.attrId];
    const confidenceLabel =
      aiMarker?.confidenceLevel === "high"
        ? "高可信"
        : aiMarker?.confidenceLevel === "medium"
          ? "中可信"
          : "低可信";
    const confidenceColor =
      aiMarker?.confidenceLevel === "high"
        ? "success"
        : aiMarker?.confidenceLevel === "medium"
          ? "warning"
          : "error";
    const reviewLabel =
      aiMarker?.confidenceLevel === "low"
        ? "重点核对"
        : aiMarker?.confidenceLevel === "medium"
          ? "建议核对"
          : "";
    return (
      <Space size={4} wrap>
        <span>{attribute.name}</span>
        {formatHint ? <Tag>{formatHint}</Tag> : null}
        {aiMarker ? (
          <Tag
            color="processing"
            title={aiMarker.reason}
            aria-label={`${attribute.name}：AI 建议${aiMarker.reason ? `，依据：${aiMarker.reason}` : ""}`}
          >
            AI 建议
          </Tag>
        ) : null}
        {aiMarker ? (
          <Tag
            color={confidenceColor}
            aria-label={`${attribute.name}：${confidenceLabel}`}
          >
            {confidenceLabel}
          </Tag>
        ) : null}
        {aiMarker?.requiresReview && reviewLabel ? (
          <Tag
            color={aiMarker.confidenceLevel === "low" ? "error" : "warning"}
            aria-label={`${attribute.name}：${reviewLabel}`}
          >
            {reviewLabel}
          </Tag>
        ) : null}
      </Space>
    );
  };

  const renderAttributeHelp = (
    attribute: OzonCategoryAttribute,
    instanceKey: string,
  ) => {
    const description = String(attribute.description || "").trim();
    const maximum =
      attribute.isCollection && attribute.maxValueCount
        ? `最多 ${attribute.maxValueCount} 个值`
        : "";
    if (!description && !maximum) return undefined;

    const expandable = description.length > 16;
    const expanded = expandedAttributeDescriptions.has(instanceKey);
    const descriptionID = `ozon-attribute-description-${instanceKey.replace(
      /[^a-zA-Z0-9_-]/g,
      "-",
    )}`;
    return (
      <div className="publishing-center__attribute-help">
        {description ? (
          <div className="publishing-center__attribute-description-row">
            <span
              id={descriptionID}
              className={
                expanded
                  ? "publishing-center__attribute-description publishing-center__attribute-description--expanded"
                  : "publishing-center__attribute-description"
              }
            >
              {description}
            </span>
            {expandable ? (
              <button
                type="button"
                className="publishing-center__attribute-description-toggle"
                aria-controls={descriptionID}
                aria-expanded={expanded}
                aria-label={`${expanded ? "收起" : "展开"}${attribute.name}说明`}
                onClick={() =>
                  setExpandedAttributeDescriptions((current) => {
                    const next = new Set(current);
                    if (next.has(instanceKey)) next.delete(instanceKey);
                    else next.add(instanceKey);
                    return next;
                  })
                }
              >
                {expanded ? "收起" : "展开全文"}
              </button>
            ) : null}
          </div>
        ) : null}
        {maximum ? (
          <Typography.Text type="secondary">{maximum}</Typography.Text>
        ) : null}
      </div>
    );
  };

  const renderAttributeInput = (attribute: OzonCategoryAttribute) => {
    if (attribute.dictionaryId) {
      return (
        <Select
          showSearch
          allowClear={!attribute.required}
          mode={attribute.isCollection ? "multiple" : undefined}
          maxCount={
            attribute.isCollection
              ? attribute.maxValueCount || undefined
              : undefined
          }
          filterOption={(input, option) =>
            String(option?.label || "")
              .toLowerCase()
              .includes(input.toLowerCase())
          }
          loading={searchingAttribute === attribute.attrId}
          placeholder={
            attribute.isCollection
              ? "可多选；输入至少 2 个字符搜索 Ozon 词典"
              : "选择或搜索 Ozon 词典值"
          }
          options={(attribute.options || []).map((option) => ({
            value: option.id,
            label: option.value,
          }))}
          onSearch={(keyword) => void searchDictionary(attribute, keyword)}
          onChange={() => markDirty()}
        />
      );
    }
    if (!supportsOzonAttributeInput(attribute)) {
      return (
        <Input
          disabled
          status="error"
          placeholder={`暂不支持 valueType=${attribute.valueType || "未知"}`}
        />
      );
    }
    const valueType = normalizedOzonValueType(attribute);
    if (["boolean", "bool"].includes(valueType)) {
      return (
        <Select
          allowClear={!attribute.required}
          mode={attribute.isCollection ? "multiple" : undefined}
          maxCount={
            attribute.isCollection
              ? attribute.maxValueCount || undefined
              : undefined
          }
          placeholder="选择是或否"
          options={[
            { value: "true", label: "是（true）" },
            { value: "false", label: "否（false）" },
          ]}
          onChange={() => markDirty()}
        />
      );
    }
    if (
      [
        "integer",
        "int",
        "int64",
        "decimal",
        "float",
        "double",
        "number",
      ].includes(valueType) &&
      !attribute.isCollection
    ) {
      const integer = ["integer", "int", "int64"].includes(valueType);
      return (
        <InputNumber
          stringMode
          precision={integer ? 0 : undefined}
          controls
          style={{ width: "100%" }}
          placeholder={integer ? "填写整数" : "填写十进制数"}
          onChange={() => markDirty()}
        />
      );
    }
    if (["url", "uri", "image"].includes(valueType)) {
      if (attribute.isCollection) {
        return (
          <Select
            mode="tags"
            maxCount={attribute.maxValueCount || undefined}
            tokenSeparators={[",", "，"]}
            placeholder="输入完整 http/https URL 后按回车"
            onChange={() => markDirty()}
          />
        );
      }
      return (
        <Input
          type="url"
          placeholder="https://example.com/..."
          onChange={markDirty}
        />
      );
    }
    if (valueType === "date" && !attribute.isCollection) {
      return <Input type="date" onChange={markDirty} />;
    }
    if (
      ["datetime", "date_time", "timestamp"].includes(valueType) &&
      !attribute.isCollection
    ) {
      return (
        <Input
          placeholder="例如 2026-08-10T12:00:00+03:00"
          onChange={markDirty}
        />
      );
    }
    if (attribute.isCollection) {
      return (
        <Select
          mode="tags"
          maxCount={attribute.maxValueCount || undefined}
          tokenSeparators={[",", "，"]}
          placeholder={
            ["integer", "int", "int64"].includes(valueType)
              ? "输入多个整数后按回车"
              : ["decimal", "float", "double", "number"].includes(valueType)
                ? "输入多个十进制数后按回车"
                : "输入多个值后按回车"
          }
          onChange={() => markDirty()}
        />
      );
    }
    return <Input placeholder={`填写${attribute.name}`} onChange={markDirty} />;
  };

  const renderOrdinaryAttribute = (attribute: OzonCategoryAttribute) => (
    <Form.Item
      key={attribute.attrId}
      name={["attributes", attribute.attrId]}
      label={renderAttributeLabel(attribute)}
      required={attribute.required}
      preserve
      extra={renderAttributeHelp(attribute, `ordinary-${attribute.attrId}`)}
    >
      {renderAttributeInput(attribute)}
    </Form.Item>
  );

  const localSKUByID = new Map(
    (product?.skus || []).map((sku) => [sku.id, sku]),
  );
  const currentAttributePayload = buildOzonPlatformAttributesV3(attributes, {
    attributes: watched.attributes,
    complexGroups: watched.complexGroups,
    skuVariantAttributeIds: selectedVariantAttributeIDs,
    skuAttributeOverrides: watched.skuAttributeOverrides,
  });
  const previewSKUs =
    finalPreview?.skus ||
    (product?.skus || []).map((sku) => ({
      skuId: sku.id,
      skuCode: sku.skuCode,
      skuName: sku.skuName,
      price: {
        value: Number(watched.skuPrices?.[sku.id] || 0),
        source:
          (config?.ozonListing?.skuPriceOverrides?.[sku.id] !== undefined &&
            config?.ozonListing?.skuPriceOverrides?.[sku.id] !== null) ||
          watched.skuPrices?.[sku.id] !== sku.price
            ? "ozon_product_shop_config"
            : "product",
      },
      localStock: sku.stock || 0,
      stockSource: "local_inventory",
      images:
        skuImages.find((item) => item.skuId === sku.id)?.finalImages || [],
      platformAttributes: {
        version: 3 as const,
        attributes: {
          ...currentAttributePayload.attributes,
          ...(currentAttributePayload.skuAttributeOverrides[sku.id] || {}),
        },
        complexGroups: currentAttributePayload.complexGroups,
        skuVariantAttributeIds: currentAttributePayload.skuVariantAttributeIds,
      },
      attributeSources: Object.fromEntries(
        selectedVariantAttributeIDs.map((attributeID) => [
          attributeID,
          "ozon_sku_shop_config",
        ]),
      ),
      canSubmit: !immediateIssues.some((issue) => issue.key.includes(sku.id)),
      issues: [],
    }));
  const previewProblemSKUs = previewSKUs.filter(
    (sku) => sku.canSubmit !== true || (sku.issues || []).length > 0,
  );
  const previewStockTotal = previewSKUs.reduce(
    (total, sku) => total + Number(sku.localStock || 0),
    0,
  );
  const previewImageTotal = previewSKUs.reduce(
    (total, sku) => total + sku.images.length,
    0,
  );

  return (
    <PermissionGuard require={PERMISSIONS.PRODUCT_VIEW} showForbiddenPage>
      <TmPageContainer
        title="刊登中心"
        subTitle="统一配置商品、平台和店铺；首期完整支持 Ozon。保存只写入 TradeMind 配置，不会提交平台。"
      >
        {loadError ? (
          <Alert
            type="error"
            showIcon
            message="刊登中心加载失败"
            description={loadError}
            closable
            onClose={() => setLoadError(undefined)}
          />
        ) : null}
        {loading ? <Spin fullscreen tip="正在加载刊登中心…" /> : null}
        <div
          className="publishing-center__wizard"
          data-config-ready={String(!loadingConfig && !!config)}
        >
          <Steps
            current={activeStep}
            responsive
            onChange={(step) => setActiveStep(step as PublishingStep)}
            items={publishingStepItems.map((item, step) => {
              const blockerCount = immediateIssues.filter(
                (issue) => stepForIssue(issue) === step,
              ).length;
              return {
                ...item,
                description: blockerCount
                  ? `${blockerCount} 项阻断`
                  : "无即时阻断",
                status:
                  blockerCount > 0 && step <= activeStep ? "error" : undefined,
              };
            })}
          />
          <Alert
            type={immediateIssues.length > 0 ? "error" : "success"}
            showIcon
            message={
              immediateIssues.length > 0
                ? `发布前仍有 ${immediateIssues.length} 项阻断`
                : "即时阻断项已清零"
            }
            description={
              immediateIssues.length > 0
                ? "可继续查看其他步骤，但真实提交始终保持锁定。"
                : "请在最后一步运行服务端只读检查；即时检查通过不等于平台已上架。"
            }
            action={
              immediateIssues.length > 0 ? (
                <Button onClick={() => goToIssue(immediateIssues[0])}>
                  定位并修复第一项
                </Button>
              ) : undefined
            }
          />
        </div>
        <Form<PublishingFormValues>
          form={form}
          layout="vertical"
          onValuesChange={handlePublishingValuesChange}
          disabled={!canEdit || loadingConfig}
        >
          {activeStep === 0 || activeStep === 2 ? (
            <SectionCard
              title={
                activeStep === 2 ? "Ozon 类目选择与映射证据" : "店铺与商品"
              }
              description={
                activeStep === 2
                  ? "使用完整中文路径选择叶子类目；智能推荐只带入候选，必须人工确认映射。"
                  : "先选择商品草稿与已授权 Ozon 店铺。切换上下文前会保护未保存编辑。"
              }
              className="publishing-center__context"
            >
              <div className="publishing-center__context-grid">
                {activeStep === 0 ? (
                  <>
                    <div>
                      <Typography.Text strong>商品</Typography.Text>
                      <Select
                        showSearch
                        value={productId}
                        placeholder="选择商品草稿"
                        filterOption={false}
                        loading={searchingProducts}
                        options={products.map((item) => ({
                          value: item.id,
                          label: productTitle(item) || item.id,
                        }))}
                        onSearch={(keyword) => void loadProductOptions(keyword)}
                        onChange={(value) => void selectProduct(value)}
                      />
                    </div>
                    <div>
                      <Typography.Text strong>目标平台</Typography.Text>
                      <Select
                        value="ozon"
                        options={[
                          { value: "ozon", label: "Ozon（首期完整支持）" },
                          {
                            value: "shopee",
                            label: "Shopee（尚未接入完整字段）",
                            disabled: true,
                          },
                          {
                            value: "lazada",
                            label: "Lazada（尚未接入完整字段）",
                            disabled: true,
                          },
                          {
                            value: "tiktok",
                            label: "TikTok Shop（尚未接入完整字段）",
                            disabled: true,
                          },
                        ]}
                      />
                    </div>
                    <div>
                      <Typography.Text strong>Ozon 店铺</Typography.Text>
                      <Select
                        showSearch
                        value={shopId}
                        placeholder="选择已授权店铺"
                        optionFilterProp="label"
                        options={shops.map((shop) => ({
                          value: shop.id,
                          label: shop.shopName || shop.id,
                        }))}
                        onChange={(value) => void selectShop(value)}
                      />
                    </div>
                  </>
                ) : null}
              </div>
              {activeStep === 2 ? (
                <>
                  <Form.Item name="categoryId" hidden>
                    <Input />
                  </Form.Item>
                  <AICategoryRecommendationPanel
                    productId={productId}
                    shopId={shopId}
                    disabled={loadingConfig}
                    readOnly={!canEdit}
                    onApply={async (
                      candidate: OzonProductCategoryRecommendationCandidate,
                    ) => {
                      setCategoryFocusTarget({
                        categoryId: candidate.categoryId,
                        requestId: Date.now(),
                      });
                      await onCategoryChange(
                        candidate.categoryId,
                        candidate.categoryPath || candidate.categoryId,
                      );
                      setAppliedRecommendationCategoryID(candidate.categoryId);
                      message.success(
                        "候选类目已应用到当前未保存表单；请人工核对属性后明确保存",
                      );
                    }}
                  />
                  <OzonCategoryNavigator
                    value={selectedCategoryId}
                    valuePath={categoryPath}
                    disabled={!canEdit || loadingConfig}
                    refreshToken={categoryNavigatorRefreshToken}
                    focusTarget={categoryFocusTarget}
                    onConfirmLeaf={(category) => {
                      setAppliedRecommendationCategoryID(undefined);
                      return onCategoryChange(
                        category.categoryId,
                        category.path || category.name || category.categoryId,
                      );
                    }}
                  />
                </>
              ) : null}
              {activeStep === 0 && config?.legacyFallback ? (
                <Alert
                  type="warning"
                  showIcon
                  message="正在读取旧版商品级 Ozon 配置"
                  description="保存后会为当前 Ozon 店铺创建独立配置，其他店铺不会被覆盖。"
                />
              ) : null}
              {activeStep === 2 && config?.sourceCategoryKey ? (
                <div className="publishing-center__mapping-evidence">
                  <div className="publishing-center__mapping-evidence-heading">
                    <div>
                      <Typography.Text strong>映射证据确认</Typography.Text>
                      <Typography.Paragraph type="secondary">
                        先完成叶子类目和模板核对，再确认该来源类目在当前店铺中的复用映射。
                      </Typography.Paragraph>
                    </div>
                    <Space wrap>
                      <Button
                        type="primary"
                        size="small"
                        loading={confirmingCategoryMapping}
                        disabled={
                          !canEdit ||
                          !selectedCategoryId ||
                          !attributes.length ||
                          categoryMappingLoading
                        }
                        onClick={confirmCurrentCategoryMapping}
                      >
                        确认当前类目映射
                      </Button>
                    </Space>
                  </div>
                  <Descriptions bordered size="small" column={{ xs: 1, md: 2 }}>
                    <Descriptions.Item label="来源类目">
                      {config.sourceCategoryName || config.sourceCategoryKey}
                    </Descriptions.Item>
                    <Descriptions.Item label="适用范围">
                      当前 Ozon 店铺下相同来源类目的商品
                    </Descriptions.Item>
                    <Descriptions.Item label="Ozon 完整路径">
                      {categoryPath ||
                        selectedCategoryId ||
                        "尚未完成叶子类目选择"}
                    </Descriptions.Item>
                    <Descriptions.Item label="属性模板">
                      {attributes.length
                        ? `${attributes.length} 个属性，其中 ${requiredCategoryAttributeCount} 个必填`
                        : "尚未加载"}
                    </Descriptions.Item>
                    <Descriptions.Item label="模板同步时间">
                      {categoryTemplateSyncedAt
                        ? formatDateTime(categoryTemplateSyncedAt)
                        : "尚无同步证据"}
                    </Descriptions.Item>
                    <Descriptions.Item label="SKU 变体资格">
                      {variantPolicy
                        ? `${variantPolicy.eligibleAttributeCount} 个可选维度；${variantPolicy.variantEligibilityFullyKnown ? "资格完整" : "存在未知资格"}`
                        : "尚未加载"}
                    </Descriptions.Item>
                    <Descriptions.Item label="选择方式">
                      {categoryMapping?.categoryId === selectedCategoryId
                        ? categoryMapping?.selectionMethod ===
                          "recommended_then_manual"
                          ? "系统推荐后人工逐级确认"
                          : "人工逐级选择"
                        : "待确认"}
                    </Descriptions.Item>
                    <Descriptions.Item label="确认理由">
                      {categoryMapping?.categoryId === selectedCategoryId
                        ? categoryMapping?.confirmationReason ||
                          "历史确认未记录理由，需要重新确认以补齐证据"
                        : "待确认"}
                    </Descriptions.Item>
                    <Descriptions.Item label="确认时间">
                      {categoryMapping?.categoryId === selectedCategoryId &&
                      categoryMapping?.confirmedAt
                        ? formatDateTime(categoryMapping?.confirmedAt)
                        : "待确认"}
                    </Descriptions.Item>
                  </Descriptions>
                  <Collapse
                    ghost
                    size="small"
                    items={[
                      {
                        key: "mapping-technical-info",
                        label: "技术信息 / 高级",
                        children: (
                          <Descriptions bordered size="small" column={1}>
                            <Descriptions.Item label="description_category_id">
                              {categoryMapping?.categoryId ===
                              selectedCategoryId
                                ? categoryMapping?.descriptionCategoryId ||
                                  selectedCategoryParts[0] ||
                                  "—"
                                : selectedCategoryParts[0] || "—"}
                            </Descriptions.Item>
                            <Descriptions.Item label="type_id">
                              {categoryMapping?.categoryId ===
                              selectedCategoryId
                                ? categoryMapping?.typeId ||
                                  selectedCategoryParts[1] ||
                                  "—"
                                : selectedCategoryParts[1] || "—"}
                            </Descriptions.Item>
                            <Descriptions.Item label="模板摘要">
                              {categoryMapping?.categoryId ===
                              selectedCategoryId
                                ? categoryMapping?.schemaHash || "未记录"
                                : "待确认"}
                            </Descriptions.Item>
                          </Descriptions>
                        ),
                      },
                    ]}
                  />
                  <Alert
                    type={
                      categoryMappingLoading
                        ? "info"
                        : categoryMapping?.status === "active" &&
                            categoryMapping.categoryId === selectedCategoryId &&
                            Boolean(categoryMapping.confirmationReason)
                          ? "success"
                          : "error"
                    }
                    showIcon
                    message={
                      categoryMappingLoading
                        ? "正在核对来源类目映射"
                        : categoryMapping?.status === "active" &&
                            categoryMapping.categoryId === selectedCategoryId &&
                            Boolean(categoryMapping.confirmationReason)
                          ? "来源类目映射证据完整"
                          : categoryMapping?.status === "active" &&
                              categoryMapping.categoryId === selectedCategoryId
                            ? "历史映射缺少确认理由"
                            : categoryMapping?.status === "active"
                              ? "当前类目与已确认映射冲突"
                              : "来源类目映射尚未确认"
                    }
                    description={
                      categoryMappingError ||
                      (categoryMapping?.status === "active" &&
                      categoryMapping.categoryId !== selectedCategoryId
                        ? `已确认映射：${categoryMapping.categoryPath || categoryMapping.categoryId}`
                        : "映射确认不会自动提交到 Ozon；发布前仍会重新检查模板摘要。")
                    }
                  />
                </div>
              ) : activeStep === 2 ? (
                <Alert
                  type="info"
                  showIcon
                  message="当前商品没有可用的来源类目"
                  description="服务端仍会要求当前叶子类目路径和属性模板已明确保存确认。"
                />
              ) : null}
              {activeStep === 2 ? (
                <Collapse
                  ghost
                  defaultActiveKey={
                    new URLSearchParams(location.search).get("advanced") === "1"
                      ? ["advanced-category"]
                      : undefined
                  }
                  items={[
                    {
                      key: "advanced-category",
                      label: "高级类目维护",
                      children: (
                        <Space
                          direction="vertical"
                          size={12}
                          style={{ width: "100%" }}
                        >
                          <Typography.Text type="secondary">
                            类目全量同步、模板刷新和映射维护不是日常刊登步骤。以下操作只调用
                            Ozon 只读类目接口，不会创建刊登提交。
                          </Typography.Text>
                          <Space wrap>
                            <Button
                              icon={<SyncOutlined />}
                              loading={syncing}
                              disabled={!shopId}
                              onClick={() => void syncCategoryCache()}
                            >
                              同步类目缓存
                            </Button>
                            <Button
                              icon={<SyncOutlined />}
                              loading={syncing}
                              disabled={!shopId || !selectedCategoryId}
                              onClick={() => void syncCurrentTemplate()}
                            >
                              刷新当前类目属性模板
                            </Button>
                            <Link to="/settings/platform-publish?platform=ozon">
                              打开 Ozon 全局刊登预设
                            </Link>
                            <Link
                              to={`/product/publishing-center?step=category&advanced=1${productId ? `&productId=${encodeURIComponent(productId)}` : ""}${shopId ? `&shopId=${encodeURIComponent(shopId)}` : ""}`}
                            >
                              复制可用的类目维护深链
                            </Link>
                          </Space>
                        </Space>
                      ),
                    },
                  ]}
                />
              ) : null}
            </SectionCard>
          ) : null}

          {!productId || !shopId ? (
            <SectionCard>
              <EmptyState
                title="请选择商品和 Ozon 店铺"
                description="选择完成后会读取该商品、平台和店铺独立保存的配置。"
              />
            </SectionCard>
          ) : loadingConfig ? (
            <SectionCard>
              <Spin spinning tip="正在读取当前店铺配置…">
                <div style={{ minHeight: 96 }} />
              </Spin>
            </SectionCard>
          ) : product ? (
            <div
              className={`publishing-center__layout${
                activeStep === 5 ? "" : " publishing-center__layout--single"
              }`}
            >
              <main className="publishing-center__editor">
                {activeStep === 1 ? (
                  <SectionCard
                    title="基础刊登信息"
                    description="标题、描述和币种只作用于当前商品与 Ozon 店铺。"
                  >
                    <Form.Item name="title" label="Ozon 标题">
                      <Input maxLength={500} showCount />
                    </Form.Item>
                    <Form.Item name="description" label="Ozon 描述">
                      <Input.TextArea rows={6} maxLength={6000} showCount />
                    </Form.Item>
                    <div className="publishing-center__field-with-source">
                      <Form.Item name="currencyCode" label="币种">
                        <Select
                          allowClear
                          placeholder="留空则预检读取 Ozon 店铺合同币种"
                          options={(selectedShopCurrency
                            ? [selectedShopCurrency]
                            : ["RUB", "CNY", "USD", "EUR"]
                          ).map((value) => ({
                            value,
                            label: selectedShopCurrency
                              ? `${value}（Ozon 店铺合同币种）`
                              : value,
                          }))}
                        />
                      </Form.Item>
                      {sourceTag(
                        preflight?.resolvedOzon?.currency.source ||
                          resolvedPreview?.currency.source,
                      )}
                    </div>
                  </SectionCard>
                ) : null}

                {activeStep === 3 ? (
                  <SectionCard
                    title="SKU 售价与本地库存"
                    description="Ozon 售价可按 SKU 独立覆盖；库存始终读取本地 SKU，调整继续使用现有审计链路。"
                  >
                    {product.skus.some((sku) => sku.stock === 0) ? (
                      <Alert
                        type="warning"
                        showIcon
                        message="存在 0 库存 SKU：允许创建，但暂不可售"
                        description="系统会让 Ozon 商品保持零库存；补货前商品不会形成可售库存。未知库存和负库存仍会阻止提交。"
                      />
                    ) : null}
                    <div className="publishing-center__sku-grid">
                      {product.skus.map((sku) => (
                        <section
                          key={sku.id}
                          className="publishing-center__sku-card"
                          id={`field-skuStock-${sku.id}`}
                        >
                          <div className="publishing-center__sku-heading">
                            <div>
                              <Typography.Text strong>
                                {sku.skuName || sku.skuCode || sku.id}
                              </Typography.Text>
                              {sku.skuCode ? (
                                <Typography.Text type="secondary">
                                  SKU：{sku.skuCode}
                                </Typography.Text>
                              ) : null}
                            </div>
                            <Tag>
                              {sku.attrs
                                ? Object.values(sku.attrs)
                                    .map(String)
                                    .join(" / ") || "默认规格"
                                : "默认规格"}
                            </Tag>
                          </div>
                          <div className="publishing-center__sku-values">
                            <Form.Item
                              name={["skuPrices", sku.id]}
                              label="Ozon 售价"
                            >
                              <InputNumber
                                min={0.01}
                                precision={2}
                                step={1}
                                stringMode={false}
                              />
                            </Form.Item>
                            <div className="publishing-center__stock-value">
                              <Typography.Text strong>本地库存</Typography.Text>
                              <Typography.Title level={4}>
                                {sku.stock ?? 0}
                              </Typography.Title>
                              <Typography.Text type="secondary">
                                唯一来源：本地库存
                              </Typography.Text>
                            </div>
                          </div>
                          <Space wrap>
                            <Typography.Text type="secondary">
                              本地销售价：{sku.price ?? "未设置"}{" "}
                              {product.currency || ""}
                            </Typography.Text>
                            <Link
                              to={`/product/drafts/${product.id}?tab=inventory&skuId=${encodeURIComponent(sku.id)}#inventory`}
                            >
                              调整库存与查看审计
                            </Link>
                          </Space>
                        </section>
                      ))}
                    </div>
                  </SectionCard>
                ) : null}

                {activeStep === 1 ? (
                  <SectionCard
                    title="SKU 图片与最终顺序"
                    description="SKU 原始主图固定第一；人工公共图随后，URL 去重且顺序稳定。"
                    className="publishing-center__images-card"
                  >
                    <div id="ozon-sku-images">
                      <OzonSKUImageConfigurator
                        config={currentImageView}
                        skus={skuImages}
                        bulkImageIds={bulkImageIds}
                        disabled={!canEdit}
                        onBulkImageIdsChange={setBulkImageIds}
                        onApplyBulk={applyBulkImages}
                        onUpdateSKU={updateSKUImage}
                        compactBulkSelection
                      />
                    </div>
                  </SectionCard>
                ) : null}

                {activeStep === 4 ? (
                  <SectionCard
                    title="包裹与仓库配置"
                    description="商品店铺级配置优先；留空时读取全局 Ozon 刊登预设，仍缺失会阻止提交。"
                  >
                    <div className="publishing-center__package-grid">
                      {(
                        [
                          ["weightG", "重量（g）"],
                          ["widthMm", "宽度（mm）"],
                          ["heightMm", "高度（mm）"],
                          ["depthMm", "深度（mm）"],
                        ] as const
                      ).map(([key, label]) => (
                        <div
                          className="publishing-center__field-with-source"
                          key={key}
                        >
                          <Form.Item
                            name={["package", key]}
                            label={label}
                            extra={
                              resolvedPreview?.package[key].value
                                ? `留空使用有效值：${resolvedPreview.package[key].value}`
                                : "当前没有可用预设"
                            }
                          >
                            <InputNumber
                              min={1}
                              precision={0}
                              placeholder={
                                resolvedPreview?.package[key].value
                                  ? String(resolvedPreview.package[key].value)
                                  : undefined
                              }
                            />
                          </Form.Item>
                          {sourceTag(resolvedPreview?.package[key].source)}
                        </div>
                      ))}
                      <div className="publishing-center__field-with-source">
                        <Form.Item
                          name={["package", "warehouseId"]}
                          label="Ozon 仓库"
                          extra={
                            warehouseError ||
                            (resolvedPreview?.package.warehouseId.value
                              ? `当前预设仓库：${resolvedPreview.package.warehouseId.value}；仍须在店铺仓库列表中有效`
                              : "只显示当前已授权 Ozon 店铺返回的仓库")
                          }
                        >
                          <Select
                            showSearch
                            allowClear
                            loading={warehouseLoading}
                            optionFilterProp="label"
                            placeholder="选择当前店铺的 Ozon 仓库"
                            options={warehouseOptions.map((warehouse) => ({
                              value: warehouse.id,
                              label: `${warehouse.name}（${warehouse.id}）${warehouse.isRfbs ? " · rFBS" : " · FBS"}`,
                            }))}
                          />
                        </Form.Item>
                        {sourceTag(resolvedPreview?.package.warehouseId.source)}
                      </div>
                      <div className="publishing-center__field-with-source">
                        <Form.Item
                          name={["package", "vat"]}
                          label="增值税率"
                          extra={
                            resolvedPreview?.package.vat.value
                              ? `留空使用有效值：${resolvedPreview.package.vat.value}`
                              : undefined
                          }
                        >
                          <Select
                            allowClear
                            placeholder={
                              resolvedPreview?.package.vat.value
                                ? ozonVATLabel(
                                    resolvedPreview.package.vat.value,
                                  )
                                : "选择适用税率"
                            }
                            options={ozonVATOptions}
                          />
                        </Form.Item>
                        {sourceTag(resolvedPreview?.package.vat.source)}
                      </div>
                    </div>
                  </SectionCard>
                ) : null}

                {activeStep === 2 ? (
                  <SectionCard
                    title="Ozon 类目属性"
                    description="多值属性使用多选；复杂属性按字段组保存，只有 Ozon 明确允许时才能重复添加。"
                    className="publishing-center__attributes-card"
                  >
                    <AIAttributeFillControls
                      context={aiAttributeContext}
                      readOnly={!canEdit}
                      disabled={aiAttributeControlsDisabled}
                      disabledReason={aiAttributeDisabledReason}
                      canUndo={Boolean(
                        aiAttributeUndo &&
                        aiAttributeUndo.contextKey ===
                          aiAttributeContextKey(aiAttributeContext),
                      )}
                      getCurrentValues={currentAIAttributeValues}
                      onApplyResult={applyAIAttributeResult}
                      onUndo={undoAIAttributeResult}
                    />
                    {templateRefreshFeedback &&
                    templateRefreshFeedback.categoryId ===
                      selectedCategoryId ? (
                      <Alert
                        type="success"
                        showIcon
                        message="最新类目属性模板已重新加载，页面已生效"
                        description={`当前展示 ${templateRefreshFeedback.attributeCount} 个属性，其中 ${templateRefreshFeedback.requiredCount} 个必填${templateRefreshFeedback.syncedAt ? `；同步时间 ${formatDateTime(templateRefreshFeedback.syncedAt)}` : ""}。请核对字段后单独保存当前商品配置；本次刷新没有提交 Ozon。`}
                      />
                    ) : null}
                    {!selectedCategoryId ? (
                      <Alert
                        type="info"
                        showIcon
                        message="选择类目后显示属性"
                      />
                    ) : attributeTemplateError && attributes.length === 0 ? (
                      <Alert
                        type="error"
                        showIcon
                        message="当前类目属性模板加载失败"
                        description={attributeTemplateError}
                      />
                    ) : attributes.length === 0 ? (
                      <Alert
                        type="warning"
                        showIcon
                        message="当前类目属性模板为空"
                        description="请在上方“高级类目维护”中刷新模板；未获得模板前不能提交。"
                      />
                    ) : (
                      <Space
                        direction="vertical"
                        size={20}
                        style={{ width: "100%" }}
                      >
                        {attributeTemplateError ? (
                          <Alert
                            type="error"
                            showIcon
                            message="类目属性模板刷新失败"
                            description={attributeTemplateError}
                            closable
                            onClose={() => setAttributeTemplateError(undefined)}
                          />
                        ) : null}
                        {templateCacheStale ? (
                          <Alert
                            type="warning"
                            showIcon
                            message="当前类目属性模板可能已过期"
                            description="页面保留现有字段和值；请在高级类目维护中刷新模板，确认成功后再保存。"
                          />
                        ) : null}
                        {unsupportedAttributes.length > 0 ? (
                          <Alert
                            type="warning"
                            showIcon
                            message={`${unsupportedAttributes.length} 个属性类型暂不支持编辑`}
                            description={`${unsupportedAttributes
                              .slice(0, 3)
                              .map(
                                (attribute) =>
                                  `${attribute.name}（${attribute.valueType || "未知类型"}）`,
                              )
                              .join(
                                "、",
                              )}${unsupportedAttributes.length > 3 ? "等" : ""}。这些字段仍参与既有保存前校验，不会被静默忽略。`}
                          />
                        ) : null}
                        <div
                          className="publishing-center__attribute-summary"
                          aria-label="属性填写摘要"
                          aria-live="polite"
                        >
                          <div className="publishing-center__attribute-metric">
                            <span>属性总数</span>
                            <strong>{attributeProgress.total}</strong>
                          </div>
                          <div className="publishing-center__attribute-metric">
                            <span>必填完成</span>
                            <strong>
                              {attributeProgress.requiredCompleted} /{" "}
                              {attributeProgress.requiredTotal}
                            </strong>
                          </div>
                          <div className="publishing-center__attribute-metric">
                            <span>已填写</span>
                            <strong>
                              {attributeProgress.filled} /{" "}
                              {attributeProgress.total}
                            </strong>
                          </div>
                        </div>
                        <div className="publishing-center__attribute-toolbar">
                          <Input.Search
                            allowClear
                            value={attributeSearch}
                            aria-label="搜索 Ozon 属性"
                            placeholder="搜索属性名称或说明"
                            disabled={loadingConfig}
                            onChange={(event) =>
                              setAttributeSearch(event.target.value)
                            }
                          />
                          <div className="publishing-center__attribute-filters">
                            <Segmented
                              aria-label="筛选 Ozon 属性"
                              value={attributeFilter}
                              disabled={loadingConfig}
                              options={[
                                { label: "全部", value: "all" },
                                { label: "必填", value: "required" },
                                { label: "未填写", value: "unfilled" },
                              ]}
                              onChange={(value) =>
                                setAttributeFilter(value as OzonAttributeFilter)
                              }
                            />
                            {!canEdit ? <Tag>只读</Tag> : null}
                          </div>
                        </div>
                        <Collapse
                          className="publishing-center__attribute-rules"
                          size="small"
                          activeKey={formatRulesOpen ? ["format-rules"] : []}
                          onChange={(keys) => {
                            const activeKeys = Array.isArray(keys)
                              ? keys
                              : [keys];
                            setFormatRulesOpen(
                              activeKeys.includes("format-rules"),
                            );
                          }}
                          items={[
                            {
                              key: "format-rules",
                              label: "格式与校验规则",
                              children: (
                                <Typography.Text type="secondary">
                                  输入控件与格式提示来自当前 Ozon
                                  模板的字段类型；整数、数值、是/否、链接和日期会使用对应校验。模板未提供的范围、精度、长度或单位不会由页面臆造，隐藏字段仍参与保存与发布前检查。
                                </Typography.Text>
                              ),
                            },
                          ]}
                        />
                        {visibleRequiredAttributes.length > 0 ? (
                          <section className="publishing-center__attribute-section">
                            <div className="publishing-center__attribute-section-heading">
                              <div>
                                <Typography.Text strong>
                                  必填属性（{visibleRequiredAttributes.length}
                                  {hasActiveAttributeView
                                    ? ` / ${requiredAttributes.length}`
                                    : ""}
                                  项）
                                </Typography.Text>
                                <Typography.Text type="secondary">
                                  由当前叶子类目模板标记为必填
                                </Typography.Text>
                              </div>
                            </div>
                            <div className="publishing-center__attribute-grid">
                              {visibleRequiredAttributes.map(
                                renderOrdinaryAttribute,
                              )}
                            </div>
                          </section>
                        ) : null}
                        {visibleOptionalAttributes.length > 0 ? (
                          <Collapse
                            activeKey={
                              optionalAttributesOpen || hasActiveAttributeView
                                ? ["optional-attributes"]
                                : []
                            }
                            onChange={(keys) => {
                              const activeKeys = Array.isArray(keys)
                                ? keys
                                : [keys];
                              setOptionalAttributesOpen(
                                activeKeys.includes("optional-attributes"),
                              );
                            }}
                            items={[
                              {
                                key: "optional-attributes",
                                label: `可选属性（${visibleOptionalAttributes.length}${
                                  hasActiveAttributeView
                                    ? ` / ${optionalAttributes.length}`
                                    : ""
                                } 项）`,
                                children: (
                                  <div className="publishing-center__attribute-grid">
                                    {visibleOptionalAttributes.map(
                                      renderOrdinaryAttribute,
                                    )}
                                  </div>
                                ),
                              },
                            ]}
                          />
                        ) : null}
                        {visibleLowFrequencyAttributes.length > 0 ? (
                          <Collapse
                            activeKey={
                              lowFrequencyAttributesOpen ||
                              lowFrequencyAttributeIssueCount > 0 ||
                              hasActiveAttributeView
                                ? ["low-frequency-attributes"]
                                : []
                            }
                            onChange={(keys) => {
                              const activeKeys = Array.isArray(keys)
                                ? keys
                                : [keys];
                              setLowFrequencyAttributesOpen(
                                activeKeys.includes("low-frequency-attributes"),
                              );
                            }}
                            items={[
                              {
                                key: "low-frequency-attributes",
                                label: (
                                  <Space wrap>
                                    <span>
                                      低频与技术属性（
                                      {visibleLowFrequencyAttributes.length}
                                      {hasActiveAttributeView
                                        ? ` / ${lowFrequencyAttributes.length}`
                                        : ""}{" "}
                                      项）
                                    </span>
                                    {lowFrequencyAttributeIssueCount > 0 ? (
                                      <Tag color="red">
                                        {lowFrequencyAttributeIssueCount}{" "}
                                        项待处理
                                      </Tag>
                                    ) : (
                                      <Tag>默认折叠</Tag>
                                    )}
                                  </Space>
                                ),
                                children: (
                                  <Space
                                    direction="vertical"
                                    size={12}
                                    style={{ width: "100%" }}
                                  >
                                    <Typography.Text type="secondary">
                                      JSON
                                      富内容、PDF、调试或诊断字段默认收起，仅在确有素材或平台要求时填写；展开和编辑不会绕过既有格式校验。
                                    </Typography.Text>
                                    <div className="publishing-center__attribute-grid">
                                      {visibleLowFrequencyAttributes.map(
                                        renderOrdinaryAttribute,
                                      )}
                                    </div>
                                  </Space>
                                ),
                              },
                            ]}
                          />
                        ) : null}
                        {visibleComplexGroups.map(([complexId, defs]) => {
                          const repeatable = defs.some(
                            (attribute) => attribute.complexIsCollection,
                          );
                          return (
                            <section
                              className="publishing-center__complex"
                              key={complexId}
                            >
                              <div className="publishing-center__complex-heading">
                                <div>
                                  <Typography.Text strong>
                                    组合属性组 {complexId}
                                  </Typography.Text>
                                  <Typography.Paragraph type="secondary">
                                    每一组会作为一个 complex_attributes
                                    项提交；字段不会被拆成普通单值。
                                  </Typography.Paragraph>
                                </div>
                                <Tag color={repeatable ? "blue" : undefined}>
                                  {repeatable ? "允许重复字段组" : "仅允许一组"}
                                </Tag>
                              </div>
                              <Form.List
                                name={["complexGroups", String(complexId)]}
                              >
                                {(fields, { add, remove }) => (
                                  <Space
                                    direction="vertical"
                                    size={12}
                                    style={{ width: "100%" }}
                                  >
                                    {fields.map((field, index) => (
                                      <div
                                        className="publishing-center__complex-group"
                                        key={field.key}
                                      >
                                        <div className="publishing-center__complex-group-heading">
                                          <Typography.Text strong>
                                            第 {index + 1} 组
                                          </Typography.Text>
                                          {repeatable || fields.length > 1 ? (
                                            <Button
                                              type="link"
                                              danger
                                              onClick={() => {
                                                remove(field.name);
                                                markDirty();
                                              }}
                                            >
                                              移除
                                            </Button>
                                          ) : null}
                                        </div>
                                        <div className="publishing-center__attribute-grid">
                                          {defs.map((attribute) => (
                                            <Form.Item
                                              key={attribute.attrId}
                                              name={[
                                                field.name,
                                                attribute.attrId,
                                              ]}
                                              label={renderAttributeLabel(
                                                attribute,
                                              )}
                                              required={attribute.required}
                                              preserve
                                              extra={renderAttributeHelp(
                                                attribute,
                                                `complex-${complexId}-${field.key}-${attribute.attrId}`,
                                              )}
                                            >
                                              {renderAttributeInput(attribute)}
                                            </Form.Item>
                                          ))}
                                        </div>
                                      </div>
                                    ))}
                                    {repeatable || fields.length === 0 ? (
                                      <Button
                                        icon={<PlusOutlined />}
                                        onClick={() => {
                                          add({});
                                          markDirty();
                                        }}
                                      >
                                        添加一组组合属性
                                      </Button>
                                    ) : null}
                                  </Space>
                                )}
                              </Form.List>
                            </section>
                          );
                        })}
                        {!hasVisibleAttributes ? (
                          <EmptyState
                            title="没有匹配的 Ozon 属性"
                            description="调整搜索词或筛选条件；已填写的隐藏字段和值仍会保留并参与保存校验。"
                          />
                        ) : null}
                      </Space>
                    )}
                  </SectionCard>
                ) : null}

                {activeStep === 3 ? (
                  <SectionCard
                    title="SKU 变体属性映射"
                    description="只允许选择 Ozon 实时模板以 is_aspect 明确标记的变体属性，再逐个 SKU 选择平台值；未确认资格的属性默认禁用。"
                  >
                    <Space
                      id="ozon-sku-variant-mapping"
                      direction="vertical"
                      size={16}
                      style={{ width: "100%" }}
                    >
                      {product.skus.length <= 1 ? (
                        <Alert
                          type="info"
                          showIcon
                          message="当前是单 SKU 商品，不强制配置变体属性"
                          description="如后续增加 SKU，发布前检查会要求每个 SKU 都有唯一的变体组合。"
                        />
                      ) : null}
                      {complexGroups.length > 0 ? (
                        <Alert
                          type="warning"
                          showIcon
                          message="组合属性不能作为 SKU 变体维度"
                          description="attribute_complex_id 大于 0 的属性仍按上方商品级字段组提交。若当前类目只能依靠组合属性区分 SKU，系统会阻止提交，请拆分为单品或暂停刊登。"
                        />
                      ) : null}
                      {unknownVariantEligibilityCount > 0 ? (
                        <Alert
                          type="warning"
                          showIcon
                          message={`${unknownVariantEligibilityCount} 个普通属性缺少 is_aspect 资格证据，已禁止作为 SKU 维度`}
                          description="请同步当前叶子类目的属性模板；缺少证据不会按“允许”处理。"
                          action={
                            <Button
                              size="small"
                              icon={<SyncOutlined />}
                              loading={syncing}
                              disabled={!shopId || !selectedCategoryId}
                              onClick={() => void syncCurrentTemplate()}
                            >
                              刷新当前类目模板
                            </Button>
                          }
                        />
                      ) : null}
                      {selectedCategoryId &&
                      selectableVariantAttributes.length === 0 ? (
                        <Alert
                          type="error"
                          showIcon
                          message="当前类目没有可选择的 SKU 变体属性"
                          description={`当前类目：${categoryPath || selectedCategoryId}。下拉框会保留全部模板属性并逐项显示禁用原因；刷新后仍无可选项时，请更换正确类目或拆分为多个独立 Ozon 商品。`}
                          action={
                            <Space wrap>
                              <Button
                                size="small"
                                onClick={() => setActiveStep(2)}
                              >
                                返回类目与属性
                              </Button>
                              <Button
                                size="small"
                                icon={<SyncOutlined />}
                                loading={syncing}
                                disabled={!shopId}
                                onClick={() => void syncCurrentTemplate()}
                              >
                                刷新当前类目模板
                              </Button>
                            </Space>
                          }
                        />
                      ) : null}
                      {variantPolicy ? (
                        <Space wrap size={8}>
                          <Tag color="blue">
                            单次 SKU 上限 {variantPolicy.maxSkuCount}
                          </Tag>
                          <Tag color="geekblue">
                            当前可选变体维度{" "}
                            {selectableVariantAttributes.length}
                          </Tag>
                          <Tag color="purple">
                            组合上限 {variantPolicy.maxVariantCombinationCount}
                          </Tag>
                          <Typography.Text type="secondary">
                            Ozon is_aspect 符合{" "}
                            {variantPolicy.eligibleAttributeCount}
                            个；系统仅开放支持可靠校验的 valueType。
                          </Typography.Text>
                        </Space>
                      ) : null}
                      <div className="publishing-center__variant-toolbar">
                        <Form.Item
                          name="skuVariantAttributeIds"
                          label="用于区分 SKU 的 Ozon 属性"
                          extra="全部模板属性都可见；不符合 is_aspect、组合属性或暂不支持的 valueType 会禁用并直接显示原因。"
                        >
                          <Select
                            mode="multiple"
                            allowClear
                            maxCount={
                              variantPolicy?.maxVariantAttributeCount ||
                              undefined
                            }
                            optionFilterProp="label"
                            placeholder="选择 Ozon 明确允许的颜色、尺码等属性"
                            notFoundContent="当前模板没有属性"
                            options={variantAttributeOptions}
                            onChange={onVariantAttributesChange}
                          />
                        </Form.Item>
                        <Button
                          icon={<ThunderboltOutlined />}
                          disabled={
                            validSelectedVariantAttributeIDs.length === 0
                          }
                          onClick={autoMatchSKUAttributes}
                        >
                          从本地 SKU 属性自动匹配
                        </Button>
                      </div>
                      {invalidSelectedVariantAttributes.length > 0 ? (
                        <Alert
                          type="error"
                          showIcon
                          message={`${invalidSelectedVariantAttributes.length} 个已选变体属性现在不可用`}
                          description={invalidSelectedVariantAttributes
                            .map(({ attributeID, state }) =>
                              state
                                ? `${state.attribute.name}：${state.disabledReason}`
                                : `${attributeID}：当前模板已不存在`,
                            )
                            .join("；")}
                        />
                      ) : null}
                      {product.skus.length > 1 &&
                      validSelectedVariantAttributeIDs.length === 0 ? (
                        <Alert
                          type="error"
                          showIcon
                          message="当前配置可以保存，但不能提交 Ozon"
                          description="多 SKU 必须明确选择变体维度并为每个 SKU 分配唯一值；系统不会把商品级同一属性复制给所有 SKU。"
                        />
                      ) : null}
                      <Collapse
                        activeKey={
                          skuVariantDetailsOpen ? ["sku-variant-details"] : []
                        }
                        onChange={(keys) => {
                          const activeKeys = Array.isArray(keys)
                            ? keys
                            : [keys];
                          setSKUVariantDetailsOpen(
                            activeKeys.includes("sku-variant-details"),
                          );
                        }}
                        items={[
                          {
                            key: "sku-variant-details",
                            label: (
                              <Space wrap>
                                <span>
                                  逐 SKU 变体明细（{product.skus.length} 个）
                                </span>
                                {skuVariantIssueCount > 0 ? (
                                  <Tag color="red">
                                    {skuVariantIssueCount} 项待处理
                                  </Tag>
                                ) : (
                                  <Tag>按需展开</Tag>
                                )}
                              </Space>
                            ),
                            children: (
                              <div className="publishing-center__variant-grid">
                                {product.skus.map((sku) => (
                                  <section
                                    key={sku.id}
                                    id={`field-skuAttributeOverrides-${sku.id}`}
                                    className="publishing-center__variant-card"
                                  >
                                    <div className="publishing-center__sku-heading">
                                      <div>
                                        <Typography.Text strong>
                                          {sku.skuName || sku.skuCode || sku.id}
                                        </Typography.Text>
                                        {sku.skuCode ? (
                                          <Typography.Text type="secondary">
                                            SKU：{sku.skuCode}
                                          </Typography.Text>
                                        ) : null}
                                      </div>
                                      <Tag color="blue">逐 SKU 配置</Tag>
                                    </div>
                                    <div className="publishing-center__local-attrs">
                                      <Typography.Text type="secondary">
                                        本地属性候选
                                      </Typography.Text>
                                      <Space size={[4, 4]} wrap>
                                        {Object.entries(sku.attrs || {})
                                          .length > 0 ? (
                                          Object.entries(sku.attrs || {}).map(
                                            ([key, value]) => (
                                              <Tag key={key}>
                                                {key}：
                                                {Array.isArray(value)
                                                  ? value
                                                      .map(String)
                                                      .join(" / ")
                                                  : String(value ?? "")}
                                              </Tag>
                                            ),
                                          )
                                        ) : (
                                          <Typography.Text type="secondary">
                                            无；请手动选择 Ozon 值
                                          </Typography.Text>
                                        )}
                                      </Space>
                                    </div>
                                    {variantAttributes.length === 0 ? (
                                      <Typography.Text type="secondary">
                                        选择上方变体属性后在此分配对应值。
                                      </Typography.Text>
                                    ) : (
                                      <div className="publishing-center__variant-fields">
                                        {variantAttributes.map((attribute) => (
                                          <Form.Item
                                            key={attribute.attrId}
                                            name={[
                                              "skuAttributeOverrides",
                                              sku.id,
                                              attribute.attrId,
                                            ]}
                                            label={renderAttributeLabel(
                                              attribute,
                                            )}
                                            required
                                            preserve
                                            extra={
                                              <Space
                                                direction="vertical"
                                                size={2}
                                                style={{ width: "100%" }}
                                              >
                                                {attribute.dictionaryId ? (
                                                  <Typography.Text type="secondary">
                                                    必须选择 Ozon
                                                    词典值；自动匹配失败时请手动搜索。
                                                  </Typography.Text>
                                                ) : null}
                                                {renderAttributeHelp(
                                                  attribute,
                                                  `variant-${sku.id}-${attribute.attrId}`,
                                                )}
                                              </Space>
                                            }
                                          >
                                            {renderAttributeInput(attribute)}
                                          </Form.Item>
                                        ))}
                                      </div>
                                    )}
                                  </section>
                                ))}
                              </div>
                            ),
                          },
                        ]}
                      />
                    </Space>
                  </SectionCard>
                ) : null}

                {activeStep === 5 ? (
                  <div className="publishing-center__actions">
                    <Alert
                      type={dirty ? "warning" : "info"}
                      showIcon
                      message={
                        dirty
                          ? "有未保存修改：发布前检查不会自动保存"
                          : "保存、只读检查、真实提交是三个独立动作"
                      }
                      description={
                        dirty
                          ? "当前试探类目、误选属性和临时编辑不会被检查或写入配置。请先返回核对；确认无误后，单独保存，再运行只读检查。"
                          : "保存只写 TradeMind 配置；发布前检查只读取已保存配置且不调用 Ozon 写接口；检查通过后仍需进入真实提交确认。"
                      }
                    />
                    <Space wrap>
                      <Button
                        icon={<SaveOutlined />}
                        loading={saving}
                        disabled={!canEdit || !productId || !shopId}
                        onClick={() =>
                          void saveCurrent().catch((error: unknown) =>
                            message.error(errorMessage(error, "保存失败")),
                          )
                        }
                      >
                        保存当前编辑（不提交）
                      </Button>
                      <Button
                        type="primary"
                        icon={<CheckCircleOutlined />}
                        loading={checking}
                        disabled={!canEdit || !selectedCategoryId}
                        onClick={() => void runPreflight()}
                      >
                        运行发布前检查
                      </Button>
                      <Button
                        danger
                        type="primary"
                        loading={submitting}
                        disabled={!canPublish || !submitGate.ready}
                        onClick={confirmSubmit}
                      >
                        进入提交确认
                      </Button>
                    </Space>
                    <Typography.Text type="secondary">
                      {dirty
                        ? "当前页面未保存，真实提交保持锁定。"
                        : "当前编辑已保存；任一字段变化后都必须重新保存并重新运行检查。"}
                    </Typography.Text>
                    {!submitGate.ready ? (
                      <Typography.Text type="danger">
                        真实提交已锁定：{submitGate.reasons.join("；")}
                      </Typography.Text>
                    ) : null}
                  </div>
                ) : null}

                <div className="publishing-center__wizard-actions">
                  <Button
                    disabled={activeStep === 0}
                    onClick={() =>
                      setActiveStep((activeStep - 1) as PublishingStep)
                    }
                  >
                    上一步
                  </Button>
                  <Space wrap>
                    {activeStep < 5 ? (
                      <Button
                        icon={<SaveOutlined />}
                        loading={saving}
                        disabled={!canEdit || !productId || !shopId}
                        onClick={() =>
                          void saveCurrent().catch((error: unknown) =>
                            message.error(errorMessage(error, "保存失败")),
                          )
                        }
                      >
                        保存当前编辑（不提交）
                      </Button>
                    ) : null}
                    {activeStep < 5 ? (
                      <Button
                        type="primary"
                        onClick={() =>
                          setActiveStep((activeStep + 1) as PublishingStep)
                        }
                      >
                        下一步：{publishingStepItems[activeStep + 1].title}
                      </Button>
                    ) : null}
                  </Space>
                </div>
              </main>

              {activeStep === 5 ? (
                <aside
                  className="publishing-center__check-panel"
                  aria-label="即时检查与最终提交预览"
                >
                  <SectionCard
                    title="即时检查"
                    description="这里只做基础字段提示；正式检查以服务端只读预检为准。"
                    compact
                  >
                    {immediateIssues.length === 0 ? (
                      <Alert
                        type="success"
                        showIcon
                        message="基础字段暂未发现错误"
                      />
                    ) : (
                      <Space
                        direction="vertical"
                        size={8}
                        style={{ width: "100%" }}
                      >
                        {immediateIssues.map((issue) => (
                          <Alert
                            key={issue.key}
                            type="error"
                            showIcon
                            message={issue.message}
                            description={issue.suggestion}
                            action={
                              issue.field ? (
                                <Button
                                  size="small"
                                  onClick={() => goToIssue(issue)}
                                >
                                  去修改
                                </Button>
                              ) : undefined
                            }
                          />
                        ))}
                      </Space>
                    )}
                  </SectionCard>

                  {preflight ? (
                    <SectionCard title="服务端发布前检查" compact>
                      <Alert
                        type={preflight.canPublish ? "success" : "error"}
                        showIcon
                        message={
                          preflight.canPublish
                            ? "只读检查通过"
                            : `检查未通过（${preflight.errorCount ?? preflight.checks?.length ?? 0} 项错误）`
                        }
                        description={
                          preflight.canPublish
                            ? "下方值与真实任务快照共用后端解析器；仍需二次确认才会提交。"
                            : "修正后重新检查；当前不会调用 Ozon 写接口。"
                        }
                      />
                      {(preflight.checks || []).map((check, index) => (
                        <Alert
                          key={`${check.code || "check"}-${index}`}
                          type={check.level === "warning" ? "warning" : "error"}
                          showIcon
                          message={check.message || check.title || check.code}
                          description={check.suggestion}
                        />
                      ))}
                    </SectionCard>
                  ) : null}

                  <SectionCard
                    title="最终提交预览"
                    description={
                      finalPreview
                        ? "来自服务端只读预检"
                        : "当前为即时估算，正式提交以预检返回值为准"
                    }
                    compact
                  >
                    <Descriptions size="small" column={1} bordered>
                      <Descriptions.Item label="标题">
                        {finalPreview?.title.value || watched.title || "—"}{" "}
                        {sourceTag(
                          finalPreview?.title.source ||
                            resolvedPreview?.title.source,
                        )}
                      </Descriptions.Item>
                      <Descriptions.Item label="币种">
                        {finalPreview?.currency.value ||
                          watched.currencyCode ||
                          "—"}{" "}
                        {sourceTag(
                          finalPreview?.currency.source ||
                            resolvedPreview?.currency.source,
                        )}
                      </Descriptions.Item>
                      <Descriptions.Item label="类目">
                        {finalPreview?.categoryPath ||
                          categoryPath ||
                          selectedCategoryId ||
                          "—"}
                      </Descriptions.Item>
                    </Descriptions>
                    <Divider />
                    <Space wrap size={[8, 8]}>
                      <Tag color="blue">SKU：{previewSKUs.length} 个</Tag>
                      <Tag
                        color={previewProblemSKUs.length > 0 ? "red" : "green"}
                      >
                        {previewProblemSKUs.length > 0
                          ? `待修正 ${previewProblemSKUs.length} 个`
                          : "全部可提交"}
                      </Tag>
                      <Typography.Text>
                        库存：{previewStockTotal}（本地库存）
                      </Typography.Text>
                      <Typography.Text>
                        图片：{previewImageTotal} 张
                      </Typography.Text>
                    </Space>
                    {previewProblemSKUs.length > 0 ? (
                      <Alert
                        type="error"
                        showIcon
                        message="存在待修正 SKU"
                        description={previewProblemSKUs
                          .map((sku) => sku.skuName || sku.skuCode || sku.skuId)
                          .join("、")}
                      />
                    ) : null}
                    <Collapse
                      items={[
                        {
                          key: "sku-preview-details",
                          label: `查看全部规格明细（${previewSKUs.length} 个）`,
                          children: (
                            <Space
                              direction="vertical"
                              size={12}
                              style={{ width: "100%" }}
                            >
                              {previewSKUs.map((sku) => {
                                const local = localSKUByID.get(sku.skuId) as
                                  | ProductSKURow
                                  | undefined;
                                return (
                                  <div
                                    className="publishing-center__preview-sku"
                                    key={sku.skuId}
                                  >
                                    <div className="publishing-center__preview-sku-heading">
                                      <Typography.Text strong>
                                        {sku.skuName ||
                                          sku.skuCode ||
                                          sku.skuId}
                                      </Typography.Text>
                                      <Tag
                                        color={sku.canSubmit ? "green" : "red"}
                                      >
                                        {sku.canSubmit ? "可提交" : "待修正"}
                                      </Tag>
                                    </div>
                                    <Typography.Text>
                                      售价：{sku.price.value || "—"}{" "}
                                      {finalPreview?.currency.value ||
                                        watched.currencyCode ||
                                        ""}{" "}
                                      {sourceTag(sku.price.source)}
                                    </Typography.Text>
                                    <Typography.Text>
                                      库存：
                                      {sku.localStock ?? local?.stock ?? 0}{" "}
                                      {sourceTag(sku.stockSource)}
                                    </Typography.Text>
                                    <Typography.Text>
                                      图片：{sku.images.length} 张（顺序{" "}
                                      {sku.images
                                        .map((image) => image.position)
                                        .join(" → ") || "—"}
                                      ）
                                    </Typography.Text>
                                    {variantAttributes.length > 0 ? (
                                      <div className="publishing-center__preview-attributes">
                                        {variantAttributes.map((attribute) => {
                                          const values =
                                            sku.platformAttributes
                                              ?.attributes?.[
                                              attribute.attrId
                                            ] || [];
                                          return (
                                            <Typography.Text
                                              key={attribute.attrId}
                                            >
                                              {attribute.name}：
                                              {values
                                                .map(
                                                  (selection) =>
                                                    selection.value,
                                                )
                                                .filter(Boolean)
                                                .join(" / ") || "—"}{" "}
                                              {sourceTag(
                                                sku.attributeSources?.[
                                                  attribute.attrId
                                                ],
                                              )}
                                            </Typography.Text>
                                          );
                                        })}
                                      </div>
                                    ) : null}
                                    {sku.issues.map((issue, index) => (
                                      <Typography.Text
                                        type="danger"
                                        key={`${issue.code}-${index}`}
                                      >
                                        {issue.message}
                                      </Typography.Text>
                                    ))}
                                  </div>
                                );
                              })}
                            </Space>
                          ),
                        },
                      ]}
                    />
                  </SectionCard>
                </aside>
              ) : null}
            </div>
          ) : null}
        </Form>
      </TmPageContainer>
    </PermissionGuard>
  );
}

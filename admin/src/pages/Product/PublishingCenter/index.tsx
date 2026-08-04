import {
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  PlusOutlined,
  SaveOutlined,
  SyncOutlined,
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
  Select,
  Space,
  Spin,
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
} from "@/services/ozonCategories";
import {
  buildOzonPlatformAttributesV2,
  buildOzonSKUImagePreview,
  getOzonProductConfig,
  normalizeOzonAttributeEditorValues,
  publishOzonProduct,
  saveOzonProductConfig,
  searchOzonLeafCategories,
  syncOzonCategoryFlow,
  toOzonAttributeEditorValues,
  toOzonImageConfigInput,
  validateOzonReadiness,
  type OzonAttributeEditorValues,
  type OzonImageConfigView,
  type OzonProductConfig,
  type OzonReadinessResult,
  type OzonResolvedListing,
  type OzonSKUImageConfig,
  type OzonValueSource,
} from "@/services/ozonPublish";
import {
  fetchProductDetail,
  fetchProducts,
  type ProductDetail,
  type ProductListRow,
  type ProductSKURow,
} from "@/services/products";
import { queryShops, type ShopListRow } from "@/services/shops";
import { PERMISSIONS } from "@/utils/permission";
import OzonSKUImageConfigurator, {
  type OzonSKUImageSelectionPatch,
} from "../OzonPublish/OzonSKUImageConfigurator";
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

type CategoryOption = { value: string; label: string };

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
    raw.version === 2 &&
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
  const [products, setProducts] = useState<ProductListRow[]>([]);
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [productId, setProductId] = useState<string | undefined>(
    initialProductId,
  );
  const [shopId, setShopId] = useState<string | undefined>(initialShopId);
  const [product, setProduct] = useState<ProductDetail>();
  const [config, setConfig] = useState<OzonProductConfig>();
  const [attributes, setAttributes] = useState<OzonCategoryAttribute[]>([]);
  const [categoryOptions, setCategoryOptions] = useState<CategoryOption[]>([]);
  const [categoryPath, setCategoryPath] = useState("");
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
  const [loadError, setLoadError] = useState<string>();
  const [searchingAttribute, setSearchingAttribute] = useState<string>();
  const [searchingProducts, setSearchingProducts] = useState(false);
  const productSearchSequence = useRef(0);
  const dictionarySearchSequences = useRef<Record<string, number>>({});
  const watched = (Form.useWatch([], form) || {}) as PublishingFormValues;

  const canEdit =
    !readonly &&
    can(PERMISSIONS.PRODUCT_WRITE) &&
    can(PERMISSIONS.STORE_OPERATE);
  const canPublish = canEdit && can(PERMISSIONS.PUBLISH_CREATE_DRAFT);
  const selectedCategoryId = watched.categoryId;

  const loadCategoryOptions = useCallback(async (keyword?: string) => {
    const result = await searchOzonLeafCategories(keyword);
    const options = (result.list || []).map((item) => ({
      value: item.categoryId,
      label: item.name || item.categoryId,
    }));
    setCategoryOptions((current) => {
      const merged = new Map(current.map((item) => [item.value, item]));
      options.forEach((item) => merged.set(item.value, item));
      return Array.from(merged.values());
    });
    return options;
  }, []);

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
          loadCategoryOptions(),
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
  }, [loadCategoryOptions]);

  useEffect(() => {
    if (!productId) {
      setProduct(undefined);
      setConfig(undefined);
      setAttributes([]);
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
      if (categoryId) {
        const result = await queryOzonCategoryAttributes(categoryId);
        nextAttributes = addSavedDictionaryOptions(
          result.list || [],
          next.platformAttributes,
        );
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
      });
      setAttributes(nextAttributes);
      setCategoryPath(next.categoryPath || categoryId || "");
      if (categoryId && next.categoryPath) {
        setCategoryOptions((current) =>
          current.some((item) => item.value === categoryId)
            ? current
            : [
                ...current,
                { value: categoryId, label: next.categoryPath || categoryId },
              ],
        );
      }
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
    },
    [form],
  );

  useEffect(() => {
    if (!product || !shopId) {
      setConfig(undefined);
      setAttributes([]);
      setSKUImages([]);
      setPreflight(undefined);
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
  }, [hydrateConfig, product, shopId]);

  const markDirty = () => {
    setDirty(true);
    setPreflight(undefined);
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
    setDirty(false);
    setProduct(undefined);
    setConfig(undefined);
    setAttributes([]);
    setSKUImages([]);
    form.resetFields();
    setProductId(value);
    setPreflight(undefined);
    history.replace(
      `/product/publishing-center?productId=${encodeURIComponent(value)}${shopId ? `&shopId=${encodeURIComponent(shopId)}` : ""}`,
    );
  };

  const selectShop = async (value: string) => {
    if (value === shopId || !(await confirmContextChange())) return;
    setDirty(false);
    setConfig(undefined);
    setAttributes([]);
    setSKUImages([]);
    form.resetFields();
    setShopId(value);
    setPreflight(undefined);
    if (productId)
      history.replace(
        `/product/publishing-center?productId=${encodeURIComponent(productId)}&shopId=${encodeURIComponent(value)}`,
      );
  };

  const onCategoryChange = async (categoryId?: string) => {
    form.setFieldValue("categoryId", categoryId);
    setCategoryPath(
      categoryOptions.find((item) => item.value === categoryId)?.label || "",
    );
    setAttributes([]);
    form.setFieldsValue({ attributes: {}, complexGroups: {} });
    markDirty();
    if (!categoryId) return;
    try {
      const result = await queryOzonCategoryAttributes(categoryId);
      const nextAttributes = result.list || [];
      setAttributes(nextAttributes);
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
      message.error(errorMessage(error, "类目属性模板加载失败"));
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
  const ordinaryAttributes = attributes.filter(
    (attribute) => !attribute.attributeComplexId,
  );
  const complexGroups = complexAttributeGroups(attributes);

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
    if (!isFilled(watched.title))
      add("title", "Ozon 标题未填写", "填写标题或补齐商品标题。", "title");
    if (!isFilled(watched.description))
      add("description", "Ozon 描述未填写", "填写商品描述。", "description");
    if (
      !isFilled(watched.currencyCode || preflight?.resolvedOzon?.currency.value)
    )
      add(
        "currency",
        "Ozon 币种无法确定",
        "选择币种或检查 Ozon 刊登预设。",
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
      if ((sku.stock ?? 0) < 0)
        add(
          `stock-${sku.id}`,
          `SKU「${sku.skuName || sku.skuCode || sku.id}」的本地库存为负数`,
          "通过现有库存调整入口修正并保留审计记录。",
          `skuStock.${sku.id}`,
        );
    });
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
    if (
      !isFilled(
        effectivePackageValue(pkg.warehouseId, resolvedPackage?.warehouseId),
      )
    ) {
      add(
        "warehouse",
        "Ozon 仓库未设置",
        "填写仓库编号，或维护全局 Ozon 刊登预设。",
        "package.warehouseId",
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
    ordinaryAttributes.forEach((attribute) => {
      const value = watched.attributes?.[attribute.attrId];
      if (attribute.required && !isFilled(value))
        add(
          `attr-${attribute.attrId}`,
          `Ozon 必填属性未填写：${attribute.name}`,
          "请补全当前类目属性。",
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
          if (attribute.required && !isFilled(group?.[attribute.attrId])) {
            add(
              `complex-${complexId}-${index}-${attribute.attrId}`,
              `组合属性第 ${index + 1} 组缺少：${attribute.name}`,
              "补全该字段组；系统不会按单值静默提交。",
              `complexGroups.${complexId}`,
            );
          }
        }),
      );
    });
    return issues;
  }, [
    complexGroups,
    ordinaryAttributes,
    product,
    productId,
    preflight,
    resolvedPreview,
    shopId,
    skuImages,
    watched,
  ]);

  const saveCurrent = useCallback(
    async (silent = false) => {
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
          platformAttributes: buildOzonPlatformAttributesV2(attributes, {
            attributes: values.attributes,
            complexGroups: values.complexGroups,
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
        if (!silent) message.success("当前编辑已保存，不会提交 Ozon");
        return saved;
      } finally {
        setSaving(false);
      }
    },
    [
      attributes,
      categoryPath,
      config,
      form,
      product,
      productId,
      shopId,
      skuImages,
    ],
  );

  const runPreflight = async () => {
    if (!productId || !shopId) return;
    setChecking(true);
    try {
      if (dirty || !config?.id) await saveCurrent(true);
      const result = await validateOzonReadiness(productId, shopId);
      setPreflight(result);
      if (result.canPublish)
        message.success("只读发布前检查已通过，可以进入提交确认");
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

  const confirmSubmit = () => {
    if (!productId || !shopId || !preflight?.canPublish) return;
    const idempotencyKey = newIdempotencyKey();
    Modal.confirm({
      title: "确认提交到 Ozon？",
      icon: <ExclamationCircleOutlined />,
      width: 560,
      content: (
        <Space direction="vertical" size={8}>
          <Typography.Text>
            该操作将调用 Ozon 写接口，并按发布前检查中展示的最终 SKU 快照提交。
          </Typography.Text>
          <Typography.Text type="secondary">
            请再次确认店铺、类目、售价、库存、图片和包裹参数。提交结果以后续“刊登进度”为准。
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
    setSyncing(true);
    try {
      await syncOzonCategoryAttributes(selectedCategoryId, shopId);
      const result = await queryOzonCategoryAttributes(selectedCategoryId);
      setAttributes(
        addSavedDictionaryOptions(
          result.list || [],
          config?.platformAttributes,
        ),
      );
      message.success("已通过 Ozon 只读接口刷新当前类目属性模板");
    } catch (error) {
      message.error(errorMessage(error, "属性模板同步失败"));
    } finally {
      setSyncing(false);
    }
  };

  const syncCategoryCache = async () => {
    if (!shopId) return;
    setSyncing(true);
    try {
      await syncOzonCategoryFlow(shopId);
      await loadCategoryOptions();
      message.success("Ozon 类目缓存同步已启动");
    } catch (error) {
      message.error(errorMessage(error, "类目缓存同步失败"));
    } finally {
      setSyncing(false);
    }
  };

  const goToIssue = (issue: EditorIssue) => {
    if (!issue.field) return;
    if (issue.field.startsWith("skuImages.")) {
      document
        .getElementById("ozon-sku-images")
        ?.scrollIntoView({ behavior: "smooth", block: "start" });
      return;
    }
    if (issue.field.startsWith("skuStock.")) {
      document
        .getElementById(`field-${issue.field.replace(".", "-")}`)
        ?.scrollIntoView({ behavior: "smooth", block: "center" });
      return;
    }
    form.scrollToField(issue.field.split("."), {
      behavior: "smooth",
      block: "center",
    });
  };

  const currentImageView: OzonImageConfigView | undefined = config?.ozonImages
    ? { ...config.ozonImages, skus: skuImages }
    : undefined;
  const finalPreview: OzonResolvedListing | undefined = preflight?.resolvedOzon;

  const renderAttributeInput = (attribute: OzonCategoryAttribute) => {
    if (attribute.dictionaryId) {
      return (
        <Select
          showSearch
          allowClear={!attribute.required}
          mode={attribute.isCollection ? "multiple" : undefined}
          maxCount={attribute.maxValueCount || undefined}
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
    if (attribute.isCollection) {
      return (
        <Select
          mode="tags"
          maxCount={attribute.maxValueCount || undefined}
          tokenSeparators={[",", "，"]}
          placeholder="输入多个值后按回车"
          onChange={() => markDirty()}
        />
      );
    }
    return <Input placeholder={`填写${attribute.name}`} onChange={markDirty} />;
  };

  const localSKUByID = new Map(
    (product?.skus || []).map((sku) => [sku.id, sku]),
  );
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
      canSubmit: !immediateIssues.some((issue) => issue.key.includes(sku.id)),
      issues: [],
    }));

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
        <Form<PublishingFormValues>
          form={form}
          layout="vertical"
          onValuesChange={markDirty}
          disabled={!canEdit || loadingConfig}
        >
          <SectionCard
            title="刊登上下文"
            description="先选择商品、目标平台、店铺和叶子类目。平台字段只在已完整接入时开放。"
            className="publishing-center__context"
          >
            <div className="publishing-center__context-grid">
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
              <Form.Item
                name="categoryId"
                label="Ozon 叶子类目"
                className="publishing-center__context-form-item"
              >
                <Select
                  showSearch
                  allowClear
                  placeholder="搜索并选择平台类目"
                  filterOption={false}
                  options={categoryOptions}
                  onSearch={(keyword) => void loadCategoryOptions(keyword)}
                  onChange={(value) => void onCategoryChange(value)}
                />
              </Form.Item>
            </div>
            {config?.legacyFallback ? (
              <Alert
                type="warning"
                showIcon
                message="正在读取旧版商品级 Ozon 配置"
                description="保存后会为当前 Ozon 店铺创建独立配置，其他店铺不会被覆盖。"
              />
            ) : null}
            <Collapse
              ghost
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
                        <Link to="/settings/platforms?section=ozon-category">
                          打开类目与映射维护
                        </Link>
                      </Space>
                    </Space>
                  ),
                },
              ]}
            />
          </SectionCard>

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
            <div className="publishing-center__layout">
              <main className="publishing-center__editor">
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
                        options={["RUB", "CNY", "USD", "EUR"].map((value) => ({
                          value,
                          label: value,
                        }))}
                      />
                    </Form.Item>
                    {sourceTag(
                      preflight?.resolvedOzon?.currency.source ||
                        resolvedPreview?.currency.source,
                    )}
                  </div>
                </SectionCard>

                <SectionCard
                  title="SKU 售价与本地库存"
                  description="Ozon 售价可按 SKU 独立覆盖；库存始终读取本地 SKU，调整继续使用现有审计链路。"
                >
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
                    />
                  </div>
                </SectionCard>

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
                        label="仓库编号"
                        extra={
                          resolvedPreview?.package.warehouseId.value
                            ? `留空使用有效值：${resolvedPreview.package.warehouseId.value}`
                            : "当前没有可用预设"
                        }
                      >
                        <Input
                          placeholder={
                            resolvedPreview?.package.warehouseId.value ||
                            "填写 Ozon warehouse_id"
                          }
                        />
                      </Form.Item>
                      {sourceTag(resolvedPreview?.package.warehouseId.source)}
                    </div>
                    <div className="publishing-center__field-with-source">
                      <Form.Item
                        name={["package", "vat"]}
                        label="VAT"
                        extra={
                          resolvedPreview?.package.vat.value
                            ? `留空使用有效值：${resolvedPreview.package.vat.value}`
                            : undefined
                        }
                      >
                        <Select
                          allowClear
                          placeholder={
                            resolvedPreview?.package.vat.value || "选择 VAT"
                          }
                          options={["0", "0.1", "0.2"].map((value) => ({
                            value,
                            label: value,
                          }))}
                        />
                      </Form.Item>
                      {sourceTag(resolvedPreview?.package.vat.source)}
                    </div>
                  </div>
                </SectionCard>

                <SectionCard
                  title="Ozon 类目属性"
                  description="多值属性使用多选；复杂属性按字段组保存，只有 Ozon 明确允许时才能重复添加。"
                >
                  {!selectedCategoryId ? (
                    <Alert type="info" showIcon message="选择类目后显示属性" />
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
                      <div className="publishing-center__attribute-grid">
                        {ordinaryAttributes.map((attribute) => (
                          <Form.Item
                            key={attribute.attrId}
                            name={["attributes", attribute.attrId]}
                            label={
                              <Space size={4}>
                                {attribute.name}
                                {attribute.required ? (
                                  <Tag color="red">必填</Tag>
                                ) : null}
                                {attribute.isCollection ? (
                                  <Tag color="blue">多值</Tag>
                                ) : null}
                              </Space>
                            }
                            extra={
                              attribute.maxValueCount
                                ? `最多 ${attribute.maxValueCount} 个值`
                                : undefined
                            }
                          >
                            {renderAttributeInput(attribute)}
                          </Form.Item>
                        ))}
                      </div>
                      {complexGroups.map(([complexId, defs]) => {
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
                                            label={
                                              <Space size={4}>
                                                {attribute.name}
                                                {attribute.required ? (
                                                  <Tag color="red">必填</Tag>
                                                ) : null}
                                                {attribute.isCollection ? (
                                                  <Tag color="blue">多值</Tag>
                                                ) : null}
                                              </Space>
                                            }
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
                    </Space>
                  )}
                </SectionCard>

                <div className="publishing-center__actions">
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
                      检查并进入提交确认
                    </Button>
                    {preflight?.canPublish ? (
                      <Button
                        danger
                        type="primary"
                        loading={submitting}
                        disabled={!canPublish || dirty}
                        onClick={confirmSubmit}
                      >
                        确认提交到 Ozon
                      </Button>
                    ) : null}
                  </Space>
                  <Typography.Text type="secondary">
                    {dirty
                      ? "有未保存修改；检查时会先保存当前编辑。"
                      : "当前编辑已保存。保存不会创建刊登提交，也不会调用 Ozon 写接口。"}
                  </Typography.Text>
                </div>
              </main>

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
                              {sku.skuName || sku.skuCode || sku.skuId}
                            </Typography.Text>
                            <Tag color={sku.canSubmit ? "green" : "red"}>
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
                            库存：{sku.localStock ?? local?.stock ?? 0}{" "}
                            {sourceTag(sku.stockSource)}
                          </Typography.Text>
                          <Typography.Text>
                            图片：{sku.images.length} 张（顺序{" "}
                            {sku.images
                              .map((image) => image.position)
                              .join(" → ") || "—"}
                            ）
                          </Typography.Text>
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
                </SectionCard>
              </aside>
            </div>
          ) : null}
        </Form>
      </TmPageContainer>
    </PermissionGuard>
  );
}

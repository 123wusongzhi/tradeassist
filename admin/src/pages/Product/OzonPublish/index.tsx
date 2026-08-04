import { ReloadOutlined, SaveOutlined, SyncOutlined } from '@ant-design/icons';
import { history, Link, useLocation } from '@umijs/max';
import {
  Alert,
  Button,
  Checkbox,
  Descriptions,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Spin,
  Steps,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PermissionGuard from '@/components/PermissionGuard';
import {
  EmptyState,
  OperationToolbar,
  SectionCard,
  TmPageContainer,
  TmProTable,
} from '@/components/ui';
import { formatUserErrorMessage } from '@/constants/errorMessages';
import { usePermission } from '@/hooks/usePermission';
import {
  queryOzonCategoryAttributes,
  searchOzonDictionaryValues,
  syncOzonCategoryAttributes,
  type OzonCategoryAttribute,
} from '@/services/ozonCategories';
import { fetchProducts, type ProductListRow } from '@/services/products';
import { ApiRequestError } from '@/services/request';
import {
  createPublishTargetDrafts,
  getProductPublishTask,
  type ProductPublishTaskDTO,
} from '@/services/productPublish';
import { queryShops, type ShopListRow } from '@/services/shops';
import {
  buildOzonPlatformAttributes,
  buildOzonSKUImagePreview,
  checkOzonCategoryGroups,
  confirmOzonCategoryGroup,
  getOzonCategoryFlowStats,
  getOzonProductConfig,
  listOzonCategoryChanges,
  listOzonCategoryMappings,
  listOzonCategorySyncRuns,
  publishOzonProduct,
  recommendOzonCategory,
  saveOzonCategoryMapping,
  saveOzonProductConfig,
  searchOzonLeafCategories,
  syncOzonCategoryFlow,
  toOzonImageConfigInput,
  toOzonAttributeFormValues,
  validateOzonReadiness,
  type OzonCategoryChange,
  type OzonCategoryGroup,
  type OzonCategoryMapping,
  type OzonCategorySyncRun,
  type OzonProductConfig,
  type OzonSKUImageConfig,
  type OzonReadinessResult,
} from '@/services/ozonPublish';
import { PERMISSIONS } from '@/utils/permission';
import OzonSKUImageConfigurator from './OzonSKUImageConfigurator';
import './index.less';

type Stage = 'sync' | 'mapping' | 'config' | 'preflight' | 'submit';
type OzonProductConfigForm = Omit<
  OzonProductConfig,
  'platformAttributes' | 'ozonImages'
> & {
  platformAttributes?: Record<string, string>;
};
const stages: Array<{ key: Stage; title: string }> = [
  { key: 'sync', title: '同步状态' },
  { key: 'mapping', title: '类目映射库' },
  { key: 'config', title: '商品配置' },
  { key: 'preflight', title: '发布前检查' },
  { key: 'submit', title: '草稿与提交' },
];

function stageFromSearch(search: string): Stage {
  const candidate = new URLSearchParams(search).get('stage') as Stage | null;
  return stages.some((item) => item.key === candidate) ? candidate! : 'sync';
}
function idempotencyKey() {
  return `ozon-submit:${crypto.randomUUID()}`;
}
function statusColor(status?: string) {
  if (status === 'succeeded' || status === 'ready' || status === 'success')
    return 'green';
  if (status === 'partial' || status === 'running' || status === 'needs_work')
    return 'orange';
  if (status === 'failed' || status === 'stale') return 'red';
  return 'default';
}

function includeConfiguredDictionaryOptions(
  attributes: OzonCategoryAttribute[],
  raw?: Record<string, unknown>,
): OzonCategoryAttribute[] {
  return attributes.map((attribute) => {
    if (!attribute.dictionaryId) return attribute;
    const saved = raw?.[attribute.attrId];
    if (!saved) return attribute;
    const typed =
      typeof saved === 'object'
        ? (saved as { value?: unknown; dictionaryValueId?: unknown })
        : { value: saved };
    const value = String(typed.value ?? '').trim();
    const id = String(typed.dictionaryValueId ?? `legacy:${value}`).trim();
    if (!id || !value || attribute.options?.some((option) => option.id === id))
      return attribute;
    return {
      ...attribute,
      options: [{ id, value }, ...(attribute.options || [])],
    };
  });
}

function ozonActionErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiRequestError) {
    const data = error.data as { errorCode?: unknown } | null;
    const errorCode =
      typeof data?.errorCode === 'string' ? data.errorCode : undefined;
    return formatUserErrorMessage(errorCode, error.message || fallback);
  }
  return (error as Error)?.message || fallback;
}

function OzonPublishPageContent() {
  const location = useLocation();
  const { can, readonly, role } = usePermission();
  const params = useMemo(
    () => new URLSearchParams(location.search),
    [location.search],
  );
  const [stage, setStage] = useState<Stage>(() =>
    stageFromSearch(location.search),
  );
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [products, setProducts] = useState<ProductListRow[]>([]);
  const [shopId, setShopId] = useState<string>();
  const [productId, setProductId] = useState<string | undefined>(
    () => params.get('productId') || undefined,
  );
  const [stats, setStats] =
    useState<Awaited<ReturnType<typeof getOzonCategoryFlowStats>>>();
  const [runs, setRuns] = useState<OzonCategorySyncRun[]>([]);
  const [changes, setChanges] = useState<OzonCategoryChange[]>([]);
  const [mappings, setMappings] = useState<OzonCategoryMapping[]>([]);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [config, setConfig] = useState<OzonProductConfig>();
  const [skuImageConfigs, setSKUImageConfigs] = useState<
    OzonSKUImageConfig[]
  >([]);
  const [bulkSharedImageIds, setBulkSharedImageIds] = useState<string[]>([]);
  const [configDirty, setConfigDirty] = useState(false);
  const [attributes, setAttributes] = useState<OzonCategoryAttribute[]>([]);
  const [attributeTemplateCategoryId, setAttributeTemplateCategoryId] =
    useState<string>();
  const [attributeTemplateError, setAttributeTemplateError] =
    useState<string>();
  const [loadingAttributes, setLoadingAttributes] = useState(false);
  const [searchingAttribute, setSearchingAttribute] = useState<string>();
  const [categoryOptions, setCategoryOptions] = useState<
    Array<{ label: string; value: string }>
  >([]);
  const [saving, setSaving] = useState(false);
  const [saveAsMapping, setSaveAsMapping] = useState(false);
  const [preflight, setPreflight] = useState<OzonReadinessResult>();
  const [checking, setChecking] = useState(false);
  const [task, setTask] = useState<ProductPublishTaskDTO>();
  const [submitting, setSubmitting] = useState(false);
  const [confirmingSubmission, setConfirmingSubmission] = useState(false);
  const [submissionKey, setSubmissionKey] = useState<string>();
  const submitLockRef = useRef(false);
  const dictionarySearchSeq = useRef<Record<string, number>>({});
  const attributeSyncSeq = useRef(0);
  const configLoadSeq = useRef(0);
  const [creatingDraft, setCreatingDraft] = useState(false);
  const [groups, setGroups] = useState<OzonCategoryGroup[]>([]);
  const [groupProductIds, setGroupProductIds] = useState<string[]>([]);
  const [form] = Form.useForm<OzonProductConfigForm>();
  const selectedCategoryId = Form.useWatch('categoryId', form);

  const selectedShop = shops.find((item) => item.id === shopId);
  const selectedProduct = products.find((item) => item.id === productId);
  const crossTenantGlobalView = Boolean(
    role === 'admin' &&
    Number(selectedProduct?.tenantId || selectedShop?.tenantId || 0) > 0,
  );
  const canProductWrite =
    !readonly && !crossTenantGlobalView && can(PERMISSIONS.PRODUCT_WRITE);
  const canPublish =
    !readonly &&
    !crossTenantGlobalView &&
    can(PERMISSIONS.PUBLISH_CREATE_DRAFT);
  const canOperateShop =
    !readonly && !crossTenantGlobalView && can(PERMISSIONS.STORE_OPERATE);
  const canManageConfig =
    !readonly && !crossTenantGlobalView && can(PERMISSIONS.CONFIG_MANAGE);
  const canManageSettings = !readonly && can(PERMISSIONS.SETTINGS_MANAGE);
  const canRunPublishFlow = canProductWrite && canPublish && canOperateShop;
  const imageControlsDisabled =
    !productId || !canProductWrite || !canOperateShop;
  const configMatchesSelection = Boolean(
    config && config.productId === productId && config.shopId === shopId,
  );
  const attributeTemplateReady = Boolean(
    attributeTemplateCategoryId && attributes.length > 0,
  );
  const selectedAttributeTemplateReady = Boolean(
    attributeTemplateReady &&
    selectedCategoryId === attributeTemplateCategoryId,
  );
  const configReady = Boolean(
    productId &&
    shopId &&
    config?.categoryId &&
    configMatchesSelection &&
    attributeTemplateReady &&
    config.categoryId === attributeTemplateCategoryId &&
    !configDirty,
  );
  const sharedImages = config?.ozonImages?.sharedImages ?? [];
  const maxImagesPerSku = config?.ozonImages?.maxImagesPerSku ?? 10;

  const updateQuery = useCallback(
    (patch: Record<string, string | undefined>) => {
      const nextParams = new URLSearchParams(location.search);
      Object.entries(patch).forEach(([key, value]) =>
        value ? nextParams.set(key, value) : nextParams.delete(key),
      );
      history.replace(`${location.pathname}?${nextParams.toString()}`);
    },
    [location.pathname, location.search],
  );
  const setStageInUrl = useCallback(
    (next: Stage) => {
      updateQuery({ stage: next });
      setStage(next);
    },
    [updateQuery],
  );

  const loadPage = useCallback(async () => {
    setLoading(true);
    try {
      const [shopRes, productRes, statRes, runRes, changeRes, mappingRes] =
        await Promise.all([
          queryShops({
            page: 1,
            pageSize: 100,
            platform: 'ozon',
            authStatus: 'authorized',
          }),
          fetchProducts({ page: 1, pageSize: 100, keyword: '' }),
          getOzonCategoryFlowStats(),
          listOzonCategorySyncRuns(),
          listOzonCategoryChanges(),
          listOzonCategoryMappings(canOperateShop ? shopId : undefined),
        ]);
      const authorized = (shopRes.list ?? []).filter(
        (item) =>
          item.platform === 'ozon' &&
          item.authStatus === 'authorized' &&
          item.status === 'active',
      );
      setShops(authorized);
      setProducts(productRes.list ?? []);
      setStats(statRes);
      setRuns(runRes.list ?? []);
      setChanges(changeRes.list ?? []);
      setMappings(mappingRes.list ?? []);
    } catch (error) {
      message.error((error as Error).message || '加载 Ozon 刊登流程失败');
    } finally {
      setLoading(false);
    }
  }, [canOperateShop, shopId]);

  useEffect(() => {
    void loadPage();
  }, [loadPage]);
  useEffect(() => {
    setProductId(params.get('productId') || undefined);
  }, [params]);
  useEffect(() => {
    setStage(stageFromSearch(location.search));
  }, [location.search]);

  const invalidatePreflight = useCallback(() => {
    setPreflight(undefined);
    setSubmissionKey(undefined);
  }, []);
  const loadConfig = useCallback(
    async (id: string) => {
      const sequence = configLoadSeq.current + 1;
      configLoadSeq.current = sequence;
      setAttributeTemplateError(undefined);
      try {
        const next = await getOzonProductConfig(id);
        if (configLoadSeq.current !== sequence) return;
        const {
          platformAttributes: _platformAttributes,
          ozonImages,
          ...configFields
        } = next;
        setConfig(next);
        setSKUImageConfigs(
          (ozonImages?.skus ?? []).map((sku) =>
            buildOzonSKUImagePreview(
              sku,
              ozonImages?.sharedImages ?? [],
              ozonImages?.maxImagesPerSku ?? 10,
            ),
          ),
        );
        setBulkSharedImageIds([]);
        setConfigDirty(false);
        setShopId((current) => next.shopId || current);
        form.setFieldsValue({ ...configFields, platformAttributes: {} });
        if (next.categoryId) {
          const attrs = await queryOzonCategoryAttributes(next.categoryId);
          if (configLoadSeq.current !== sequence) return;
          const displayAttrs = includeConfiguredDictionaryOptions(
            attrs.list ?? [],
            next.platformAttributes,
          );
          setAttributes(displayAttrs);
          if (displayAttrs.length > 0) {
            setAttributeTemplateCategoryId(next.categoryId);
          } else {
            setAttributeTemplateCategoryId(undefined);
            setAttributeTemplateError(
              '当前 Ozon 类目没有可用的属性模板，请重新选择类目并同步；若持续失败，请检查店铺凭证。',
            );
          }
          form.setFieldValue(
            'platformAttributes',
            toOzonAttributeFormValues(displayAttrs, next.platformAttributes),
          );
        } else {
          setAttributes([]);
          setAttributeTemplateCategoryId(undefined);
        }
      } catch (error) {
        if (configLoadSeq.current === sequence) {
          const detail = (error as Error).message || '加载商品级 Ozon 配置失败';
          setAttributeTemplateCategoryId(undefined);
          setAttributeTemplateError(detail);
          message.error(detail);
        }
      }
    },
    [form],
  );
  useEffect(() => {
    configLoadSeq.current += 1;
    setConfig(undefined);
    setSKUImageConfigs([]);
    setBulkSharedImageIds([]);
    setConfigDirty(false);
    setAttributes([]);
    setAttributeTemplateCategoryId(undefined);
    setAttributeTemplateError(undefined);
    form.resetFields();
    invalidatePreflight();
    if (productId) void loadConfig(productId);
  }, [form, invalidatePreflight, loadConfig, productId]);
  useEffect(() => {
    setTask(undefined);
    setConfirmingSubmission(false);
    setSubmitting(false);
    setSubmissionKey(undefined);
    submitLockRef.current = false;
  }, [productId, shopId]);

  const searchCategories = useCallback(async (keyword: string) => {
    try {
      const result = await searchOzonLeafCategories(keyword);
      setCategoryOptions(
        (result.list ?? []).map((item) => ({
          label: `${item.name}（${item.descriptionCategoryId || item.categoryId}）`,
          value: item.categoryId,
          categoryPath: item.name,
          cacheId: item.id,
        })),
      );
    } catch (error) {
      message.error((error as Error).message || '搜索 Ozon 类目失败');
    }
  }, []);
  const onCategoryChange = useCallback(
    async (categoryId: string) => {
      if (!shopId || !canOperateShop || !canProductWrite) {
        message.warning('当前账号不能同步并修改该商品的 Ozon 类目配置。');
        return;
      }
      const categoryPath = (
        categoryOptions.find((item) => item.value === categoryId) as
          | { categoryPath?: string }
          | undefined
      )?.categoryPath;
      const previousCategoryId = attributeTemplateCategoryId;
      const previousCategoryPath = form.getFieldValue('categoryPath');
      const previousPlatformAttributes =
        form.getFieldValue('platformAttributes');
      const previousAttributes = attributes;
      const previousDirty = configDirty;
      const sequence = attributeSyncSeq.current + 1;
      attributeSyncSeq.current = sequence;
      setLoadingAttributes(true);
      setAttributeTemplateError(undefined);
      try {
        await syncOzonCategoryAttributes(categoryId, shopId);
        const result = await queryOzonCategoryAttributes(categoryId);
        if (attributeSyncSeq.current !== sequence) return;
        const nextAttributes = result.list ?? [];
        if (nextAttributes.length === 0)
          throw new Error(
            'Ozon 返回的类目属性模板为空，请重新同步类目或检查店铺凭证。',
          );
        setAttributes(nextAttributes);
        setAttributeTemplateCategoryId(categoryId);
        form.setFieldsValue({
          categoryId,
          categoryPath,
          platformAttributes: {},
        });
        setConfigDirty(true);
        invalidatePreflight();
      } catch (error) {
        if (attributeSyncSeq.current !== sequence) return;
        const detail =
          (error as Error).message ||
          '同步或加载 Ozon 属性模板失败；商品配置尚未保存。';
        form.setFieldsValue({
          categoryId: previousCategoryId,
          categoryPath: previousCategoryId ? previousCategoryPath : undefined,
          platformAttributes: previousCategoryId
            ? previousPlatformAttributes
            : {},
        });
        setAttributes(previousCategoryId ? previousAttributes : []);
        setAttributeTemplateCategoryId(previousCategoryId);
        setConfigDirty(previousDirty);
        setAttributeTemplateError(
          `${detail}；新类目未应用，${
            previousCategoryId ? '已恢复上一个可用类目。' : '已清除本次选择。'
          }`,
        );
        message.error(detail);
      } finally {
        if (attributeSyncSeq.current === sequence) setLoadingAttributes(false);
      }
    },
    [
      canOperateShop,
      canProductWrite,
      attributeTemplateCategoryId,
      attributes,
      categoryOptions,
      configDirty,
      form,
      invalidatePreflight,
      shopId,
    ],
  );

  const searchDictionaryOptions = useCallback(
    async (attributeId: string, keyword: string) => {
      const value = keyword.trim();
      const categoryId = String(form.getFieldValue('categoryId') || '').trim();
      if (!shopId || !categoryId || Array.from(value).length < 2) return;
      const sequence = (dictionarySearchSeq.current[attributeId] || 0) + 1;
      dictionarySearchSeq.current[attributeId] = sequence;
      setSearchingAttribute(attributeId);
      try {
        const result = await searchOzonDictionaryValues(
          categoryId,
          attributeId,
          shopId,
          value,
        );
        if (dictionarySearchSeq.current[attributeId] !== sequence) return;
        const selectedID = String(
          form.getFieldValue(['platformAttributes', attributeId]) || '',
        );
        setAttributes((current) =>
          current.map((attribute) => {
            if (attribute.attrId !== attributeId) return attribute;
            const selected = attribute.options?.find(
              (option) => option.id === selectedID,
            );
            const options = [...(result.list || [])];
            if (
              selected &&
              !options.some((option) => option.id === selected.id)
            )
              options.push(selected);
            return { ...attribute, options };
          }),
        );
      } catch (error) {
        message.error((error as Error).message || '搜索 Ozon 词典值失败');
      } finally {
        if (dictionarySearchSeq.current[attributeId] === sequence)
          setSearchingAttribute(undefined);
      }
    },
    [form, shopId],
  );

  const runSync = async () => {
    if (!shopId || !canOperateShop) {
      message.warning('当前账号不能同步该 Ozon 店铺的类目。');
      return;
    }
    setSyncing(true);
    try {
      const next = await syncOzonCategoryFlow(shopId);
      if (next.stats) setStats(next.stats);
      const run = next.run;
      const runID = next.runId || run?.id;
      if (run?.status === 'pending' || run?.status === 'running')
        message.success(
          `同步任务已创建/处理中；同步记录：${runID || '已受理'}。同步只更新 TradeMind 类目树缓存，不修改商品、不提交 Ozon。`,
        );
      else
        message.success(
          `Ozon 类目同步已返回结果；同步记录：${runID || '已受理'}。同步只更新 TradeMind 类目树缓存，不修改商品、不提交 Ozon。`,
        );
      await loadPage();
    } catch (error) {
      message.error((error as Error).message || '创建类目同步任务失败');
    } finally {
      setSyncing(false);
    }
  };
  const updateSKUImageSelection = (
    skuId: string,
    patch: Partial<
      Pick<
        OzonSKUImageConfig,
        'fallbackMainImageId' | 'additionalImageIds'
      >
    >,
  ) => {
    if (
      patch.additionalImageIds &&
      patch.additionalImageIds.length > maxImagesPerSku - 1
    ) {
      message.warning(
        `每个 SKU 最多追加 ${maxImagesPerSku - 1} 张商品公共图片。`,
      );
      return;
    }
    setSKUImageConfigs((current) =>
      current.map((sku) =>
        sku.skuId === skuId
          ? buildOzonSKUImagePreview(
              { ...sku, ...patch },
              sharedImages,
              maxImagesPerSku,
            )
          : sku,
      ),
    );
    setConfigDirty(true);
    invalidatePreflight();
  };
  const applyBulkSharedImages = (imageIds: string[]) => {
    if (imageIds.length > maxImagesPerSku - 1) {
      message.warning(
        `每个 SKU 最多追加 ${maxImagesPerSku - 1} 张商品公共图片。`,
      );
      return;
    }
    setSKUImageConfigs((current) =>
      current.map((sku) =>
        buildOzonSKUImagePreview(
          { ...sku, additionalImageIds: imageIds },
          sharedImages,
          maxImagesPerSku,
        ),
      ),
    );
    setConfigDirty(true);
    invalidatePreflight();
  };
  const saveConfig = async () => {
    if (!productId || !canProductWrite || !canOperateShop) {
      message.warning('当前账号不能修改该商品的 Ozon 配置。');
      return;
    }
    if (!selectedAttributeTemplateReady) {
      message.warning('请先成功同步所选 Ozon 叶类目的属性模板。');
      return;
    }
    try {
      const values = await form.validateFields();
      setSaving(true);
      const attributesValue = buildOzonPlatformAttributes(
        attributes,
        values.platformAttributes,
      );
      const saved = await saveOzonProductConfig(productId, {
        ...values,
        shopId,
        platformAttributes: attributesValue,
        ozonImages: toOzonImageConfigInput(skuImageConfigs),
      });
      setConfig(saved);
      setSKUImageConfigs(
        (saved.ozonImages?.skus ?? []).map((sku) =>
          buildOzonSKUImagePreview(
            sku,
            saved.ozonImages?.sharedImages ?? [],
            saved.ozonImages?.maxImagesPerSku ?? 10,
          ),
        ),
      );
      setConfigDirty(false);
      const {
        platformAttributes: _savedAttributes,
        ozonImages: _savedImages,
        ...savedFields
      } = saved;
      form.setFieldsValue({
        ...savedFields,
        platformAttributes: toOzonAttributeFormValues(
          attributes,
          saved.platformAttributes,
        ),
      });
      invalidatePreflight();
      if (saveAsMapping && canManageConfig) {
        const sourceCategoryKey = String(saved.sourceCategoryKey || '').trim();
        if (!sourceCategoryKey || !saved.categoryId) {
          message.warning(
            '商品级 Ozon 配置已保存，但商品没有稳定的本地类目标识，未保存类目映射。',
          );
          return;
        }
        try {
          await saveOzonCategoryMapping({
            shopId,
            sourceCategoryKey,
            sourceCategoryName: saved.sourceCategoryName,
            categoryId: saved.categoryId,
            categoryPath: saved.categoryPath,
            status: 'active',
          });
        } catch (mappingError) {
          message.warning(
            `商品级 Ozon 配置已保存，但类目映射保存失败：${(mappingError as Error).message || '请稍后重试'}`,
          );
          return;
        }
        message.success('商品级 Ozon 配置和类目映射已保存，尚未提交到 Ozon。');
        return;
      }
      message.success('商品级 Ozon 配置已保存，尚未提交到 Ozon。');
    } catch (error) {
      if ((error as { errorFields?: unknown }).errorFields) return;
      message.error(ozonActionErrorMessage(error, '保存商品级 Ozon 配置失败'));
    } finally {
      setSaving(false);
    }
  };
  const runPreflight = async () => {
    if (configDirty) {
      message.warning('请先保存商品级 Ozon 配置，再运行发布前检查。');
      return;
    }
    if (!productId || !shopId || !canRunPublishFlow || !configReady) {
      message.warning('请先选择商品和店铺，并保存完整的 Ozon 配置。');
      return;
    }
    setChecking(true);
    try {
      const result = await validateOzonReadiness(productId, shopId);
      setPreflight(result);
      message.success(
        result.canPublish
          ? '发布前检查通过；仍需确认真实提交。'
          : '发布前检查发现需要处理的项目。',
      );
    } catch (error) {
      message.error(ozonActionErrorMessage(error, '发布前检查失败'));
    } finally {
      setChecking(false);
    }
  };
  const createLocalDraft = async () => {
    if (!productId || !shopId || !canRunPublishFlow) {
      message.warning('当前账号不能创建该商品的本地刊登草稿。');
      return;
    }
    setCreatingDraft(true);
    try {
      await createPublishTargetDrafts(productId, {
        targets: [{ platform: 'ozon', shopId }],
        onlyReady: false,
      });
      message.success('本地草稿已创建，未调用 Ozon。');
    } catch (error) {
      message.error((error as Error).message || '创建本地草稿失败');
    } finally {
      setCreatingDraft(false);
    }
  };
  const submitReal = () => {
    if (configDirty) {
      message.warning('请先保存商品级 Ozon 配置并重新运行发布前检查。');
      return;
    }
    if (
      !productId ||
      !shopId ||
      !canRunPublishFlow ||
      !preflight?.canPublish ||
      submitLockRef.current ||
      task?.id
    )
      return;
    const key = submissionKey || idempotencyKey();
    submitLockRef.current = true;
    setConfirmingSubmission(true);
    Modal.confirm({
      title: '确认提交到 Ozon？',
      width: 520,
      okText: '创建 Ozon 提交任务',
      cancelText: '取消',
      content: (
        <Descriptions size="small" column={1}>
          <Descriptions.Item label="店铺">
            {selectedShop?.shopName || shopId}
          </Descriptions.Item>
          <Descriptions.Item label="商品">
            {selectedProduct?.title || productId}
          </Descriptions.Item>
          <Descriptions.Item label="Ozon 类目">
            {config?.categoryPath || config?.categoryId || '未选择'}
          </Descriptions.Item>
        </Descriptions>
      ),
      onCancel: () => {
        submitLockRef.current = false;
        setConfirmingSubmission(false);
      },
      onOk: async () => {
        setSubmissionKey(key);
        setSubmitting(true);
        try {
          const result = await publishOzonProduct(productId, shopId, key);
          setTask(result);
          message.success('已创建提交任务，等待处理。');
        } catch (error) {
          submitLockRef.current = false;
          setConfirmingSubmission(false);
          message.error((error as Error).message || '创建 Ozon 提交任务失败');
          throw error;
        } finally {
          setSubmitting(false);
        }
      },
    });
  };
  const refreshTask = async () => {
    if (!task?.id) return;
    try {
      setTask(await getProductPublishTask(task.id));
    } catch (error) {
      message.error((error as Error).message || '刷新任务失败');
    }
  };
  const requestRecommendation = async (mapping: OzonCategoryMapping) => {
    if (!canOperateShop) {
      message.warning('当前账号不能读取该 Ozon 店铺的类目推荐。');
      return;
    }
    try {
      const result = await recommendOzonCategory({
        shopId,
        sourceCategoryKey: mapping.sourceCategoryKey,
        sourceCategoryName: mapping.sourceCategoryName,
      });
      const candidate = result.candidate;
      if (!candidate) {
        message.info('暂无推荐候选，请手动选择 Ozon 叶类目。');
        return;
      }
      message.info(
        `推荐候选，尚未确认：${candidate.categoryPath || candidate.categoryId}`,
      );
    } catch (error) {
      message.error((error as Error).message || '获取类目推荐失败');
    }
  };
  const applyMapping = async (mapping: OzonCategoryMapping) => {
    if (!canManageConfig) return;
    try {
      await saveOzonCategoryMapping(mapping);
      message.success('类目映射已保存；不会自动修改其他商品。');
      await loadPage();
    } catch (error) {
      message.error((error as Error).message || '保存类目映射失败');
    }
  };
  const checkGroups = async () => {
    if (!groupProductIds.length || !shopId || !canPublish || !canOperateShop) {
      message.warning(
        '请先选择已授权 Ozon 店铺和批量商品，并确认当前账号有刊登权限。',
      );
      return;
    }
    try {
      const result = await checkOzonCategoryGroups({
        productIds: groupProductIds,
        shopId,
      });
      setGroups(result.groups ?? []);
    } catch (error) {
      message.error((error as Error).message || '批量类目分组检查失败');
    }
  };
  const confirmGroup = async (group: OzonCategoryGroup) => {
    if (!group.recommendedCategoryId || !shopId || !canRunPublishFlow) {
      message.warning('当前账号不能确认该组 Ozon 类目配置。');
      return;
    }
    try {
      await confirmOzonCategoryGroup({
        shopId,
        groups: [
          {
            sourceCategoryKey: group.sourceCategoryKey,
            sourceCategoryName: group.sourceCategoryName,
            productIds: group.productIds,
            categoryId: group.recommendedCategoryId,
            categoryPath: group.recommendedCategoryPath,
          },
        ],
        saveMappings: false,
      });
      message.success(
        '该组类目配置已确认，仅更新商品本地配置，未保存共享类目映射、未提交 Ozon。',
      );
      await checkGroups();
    } catch (error) {
      message.error((error as Error).message || '确认类目分组失败');
    }
  };

  const activeIndex = stages.findIndex((item) => item.key === stage);
  const taskCreated =
    (task?.status === 'success' || task?.publishStatus === 'success') &&
    Boolean(String(task?.platformProductId || '').trim());
  const taskTerminalFailure = ['failed', 'cancelled', 'canceled'].includes(
    String(task?.status || task?.publishStatus || '').toLowerCase(),
  );
  return (
    <TmPageContainer
      title="Ozon 类目与刊登"
      subTitle="从类目同步、商品级配置到提交任务；仅在最后一步明确确认后才会调用 Ozon。"
    >
      <Spin spinning={loading}>
        <div className="ozon-publish-page">
          <Steps
            current={activeIndex}
            items={stages.map((item) => ({ title: item.title }))}
            responsive
          />
          <OperationToolbar
            extra={
              canManageSettings ? (
                <Link to="/settings/platforms?platform=ozon">
                  管理 Ozon 平台设置
                </Link>
              ) : undefined
            }
          >
            <Select
              aria-label="选择 Ozon 店铺"
              placeholder="选择已授权 Ozon 店铺"
              value={shopId}
              onChange={(value) => {
                setShopId(value);
                setConfigDirty(Boolean(productId));
                invalidatePreflight();
              }}
              options={shops.map((item) => ({
                label: item.shopName,
                value: item.id,
              }))}
              style={{ minWidth: 220 }}
            />
            <Select
              aria-label="选择商品"
              placeholder="选择商品"
              value={productId}
              onChange={(value) => {
                updateQuery({ productId: value });
                setProductId(value);
                invalidatePreflight();
              }}
              options={products.map((item) => ({
                label: item.title,
                value: item.id,
              }))}
              showSearch
              optionFilterProp="label"
              style={{ minWidth: 260 }}
            />
          </OperationToolbar>
          <div
            role="tablist"
            aria-label="Ozon 刊登流程"
            className="ozon-publish-page__tabs"
          >
            {stages.map((item) => (
              <Button
                key={item.key}
                type={stage === item.key ? 'primary' : 'text'}
                role="tab"
                aria-selected={stage === item.key}
                onClick={() => setStageInUrl(item.key)}
              >
                {item.title}
              </Button>
            ))}
          </div>
          {crossTenantGlobalView ? (
            <Alert
              type="warning"
              showIcon
              message="当前为跨租户只读查看"
              description="全局管理员可以查看该租户的商品和 Ozon 配置，但不能代表目标租户保存配置、运行发布前检查或提交；请使用目标租户管理员账号。"
            />
          ) : !canRunPublishFlow ? (
            <Alert
              type="info"
              showIcon
              message="当前账号仅可查看 Ozon 刊登流程"
              description="类目同步、商品配置、发布前检查和提交操作需要商品编辑、刊登及店铺操作权限。"
            />
          ) : null}
          {configDirty ? (
            <Alert
              type="warning"
              showIcon
              message="商品级 Ozon 配置有未保存的修改"
              description="请先保存配置；在保存前，发布前检查、本地草稿和真实提交均不可用。"
            />
          ) : null}
          {stage === 'sync' && (
            <SectionCard
              title="同步状态"
              description="此处全量同步只更新 TradeMind 的 Ozon 类目树缓存；选定叶类目时再按需刷新属性模板。两者都不修改商品、不提交 Ozon。"
              headerExtra={
                <Button
                  icon={<SyncOutlined />}
                  loading={syncing}
                  disabled={!shopId || !canOperateShop}
                  onClick={() => void runSync()}
                >
                  同步 Ozon 类目
                </Button>
              }
            >
              <Descriptions size="small" column={{ xs: 1, sm: 2, lg: 4 }}>
                <Descriptions.Item label="有效类目">
                  {stats?.activeCount ?? '—'}
                </Descriptions.Item>
                <Descriptions.Item label="已停用类目">
                  {stats?.inactiveCount ?? '—'}
                </Descriptions.Item>
                <Descriptions.Item label="最近同步">
                  {stats?.lastSyncedAt || '从未同步'}
                </Descriptions.Item>
                <Descriptions.Item label="当前状态">
                  <Tag color={statusColor(stats?.lastRun?.status)}>
                    {stats?.lastRun?.statusLabel ||
                      stats?.lastRun?.status ||
                      '未同步'}
                  </Tag>
                </Descriptions.Item>
              </Descriptions>
              <TmProTable<OzonCategoryChange>
                rowKey="id"
                search={false}
                options={false}
                pagination={false}
                dataSource={changes}
                scroll={{ x: 680 }}
                locale={{
                  emptyText: (
                    <EmptyState
                      compact
                      title="暂无类目变更"
                      description="完成一次同步后会显示新增、变更、停用和恢复的类目差异。"
                    />
                  ),
                }}
                columns={[
                  {
                    title: '变更',
                    dataIndex: 'changeType',
                    render: (value) => {
                      const text = String(value || '');
                      return (
                        <Tag
                          color={statusColor(
                            text === 'deactivated' ? 'failed' : text,
                          )}
                        >
                          {text}
                        </Tag>
                      );
                    },
                  },
                  { title: '类目', dataIndex: 'categoryName', ellipsis: true },
                  { title: '时间', dataIndex: 'occurredAt', width: 180 },
                  { title: '说明', dataIndex: 'detail', ellipsis: true },
                ]}
              />
              <Typography.Text type="secondary">
                最近同步记录：{runs[0]?.id || stats?.lastRun?.id || '暂无'} ·{' '}
                {runs[0]?.statusLabel ||
                  runs[0]?.status ||
                  stats?.lastRun?.status ||
                  '未同步'}
                ；新增{' '}
                {stats?.diffCounts?.added ??
                  stats?.lastRun?.summary?.added ??
                  0}{' '}
                · 变更{' '}
                {stats?.diffCounts?.changed ??
                  stats?.lastRun?.summary?.changed ??
                  0}{' '}
                · 停用{' '}
                {stats?.diffCounts?.deactivated ??
                  stats?.lastRun?.summary?.deactivated ??
                  0}{' '}
                · 恢复{' '}
                {stats?.diffCounts?.reactivated ??
                  stats?.lastRun?.summary?.reactivated ??
                  0}
              </Typography.Text>
            </SectionCard>
          )}
          {stage === 'mapping' && (
            <SectionCard
              title="类目映射库"
              description="推荐只是一项候选，必须由你确认；保存映射不会静默批量修改商品。"
            >
              <OperationToolbar>
                <Button
                  disabled={!canPublish || !canOperateShop}
                  onClick={() => void checkGroups()}
                >
                  检查批量类目分组
                </Button>
                <Select
                  mode="multiple"
                  disabled={!canPublish || !canOperateShop}
                  placeholder="选择要分组的商品"
                  value={groupProductIds}
                  onChange={setGroupProductIds}
                  options={products.map((item) => ({
                    label: item.title,
                    value: item.id,
                  }))}
                  style={{ minWidth: 300, maxWidth: '100%' }}
                />
              </OperationToolbar>
              <TmProTable<OzonCategoryMapping>
                rowKey={(row) => row.id || row.sourceCategoryKey}
                search={false}
                options={false}
                pagination={false}
                dataSource={mappings}
                scroll={{ x: 760 }}
                columns={[
                  {
                    title: '本地类目',
                    dataIndex: 'sourceCategoryName',
                    ellipsis: true,
                  },
                  {
                    title: 'Ozon 类目',
                    dataIndex: 'categoryPath',
                    ellipsis: true,
                  },
                  {
                    title: '状态',
                    dataIndex: 'status',
                    render: (value) => <Tag>{value || '已保存'}</Tag>,
                  },
                  {
                    title: '操作',
                    width: 220,
                    render: (_, row) => (
                      <Space wrap>
                        <Button
                          size="small"
                          disabled={!canOperateShop}
                          onClick={() => void requestRecommendation(row)}
                        >
                          获取推荐候选
                        </Button>
                        {canManageConfig ? (
                          <Button
                            size="small"
                            onClick={() => void applyMapping(row)}
                          >
                            保存映射
                          </Button>
                        ) : null}
                      </Space>
                    ),
                  },
                ]}
              />
              {groups.map((group) => (
                <Alert
                  key={group.key}
                  type={
                    group.status === 'ready'
                      ? 'success'
                      : group.status === 'skipped'
                        ? 'info'
                        : 'warning'
                  }
                  showIcon
                  message={`${group.sourceCategoryName || group.sourceCategoryKey || '未分类'} · ${group.statusLabel || group.status}`}
                  description={
                    <Space wrap>
                      <span>
                        {group.productIds.length} 个商品；推荐候选，尚未确认：
                        {group.recommendedCategoryPath ||
                          group.recommendedCategoryId ||
                          '无'}
                      </span>
                      {group.issues?.[0]?.message ? (
                        <span>{group.issues[0].message}</span>
                      ) : null}
                      <Button
                        size="small"
                        disabled={
                          !canRunPublishFlow ||
                          !group.recommendedCategoryId ||
                          group.status === 'skipped'
                        }
                        onClick={() => void confirmGroup(group)}
                      >
                        确认本组类目
                      </Button>
                    </Space>
                  }
                />
              ))}
            </SectionCard>
          )}
          {stage === 'config' && (
            <SectionCard
              title="商品级 Ozon 配置"
              description="商品配置优先于全局刊登预设。保存只写入 TradeMind，不会创建 Ozon 商品。"
            >
              {attributeTemplateError ? (
                <Alert
                  className="ozon-publish-page__template-error"
                  type="error"
                  showIcon
                  message="Ozon 类目属性模板同步失败"
                  description={attributeTemplateError}
                  action={
                    canManageSettings ? (
                      <Link to="/shops/manage?platform=ozon">
                        更新 Ozon 凭证
                      </Link>
                    ) : undefined
                  }
                />
              ) : null}
              <Form
                form={form}
                layout="vertical"
                disabled={!productId || !canProductWrite || !canOperateShop}
                onValuesChange={() => {
                  setConfigDirty(true);
                  invalidatePreflight();
                }}
              >
                <Form.Item
                  name="categoryId"
                  label="Ozon 叶类目"
                  rules={[{ required: true, message: '请选择 Ozon 叶类目' }]}
                >
                  <Select
                    showSearch
                    filterOption={false}
                    loading={loadingAttributes}
                    disabled={
                      !shopId ||
                      loadingAttributes ||
                      !canProductWrite ||
                      !canOperateShop
                    }
                    onSearch={(value) => void searchCategories(value)}
                    onChange={(value) => void onCategoryChange(value)}
                    options={categoryOptions}
                    placeholder={
                      shopId
                        ? '搜索并选择 Ozon 叶类目；不会自动选择第一项'
                        : '请先选择已授权 Ozon 店铺'
                    }
                  />
                </Form.Item>
                <Form.Item name="sourceCategoryName" label="本地类目说明">
                  <Input placeholder="可选：用于保存类目映射" />
                </Form.Item>
                {attributes.map((attr) => (
                  <Form.Item
                    key={attr.attrId}
                    name={['platformAttributes', attr.attrId]}
                    label={`${attr.name}${attr.required ? '（必填）' : ''}`}
                    rules={
                      attr.required
                        ? [{ required: true, message: `请填写 ${attr.name}` }]
                        : undefined
                    }
                  >
                    {attr.dictionaryId ? (
                      <Select
                        showSearch
                        filterOption={false}
                        loading={searchingAttribute === attr.attrId}
                        onSearch={(value) =>
                          void searchDictionaryOptions(attr.attrId, value)
                        }
                        options={(attr.options || []).map((option) => ({
                          label: option.value,
                          value: option.id,
                        }))}
                        placeholder={`选择或输入至少 2 个字符搜索 ${attr.name}`}
                      />
                    ) : (
                      <Input placeholder={`填写 ${attr.name}`} />
                    )}
                  </Form.Item>
                ))}
              </Form>
              <OzonSKUImageConfigurator
                config={config?.ozonImages}
                skus={skuImageConfigs}
                bulkImageIds={bulkSharedImageIds}
                disabled={imageControlsDisabled}
                onBulkImageIdsChange={setBulkSharedImageIds}
                onApplyBulk={applyBulkSharedImages}
                onUpdateSKU={updateSKUImageSelection}
              />
              <OperationToolbar>
                {canManageConfig ? (
                  <Checkbox
                    checked={saveAsMapping}
                    onChange={(event) => setSaveAsMapping(event.target.checked)}
                    disabled={!productId}
                  >
                    保存为类目映射（否则仅应用当前商品）
                  </Checkbox>
                ) : null}
                <Button
                  type="primary"
                  icon={<SaveOutlined />}
                  loading={saving}
                  onClick={() => void saveConfig()}
                  disabled={
                    !productId ||
                    loadingAttributes ||
                    !selectedAttributeTemplateReady ||
                    !canProductWrite ||
                    !canOperateShop
                  }
                >
                  保存商品级 Ozon 配置
                </Button>
                {!productId ? (
                  <Typography.Text type="secondary">
                    请先选择商品
                  </Typography.Text>
                ) : null}
              </OperationToolbar>
            </SectionCard>
          )}
          {stage === 'preflight' && (
            <SectionCard
              title="发布前检查"
              description="检查会实时验证已保存的当前店铺、类目和动态属性；修改任何配置后需要重新保存并检查。"
            >
              <OperationToolbar>
                <Button
                  type="primary"
                  loading={checking}
                  onClick={() => void runPreflight()}
                  disabled={!configReady || !canRunPublishFlow}
                >
                  运行发布前检查
                </Button>
                {preflight?.checkedAt ? (
                  <Typography.Text type="secondary">
                    最近检查：{preflight.checkedAt}
                  </Typography.Text>
                ) : null}
              </OperationToolbar>
              {!preflight ? (
                <EmptyState
                  compact
                  title="尚未检查"
                  description="请先保存商品配置，再显式运行发布前检查；系统不会自动检查或自动提交。"
                />
              ) : (
                <>
                  <Alert
                    type={preflight.canPublish ? 'success' : 'error'}
                    showIcon
                    message={
                      preflight.canPublish
                        ? '检查通过，仍需确认真实提交'
                        : '检查未通过，不能提交到 Ozon'
                    }
                    description={
                      preflight.schemaChanged
                        ? 'Ozon 属性模板已变化，请重新确认类目和属性。'
                        : undefined
                    }
                  />
                  {(preflight.checks || preflight.items || []).map(
                    (item, index) => {
                      const meta = item as {
                        level?: string;
                        severity?: string;
                        suggestion?: string;
                      };
                      return (
                        <Alert
                          key={`${item.code}-${index}`}
                          type={
                            (meta.level || meta.severity) === 'error'
                              ? 'error'
                              : 'warning'
                          }
                          showIcon
                          message={item.title || item.code || '检查项'}
                          description={
                            <>
                              {item.message}
                              {meta.suggestion
                                ? ` 建议：${meta.suggestion}`
                                : ''}
                            </>
                          }
                        />
                      );
                    },
                  )}
                </>
              )}
            </SectionCard>
          )}
          {stage === 'submit' && (
            <SectionCard
              title="本地草稿与真实提交"
              description="本地草稿与 Ozon 提交是两个不同动作。真实提交仅在已保存配置的最新检查通过后开放，并需要二次确认。"
            >
              <OperationToolbar>
                <Button
                  loading={creatingDraft}
                  onClick={() => void createLocalDraft()}
                  disabled={!configReady || !canRunPublishFlow}
                >
                  创建本地草稿
                </Button>
                <Button
                  type="primary"
                  danger
                  loading={submitting}
                  disabled={
                    !configReady ||
                    !canRunPublishFlow ||
                    !preflight?.canPublish ||
                    submitting ||
                    confirmingSubmission ||
                    !!task?.id
                  }
                  onClick={submitReal}
                >
                  提交到 Ozon
                </Button>
                {task?.id ? (
                  <Button
                    icon={<ReloadOutlined />}
                    onClick={() => void refreshTask()}
                  >
                    刷新任务
                  </Button>
                ) : null}
                {taskTerminalFailure ? (
                  <Button onClick={() => setStageInUrl('preflight')}>
                    查看发布前检查
                  </Button>
                ) : null}
              </OperationToolbar>
              {task ? (
                <Alert
                  type={
                    taskCreated
                      ? 'success'
                      : taskTerminalFailure
                        ? 'error'
                        : 'info'
                  }
                  showIcon
                  message={
                    taskCreated
                      ? 'Ozon 商品已创建，等待平台审核'
                      : taskTerminalFailure
                        ? String(
                            task.status || task.publishStatus,
                          ).toLowerCase() === 'cancelled'
                          ? 'Ozon 提交任务已取消'
                          : 'Ozon 提交任务失败'
                        : '已创建提交任务，等待处理'
                  }
                  description={
                    taskCreated ? (
                      `平台商品 ID：${task.platformProductId}`
                    ) : taskTerminalFailure ? (
                      <>
                        {task.errorMessage ? `${task.errorMessage}。` : ''}
                        提交结果不确定/失败，请先在 Ozon
                        后台或任务中心核对，系统不会自动重提。
                      </>
                    ) : (
                      '请手动刷新任务状态；本页不会自动轮询。'
                    )
                  }
                />
              ) : (
                <Alert
                  type="info"
                  showIcon
                  message="尚未创建提交任务"
                  description="创建本地草稿不会调用 Ozon；只有点击“提交到 Ozon”并确认后才会创建真实提交任务。"
                />
              )}
            </SectionCard>
          )}
        </div>
      </Spin>
    </TmPageContainer>
  );
}

export default function OzonPublishPage() {
  return (
    <PermissionGuard require={PERMISSIONS.PRODUCT_VIEW} showForbiddenPage>
      <OzonPublishPageContent />
    </PermissionGuard>
  );
}

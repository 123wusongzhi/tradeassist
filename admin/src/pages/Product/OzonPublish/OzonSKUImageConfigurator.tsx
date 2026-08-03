import {
  Alert,
  Button,
  Checkbox,
  Image,
  Select,
  Space,
  Tag,
  Typography,
} from 'antd';
import { EmptyState } from '@/components/ui';
import type {
  OzonImageConfigView,
  OzonSKUImageConfig,
} from '@/services/ozonPublish';

export type OzonSKUImageSelectionPatch = Partial<
  Pick<
    OzonSKUImageConfig,
    'fallbackMainImageId' | 'additionalImageIds'
  >
>;

type Props = {
  config?: OzonImageConfigView;
  skus: OzonSKUImageConfig[];
  bulkImageIds: string[];
  disabled: boolean;
  onBulkImageIdsChange: (imageIds: string[]) => void;
  onApplyBulk: (imageIds: string[]) => void;
  onUpdateSKU: (skuId: string, patch: OzonSKUImageSelectionPatch) => void;
};

function skuDisplayName(sku: OzonSKUImageConfig) {
  return sku.skuName || sku.skuCode || sku.skuId;
}

function skuAttrsText(sku: OzonSKUImageConfig) {
  if (!sku.attrs) return '';
  return Object.entries(sku.attrs)
    .map(([key, value]) => `${key}：${String(value ?? '')}`)
    .join(' / ');
}

function imageSourceLabel(source: string) {
  if (source === 'sku_original') return 'SKU 原始主图';
  if (source === 'manual_fallback') return '人工替代主图';
  return '商品公共图片';
}

export default function OzonSKUImageConfigurator({
  config,
  skus,
  bulkImageIds,
  disabled,
  onBulkImageIdsChange,
  onApplyBulk,
  onUpdateSKU,
}: Props) {
  const sharedImages = config?.sharedImages ?? [];
  const maxImagesPerSku = config?.maxImagesPerSku ?? 10;
  const errorCount =
    (config?.issues?.length ?? 0) +
    skus.reduce((total, sku) => total + sku.issues.length, 0);
  const imagesReady = skus.length > 0 && errorCount === 0;

  return (
    <div className="ozon-publish-page__sku-images">
      <div className="ozon-publish-page__sku-images-heading">
        <div>
          <Typography.Title level={5}>SKU 图片配置</Typography.Title>
          <Typography.Paragraph type="secondary">
            每个 SKU 的采集原图固定排在第一张；商品公共图片只在你明确选择后追加。缺少原图时，必须明确选择并保存替代主图。
          </Typography.Paragraph>
        </div>
        <Tag color={imagesReady ? 'green' : 'red'}>
          {skus.length === 0
            ? '没有可配置的 SKU'
            : errorCount > 0
            ? `${errorCount} 项缺图或失效配置`
            : `${skus.length} 个 SKU 图片可提交`}
        </Tag>
      </div>
      {config?.compatibilityMode === 'sku_original_only' ? (
        <Alert
          type="info"
          showIcon
          message="旧商品兼容模式：仅使用各 SKU 原始主图"
          description="系统不会把全部商品公共图片默认塞给每个 SKU。检查后保存，即可形成可追溯的 SKU 图片配置。"
        />
      ) : null}
      {(config?.issues ?? []).map((issue, index) => (
        <Alert
          key={`${issue.code}-${issue.skuId || 'config'}-${index}`}
          type="error"
          showIcon
          message={issue.message}
          description={issue.suggestion}
        />
      ))}
      {sharedImages.length > 0 ? (
        <div className="ozon-publish-page__bulk-images">
          <Typography.Text strong>批量追加相同商品公共图片</Typography.Text>
          <Typography.Paragraph type="secondary">
            先勾选，再应用到所有 SKU；应用后仍可逐个 SKU 调整。
          </Typography.Paragraph>
          <Checkbox.Group
            aria-label="批量选择商品公共图片"
            value={bulkImageIds}
            disabled={disabled}
            onChange={(values) => onBulkImageIdsChange(values.map(String))}
          >
            <div className="ozon-publish-page__shared-image-grid">
              {sharedImages.map((image, index) => (
                <Checkbox key={image.id} value={image.id}>
                  <span className="ozon-publish-page__shared-image-option">
                    <Image
                      src={image.url}
                      alt={`商品公共图片 ${index + 1}`}
                      width={56}
                      height={56}
                      preview={false}
                    />
                    <span>
                      图片 {index + 1}
                      <Typography.Text type="secondary">
                        {image.imageType || '商品图片'}
                      </Typography.Text>
                    </span>
                  </span>
                </Checkbox>
              ))}
            </div>
          </Checkbox.Group>
          <Space wrap>
            <Button
              disabled={disabled || bulkImageIds.length === 0}
              onClick={() => onApplyBulk(bulkImageIds)}
            >
              应用到所有 SKU
            </Button>
            <Button disabled={disabled} onClick={() => onApplyBulk([])}>
              清空所有 SKU 的追加图片
            </Button>
          </Space>
        </div>
      ) : (
        <Alert
          type="info"
          showIcon
          message="暂无可追加的商品公共图片"
          description="SKU 原始主图仍可单独提交；如需追加或指定替代图，请先在商品图片银行中添加可用图片。"
        />
      )}
      {skus.length === 0 ? (
        <EmptyState
          compact
          title="当前商品没有 SKU"
          description="请先在商品 SKU 配置中补充规格，Ozon 刊登至少需要一个 SKU。"
        />
      ) : (
        <div className="ozon-publish-page__sku-image-list">
          {skus.map((sku) => {
            const displayName = skuDisplayName(sku);
            const attrsText = skuAttrsText(sku);
            return (
              <section
                className="ozon-publish-page__sku-image-card"
                key={sku.skuId}
                aria-label={`SKU 图片配置：${displayName}`}
              >
                <div className="ozon-publish-page__sku-image-card-header">
                  <div>
                    <Typography.Text strong>{displayName}</Typography.Text>
                    {sku.skuCode && sku.skuCode !== displayName ? (
                      <Typography.Text type="secondary">
                        SKU 编码：{sku.skuCode}
                      </Typography.Text>
                    ) : null}
                    {attrsText ? (
                      <Typography.Text type="secondary">
                        {attrsText}
                      </Typography.Text>
                    ) : null}
                  </div>
                  <Tag color={sku.canPublish ? 'green' : 'red'}>
                    {sku.canPublish ? '图片可提交' : '图片未就绪'}
                  </Tag>
                </div>
                <div className="ozon-publish-page__sku-primary-row">
                  <div className="ozon-publish-page__sku-original">
                    <Typography.Text strong>SKU 原始主图</Typography.Text>
                    {sku.originalMainImageUrl ? (
                      <Image
                        src={sku.originalMainImageUrl}
                        alt={`${displayName} 原始主图`}
                        width={104}
                        height={104}
                      />
                    ) : (
                      <div className="ozon-publish-page__missing-image">
                        未采集到原始主图
                      </div>
                    )}
                  </div>
                  {!sku.originalMainImageUrl ? (
                    <div className="ozon-publish-page__fallback-select">
                      <Typography.Text strong>人工指定替代主图</Typography.Text>
                      <Select
                        aria-label={`为 ${displayName} 选择替代主图`}
                        allowClear
                        value={sku.fallbackMainImageId}
                        disabled={disabled}
                        placeholder="必须明确选择，不会自动借用其他 SKU 图片"
                        options={sharedImages.map((image, index) => ({
                          value: image.id,
                          label: `图片 ${index + 1} · ${image.imageType || '商品图片'}`,
                        }))}
                        onChange={(value) =>
                          onUpdateSKU(sku.skuId, {
                            fallbackMainImageId: value,
                          })
                        }
                      />
                      <Typography.Text type="secondary">
                        保存后会记录所选商品图片 ID，便于追溯。
                      </Typography.Text>
                    </div>
                  ) : sku.fallbackMainImageId ? (
                    <Alert
                      type="error"
                      showIcon
                      message="SKU 已补回原始主图"
                      description={
                        <Space direction="vertical" size={8}>
                          <span>
                            原始主图将固定为第 1 张；请清除此前保存的替代主图后再保存配置。
                          </span>
                          <Button
                            size="small"
                            disabled={disabled}
                            onClick={() =>
                              onUpdateSKU(sku.skuId, {
                                fallbackMainImageId: undefined,
                              })
                            }
                          >
                            清除已保存的替代主图
                          </Button>
                        </Space>
                      }
                    />
                  ) : (
                    <Alert
                      type="success"
                      showIcon
                      message="原始主图固定为第 1 张"
                      description="不能被商品公共图片覆盖或后移。"
                    />
                  )}
                </div>
                <div className="ozon-publish-page__sku-additional">
                  <Typography.Text strong>
                    追加商品公共图片（最多 {maxImagesPerSku - 1} 张）
                  </Typography.Text>
                  {sharedImages.length > 0 ? (
                    <Checkbox.Group
                      aria-label={`为 ${displayName} 选择追加图片`}
                      value={sku.additionalImageIds}
                      disabled={disabled}
                      onChange={(values) =>
                        onUpdateSKU(sku.skuId, {
                          additionalImageIds: values.map(String),
                        })
                      }
                    >
                      <div className="ozon-publish-page__shared-image-grid">
                        {sharedImages.map((image, index) => (
                          <Checkbox key={image.id} value={image.id}>
                            图片 {index + 1}
                          </Checkbox>
                        ))}
                      </div>
                    </Checkbox.Group>
                  ) : (
                    <Typography.Text type="secondary">
                      暂无可选公共图片
                    </Typography.Text>
                  )}
                  {sku.additionalImageIds.length > 0 ? (
                    <Button
                      size="small"
                      disabled={disabled}
                      onClick={() =>
                        onUpdateSKU(sku.skuId, { additionalImageIds: [] })
                      }
                    >
                      清空该 SKU 的追加图片
                    </Button>
                  ) : null}
                </div>
                <div className="ozon-publish-page__final-images">
                  <Typography.Text strong>
                    最终提交给 Ozon 的图片顺序
                  </Typography.Text>
                  {sku.finalImages.length > 0 ? (
                    <div className="ozon-publish-page__final-image-list">
                      {sku.finalImages.map((image) => (
                        <div
                          className="ozon-publish-page__final-image"
                          key={`${image.position}-${image.url}`}
                        >
                          <Tag color={image.position === 1 ? 'blue' : undefined}>
                            {image.position}
                          </Tag>
                          <Image
                            src={image.url}
                            alt={`${displayName} 提交图片 ${image.position}`}
                            width={72}
                            height={72}
                          />
                          <Typography.Text type="secondary">
                            {imageSourceLabel(image.source)}
                          </Typography.Text>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <Typography.Text type="danger">
                      尚无可提交图片
                    </Typography.Text>
                  )}
                </div>
                {sku.issues.map((issue, index) => (
                  <Alert
                    key={`${issue.code}-${index}`}
                    type="error"
                    showIcon
                    message={issue.message}
                    description={issue.suggestion}
                  />
                ))}
              </section>
            );
          })}
        </div>
      )}
    </div>
  );
}

import { ModalForm, ProFormCheckbox, ProFormRadio, ProFormSelect } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import {
  Alert,
  Button,
  Collapse,
  Form,
  Image,
  Space,
  Spin,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  CloudUploadOutlined,
  FileImageOutlined,
  FontColorsOutlined,
  InfoCircleOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  TranslationOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useImageProviders } from '@/hooks/useImageProviders';
import type { ProductImageRow } from '@/services/products';
import {
  buildTranslateImageTextInput,
  createImageTask,
  TRANSLATE_IMAGE_TEXT_AI_SETTINGS_HINT,
  TRANSLATE_IMAGE_TEXT_LAYOUT_MODE_OPTIONS,
  TRANSLATE_IMAGE_TEXT_RENDER_MODE_OPTIONS,
  TRANSLATE_IMAGE_TEXT_SOURCE_LANG_OPTIONS,
  TRANSLATE_IMAGE_TEXT_TARGET_LANG_OPTIONS,
  type TranslateImageTextLayoutMode,
  type TranslateRenderMode,
} from '@/services/imageTasks';
import { testOCRConnection } from '@/services/settings';
import './index.less';

export function TranslateImageTextAiSettingsHint() {
  return (
    <Alert
      type="info"
      showIcon
      className="tm-translate-image-text-modal__settings-hint"
      message="识别与翻译使用 AI 设置"
      description={
        <>
          {TRANSLATE_IMAGE_TEXT_AI_SETTINGS_HINT}{' '}
          <Typography.Link onClick={() => history.push('/settings/image')}>前往 OCR 配置</Typography.Link>
        </>
      }
    />
  );
}

export type TranslateImageTextPrefill = {
  productId?: string;
  sourceImageId?: string;
  sourceImageUrl?: string;
  provider?: string;
  sourceLanguage?: string;
  targetLanguage?: string;
};

type FormValues = {
  sourceLanguage: string;
  targetLanguage: string;
  layoutMode: TranslateImageTextLayoutMode;
  autoSaveToProductImages: boolean;
  outputAsDetail: boolean;
  autoSetAsMain: boolean;
  // Advanced
  provider?: string;
  ocrProvider?: string;
  renderMode: TranslateRenderMode;
  eraseMode?: string;
  advancedJson?: string;
};

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess?: () => void;
  prefill?: TranslateImageTextPrefill;
  fixedProductId?: string;
  sourceImage?: ProductImageRow;
};

export function TranslateImageTextModal({
  open,
  onOpenChange,
  onSuccess,
  prefill,
  fixedProductId,
  sourceImage,
}: Props) {
  const [form] = Form.useForm<FormValues>();
  const { optionsForTask } = useImageProviders();
  const renderMode = Form.useWatch('renderMode', form) ?? 'pure_text_replace';
  const [ocrChecking, setOcrChecking] = useState(false);
  const [ocrReady, setOcrReady] = useState(false);
  const [ocrMessage, setOcrMessage] = useState('');
  const [ocrCheckFailed, setOcrCheckFailed] = useState(false);
  const [sourceImageFailed, setSourceImageFailed] = useState(false);
  const watchedSourceLanguage = Form.useWatch('sourceLanguage', form) ?? prefill?.sourceLanguage ?? 'auto';
  const watchedTargetLanguage = Form.useWatch('targetLanguage', form) ?? prefill?.targetLanguage ?? 'en';

  const productId = (fixedProductId || prefill?.productId || '').trim();
  const sourceImageId = (prefill?.sourceImageId || sourceImage?.id || '').trim();
  const sourceImageUrl = (
    prefill?.sourceImageUrl ||
    sourceImage?.publicUrl ||
    sourceImage?.originUrl ||
    ''
  ).trim();

  const providerOptions = useMemo(() => optionsForTask('translate_image_text'), [optionsForTask]);
  const sourceLanguageLabel = TRANSLATE_IMAGE_TEXT_SOURCE_LANG_OPTIONS.find((item) => item.value === watchedSourceLanguage)?.label ?? '自动识别';
  const targetLanguageLabel = TRANSLATE_IMAGE_TEXT_TARGET_LANG_OPTIONS.find((item) => item.value === watchedTargetLanguage)?.label ?? '英文';
  const ocrStatusType = ocrChecking ? 'info' : ocrReady ? 'success' : ocrCheckFailed ? 'error' : 'warning';
  const ocrStatusTitle = ocrChecking
    ? '正在检测 OCR 服务'
    : ocrReady
      ? 'OCR 服务可用'
      : ocrCheckFailed
        ? 'OCR 检测请求失败'
        : '图片文字翻译需要 OCR 服务';

  useEffect(() => {
    if (!open) return;
    form.setFieldsValue({
      sourceLanguage: prefill?.sourceLanguage ?? 'auto',
      targetLanguage: prefill?.targetLanguage ?? 'en',
      layoutMode: 'auto',
      autoSaveToProductImages: true,
      outputAsDetail: true,
      autoSetAsMain: false,
      renderMode: 'pure_text_replace',
      provider: prefill?.provider ?? '',
      ocrProvider: '',
      eraseMode: 'text_pixel_mask',
      advancedJson: '',
    });
  }, [open, form, prefill]);

  useEffect(() => {
    if (!open) return;
    setSourceImageFailed(false);
  }, [open, sourceImageUrl]);

  const checkOCRReady = useCallback(async () => {
    setOcrChecking(true);
    setOcrCheckFailed(false);
    try {
      const res = await testOCRConnection();
      setOcrReady(Boolean(res.ok));
      setOcrMessage(res.message || '当前 OCR 服务可用');
    } catch (e: unknown) {
      setOcrReady(false);
      setOcrCheckFailed(true);
      setOcrMessage((e as Error)?.message || '图片文字翻译需要 OCR 服务。请先到「设置 → 图片 AI 设置」选择 OCR 服务并测试通过。');
    } finally {
      setOcrChecking(false);
    }
  }, []);

  useEffect(() => {
    if (!open) {
      setOcrReady(false);
      setOcrMessage('');
      setOcrCheckFailed(false);
      return;
    }
    void checkOCRReady();
  }, [open, checkOCRReady]);

  return (
    <ModalForm<FormValues>
      form={form}
      title="图片文字翻译"
      open={open}
      onOpenChange={onOpenChange}
      width={560}
      modalProps={{ destroyOnHidden: true, className: 'tm-translate-image-text-modal' }}
      submitter={{
        render: (props) => {
          const submitButtonProps = props.submitButtonProps === false ? undefined : props.submitButtonProps;

          return [
            <Button key="cancel" onClick={() => onOpenChange(false)}>
              取消
            </Button>,
            <Button
              key="submit"
              type="primary"
              loading={ocrChecking || submitButtonProps?.loading}
              disabled={!ocrReady}
              onClick={() => props.submit?.()}
            >
              创建翻译任务
            </Button>,
          ];
        },
      }}
      onFinish={async (values) => {
        if (!sourceImageId && !sourceImageUrl) {
          message.error('请选择要翻译的商品图片');
          return false;
        }
        if (!ocrReady) {
          message.error('图片文字翻译需要 OCR 服务。请先到「设置 → 图片 AI 设置」选择 OCR 服务并测试通过。');
          return false;
        }
        const input = buildTranslateImageTextInput({
          sourceLanguage: values.sourceLanguage,
          targetLanguage: values.targetLanguage,
          renderMode: values.renderMode,
          layoutMode: values.layoutMode,
          autoWrap: true,
          autoFontSize: true,
          allowTextSimplify: true,
          keepProductUnchanged: true,
          autoSaveToProductImages: values.autoSaveToProductImages,
          outputAsDetail: values.outputAsDetail,
          autoSetAsMain: values.autoSetAsMain,
          removeOriginalText: true,
          preserveLayout: values.layoutMode !== 'readable',
          eraseMode: values.eraseMode || undefined,
          advancedJson: values.advancedJson || undefined,
        });
        try {
          const task = await createImageTask({
            taskType: 'translate_image_text',
            provider: values.renderMode === 'ai_edit' ? values.provider?.trim() || undefined : undefined,
            productId: productId || undefined,
            sourceImageId: sourceImageId || undefined,
            sourceImageUrl: sourceImageUrl || undefined,
            input,
          });
          if (task.status === 'pending' || task.status === 'running') {
            message.success('图片文字翻译任务已提交，正在后台处理');
          } else if (task.status === 'success' || task.status === 'success_with_warnings') {
            message.success(
              task.status === 'success_with_warnings'
                ? '翻译完成（存在警告，请人工检查）'
                : '翻译已完成',
            );
          } else if (task.status === 'failed') {
            message.warning(task.errorMessage || '任务失败');
          } else {
            message.success('已创建');
          }
          onSuccess?.();
          return true;
        } catch (e: unknown) {
          message.error((e as Error)?.message || '提交失败');
          return false;
        }
      }}
    >
      <div className="tm-translate-image-text-modal__body">
        <section className="tm-translate-image-text-modal__intro">
          <div className="tm-translate-image-text-modal__intro-icon">
            <TranslationOutlined />
          </div>
          <div className="tm-translate-image-text-modal__intro-copy">
            <Typography.Text strong>提交后将创建后台图片翻译任务</Typography.Text>
            <Typography.Paragraph type="secondary">
              系统会识别图片文字并翻译为目标语言，再生成新的处理后图片。原图不会被覆盖，处理结果用于刊登前建议人工检查文字、版式和商品主体；AI 处理可能消耗模型额度。
            </Typography.Paragraph>
          </div>
        </section>

        <section className="tm-translate-image-text-modal__section">
          <div className="tm-translate-image-text-modal__section-head">
            <Typography.Text strong>
              <FileImageOutlined />
              待翻译图片
            </Typography.Text>
            {sourceImageId ? <Tag bordered={false}>源图 ID {sourceImageId}</Tag> : <Tag bordered={false}>未提供源图 ID</Tag>}
          </div>
          {sourceImageUrl ? (
            <div className="tm-translate-image-text-modal__image-panel">
              {sourceImageFailed ? (
                <div className="tm-translate-image-text-modal__image-fallback">
                  <FileImageOutlined />
                  <Typography.Text type="secondary">图片加载失败，请检查源图地址</Typography.Text>
                </div>
              ) : (
                <Image
                  src={sourceImageUrl}
                  className="tm-translate-image-text-modal__image"
                  onError={() => setSourceImageFailed(true)}
                />
              )}
              <Typography.Text
                type="secondary"
                className="tm-translate-image-text-modal__image-url"
                title={sourceImageUrl}
              >
                {sourceImageUrl}
              </Typography.Text>
            </div>
          ) : (
            <Alert
              type="warning"
              showIcon
              message="未提供源图地址"
              description="当前任务缺少可翻译的图片 URL，请返回图片列表选择带有图片地址的商品图片。"
            />
          )}
        </section>

        <section className="tm-translate-image-text-modal__section">
          <div className="tm-translate-image-text-modal__section-head">
            <Typography.Text strong>
              <SafetyCertificateOutlined />
              OCR 服务状态
            </Typography.Text>
            {ocrChecking ? <Spin size="small" /> : null}
          </div>
          <TranslateImageTextAiSettingsHint />
          <Alert
            type={ocrStatusType}
            showIcon
            message={ocrStatusTitle}
            description={
              <span>
                {ocrMessage || '请先到「设置 → 图片 AI 设置」选择 OCR 服务并测试通过。'}{' '}
                {!ocrReady ? <Typography.Link onClick={() => history.push('/settings/image')}>去配置 OCR</Typography.Link> : null}
              </span>
            }
            action={
              <Button size="small" loading={ocrChecking} onClick={() => void checkOCRReady()}>
                重新检测
              </Button>
            }
          />
        </section>

        <section className="tm-translate-image-text-modal__section">
          <div className="tm-translate-image-text-modal__section-head">
            <Typography.Text strong>
              <FontColorsOutlined />
              翻译设置
            </Typography.Text>
            <Space size={[6, 4]} wrap>
              <Tag bordered={false}>{sourceLanguageLabel}</Tag>
              <Tag color="blue" bordered={false}>
                {targetLanguageLabel}
              </Tag>
            </Space>
          </div>
          <div className="tm-translate-image-text-modal__field-grid">
            <ProFormSelect
              name="sourceLanguage"
              label="源语言"
              options={TRANSLATE_IMAGE_TEXT_SOURCE_LANG_OPTIONS}
              rules={[{ required: true, message: '请选择源语言' }]}
              extra="自动识别会先尝试判断图片中文字语言，仍建议检查结果。"
            />
            <ProFormSelect
              name="targetLanguage"
              label="目标语言"
              options={TRANSLATE_IMAGE_TEXT_TARGET_LANG_OPTIONS}
              rules={[{ required: true, message: '请选择目标语言' }]}
              extra="生成的新图片会使用该语言绘制译文。"
            />
          </div>

          <ProFormRadio.Group
            name="layoutMode"
            label="排版方式"
            options={TRANSLATE_IMAGE_TEXT_LAYOUT_MODE_OPTIONS}
            rules={[{ required: true, message: '请选择排版方式' }]}
            extra="自动适配偏均衡；保持原图更保守；清晰可读会优先译文阅读体验。"
          />
        </section>

        <section className="tm-translate-image-text-modal__section">
          <div className="tm-translate-image-text-modal__section-head">
            <Typography.Text strong>
              <CloudUploadOutlined />
              结果处理
            </Typography.Text>
            <Tag color="default" bordered={false}>
              不覆盖原图
            </Tag>
          </div>
          <div className="tm-translate-image-text-modal__result-options">
            <div className="tm-translate-image-text-modal__option">
              <ProFormCheckbox name="autoSaveToProductImages">自动保存到商品图片库</ProFormCheckbox>
              <Typography.Text type="secondary">处理结果会作为新图片保存，便于后续选择和刊登。</Typography.Text>
            </div>
            <div className="tm-translate-image-text-modal__option">
              <ProFormCheckbox name="outputAsDetail">处理后设为详情图</ProFormCheckbox>
              <Typography.Text type="secondary">保存后会把处理结果作为详情图用途。</Typography.Text>
            </div>
            <div className="tm-translate-image-text-modal__option tm-translate-image-text-modal__option--main">
              <ProFormCheckbox name="autoSetAsMain">处理后设为主图</ProFormCheckbox>
              <Typography.Text type="secondary">影响更高：处理结果可能成为商品主图，发布前请重点检查。</Typography.Text>
            </div>
          </div>
        </section>

        <Collapse
          className="tm-translate-image-text-modal__advanced"
          items={[
            {
              key: 'advanced',
              label: (
                <span className="tm-translate-image-text-modal__advanced-label">
                  <SettingOutlined />
                  高级设置
                  <Typography.Text type="secondary">普通翻译通常无需修改</Typography.Text>
                </span>
              ),
              children: (
                <>
                  <Alert
                    type="info"
                    showIcon
                    className="tm-translate-image-text-modal__advanced-hint"
                    message="仅在需要覆盖默认渲染或擦除策略时调整"
                    description="这些字段会随翻译任务一起提交，不会保存为全局配置。"
                  />
                  <ProFormRadio.Group
                    name="renderMode"
                    label="覆盖渲染方式"
                    options={TRANSLATE_IMAGE_TEXT_RENDER_MODE_OPTIONS}
                  />
                  <Form.Item
                    noStyle
                    shouldUpdate={(prevValues, currentValues) => prevValues.renderMode !== currentValues.renderMode}
                  >
                    {({ getFieldValue }) => {
                      const mode = getFieldValue('renderMode');
                      if (mode !== 'ai_edit') return null;
                      return (
                        <ProFormSelect
                          name="provider"
                          label="覆盖图片 AI 服务"
                          options={providerOptions}
                          extra="仅在 AI 编辑实验模式下生效"
                        />
                      );
                    }}
                  </Form.Item>
                  <ProFormSelect
                    name="eraseMode"
                    label="覆盖擦除方式"
                    options={[
                      { label: '默认（读取设置）', value: '' },
                      { label: '自动', value: 'auto' },
                      { label: '精细擦字（推荐）', value: 'precise_mask' },
                      { label: '背景采样', value: 'background_sample' },
                      { label: '模糊填充', value: 'blur_fill' },
                      { label: 'OpenCV 修复', value: 'opencv_inpaint' },
                      { label: 'AI 局部擦除', value: 'ai_inpaint' },
                    ]}
                  />
                </>
              ),
            },
          ]}
        />

        {!ocrReady ? (
          <div className="tm-translate-image-text-modal__submit-note">
            <WarningOutlined />
            <Typography.Text type="secondary">OCR 服务检测通过后才能创建翻译任务。</Typography.Text>
          </div>
        ) : (
          <div className="tm-translate-image-text-modal__submit-note">
            <InfoCircleOutlined />
            <Typography.Text type="secondary">点击“创建翻译任务”后会进入后台处理，结果以任务状态为准。</Typography.Text>
          </div>
        )}
      </div>
    </ModalForm>
  );
}

/** AI 图片结构化 warning / error 码（与 backend aiproductimage/warning_codes.go 对齐） */
export const AI_IMAGE_WARNING_CODES = [
  'provider_config_missing',
  'dashscope_key_missing',
  'background_remove_unsupported',
  'white_background_provider_missing',
  'logo_remove_unsupported',
  'image_download_failed',
  'image_mime_invalid',
  'image_too_large',
  'image_decode_failed',
  'provider_timeout',
  'provider_rate_limited',
  'provider_return_invalid_url',
  'storage_public_url_missing',
  'unsupported_operation',
] as const;

export type AIImageWarningCode = (typeof AI_IMAGE_WARNING_CODES)[number];

export const AI_IMAGE_WARNING_LABEL: Record<string, string> = {
  provider_config_missing: '图片 AI 服务未配置',
  dashscope_key_missing: '通义万相 API Key 未配置',
  background_remove_unsupported: '当前图片 AI 服务不支持去背景',
  white_background_provider_missing: '白底图能力不可用',
  logo_remove_unsupported: '去 Logo 能力暂不支持',
  image_download_failed: '图片下载失败',
  image_mime_invalid: '图片格式无效',
  image_too_large: '图片文件过大',
  image_decode_failed: '图片解码失败',
  provider_timeout: '图片 AI 服务超时',
  provider_rate_limited: '图片 AI 服务限流',
  provider_return_invalid_url: '处理结果无效',
  storage_public_url_missing: '存储公网地址未配置',
  unsupported_operation: '不支持的处理类型',
};

export const AI_IMAGE_WARNING_MESSAGE: Record<string, string> = {
  provider_config_missing:
    '当前未选择可用的图片 AI 服务，无法执行去背景、白底图等处理。',
  dashscope_key_missing:
    '白底图 / 背景优化等能力依赖通义万相，请补充 dashscope_image_api_key 后重新处理。',
  background_remove_unsupported:
    '所选图片 AI 服务不支持去背景能力，可更换接入服务或改用白底图 / 背景优化。',
  white_background_provider_missing:
    '当前图片 AI 配置不支持白底图生成，请配置通义万相或 remove.bg 等支持白底图的接入服务。',
  logo_remove_unsupported: '当前图片 AI 服务不支持去 Logo，该能力可能处于预留或降级状态。',
  image_download_failed: '无法从源图链接下载图片，请确认图片 URL 可访问或重新上传图片。',
  image_mime_invalid: '源图格式不被支持，请上传 JPG / PNG / WebP 等常见格式。',
  image_too_large: '源图超过处理上限，请压缩后再试。',
  image_decode_failed: '无法解析源图内容，请确认文件未损坏。',
  provider_timeout: '图片处理超时，可稍后重试或检查接入服务网络与超时设置。',
  provider_rate_limited: '图片 AI 服务返回限流，请稍后重试或降低并发。',
  provider_return_invalid_url: '图片 AI 返回的结果链接无效，请重试或检查接入服务配置。',
  storage_public_url_missing:
    '图片结果需要公网可访问地址，请在存储设置配置 public_base 并测试公网访问。',
  unsupported_operation: '当前配置不支持该图片处理类型。',
};

export const AI_IMAGE_WARNING_SETTINGS_URL: Record<string, string> = {
  provider_config_missing: '/settings/image',
  dashscope_key_missing: '/settings/image',
  background_remove_unsupported: '/settings/image',
  white_background_provider_missing: '/settings/image',
  logo_remove_unsupported: '/settings/image',
  storage_public_url_missing: '/settings/storage',
  unsupported_operation: '/settings/image',
};

export function aiImageWarningLabel(code?: string | null): string {
  const k = (code || '').trim().toLowerCase();
  if (!k) return '—';
  return AI_IMAGE_WARNING_LABEL[k] || k;
}

export function aiImageWarningMessage(code?: string | null): string {
  const k = (code || '').trim().toLowerCase();
  if (!k) return '';
  return AI_IMAGE_WARNING_MESSAGE[k] || '';
}

export function aiImageWarningSettingsUrl(code?: string | null): string | undefined {
  const k = (code || '').trim().toLowerCase();
  if (!k) return undefined;
  return AI_IMAGE_WARNING_SETTINGS_URL[k];
}

/** 批次概览降级说明（H1.3 统一文案） */
export const AI_IMAGE_DEGRADED_SUMMARY =
  '当前图片处理已完成，但部分能力因接入服务配置缺失降级。你可以补充对应密钥后重新处理。';

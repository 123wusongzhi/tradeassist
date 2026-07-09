package aiproductimage

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
)

// Structured warning / error codes for AI image processing (H1.3).
const (
	CodeProviderConfigMissing          = "provider_config_missing"
	CodeDashscopeKeyMissing            = "dashscope_key_missing"
	CodeBackgroundRemoveUnsupported    = "background_remove_unsupported"
	CodeWhiteBackgroundProviderMissing = "white_background_provider_missing"
	CodeLogoRemoveUnsupported          = "logo_remove_unsupported"
	CodeImageDownloadFailed            = "image_download_failed"
	CodeImageMimeInvalid               = "image_mime_invalid"
	CodeImageTooLarge                  = "image_too_large"
	CodeImageDecodeFailed              = "image_decode_failed"
	CodeProviderTimeout                = "provider_timeout"
	CodeProviderRateLimited            = "provider_rate_limited"
	CodeProviderReturnInvalidURL       = "provider_return_invalid_url"
	CodeStoragePublicURLMissing        = "storage_public_url_missing"
	CodeUnsupportedOperation           = "unsupported_operation"
)

// warningCodeMeta is user-facing copy (no secrets).
type warningCodeMeta struct {
	Title           string
	Message         string
	Recoverable     bool
	NeedsConfig     bool
	SettingsURL     string
	FailureCategory string
}

var warningCodeCatalog = map[string]warningCodeMeta{
	CodeProviderConfigMissing: {
		Title:           "图片 AI 服务未配置",
		Message:         "当前未选择可用的图片 AI Provider，无法执行去背景、白底图等处理。",
		Recoverable:     true,
		NeedsConfig:     true,
		SettingsURL:     "/settings/image",
		FailureCategory: "ai_image_provider_config_missing",
	},
	CodeDashscopeKeyMissing: {
		Title:           "通义万相 API Key 未配置",
		Message:         "白底图 / 背景优化等能力依赖通义万相，请补充 dashscope_image_api_key 后重新处理。",
		Recoverable:     true,
		NeedsConfig:     true,
		SettingsURL:     "/settings/image",
		FailureCategory: "ai_image_dashscope_key_missing",
	},
	CodeBackgroundRemoveUnsupported: {
		Title:           "当前 Provider 不支持去背景",
		Message:         "所选图片 AI 服务不支持去背景能力，可更换 Provider 或改用白底图 / 背景优化。",
		Recoverable:     true,
		NeedsConfig:     true,
		SettingsURL:     "/settings/image",
		FailureCategory: "ai_image_unsupported_operation",
	},
	CodeWhiteBackgroundProviderMissing: {
		Title:           "白底图能力不可用",
		Message:         "当前图片 AI 配置不支持白底图生成，请配置通义万相或 remove.bg 等支持白底图的 Provider。",
		Recoverable:     true,
		NeedsConfig:     true,
		SettingsURL:     "/settings/image",
		FailureCategory: "ai_image_unsupported_operation",
	},
	CodeLogoRemoveUnsupported: {
		Title:           "去 Logo 能力暂不支持",
		Message:         "当前图片 AI 服务不支持去 Logo，该能力可能处于预留或降级状态。",
		Recoverable:     false,
		NeedsConfig:     false,
		SettingsURL:     "/settings/image",
		FailureCategory: "ai_image_unsupported_operation",
	},
	CodeImageDownloadFailed: {
		Title:           "图片下载失败",
		Message:         "无法从源图链接下载图片，请确认图片 URL 可访问或重新上传图片。",
		Recoverable:     true,
		NeedsConfig:     false,
		SettingsURL:     "",
		FailureCategory: "ai_image_download_failed",
	},
	CodeImageMimeInvalid: {
		Title:           "图片格式无效",
		Message:         "源图格式不被支持，请上传 JPG / PNG / WebP 等常见格式。",
		Recoverable:     true,
		NeedsConfig:     false,
		SettingsURL:     "",
		FailureCategory: "ai_image_processing_failed",
	},
	CodeImageTooLarge: {
		Title:           "图片文件过大",
		Message:         "源图超过处理上限，请压缩后再试。",
		Recoverable:     true,
		NeedsConfig:     false,
		SettingsURL:     "",
		FailureCategory: "ai_image_processing_failed",
	},
	CodeImageDecodeFailed: {
		Title:           "图片解码失败",
		Message:         "无法解析源图内容，请确认文件未损坏。",
		Recoverable:     true,
		NeedsConfig:     false,
		SettingsURL:     "",
		FailureCategory: "ai_image_processing_failed",
	},
	CodeProviderTimeout: {
		Title:           "图片 AI 服务超时",
		Message:         "图片处理超时，可稍后重试或检查 Provider 网络与超时设置。",
		Recoverable:     true,
		NeedsConfig:     false,
		SettingsURL:     "/settings/image",
		FailureCategory: "ai_image_processing_failed",
	},
	CodeProviderRateLimited: {
		Title:           "图片 AI 服务限流",
		Message:         "Provider 返回限流，请稍后重试或降低并发。",
		Recoverable:     true,
		NeedsConfig:     false,
		SettingsURL:     "/settings/image",
		FailureCategory: "ai_image_processing_failed",
	},
	CodeProviderReturnInvalidURL: {
		Title:           "处理结果无效",
		Message:         "图片 AI 返回的结果链接无效，请重试或检查 Provider 配置。",
		Recoverable:     true,
		NeedsConfig:     false,
		SettingsURL:     "/settings/image",
		FailureCategory: "ai_image_processing_failed",
	},
	CodeStoragePublicURLMissing: {
		Title:           "Storage 公网地址未配置",
		Message:         "图片结果需要公网可访问地址，请在存储设置配置 public_base 并测试公网访问。",
		Recoverable:     true,
		NeedsConfig:     true,
		SettingsURL:     "/settings/storage",
		FailureCategory: "ai_image_storage_public_url_missing",
	},
	CodeUnsupportedOperation: {
		Title:           "不支持的处理类型",
		Message:         "当前配置不支持该图片处理类型。",
		Recoverable:     false,
		NeedsConfig:     true,
		SettingsURL:     "/settings/image",
		FailureCategory: "ai_image_unsupported_operation",
	},
}

// WarningCodeLabel returns Chinese title for a code.
func WarningCodeLabel(code string) string {
	code = strings.TrimSpace(strings.ToLower(code))
	if m, ok := warningCodeCatalog[code]; ok {
		return m.Title
	}
	return code
}

// WarningCodeMessage returns user-facing message for a code.
func WarningCodeMessage(code string) string {
	code = strings.TrimSpace(strings.ToLower(code))
	if m, ok := warningCodeCatalog[code]; ok {
		return m.Message
	}
	return ""
}

// WarningCodeSettingsURL returns config page path for recoverable config issues.
func WarningCodeSettingsURL(code string) string {
	code = strings.TrimSpace(strings.ToLower(code))
	if m, ok := warningCodeCatalog[code]; ok {
		return m.SettingsURL
	}
	return ""
}

// WarningCodeFailureCategory maps structured code to task-center failure category.
func WarningCodeFailureCategory(code string) string {
	code = strings.TrimSpace(strings.ToLower(code))
	if m, ok := warningCodeCatalog[code]; ok && m.FailureCategory != "" {
		return m.FailureCategory
	}
	return "ai_image_process_failed"
}

// IsWarningCodeRecoverable reports whether user can fix via config or retry.
func IsWarningCodeRecoverable(code string) bool {
	code = strings.TrimSpace(strings.ToLower(code))
	if m, ok := warningCodeCatalog[code]; ok {
		return m.Recoverable
	}
	return true
}

// NeedsConfigForCode reports whether failure is blocked by missing configuration.
func NeedsConfigForCode(code string) bool {
	code = strings.TrimSpace(strings.ToLower(code))
	if m, ok := warningCodeCatalog[code]; ok {
		return m.NeedsConfig
	}
	return false
}

// ClassifyErrorMessage maps free-text errors to structured codes (best-effort).
func ClassifyErrorMessage(msg string) string {
	low := strings.ToLower(strings.TrimSpace(msg))
	switch {
	case strings.Contains(low, "api key") || strings.Contains(low, "未配置通义万相"):
		return CodeDashscopeKeyMissing
	case strings.Contains(low, "未配置") && strings.Contains(low, "provider"):
		return CodeProviderConfigMissing
	case strings.Contains(low, "timeout") || strings.Contains(low, "超时"):
		return CodeProviderTimeout
	case strings.Contains(low, "429") || strings.Contains(low, "rate limit") || strings.Contains(low, "限流"):
		return CodeProviderRateLimited
	case strings.Contains(low, "download") || strings.Contains(low, "下载"):
		return CodeImageDownloadFailed
	case strings.Contains(low, "mime") || strings.Contains(low, "content-type"):
		return CodeImageMimeInvalid
	case strings.Contains(low, "too large") || strings.Contains(low, "过大"):
		return CodeImageTooLarge
	case strings.Contains(low, "decode") || strings.Contains(low, "解码"):
		return CodeImageDecodeFailed
	case strings.Contains(low, "public_base") || strings.Contains(low, "公网"):
		return CodeStoragePublicURLMissing
	case strings.Contains(low, "不支持"):
		return CodeUnsupportedOperation
	default:
		return ""
	}
}

// NormalizeItemErrorCode maps internal item error_code + message to structured code.
func NormalizeItemErrorCode(code, msg string) string {
	code = strings.TrimSpace(strings.ToLower(code))
	switch code {
	case "unsupported_operation":
		return CodeUnsupportedOperation
	case "no_result":
		return CodeProviderReturnInvalidURL
	case "generation_failed":
		if c := ClassifyErrorMessage(msg); c != "" {
			return c
		}
		return CodeProviderReturnInvalidURL
	case "create_failed", "enqueue_failed":
		if c := ClassifyErrorMessage(msg); c != "" {
			return c
		}
		return CodeProviderConfigMissing
	default:
		if code != "" {
			if _, ok := warningCodeCatalog[code]; ok {
				return code
			}
		}
		if c := ClassifyErrorMessage(msg); c != "" {
			return c
		}
		return code
	}
}

// ProviderReadiness describes image AI configuration for batch overview.
type ProviderReadiness struct {
	Provider     string   `json:"provider"`
	Status       string   `json:"status"`
	StatusLabel  string   `json:"statusLabel"`
	Summary      string   `json:"summary,omitempty"`
	MissingCodes []string `json:"missingCodes,omitempty"`
	DegradedOps  []string `json:"degradedOps,omitempty"`
	SettingsURL  string   `json:"settingsUrl,omitempty"`
}

// EvaluateProviderReadiness inspects settings without exposing secrets.
func EvaluateProviderReadiness(provider string, img map[string]string) ProviderReadiness {
	prov := strings.TrimSpace(strings.ToLower(provider))
	out := ProviderReadiness{
		Provider:    prov,
		SettingsURL: "/settings/image",
	}
	if prov == "" || prov == "noop" {
		out.Status = "missing"
		out.StatusLabel = "未配置"
		out.Summary = "未选择图片 AI Provider"
		out.MissingCodes = []string{CodeProviderConfigMissing}
		return out
	}
	st := imgprov.ConfigStatus(prov, img)
	switch st {
	case "configured", "ready":
		out.Status = "ready"
		out.StatusLabel = "已配置"
		out.Summary = "Provider=" + prov
	default:
		out.Status = "degraded"
		out.StatusLabel = "当前能力降级"
		if prov == "dashscope_image" {
			out.Summary = "通义万相 API Key 未配置，白底图 / 背景优化将不可用"
			out.MissingCodes = []string{CodeDashscopeKeyMissing}
		} else {
			out.Summary = "Provider=" + prov + " 凭证待补全"
			out.MissingCodes = []string{CodeProviderConfigMissing}
		}
	}
	// Operation-level degradation hints
	degraded := make([]string, 0, 4)
	if prov != "" && prov != "noop" {
		wbOK := imgprov.SupportsTask(prov, imagetask.TaskTypeRemoveBackground) ||
			imgprov.SupportsTask(prov, imagetask.TaskTypeReplaceBackground)
		if !wbOK {
			degraded = append(degraded, OpWhiteBackground)
		}
		if !imgprov.SupportsTask(prov, imagetask.TaskTypeRemoveLogo) {
			degraded = append(degraded, OpRemoveLogo)
		}
	}
	out.DegradedOps = degraded
	if len(degraded) > 0 && out.Status == "ready" {
		out.Status = "degraded"
		out.StatusLabel = "当前能力降级"
		out.Summary = out.Summary + "；部分处理能力不可用"
	}
	return out
}

package taskcenter

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/failureclassifier"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func aiImageDetailURL(batchID, itemID string) string {
	if batchID == "" {
		return ""
	}
	path := "/product/ai-image-batches/" + url.PathEscape(batchID)
	if itemID != "" {
		q := url.Values{}
		q.Set("itemId", itemID)
		path += "?" + q.Encode()
	}
	return path
}

func aiImageFailureCategory(item *aiproductimage.AIProductImageItem) string {
	if item == nil {
		return CategoryAIImageProcessFailed
	}
	switch strings.TrimSpace(item.Status) {
	case aiproductimage.ItemConflict:
		return CategoryAIImageApplyConflict
	case aiproductimage.ItemFailed:
		code := strings.TrimSpace(strings.ToLower(item.ErrorCode))
		norm := aiproductimage.NormalizeItemErrorCode(item.ErrorCode, item.ErrorMessage)
		switch norm {
		case aiproductimage.CodeProviderConfigMissing:
			return CategoryAIImageProviderConfigMissing
		case aiproductimage.CodeDashscopeKeyMissing:
			return CategoryAIImageDashscopeKeyMissing
		case aiproductimage.CodeStoragePublicURLMissing:
			return CategoryAIImageStoragePublicMissing
		case aiproductimage.CodeImageDownloadFailed:
			return CategoryAIImageDownloadFailed
		case aiproductimage.CodeUnsupportedOperation, aiproductimage.CodeWhiteBackgroundProviderMissing,
			aiproductimage.CodeLogoRemoveUnsupported, aiproductimage.CodeBackgroundRemoveUnsupported:
			return CategoryAIImageUnsupportedOperation
		}
		if code == "apply_failed" {
			return CategoryAIImageApplyFailed
		}
		if code == "undo_failed" {
			return CategoryAIImageUndoFailed
		}
		return CategoryAIImageProcessFailed
	case aiproductimage.ItemPendingReview, aiproductimage.ItemSuccess:
		if hasQualityWarningsJSON(item.QualityWarnings) {
			return CategoryAIImageQualityWarn
		}
	}
	return CategoryAIImageProcessFailed
}

func hasQualityWarningsJSON(raw datatypes.JSON) bool {
	if len(raw) == 0 {
		return false
	}
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" || s == "[]" {
		return false
	}
	var warnings []struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(raw, &warnings); err != nil {
		return false
	}
	return len(warnings) > 0
}

func aiImageFailureUserMessage(item *aiproductimage.AIProductImageItem) string {
	if item == nil {
		return ""
	}
	switch strings.TrimSpace(item.Status) {
	case aiproductimage.ItemConflict:
		return aiproductimage.ConflictUserMessage
	case aiproductimage.ItemFailed:
		msg := strings.TrimSpace(item.ErrorMessage)
		if msg != "" {
			return truncateRunes(msg, maxErrorMessageLen)
		}
		return "AI 图片处理失败，请重试或查看复核页详情。"
	case aiproductimage.ItemPendingReview, aiproductimage.ItemSuccess:
		if hasQualityWarningsJSON(item.QualityWarnings) {
			var warnings []struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal(item.QualityWarnings, &warnings)
			if len(warnings) > 0 && strings.TrimSpace(warnings[0].Message) != "" {
				return truncateRunes(warnings[0].Message, maxErrorMessageLen)
			}
			return "AI 图片结果需要人工复核，请查看质量提醒。"
		}
	}
	return truncateRunes(item.ErrorMessage, maxErrorMessageLen)
}

func aiImageNormalizedStatus(item *aiproductimage.AIProductImageItem) string {
	if item == nil {
		return NormFailed
	}
	switch strings.TrimSpace(item.Status) {
	case aiproductimage.ItemConflict, aiproductimage.ItemFailed:
		return NormFailed
	case aiproductimage.ItemPendingReview, aiproductimage.ItemSuccess:
		if hasQualityWarningsJSON(item.QualityWarnings) {
			return NormFailed
		}
		return NormSuccess
	default:
		return NormPending
	}
}

func aiImageFailureRowFilter(db *gorm.DB, includeResolved bool) *gorm.DB {
	if includeResolved {
		return db
	}
	return db.Where(`(
		status IN ?
		OR (status IN ? AND quality_warnings IS NOT NULL AND TRIM(quality_warnings::text) NOT IN ('null', '[]', ''))
	)`, []string{aiproductimage.ItemFailed, aiproductimage.ItemConflict},
		[]string{aiproductimage.ItemPendingReview, aiproductimage.ItemSuccess})
}

func mapAIProductImageItem(row *aiproductimage.AIProductImageItem, productTitles map[uuid.UUID]string, marks markSet, now time.Time) UnifiedTaskDTO {
	if row == nil {
		return UnifiedTaskDTO{}
	}
	cat := aiImageFailureCategory(row)
	norm := aiImageNormalizedStatus(row)
	ptitle := productTitles[row.ProductID]
	opLabel := aiproductimage.OperationTypeLabel(row.OperationType)
	title := "AI 图片 · " + opLabel
	if ptitle != "" {
		title = truncateRunes("AI 图片 · "+opLabel+" · "+ptitle, 240)
	}
	errMsg := aiImageFailureUserMessage(row)
	dto := UnifiedTaskDTO{
		ID:                   row.ID.String(),
		TaskType:             TaskTypeAIImage,
		SourceTable:          SourceTableAIProductImageItems,
		SourceID:             row.ID.String(),
		Title:                title,
		RelatedResourceType:  "product",
		RelatedResourceID:    row.ProductID.String(),
		RelatedResourceTitle: truncateRunes(ptitle, 255),
		Status:               row.Status,
		NormalizedStatus:     norm,
		Retryable:            strings.TrimSpace(row.Status) == aiproductimage.ItemFailed,
		ErrorMessage:         errMsg,
		ErrorCode:            strings.TrimSpace(row.ErrorCode),
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
		DetailURL:            aiImageDetailURL(row.BatchID.String(), row.ID.String()),
		RetryAction:          "POST /api/v1/products/ai-images/items/:id/regenerate",
		RawSummary:           truncateRunes("batchId="+row.BatchID.String()+" op="+row.OperationType, maxRawSummaryLen),
		SortKey:              row.UpdatedAt,
		FailureCategory:      cat,
	}
	applyAIImageClassification(&dto)
	applyMarks(&dto, TaskTypeAIImage, row.ID.String(), marks)
	_ = now
	return dto
}

func aiImageFailureSeverity(category string) string {
	switch category {
	case CategoryAIImageQualityWarn:
		return failureclassifier.SeverityLow
	case CategoryAIImageProviderConfigMissing, CategoryAIImageDashscopeKeyMissing, CategoryAIImageStoragePublicMissing:
		return failureclassifier.SeverityLow
	case CategoryAIImageApplyConflict:
		return failureclassifier.SeverityMedium
	case CategoryAIImageProcessFailed, CategoryAIImageApplyFailed, CategoryAIImageUndoFailed,
		CategoryAIImageDownloadFailed, CategoryAIImageUnsupportedOperation:
		return failureclassifier.SeverityMedium
	default:
		return failureclassifier.SeverityLow
	}
}

func aiImageFailureReason(category string) string {
	switch category {
	case CategoryAIImageProviderConfigMissing:
		return "AI 图片 Provider 未配置或不可用。"
	case CategoryAIImageDashscopeKeyMissing:
		return "通义万相 API Key 未配置，白底图等能力降级。"
	case CategoryAIImageStoragePublicMissing:
		return "Storage 公网访问地址未配置，部分结果无法对外使用。"
	case CategoryAIImageDownloadFailed:
		return "源图下载失败，无法继续处理。"
	case CategoryAIImageUnsupportedOperation:
		return "当前 Provider 不支持该图片处理类型。"
	case CategoryAIImageProcessFailed:
		return "AI 图片处理失败。"
	case CategoryAIImageApplyConflict:
		return "AI 图片应用时发现冲突。"
	case CategoryAIImageApplyFailed:
		return "AI 图片应用失败。"
	case CategoryAIImageUndoFailed:
		return "AI 图片撤销失败。"
	case CategoryAIImageQualityWarn:
		return "AI 图片结果需要人工复核。"
	default:
		return "AI 图片任务异常。"
	}
}

func aiImageSuggestedAction(category string) string {
	switch category {
	case CategoryAIImageProviderConfigMissing:
		return "前往「设置 → 图片 AI」选择 Provider 并填写密钥，然后在复核页重试失败项。"
	case CategoryAIImageDashscopeKeyMissing:
		return "前往「设置 → 图片 AI」补全通义万相 API Key，然后重试白底图 / 背景优化任务。"
	case CategoryAIImageStoragePublicMissing:
		return "前往「设置 → 存储」配置 public_base 并测试公网访问。"
	case CategoryAIImageDownloadFailed:
		return "请确认商品图片链接可访问，必要时重新上传图片后重试。"
	case CategoryAIImageUnsupportedOperation:
		return "请更换支持该能力的 Provider，或跳过不支持的处理类型。"
	case CategoryAIImageProcessFailed:
		return "请在 AI 图片复核页重试失败项或单独重新处理。"
	case CategoryAIImageApplyConflict:
		return "请打开复核页对比原图与结果，确认无人工修改后再应用。"
	case CategoryAIImageApplyFailed:
		return "请检查商品状态后在复核页重试应用。"
	case CategoryAIImageUndoFailed:
		return "若商品已被人工修改，撤销会被阻止；请在复核页查看详情。"
	case CategoryAIImageQualityWarn:
		return "质量提醒不阻断应用，但建议人工确认后再应用到商品。"
	default:
		return "请打开 AI 图片复核页处理。"
	}
}

func applyAIImageClassification(d *UnifiedTaskDTO) failureclassifier.Result {
	if d == nil {
		return failureclassifier.Result{}
	}
	cat := strings.TrimSpace(d.FailureCategory)
	if cat == "" {
		cat = CategoryAIImageProcessFailed
	}
	sev := aiImageFailureSeverity(cat)
	retryable := strings.TrimSpace(d.Status) == aiproductimage.ItemFailed
	if cat == CategoryAIImageProviderConfigMissing || cat == CategoryAIImageDashscopeKeyMissing ||
		cat == CategoryAIImageStoragePublicMissing {
		retryable = false
	}
	d.Retryable = retryable
	r := failureclassifier.Result{
		Category:        cat,
		Severity:        sev,
		Reason:          aiImageFailureReason(cat),
		MatchedRule:     "ai_image:status",
		SuggestedAction: aiImageSuggestedAction(cat),
	}
	d.FailureCategory = r.Category
	d.Severity = r.Severity
	d.ClassificationReason = r.Reason
	d.MatchedRule = r.MatchedRule
	d.SuggestedAction = r.SuggestedAction
	return r
}

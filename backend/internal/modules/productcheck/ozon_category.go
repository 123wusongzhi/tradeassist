package productcheck

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

// checkOzonListingConfig validates the saved, user-confirmed Ozon selection.
// It is deliberately cache-only; the publish provider still performs the
// final read-only schema lookup immediately before product/import.
func (s *Service) checkOzonListingConfig(ctx context.Context, p product.Product, requestedShopID *uuid.UUID, publishOptions map[string]any) []CheckItem {
	cfg, _, err := product.FindProductPlatformPublishConfig(ctx, s.DB, p.ID, "ozon", requestedShopID, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			out := checkOzonSKUImages(p, nil)
			return append(out, CheckItem{Group: "platform", Code: "OZON_CATEGORY_NOT_SELECTED", Level: levelError, Message: "Ozon 类目未选择", Suggestion: "请先保存商品级 Ozon 类目与属性。"})
		}
		return nil
	}
	out := checkOzonSKUImages(p, cfg.MappedImages)
	if cfg.ShopID == nil {
		return append(out, CheckItem{Group: "platform", Code: "OZON_SHOP_NOT_AUTHORIZED", Level: levelError, Message: "Ozon 店铺未选择", Suggestion: "请选择已授权的 Ozon 店铺。"})
	}
	if requestedShopID != nil && *requestedShopID != uuid.Nil && *cfg.ShopID != *requestedShopID {
		return append(out, CheckItem{Group: "platform", Code: "OZON_SHOP_CONFIG_MISMATCH", Level: levelError, Message: "商品配置的 Ozon 店铺与本次目标店铺不一致", Suggestion: "请保存当前目标店铺的商品级 Ozon 配置后重试。"})
	}
	var cat shop.PlatformCategory
	if err := s.DB.WithContext(ctx).Where("platform = ? AND category_id = ?", "ozon", cfg.CategoryID).First(&cat).Error; err != nil {
		return append(out, CheckItem{Group: "platform", Code: "OZON_CATEGORY_CACHE_MISSING", Level: levelError, Message: "本地没有 Ozon 类目缓存", Suggestion: "请同步 Ozon 类目。"})
	}
	if !cat.IsLeaf || cat.Status != "active" {
		return append(out, CheckItem{Group: "platform", Code: "OZON_CATEGORY_INACTIVE", Level: levelError, Message: "Ozon 类目已停用或不是叶子类目", Suggestion: "请重新选择有效 Ozon 类目。"})
	}
	var attrs []shop.PlatformCategoryAttribute
	_ = s.DB.WithContext(ctx).Where("platform = ? AND category_id = ?", "ozon", cfg.CategoryID).Order("attr_id ASC").Find(&attrs).Error
	if len(attrs) == 0 {
		return append(out, CheckItem{Group: "platform", Code: "OZON_ATTRIBUTE_SCHEMA_MISSING", Level: levelError, Message: "Ozon 类目属性模板尚未同步", Suggestion: "请刷新当前 Ozon 类目的属性模板。"})
	}
	hash := shop.OzonCategorySchemaHash(attrs)
	if cfg.SchemaHash == "" || cfg.SchemaHash != hash {
		out = append(out, CheckItem{Group: "platform", Code: "OZON_SCHEMA_CHANGED", Level: levelError, Message: "Ozon 类目属性模板已变化", Suggestion: "请重新确认 Ozon 类目属性后再提交。"})
	}
	out = append(out, s.checkOzonCategoryConfirmation(ctx, p, *cfg, hash)...)
	payload, decodeErr := product.DecodeOzonPlatformAttributes(cfg.PlatformAttributes)
	skuIDs := make([]uuid.UUID, 0, len(p.SKUs))
	for _, sku := range p.SKUs {
		skuIDs = append(skuIDs, sku.ID)
	}
	var attributeValidationErr error
	if decodeErr != nil {
		out = append(out, CheckItem{Group: "platform", Code: "OZON_ATTRIBUTE_PAYLOAD_INVALID", Level: levelError, Message: decodeErr.Error(), Suggestion: "请重新保存商品级 Ozon 属性。"})
	} else {
		attributeValidationErr = product.ValidateOzonPlatformAttributePayloadForSKUs(attrs, payload, skuIDs, true)
	}
	preset := map[string]string{}
	if s.Settings != nil {
		if values, settingsErr := s.Settings.PlainByGroup(ctx, 0, "platform_publish_ozon"); settingsErr == nil {
			preset = values
		}
	}
	resolved := product.ResolveOzonListing(p, cfg, preset, ozonPublishOptionString(publishOptions, "currency_code"))
	for _, issue := range resolved.Issues {
		out = append(out, CheckItem{Group: "platform", Code: issue.Code, Level: levelError, Message: issue.Message, Suggestion: issue.Suggestion})
	}
	hasDetailedVariantIssue := false
	for _, sku := range resolved.SKUs {
		for _, issue := range sku.Issues {
			if !strings.HasPrefix(issue.Code, "OZON_SKU_VARIANT_") {
				continue
			}
			hasDetailedVariantIssue = true
			out = append(out, CheckItem{
				Group: "platform", Code: issue.Code, Level: levelError, Message: issue.Message, Suggestion: issue.Suggestion,
				RelatedResourceType: "product_sku", RelatedResourceID: sku.SKUID.String(),
				TechnicalDetails: map[string]any{"field": issue.Field},
			})
		}
	}
	out = append(out, checkOzonSKUStock(p.SKUs)...)
	if attributeValidationErr != nil {
		if product.IsOzonSKUVariantValidationError(attributeValidationErr) {
			// Missing values and duplicate tuples already have field-addressable
			// per-SKU checks above. Keep one correctly scoped fallback for stale
			// or otherwise malformed mappings that cannot be attached to a row.
			if !hasDetailedVariantIssue {
				out = append(out, CheckItem{Group: "platform", Code: "OZON_SKU_VARIANT_CONFIG_INVALID", Level: levelError, Message: attributeValidationErr.Error(), Suggestion: "请重新检查每个 SKU 的 Ozon 变体维度和值，并移除已删除 SKU 的旧映射。"})
			}
		} else {
			out = append(out, CheckItem{Group: "platform", Code: "OZON_REQUIRED_ATTR_MISSING", Level: levelError, Message: attributeValidationErr.Error(), Suggestion: "请补全商品级 Ozon 属性；多值使用多选，组合属性使用可重复字段组。"})
		}
	}
	return out
}

func checkOzonSKUStock(skus []product.ProductSKU) []CheckItem {
	out := make([]CheckItem, 0)
	for _, sku := range skus {
		displayName := strings.TrimSpace(sku.SKUName)
		if displayName == "" {
			displayName = strings.TrimSpace(sku.SKUCode)
		}
		if displayName == "" {
			displayName = sku.ID.String()
		}
		switch {
		case sku.Stock == nil:
			out = append(out, CheckItem{
				Group: "inventory", Code: "OZON_SKU_STOCK_UNCONFIRMED", Level: levelError,
				Message: "SKU「" + displayName + "」尚未确认本地库存", Suggestion: "请通过库存调整入口确认库存；未知库存不能按 0 静默提交。",
				RelatedResourceType: "product_sku", RelatedResourceID: sku.ID.String(),
			})
		case *sku.Stock == 0:
			out = append(out, CheckItem{
				Group: "inventory", Code: "OZON_SKU_STOCK_ZERO", Level: levelWarning,
				Message: "SKU「" + displayName + "」的本地库存为 0", Suggestion: "仍可创建 Ozon 商品，但平台库存会同步为 0，商品暂不可售；补货后再更新库存。",
				RelatedResourceType: "product_sku", RelatedResourceID: sku.ID.String(),
			})
		}
	}
	return out
}

// checkOzonCategoryConfirmation fails closed when the selected leaf category
// is no longer backed by the product-level schema confirmation and, when a
// source category is known, the tenant/store-owned category mapping. The
// mapping is explicit operator evidence; this deliberately avoids guessing
// category compatibility from product title keywords.
func (s *Service) checkOzonCategoryConfirmation(ctx context.Context, p product.Product, cfg product.ProductPlatformPublishConfig, currentSchemaHash string) []CheckItem {
	out := make([]CheckItem, 0, 3)
	if cfg.SchemaConfirmedAt == nil {
		out = append(out, CheckItem{
			Group: "platform", Code: "OZON_CATEGORY_NOT_CONFIRMED", Level: levelError,
			Message: "当前 Ozon 类目尚未由运营确认", Suggestion: "请重新选择叶子类目、核对完整路径并保存当前店铺配置。",
		})
	}
	if strings.TrimSpace(cfg.CategoryPath) == "" {
		out = append(out, CheckItem{
			Group: "platform", Code: "OZON_CATEGORY_PATH_MISSING", Level: levelError,
			Message: "当前 Ozon 类目缺少可核对的完整路径", Suggestion: "请重新选择类目并确认完整路径后保存。",
		})
	}

	sourceKey := strings.TrimSpace(cfg.SourceCategoryKey)
	if sourceKey == "" || s == nil || s.DB == nil {
		return out
	}
	var mapping shop.OzonCategoryMapping
	query := s.DB.WithContext(ctx).Where("tenant_id = ? AND source_category_key = ?", p.TenantID, sourceKey)
	err := gorm.ErrRecordNotFound
	if cfg.ShopID != nil && *cfg.ShopID != uuid.Nil {
		err = query.Where("shop_id = ?", *cfg.ShopID).First(&mapping).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = query.Where("shop_id IS NULL").First(&mapping).Error
	}
	if err != nil {
		out = append(out, CheckItem{
			Group: "platform", Code: "OZON_CATEGORY_MAPPING_UNCONFIRMED", Level: levelError,
			Message: "来源类目尚未确认对应的 Ozon 类目", Suggestion: "请在类目映射中人工核对并确认当前叶子类目；系统不会仅凭关键词猜测。",
			TechnicalDetails: map[string]any{"sourceCategoryKey": sourceKey},
		})
		return out
	}
	if mapping.Status != shop.OzonMappingActive || mapping.ConfirmedAt == nil {
		out = append(out, CheckItem{
			Group: "platform", Code: "OZON_CATEGORY_MAPPING_UNCONFIRMED", Level: levelError,
			Message: "来源类目的 Ozon 映射仍待人工确认", Suggestion: "请核对来源类目与 Ozon 完整类目路径，并将映射状态确认后重试。",
			TechnicalDetails: map[string]any{"sourceCategoryKey": sourceKey, "mappingStatus": mapping.Status},
		})
		return out
	}
	if strings.TrimSpace(mapping.ConfirmationReason) == "" {
		out = append(out, CheckItem{
			Group: "platform", Code: "OZON_CATEGORY_MAPPING_EVIDENCE_INCOMPLETE", Level: levelError,
			Message: "来源类目的 Ozon 映射缺少人工确认理由", Suggestion: "请重新逐级核对完整类目路径和属性模板，并补充映射确认理由。",
			TechnicalDetails: map[string]any{"sourceCategoryKey": sourceKey},
		})
	}
	if strings.TrimSpace(mapping.CategoryID) != strings.TrimSpace(cfg.CategoryID) {
		out = append(out, CheckItem{
			Group: "platform", Code: "OZON_CATEGORY_MAPPING_CONFLICT", Level: levelError,
			Message: "当前商品类目与已确认的来源类目映射冲突", Suggestion: "请改回已确认类目，或在类目映射中重新人工确认后保存。",
			TechnicalDetails: map[string]any{"configuredCategoryId": cfg.CategoryID, "confirmedCategoryId": mapping.CategoryID, "confirmedCategoryPath": mapping.CategoryPath},
		})
	}
	if strings.TrimSpace(mapping.SchemaHash) == "" || strings.TrimSpace(mapping.SchemaHash) != strings.TrimSpace(currentSchemaHash) {
		out = append(out, CheckItem{
			Group: "platform", Code: "OZON_CATEGORY_MAPPING_SCHEMA_CHANGED", Level: levelError,
			Message: "已确认的类目映射使用了旧版属性模板", Suggestion: "请刷新模板并重新确认来源类目映射。",
		})
	}
	return out
}

func ozonPublishOptionString(options map[string]any, key string) string {
	if options == nil {
		return ""
	}
	value, ok := options[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func checkOzonSKUImages(p product.Product, raw []byte) []CheckItem {
	view := product.ResolveOzonImageConfig(p, raw)
	out := make([]CheckItem, 0, view.ErrorCount+len(view.SKUs))
	appendIssue := func(issue product.OzonImageIssueDTO) {
		item := CheckItem{
			Group:      "image",
			Code:       issue.Code,
			Level:      levelError,
			Message:    issue.Message,
			Suggestion: issue.Suggestion,
		}
		if issue.SKUID != nil {
			item.RelatedResourceType = "product_sku"
			item.RelatedResourceID = issue.SKUID.String()
		}
		out = append(out, item)
	}
	for _, issue := range view.Issues {
		appendIssue(issue)
	}
	for _, sku := range view.SKUs {
		for _, issue := range sku.Issues {
			appendIssue(issue)
		}
		if len(sku.FinalImages) == 0 || len(sku.Issues) > 0 {
			continue
		}
		for _, check := range classifyImagePublicness(sku.FinalImages[0].URL, "ozon") {
			check.RelatedResourceType = "product_sku"
			check.RelatedResourceID = sku.SKUID.String()
			out = append(out, check)
		}
	}
	return out
}

func ozonDictionaryID(raw []byte) string {
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil {
		return ""
	}
	switch value := values["dictionary_id"].(type) {
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err == nil && parsed > 0 {
			return strconv.FormatInt(parsed, 10)
		}
	case float64:
		if value > 0 {
			return strconv.FormatInt(int64(value), 10)
		}
	}
	return ""
}

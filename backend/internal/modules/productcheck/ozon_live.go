package productcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	platformozon "github.com/trademind-ai/trademind/backend/internal/providers/platform/ozon"
	"gorm.io/gorm"
)

// ValidateOzonReadiness refreshes the selected category template through
// Ozon's read-only Seller API and then runs the normal cache-backed checks.
// It never calls product/import or any other Ozon mutation endpoint.
func (s *Service) ValidateOzonReadiness(ctx context.Context, tenantID int64, productID, shopID uuid.UUID) (*CheckProductReadinessResult, error) {
	if s == nil || s.DB == nil || s.Shops == nil {
		return nil, fmt.Errorf("product check unavailable")
	}
	var cfg product.ProductPlatformPublishConfig
	err := s.DB.WithContext(ctx).
		Joins("JOIN products ON products.id = product_platform_publish_configs.product_id AND products.deleted_at IS NULL").
		Where("product_platform_publish_configs.product_id = ? AND product_platform_publish_configs.platform = ? AND products.tenant_id = ?", productID, "ozon", tenantID).
		First(&cfg).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	configured := err == nil
	publishOptions := map[string]any{}
	if configured {
		if cfg.ShopID == nil || *cfg.ShopID != shopID {
			return nil, ozonConfigShopMismatchError(fmt.Errorf("saved Ozon configuration does not match shopId"))
		}
		if _, err := s.Shops.RefreshOzonCategoryAttributeTemplate(ctx, tenantID, cfg.CategoryID, shopID); err != nil {
			return nil, mapOzonReadinessError(err)
		}
		parts := strings.SplitN(cfg.CategoryID, ":", 2)
		if len(parts) == 2 {
			publishOptions["description_category_id"] = parts[0]
			publishOptions["type_id"] = parts[1]
		}
		publishOptions["platform_attributes"] = cfg.PlatformAttributes
	}

	result, err := s.CheckProductReadiness(ctx, CheckProductReadinessRequest{
		TenantID:       tenantID,
		ProductID:      productID,
		Platform:       "ozon",
		ShopID:         &shopID,
		Mode:           "live",
		PublishOptions: publishOptions,
	})
	if err != nil {
		return nil, err
	}
	if configured {
		checks, validateErr := s.validateOzonDictionarySelectionsLive(ctx, tenantID, shopID, cfg)
		if validateErr != nil {
			return nil, mapOzonReadinessError(validateErr)
		}
		result.Checks = append(result.Checks, checks...)
		recalculateOzonReadiness(result)
	}
	now := time.Now().UTC()
	result.CheckedAt = &now
	if configured {
		var attrs []shop.PlatformCategoryAttribute
		if queryErr := s.DB.WithContext(ctx).Where("platform = ? AND category_id = ?", "ozon", cfg.CategoryID).Order("attr_id ASC").Find(&attrs).Error; queryErr != nil {
			return nil, queryErr
		}
		result.SchemaHash = shop.OzonCategorySchemaHash(attrs)
		result.SchemaChanged = cfg.SchemaHash != "" && cfg.SchemaHash != result.SchemaHash
	}
	return result, nil
}

func (s *Service) validateOzonDictionarySelectionsLive(ctx context.Context, tenantID int64, shopID uuid.UUID, cfg product.ProductPlatformPublishConfig) ([]CheckItem, error) {
	parts := strings.SplitN(strings.TrimSpace(cfg.CategoryID), ":", 2)
	if len(parts) != 2 {
		return nil, invalidOzonConfigError("已保存的 Ozon 类目编号无效，请重新选择类目", fmt.Errorf("invalid Ozon composite category id"))
	}
	_, auth, err := s.Shops.PlainAuthForProviderCtx(ctx, tenantID, shopID)
	if err != nil {
		return nil, err
	}
	client, err := platformozon.NewClient(auth)
	if err != nil {
		return nil, err
	}
	var schema []shop.PlatformCategoryAttribute
	if err := s.DB.WithContext(ctx).Where("platform = ? AND category_id = ?", "ozon", cfg.CategoryID).Find(&schema).Error; err != nil {
		return nil, err
	}
	byID := make(map[string]shop.PlatformCategoryAttribute, len(schema))
	for _, attr := range schema {
		byID[attr.AttrID] = attr
	}
	values := map[string]any{}
	if len(cfg.PlatformAttributes) > 0 {
		if err := json.Unmarshal(cfg.PlatformAttributes, &values); err != nil {
			return []CheckItem{ozonLiveAttributeError("OZON_ATTRIBUTE_PAYLOAD_INVALID", "Ozon 商品属性配置不是有效对象", "请重新保存商品级 Ozon 属性。")}, nil
		}
	}
	out := make([]CheckItem, 0)
	for attrID, rawValue := range values {
		attr, exists := byID[attrID]
		if !exists {
			out = append(out, ozonLiveAttributeError("OZON_ATTRIBUTE_UNKNOWN", "Ozon 当前模板中不存在已保存属性："+attrID, "请重新确认商品级 Ozon 属性。"))
			continue
		}
		nested, ok := rawValue.(map[string]any)
		if !ok {
			out = append(out, ozonLiveAttributeError("OZON_ATTRIBUTE_PAYLOAD_INVALID", "Ozon 属性格式无效："+attr.Name, "请重新保存该属性。"))
			continue
		}
		text := strings.TrimSpace(toOzonCheckString(nested["value"]))
		selectedID := strings.TrimSpace(toOzonCheckString(nested["dictionaryValueId"]))
		if ozonDictionaryID(attr.Raw) == "" {
			if selectedID != "" {
				out = append(out, ozonLiveAttributeError("OZON_DICTIONARY_ID_UNEXPECTED", "非词典属性不能包含 dictionaryValueId："+attr.Name, "请重新保存该属性。"))
			}
			continue
		}
		if text == "" {
			continue
		}
		if parsed, parseErr := strconv.ParseInt(selectedID, 10, 64); parseErr != nil || parsed <= 0 {
			out = append(out, ozonLiveAttributeError("OZON_DICTIONARY_ID_MISSING", "Ozon 词典属性缺少有效值 ID："+attr.Name, "请重新选择该属性值。"))
			continue
		}
		matched, matchErr := client.ValidateDictionaryValue(ctx, parts[0], parts[1], attr.AttrID, selectedID, text)
		if matchErr != nil {
			return nil, mapOzonProviderError(matchErr)
		}
		if !matched {
			out = append(out, ozonLiveAttributeError("OZON_DICTIONARY_VALUE_CHANGED", "Ozon 词典值已变化或不属于该属性："+attr.Name, "请重新选择该属性值。"))
		}
	}
	return out, nil
}

func ozonLiveAttributeError(code, message, suggestion string) CheckItem {
	return CheckItem{Group: "platform", Code: code, Level: levelError, Message: message, Suggestion: suggestion}
}

func recalculateOzonReadiness(result *CheckProductReadinessResult) {
	if result == nil {
		return
	}
	result.ErrorCount = 0
	result.WarningCount = 0
	for _, check := range result.Checks {
		switch check.Level {
		case levelError:
			result.ErrorCount++
		case levelWarning:
			result.WarningCount++
		}
	}
	result.CanPublish = result.ErrorCount == 0
	switch {
	case result.ErrorCount > 0:
		result.Status, result.Result = statusBlocked, "failed"
	case result.WarningCount > 0:
		result.Status, result.Result = statusWarning, "warning"
	default:
		result.Status, result.Result = statusReady, "passed"
	}
	result.Score = readinessScore(result.ErrorCount, result.WarningCount)
}

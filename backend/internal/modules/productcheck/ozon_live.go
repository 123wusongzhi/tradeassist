package productcheck

import (
	"context"
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
	var prod product.Product
	if err := s.DB.WithContext(ctx).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, created_at ASC") }).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
		Where("tenant_id = ?", tenantID).First(&prod, "id = ?", productID).Error; err != nil {
		return nil, err
	}
	cfg, _, err := product.FindProductPlatformPublishConfig(ctx, s.DB, productID, "ozon", &shopID, true)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	configured := err == nil
	publishOptions := map[string]any{}
	contractCurrency := ""
	if configured {
		if cfg.ShopID != nil && *cfg.ShopID != shopID {
			return nil, ozonConfigShopMismatchError(fmt.Errorf("saved Ozon configuration does not match shopId"))
		}
		_, auth, authErr := s.Shops.PlainAuthForProviderCtx(ctx, tenantID, shopID)
		if authErr != nil {
			return nil, authErr
		}
		client, clientErr := platformozon.NewClient(auth)
		if clientErr != nil {
			return nil, clientErr
		}
		contractCurrency, err = client.SellerCurrency(ctx)
		if err != nil {
			return nil, mapOzonReadinessError(err)
		}
		if strings.TrimSpace(contractCurrency) != "" {
			publishOptions["currency_code"] = strings.TrimSpace(contractCurrency)
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
		checks, validateErr := s.validateOzonDictionarySelectionsLive(ctx, tenantID, shopID, *cfg)
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
		preset := map[string]string{}
		if s.Settings != nil {
			preset, err = s.Settings.PlainByGroup(ctx, 0, "platform_publish_ozon")
			if err != nil {
				return nil, err
			}
		}
		resolved := product.ResolveOzonListing(prod, cfg, preset, contractCurrency)
		if currencyCheck := ozonContractCurrencyCheck(resolved, contractCurrency); currencyCheck != nil {
			result.Checks = append(result.Checks, *currencyCheck)
			recalculateOzonReadiness(result)
		}
		resolved.CanSubmit = result.CanPublish
		result.ResolvedOzon = &resolved
	}
	return result, nil
}

func ozonContractCurrencyCheck(resolved product.OzonResolvedListingDTO, contractCurrency string) *CheckItem {
	contract := strings.ToUpper(strings.TrimSpace(contractCurrency))
	configured := strings.ToUpper(strings.TrimSpace(resolved.Currency.Value))
	if contract == "" || configured == "" || contract == configured {
		return nil
	}
	return &CheckItem{
		Group:      "pricing",
		Code:       "OZON_CURRENCY_CONTRACT_MISMATCH",
		Level:      levelError,
		Message:    fmt.Sprintf("当前 Ozon 币种 %s 与店铺合同币种 %s 不一致", configured, contract),
		Suggestion: fmt.Sprintf("请将币种改为 %s；Ozon 只接受合同或卖家后台配置的币种。", contract),
	}
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
	payload, decodeErr := product.DecodeOzonPlatformAttributes(cfg.PlatformAttributes)
	if decodeErr != nil {
		return []CheckItem{ozonLiveAttributeError("OZON_ATTRIBUTE_PAYLOAD_INVALID", "Ozon 商品属性配置不是有效对象", "请重新保存商品级 Ozon 属性。")}, nil
	}
	out := make([]CheckItem, 0)
	type dictionarySelectionKey struct {
		attrID, valueID, text string
	}
	validatedDictionarySelections := make(map[dictionarySelectionKey]bool)
	validateSelections := func(attrID string, selections []product.OzonAttributeSelection) error {
		attr, exists := byID[attrID]
		if !exists {
			out = append(out, ozonLiveAttributeError("OZON_ATTRIBUTE_UNKNOWN", "Ozon 当前模板中不存在已保存属性："+attrID, "请重新确认商品级 Ozon 属性。"))
			return nil
		}
		for _, selection := range selections {
			text := strings.TrimSpace(selection.Value)
			selectedID := strings.TrimSpace(selection.DictionaryValueID)
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
			key := dictionarySelectionKey{attrID: attr.AttrID, valueID: selectedID, text: text}
			matched, alreadyValidated := validatedDictionarySelections[key]
			if !alreadyValidated {
				var matchErr error
				matched, matchErr = client.ValidateDictionaryValue(ctx, parts[0], parts[1], attr.AttrID, selectedID, text)
				if matchErr != nil {
					// A live provider/auth failure invalidates the whole readiness
					// result; partial dictionary checks must not be presented as a
					// trustworthy publish decision.
					return mapOzonProviderError(matchErr)
				}
				validatedDictionarySelections[key] = matched
			}
			if !matched {
				out = append(out, ozonLiveAttributeError("OZON_DICTIONARY_VALUE_CHANGED", "Ozon 词典值已变化或不属于该属性："+attr.Name, "请重新选择该属性值。"))
			}
		}
		return nil
	}
	for attrID, selections := range payload.Attributes {
		if err := validateSelections(attrID, selections); err != nil {
			return nil, err
		}
	}
	for _, group := range payload.ComplexGroups {
		for attrID, selections := range group.Attributes {
			if err := validateSelections(attrID, selections); err != nil {
				return nil, err
			}
		}
	}
	for skuID, attributes := range payload.SKUAttributeOverrides {
		for attrID, selections := range attributes {
			before := len(out)
			if err := validateSelections(attrID, selections); err != nil {
				return nil, err
			}
			for index := before; index < len(out); index++ {
				out[index].RelatedResourceType = "product_sku"
				out[index].RelatedResourceID = skuID
			}
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

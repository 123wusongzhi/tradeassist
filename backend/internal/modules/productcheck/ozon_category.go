package productcheck

import (
	"context"
	"encoding/json"
	"fmt"
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
func (s *Service) checkOzonListingConfig(ctx context.Context, p product.Product, requestedShopID *uuid.UUID) []CheckItem {
	var cfg product.ProductPlatformPublishConfig
	if err := s.DB.WithContext(ctx).Where("product_id = ? AND platform = ?", p.ID, "ozon").First(&cfg).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
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
	vals := map[string]any{}
	_ = json.Unmarshal(cfg.PlatformAttributes, &vals)
	for _, a := range attrs {
		if !a.Required {
			continue
		}
		v, ok := vals[a.AttrID]
		if !ok {
			v, ok = vals[a.Name]
		}
		if !ok || !ozonAttributeValuePresent(v, a) {
			out = append(out, CheckItem{Group: "platform", Code: "OZON_REQUIRED_ATTR_MISSING", Level: levelError, Message: "Ozon 必填属性未补齐：" + strings.TrimSpace(a.Name), Suggestion: "请补全商品级 Ozon 属性。"})
		}
	}
	return out
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

func ozonAttributeValuePresent(value any, attr shop.PlatformCategoryAttribute) bool {
	nested, ok := value.(map[string]any)
	if !ok {
		return false
	}
	text := strings.TrimSpace(toOzonCheckString(nested["value"]))
	if text == "" {
		return false
	}
	var raw map[string]any
	_ = json.Unmarshal(attr.Raw, &raw)
	if ozonDictionaryID(attr.Raw) != "" {
		return strings.TrimSpace(toOzonCheckString(nested["dictionaryValueId"])) != ""
	}
	return true
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

func toOzonCheckString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

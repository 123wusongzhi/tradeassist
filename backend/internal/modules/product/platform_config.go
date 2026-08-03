package product

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) GetPlatformPublishConfig(c *gin.Context, productID uuid.UUID, platform string) (*PlatformPublishConfigDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	plat := strings.TrimSpace(strings.ToLower(platform))
	preloads := []string{}
	if plat == "ozon" {
		preloads = []string{"Images", "SKUs"}
	}
	productRow, err := s.findTenantProduct(c, productID, preloads...)
	if err != nil {
		return nil, err
	}
	var row ProductPlatformPublishConfig
	if err := s.DB.WithContext(c.Request.Context()).
		Where("product_id = ? AND platform = ?", productID, plat).
		First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound && plat == "ozon" {
			images := ResolveOzonImageConfig(*productRow, nil)
			return &PlatformPublishConfigDTO{ProductID: productID, Platform: plat, PlatformAttributes: json.RawMessage(`{}`), OzonImages: &images}, nil
		}
		return nil, err
	}
	out := platformConfigDTO(row)
	if plat == "ozon" {
		images := ResolveOzonImageConfig(*productRow, row.MappedImages)
		out.OzonImages = &images
	}
	return out, nil
}

func (s *Service) PutPlatformPublishConfig(c *gin.Context, productID uuid.UUID, platform string, body PlatformPublishConfigBody, adminID *uuid.UUID) (*PlatformPublishConfigDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	plat := strings.TrimSpace(strings.ToLower(platform))
	if plat == "" {
		return nil, newPlatformConfigError("platform required")
	}
	if plat != "douyin_shop" && plat != "ozon" {
		return nil, newPlatformConfigError("platform config is currently supported for douyin_shop or ozon only")
	}
	preloads := []string{}
	if plat == "ozon" {
		preloads = []string{"Images", "SKUs"}
	}
	productRow, err := s.findTenantProduct(c, productID, preloads...)
	if err != nil {
		return nil, err
	}
	if err := adminperm.EnsureProductOperate(c, s.DB, productID); err != nil {
		return nil, err
	}
	var shopID *uuid.UUID
	if raw := strings.TrimSpace(body.ShopID); raw != "" {
		u, err := uuid.Parse(raw)
		if err != nil || u == uuid.Nil {
			return nil, newPlatformConfigError("invalid shopId")
		}
		shopID = &u
		if err := s.requirePlatformShopOperate(c, u, plat); err != nil {
			return nil, err
		}
	}
	if plat == "ozon" && shopID == nil {
		return nil, newOzonPlatformConfigError("Ozon 配置必须选择已授权店铺")
	}
	cid := strings.TrimSpace(body.CategoryID)
	var selectedCategory *shop.PlatformCategory
	if cid != "" {
		var cat shop.PlatformCategory
		if err := s.DB.WithContext(c.Request.Context()).Where("platform = ? AND category_id = ?", plat, cid).First(&cat).Error; err != nil {
			return nil, err
		}
		if !cat.IsLeaf {
			return nil, newPlatformConfigError("请选择平台叶子类目")
		}
		if plat == "ozon" && cat.Status != "active" {
			return nil, newOzonPlatformConfigError("请选择有效的 Ozon 叶子类目")
		}
		selectedCategory = &cat
	}
	if plat == "ozon" && selectedCategory == nil {
		return nil, newOzonPlatformConfigError("Ozon 配置必须选择有效叶子类目")
	}
	attrs := datatypes.JSON([]byte("{}"))
	if len(body.PlatformAttributes) > 0 && string(body.PlatformAttributes) != "null" {
		var tmp map[string]any
		if err := json.Unmarshal(body.PlatformAttributes, &tmp); err != nil || tmp == nil {
			return nil, newPlatformConfigError("platformAttributes must be valid JSON")
		}
		attrs = datatypes.JSON(body.PlatformAttributes)
	}
	if plat == "ozon" {
		if err := s.validateOzonPlatformAttributes(c.Request.Context(), cid, attrs); err != nil {
			return nil, err
		}
	}
	sourceKey := normalizeSourceCategory(firstNonEmptyProduct(body.SourceCategoryKey, body.SourceCategoryName))
	sourceName := strings.TrimSpace(body.SourceCategoryName)
	if plat == "ozon" && sourceKey == "" && productRow != nil {
		sourceKey, sourceName = SourceCategoryFromRaw(json.RawMessage(productRow.RawData))
	}
	row := ProductPlatformPublishConfig{
		ProductID:          productID,
		Platform:           plat,
		ShopID:             shopID,
		CategoryID:         cid,
		CategoryPath:       strings.TrimSpace(body.CategoryPath),
		PlatformAttributes: attrs,
		SourceCategoryKey:  sourceKey,
		SourceCategoryName: sourceName,
	}
	updates := []string{
		"shop_id", "category_id", "category_path", "platform_attributes", "source_category_key", "source_category_name", "schema_hash", "schema_confirmed_at", "updated_at",
	}
	if plat == "ozon" {
		row.CategoryPath = shop.CanonicalOzonCategoryPath(c.Request.Context(), s.DB, *selectedCategory)
		hash, err := s.ozonSchemaHash(c.Request.Context(), cid)
		if err != nil {
			return nil, err
		}
		row.SchemaHash = hash
		now := time.Now().UTC()
		row.SchemaConfirmedAt = &now
		if body.OzonImages != nil {
			rawImages, err := MarshalOzonImageConfig(*productRow, *body.OzonImages)
			if err != nil {
				return nil, err
			}
			row.MappedImages = datatypes.JSON(rawImages)
			updates = append(updates, "mapped_images")
		}
	}
	var saved ProductPlatformPublishConfig
	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "product_id"}, {Name: "platform"}},
			DoUpdates: clause.AssignmentColumns(updates),
		}).Create(&row).Error; err != nil {
			return err
		}
		// Always read into a fresh value. Reusing row would make GORM append the
		// newly generated insert UUID after an ON CONFLICT update and query the
		// wrong primary key.
		return tx.Where("product_id = ? AND platform = ?", productID, plat).First(&saved).Error
	}); err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		if cid != "" {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      plat + ".category.select",
				Resource:    "product",
				ResourceID:  productID.String(),
				Status:      "success",
				Message:     "categoryId=" + cid,
			})
		}
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      plat + ".category.attr.update",
			Resource:    "product",
			ResourceID:  productID.String(),
			Status:      "success",
			Message:     "platformAttributes updated",
		})
		if plat == "ozon" && body.OzonImages != nil {
			fallbackCount := 0
			for _, selection := range body.OzonImages.SKUSelections {
				if selection.FallbackMainImageID != nil {
					fallbackCount++
				}
			}
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "ozon.images.update",
				Resource:    "product",
				ResourceID:  productID.String(),
				Status:      "success",
				Message:     fmt.Sprintf("skuSelections=%d fallbackMainImages=%d", len(body.OzonImages.SKUSelections), fallbackCount),
			})
		}
	}
	out := platformConfigDTO(saved)
	if plat == "ozon" {
		images := ResolveOzonImageConfig(*productRow, saved.MappedImages)
		out.OzonImages = &images
	}
	return out, nil
}

func (s *Service) requirePlatformShopOperate(c *gin.Context, shopID uuid.UUID, platform string) error {
	if shopID == uuid.Nil {
		return newPlatformConfigError("invalid shopId")
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var row shop.Shop
	if err := s.DB.WithContext(c.Request.Context()).Where("id = ? AND tenant_id = ? AND platform = ? AND status = ? AND auth_status = ?", shopID, tenantID, platform, shop.StatusActive, shop.AuthAuthorized).First(&row).Error; err != nil {
		return err
	}
	return adminperm.EnsureStoreOperate(c, s.DB, shopID)
}

func (s *Service) validateOzonPlatformAttributes(ctx context.Context, categoryID string, raw datatypes.JSON) error {
	var schema []shop.PlatformCategoryAttribute
	if err := s.DB.WithContext(ctx).Where("platform = ? AND category_id = ?", "ozon", categoryID).Find(&schema).Error; err != nil {
		return err
	}
	if len(schema) == 0 {
		return newOzonPlatformConfigError("Ozon 类目属性模板尚未同步")
	}
	values := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &values); err != nil {
			return newOzonPlatformConfigError("platformAttributes must be valid JSON")
		}
	}
	byID := make(map[string]shop.PlatformCategoryAttribute, len(schema))
	for _, attr := range schema {
		byID[attr.AttrID] = attr
	}
	for attrID, value := range values {
		attr, ok := byID[attrID]
		if !ok {
			return newOzonPlatformConfigError(fmt.Sprintf("Ozon 属性模板中不存在属性 %s", attrID))
		}
		nested, ok := value.(map[string]any)
		if !ok {
			return newOzonPlatformConfigError(fmt.Sprintf("Ozon 属性 %s 必须使用 {value,dictionaryValueId} 结构", attr.Name))
		}
		text := ""
		if rawValue, exists := nested["value"]; exists && rawValue != nil {
			text = strings.TrimSpace(fmt.Sprint(rawValue))
		}
		if attr.Required && text == "" {
			return newOzonPlatformConfigError(fmt.Sprintf("Ozon 必填属性未填写：%s", attr.Name))
		}
		if dictionaryIDFromProductAttribute(attr) != "" {
			dictID := productAttributeDictionaryValueID(nested)
			if text != "" && dictID == "" {
				return newOzonPlatformConfigError(fmt.Sprintf("Ozon 词典属性缺少 dictionaryValueId：%s", attr.Name))
			}
			if dictID != "" && !cachedOzonDictionaryValueMatches(attr.Options, dictID, text) {
				return newOzonPlatformConfigError(fmt.Sprintf("Ozon 词典值与属性不匹配：%s", attr.Name))
			}
		} else if rawID, exists := nested["dictionaryValueId"]; exists && rawID != nil && strings.TrimSpace(fmt.Sprint(rawID)) != "" {
			return newOzonPlatformConfigError(fmt.Sprintf("Ozon 非词典属性不能包含 dictionaryValueId：%s", attr.Name))
		}
	}
	for _, attr := range schema {
		if !attr.Required {
			continue
		}
		value, ok := values[attr.AttrID]
		if !ok {
			return newOzonPlatformConfigError(fmt.Sprintf("Ozon 必填属性未填写：%s", attr.Name))
		}
		nested, ok := value.(map[string]any)
		if !ok || nested["value"] == nil || strings.TrimSpace(fmt.Sprint(nested["value"])) == "" {
			return newOzonPlatformConfigError(fmt.Sprintf("Ozon 必填属性未填写：%s", attr.Name))
		}
	}
	return nil
}

func productAttributeDictionaryValueID(value map[string]any) string {
	raw, ok := value["dictionaryValueId"]
	if !ok || raw == nil {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil && parsed > 0 {
			return strconv.FormatInt(parsed, 10)
		}
	case float64:
		if typed > 0 {
			return strconv.FormatInt(int64(typed), 10)
		}
	}
	return ""
}

func dictionaryIDFromProductAttribute(attr shop.PlatformCategoryAttribute) string {
	var raw map[string]any
	if json.Unmarshal(attr.Raw, &raw) != nil {
		return ""
	}
	value, ok := raw["dictionary_id"]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		parsed, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil || parsed <= 0 {
			return ""
		}
		return strconv.FormatInt(parsed, 10)
	case float64:
		if typed <= 0 {
			return ""
		}
		return strconv.FormatInt(int64(typed), 10)
	default:
		return ""
	}
}

func cachedOzonDictionaryValueMatches(raw datatypes.JSON, id, value string) bool {
	if len(raw) == 0 {
		// Some very large dictionaries are intentionally not prefetched; the
		// provider performs the authoritative live check before import.
		return true
	}
	var options []struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}
	if json.Unmarshal(raw, &options) != nil {
		return false
	}
	for _, option := range options {
		if option.ID == id {
			return strings.TrimSpace(option.Value) == strings.TrimSpace(value)
		}
	}
	// The UI cache is deliberately bounded. Absence is inconclusive and is
	// checked authoritatively by live preflight and again by the publish worker.
	return true
}

func firstNonEmptyProduct(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *Service) requireDouyinShopOperate(c *gin.Context, shopID uuid.UUID) error {
	return s.requirePlatformShopOperate(c, shopID, "douyin_shop")
}

func (s *Service) ozonSchemaHash(ctx context.Context, categoryID string) (string, error) {
	if strings.TrimSpace(categoryID) == "" {
		return "", nil
	}
	var attrs []shop.PlatformCategoryAttribute
	if err := s.DB.WithContext(ctx).Where("platform = ? AND category_id = ?", "ozon", categoryID).Order("attr_id ASC").Find(&attrs).Error; err != nil {
		return "", err
	}
	return shop.OzonCategorySchemaHash(attrs), nil
}

func platformConfigDTO(row ProductPlatformPublishConfig) *PlatformPublishConfigDTO {
	var mapping *DouyinDraftMapping
	if strings.EqualFold(strings.TrimSpace(row.Platform), "douyin_shop") {
		mapping = DouyinDraftMappingFromConfig(row)
	}
	return &PlatformPublishConfigDTO{
		ID:                 &row.ID,
		ProductID:          row.ProductID,
		Platform:           row.Platform,
		ShopID:             row.ShopID,
		CategoryID:         row.CategoryID,
		CategoryPath:       row.CategoryPath,
		PlatformAttributes: json.RawMessage(row.PlatformAttributes),
		Mapping:            mapping,
		LastMappedAt:       row.LastMappedAt,
		SourceCategoryKey:  row.SourceCategoryKey,
		SourceCategoryName: row.SourceCategoryName,
		SchemaHash:         row.SchemaHash,
		SchemaConfirmedAt:  row.SchemaConfirmedAt,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

func isRecordNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	return s.GetPlatformPublishConfigForShop(c, productID, platform, "")
}

// GetPlatformPublishConfigForShop resolves an Ozon configuration in the
// selected shop scope. The empty shop form is kept for older API clients and
// reads only the historical legacy scope; it never selects an arbitrary shop.
func (s *Service) GetPlatformPublishConfigForShop(c *gin.Context, productID uuid.UUID, platform, rawShopID string) (*PlatformPublishConfigDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	plat := strings.TrimSpace(strings.ToLower(platform))
	var requestedShopID *uuid.UUID
	if raw := strings.TrimSpace(rawShopID); raw != "" {
		parsed, parseErr := uuid.Parse(raw)
		if parseErr != nil || parsed == uuid.Nil {
			return nil, newPlatformConfigError("invalid shopId")
		}
		requestedShopID = &parsed
		if err := adminperm.EnsureStoreVisible(c, s.DB, requestedShopID); err != nil {
			return nil, err
		}
	}
	preloads := []string{}
	if plat == "ozon" {
		preloads = []string{"Images", "SKUs"}
	}
	productRow, err := s.findTenantProduct(c, productID, preloads...)
	if err != nil {
		return nil, err
	}
	row, legacyFallback, err := FindProductPlatformPublishConfig(c.Request.Context(), s.DB, productID, plat, requestedShopID, true)
	if err != nil {
		if err == gorm.ErrRecordNotFound && plat == "ozon" {
			images := ResolveOzonImageConfig(*productRow, nil)
			emptyRow := &ProductPlatformPublishConfig{ProductID: productID, Platform: plat, ShopID: requestedShopID}
			out := &PlatformPublishConfigDTO{ProductID: productID, Platform: plat, ShopID: requestedShopID, PlatformAttributes: json.RawMessage(`{}`), OzonImages: &images}
			if hydrateErr := s.hydrateOzonListingDTO(c.Request.Context(), *productRow, emptyRow, out); hydrateErr != nil {
				return nil, hydrateErr
			}
			return out, nil
		}
		return nil, err
	}
	out := platformConfigDTO(*row)
	out.LegacyFallback = legacyFallback
	if plat == "ozon" {
		images := ResolveOzonImageConfig(*productRow, row.MappedImages)
		out.OzonImages = &images
		if err := s.hydrateOzonListingDTO(c.Request.Context(), *productRow, row, out); err != nil {
			return nil, err
		}
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
	// Ozon rows are isolated by the selected shop. An operator with operate
	// permission for that shop may edit only that row; requiring operate access
	// to every other shop linked to the same product would defeat store scoping.
	// Legacy singleton platforms retain the stricter whole-product guard.
	if plat != "ozon" {
		if err := adminperm.EnsureProductOperate(c, s.DB, productID); err != nil {
			return nil, err
		}
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
		canonical, err := s.canonicalizeOzonPlatformAttributes(c.Request.Context(), cid, attrs, false)
		if err != nil {
			return nil, err
		}
		attrs = canonical
	}
	sourceKey := normalizeSourceCategory(firstNonEmptyProduct(body.SourceCategoryKey, body.SourceCategoryName))
	sourceName := strings.TrimSpace(body.SourceCategoryName)
	if plat == "ozon" && sourceKey == "" && productRow != nil {
		sourceKey, sourceName = SourceCategoryFromRaw(json.RawMessage(productRow.RawData))
	}
	row := ProductPlatformPublishConfig{
		ProductID:          productID,
		Platform:           plat,
		ConfigScopeKey:     PlatformConfigScopeKey(plat, shopID),
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
		if body.OzonListing != nil {
			rawListing, err := MarshalOzonListingConfig(*productRow, *body.OzonListing)
			if err != nil {
				return nil, newOzonPlatformConfigError(err.Error())
			}
			row.ListingConfig = datatypes.JSON(rawListing)
			updates = append(updates, "listing_config")
		}
	}
	var saved ProductPlatformPublishConfig
	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "product_id"}, {Name: "platform"}, {Name: "config_scope_key"}},
			DoUpdates: clause.AssignmentColumns(updates),
		}).Create(&row).Error; err != nil {
			return err
		}
		// Always read into a fresh value. Reusing row would make GORM append the
		// newly generated insert UUID after an ON CONFLICT update and query the
		// wrong primary key.
		return tx.Where("product_id = ? AND platform = ? AND config_scope_key = ?", productID, plat, row.ConfigScopeKey).First(&saved).Error
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
		if err := s.hydrateOzonListingDTO(c.Request.Context(), *productRow, &saved, out); err != nil {
			return nil, err
		}
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

func (s *Service) canonicalizeOzonPlatformAttributes(ctx context.Context, categoryID string, raw datatypes.JSON, requireComplete bool) (datatypes.JSON, error) {
	var schema []shop.PlatformCategoryAttribute
	if err := s.DB.WithContext(ctx).Where("platform = ? AND category_id = ?", "ozon", categoryID).Find(&schema).Error; err != nil {
		return nil, err
	}
	if len(schema) == 0 {
		return nil, newOzonPlatformConfigError("Ozon 类目属性模板尚未同步")
	}
	canonical, err := CanonicalOzonPlatformAttributes(schema, raw, requireComplete)
	if err != nil {
		return nil, newOzonPlatformConfigError(err.Error())
	}
	return canonical, nil
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

const PlatformConfigLegacyScope = "legacy"

// PlatformConfigScopeKey keeps legacy platform configurations on their old
// product+platform scope while Ozon is isolated per selected shop.
func PlatformConfigScopeKey(platform string, shopID *uuid.UUID) string {
	if strings.EqualFold(strings.TrimSpace(platform), "ozon") && shopID != nil && *shopID != uuid.Nil {
		return shopID.String()
	}
	return PlatformConfigLegacyScope
}

// FindProductPlatformPublishConfig is the shared store-scope lookup used by
// config readback, live preflight and task creation. A legacy Ozon row without
// a shop may be used only as a read-only fallback and is materialized into the
// selected shop scope on the next save.
func FindProductPlatformPublishConfig(ctx context.Context, db *gorm.DB, productID uuid.UUID, platform string, shopID *uuid.UUID, allowLegacy bool) (*ProductPlatformPublishConfig, bool, error) {
	if db == nil {
		return nil, false, fmt.Errorf("product platform config: no db")
	}
	plat := strings.ToLower(strings.TrimSpace(platform))
	query := db.WithContext(ctx).Where("product_id = ? AND platform = ?", productID, plat)
	if shopID == nil || *shopID == uuid.Nil {
		var row ProductPlatformPublishConfig
		if err := query.Where("config_scope_key = ?", PlatformConfigLegacyScope).Order("updated_at DESC").First(&row).Error; err != nil {
			return nil, false, err
		}
		return &row, false, nil
	}

	scopeKey := PlatformConfigScopeKey(plat, shopID)
	var exact ProductPlatformPublishConfig
	if err := query.Where("config_scope_key = ?", scopeKey).First(&exact).Error; err == nil {
		return &exact, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	if !allowLegacy || !strings.EqualFold(plat, "ozon") {
		return nil, false, gorm.ErrRecordNotFound
	}
	var legacy ProductPlatformPublishConfig
	if err := db.WithContext(ctx).
		Where("product_id = ? AND platform = ? AND config_scope_key = ? AND (shop_id IS NULL OR shop_id = ?)", productID, plat, PlatformConfigLegacyScope, *shopID).
		Order("updated_at DESC").First(&legacy).Error; err != nil {
		return nil, false, err
	}
	return &legacy, true, nil
}

func (s *Service) hydrateOzonListingDTO(ctx context.Context, productRow Product, row *ProductPlatformPublishConfig, out *PlatformPublishConfigDTO) error {
	if out == nil {
		return nil
	}
	preset := map[string]string{}
	if s != nil && s.Settings != nil {
		values, err := s.Settings.PlainByGroup(ctx, 0, "platform_publish_ozon")
		if err != nil {
			return err
		}
		preset = values
	}
	listing := OzonListingConfigInput{Version: OzonListingConfigVersion, SKUPriceOverrides: map[string]float64{}}
	if row != nil {
		decoded, _, err := DecodeOzonListingConfig(row.ListingConfig)
		if err != nil {
			return fmt.Errorf("decode saved Ozon listing configuration: %w", err)
		}
		listing = decoded
	}
	out.OzonListing = &listing
	preview := ResolveOzonListing(productRow, row, preset, "")
	out.OzonPreview = &preview
	return nil
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

package productpublish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OzonCategoryGroupsCheckRequest struct {
	ProductIDs []string `json:"productIds"`
	ShopID     string   `json:"shopId"`
}
type OzonCategoryGroup struct {
	Key                     string                       `json:"key"`
	SourceCategoryKey       string                       `json:"sourceCategoryKey"`
	SourceCategoryName      string                       `json:"sourceCategoryName"`
	ProductIDs              []string                     `json:"productIds"`
	Titles                  []string                     `json:"titles"`
	Count                   int                          `json:"count"`
	Status                  string                       `json:"status"`
	StatusLabel             string                       `json:"statusLabel,omitempty"`
	CategoryID              string                       `json:"categoryId,omitempty"`
	RecommendedCategoryID   string                       `json:"recommendedCategoryId,omitempty"`
	RecommendedCategoryPath string                       `json:"recommendedCategoryPath,omitempty"`
	Candidate               *shop.OzonCategoryMappingDTO `json:"candidate,omitempty"`
	Issues                  []OzonCategoryGroupIssue     `json:"issues,omitempty"`
	Products                []OzonCategoryProductResult  `json:"products,omitempty"`
}

type OzonCategoryGroupIssue struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type OzonCategoryProductResult struct {
	ProductID string `json:"productId"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}
type OzonCategoryGroupsConfirmRequest struct {
	ShopID       string                     `json:"shopId"`
	Groups       []OzonCategoryGroupConfirm `json:"groups"`
	SaveMappings bool                       `json:"saveMappings"`
}
type OzonCategoryGroupConfirm struct {
	SourceCategoryKey  string   `json:"sourceCategoryKey"`
	SourceCategoryName string   `json:"sourceCategoryName"`
	ProductIDs         []string `json:"productIds"`
	CategoryID         string   `json:"categoryId"`
	CategoryPath       string   `json:"categoryPath"`
}

func (s *Service) CheckOzonCategoryGroups(ctx context.Context, tenantID int64, req OzonCategoryGroupsCheckRequest) ([]OzonCategoryGroup, error) {
	ids, err := s.parseBatchProductIDs(req.ProductIDs)
	if err != nil {
		return nil, err
	}
	shopID, err := s.validateOzonCategoryGroupShop(ctx, tenantID, req.ShopID)
	if err != nil {
		return nil, err
	}
	var products []product.Product
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, ids).Find(&products).Error; err != nil {
		return nil, err
	}
	if len(products) != len(ids) {
		return nil, gorm.ErrRecordNotFound
	}
	by := map[string]*OzonCategoryGroup{}
	for _, p := range products {
		key, name := product.SourceCategoryFromRaw(json.RawMessage(p.RawData))
		if key == "" {
			key = "uncategorized"
			name = "未识别本地类目"
		}
		g := by[key]
		if g == nil {
			g = &OzonCategoryGroup{Key: key, SourceCategoryKey: key, SourceCategoryName: name, Status: "needs_work", StatusLabel: "需要确认"}
			by[key] = g
		}
		g.ProductIDs = append(g.ProductIDs, p.ID.String())
		g.Titles = append(g.Titles, p.Title)
		g.Count++
	}
	out := make([]OzonCategoryGroup, 0, len(by))
	for _, g := range by {
		if g.SourceCategoryKey == "uncategorized" {
			g.Status = "skipped"
			g.StatusLabel = "无法识别本地类目"
			g.Issues = append(g.Issues, OzonCategoryGroupIssue{Code: "SOURCE_CATEGORY_MISSING", Message: "商品缺少可识别的本地类目，请先在商品配置中补充。"})
			out = append(out, *g)
			continue
		}
		var mapping shop.OzonCategoryMapping
		q := s.DB.WithContext(ctx).Where("tenant_id = ? AND source_category_key = ? AND status = ?", tenantID, g.SourceCategoryKey, shop.OzonMappingActive)
		q = q.Where("shop_id IS NULL OR shop_id = ?", shopID).
			Clauses(clause.OrderBy{Expression: clause.Expr{SQL: "CASE WHEN shop_id = ? THEN 0 ELSE 1 END", Vars: []any{shopID}, WithoutParentheses: true}})
		mappingErr := q.First(&mapping).Error
		if mappingErr != nil && !errors.Is(mappingErr, gorm.ErrRecordNotFound) {
			return nil, mappingErr
		}
		if mappingErr == nil && s.isActiveOzonLeaf(ctx, mapping.CategoryID) {
			g.Status = "ready"
			g.StatusLabel = "已有已确认映射"
			g.CategoryID = mapping.CategoryID
			g.RecommendedCategoryID = mapping.CategoryID
			g.RecommendedCategoryPath = mapping.CategoryPath
		} else if s.Shops != nil {
			candidate, e := s.Shops.RecommendOzonCategoryMapping(ctx, tenantID, shop.RecommendOzonCategoryMappingBody{ShopID: req.ShopID, SourceCategoryKey: g.SourceCategoryKey, SourceCategoryName: g.SourceCategoryName})
			if e == nil && candidate != nil {
				g.Status = "needs_work"
				g.StatusLabel = "推荐候选待确认"
				g.Candidate = candidate
				g.RecommendedCategoryID = candidate.CategoryID
				g.RecommendedCategoryPath = candidate.CategoryPath
			} else {
				g.Issues = append(g.Issues, OzonCategoryGroupIssue{Code: "CATEGORY_RECOMMENDATION_UNAVAILABLE", Message: "没有可用的 Ozon 叶类目候选。"})
			}
		}
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SourceCategoryKey < out[j].SourceCategoryKey })
	return out, nil
}

func (s *Service) ConfirmOzonCategoryGroups(ctx context.Context, tenantID int64, req OzonCategoryGroupsConfirmRequest, adminID *uuid.UUID) ([]OzonCategoryGroup, error) {
	shopID, err := s.validateOzonCategoryGroupShop(ctx, tenantID, req.ShopID)
	if err != nil {
		return nil, err
	}
	if len(req.Groups) == 0 {
		return nil, fmt.Errorf("groups required")
	}
	groupProductIDs, err := s.validateOzonCategoryGroupProducts(req.Groups)
	if err != nil {
		return nil, err
	}
	out := make([]OzonCategoryGroup, 0, len(req.Groups))
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seenSourceKeys := map[string]struct{}{}
		for groupIndex, g := range req.Groups {
			if strings.TrimSpace(g.CategoryID) == "" {
				return fmt.Errorf("categoryId is required")
			}
			var cat shop.PlatformCategory
			if err := tx.Where("platform = ? AND category_id = ? AND is_leaf = ? AND status = ?", "ozon", g.CategoryID, true, "active").First(&cat).Error; err != nil {
				return err
			}
			var attrs []shop.PlatformCategoryAttribute
			if err := tx.Where("platform = ? AND category_id = ?", "ozon", g.CategoryID).Order("attr_id ASC").Find(&attrs).Error; err != nil {
				return err
			}
			if len(attrs) == 0 {
				return fmt.Errorf("Ozon 类目属性模板尚未同步")
			}
			categoryPath := shop.CanonicalOzonCategoryPath(ctx, tx, cat)
			hash := shop.OzonCategorySchemaHash(attrs)
			ids := groupProductIDs[groupIndex]
			var owned []product.Product
			if err := tx.Where("tenant_id = ? AND id IN ?", tenantID, ids).Find(&owned).Error; err != nil {
				return err
			}
			if len(owned) != len(ids) {
				return gorm.ErrRecordNotFound
			}
			derivedKey := ""
			derivedName := ""
			for _, p := range owned {
				key, name := product.SourceCategoryFromRaw(json.RawMessage(p.RawData))
				if key == "" {
					return fmt.Errorf("product %s has no source category", p.ID)
				}
				if derivedKey == "" {
					derivedKey, derivedName = key, name
				} else if derivedKey != key {
					return fmt.Errorf("group contains products from different source categories")
				}
			}
			if supplied := strings.TrimSpace(g.SourceCategoryKey); supplied != "" && supplied != derivedKey {
				return fmt.Errorf("sourceCategoryKey does not match selected products")
			}
			if _, exists := seenSourceKeys[derivedKey]; exists {
				return fmt.Errorf("source category %s appears in more than one group", derivedKey)
			}
			seenSourceKeys[derivedKey] = struct{}{}
			groupOut := OzonCategoryGroup{Key: derivedKey, SourceCategoryKey: derivedKey, SourceCategoryName: derivedName, ProductIDs: g.ProductIDs, Count: len(g.ProductIDs), Status: "ready", StatusLabel: "本地配置已确认", CategoryID: g.CategoryID, RecommendedCategoryID: g.CategoryID, RecommendedCategoryPath: categoryPath}
			for _, p := range owned {
				pid := p.ID
				var old product.ProductPlatformPublishConfig
				if err := tx.Where("product_id = ? AND platform = ?", pid, "ozon").First(&old).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				attrsJSON := old.PlatformAttributes
				if old.CategoryID != g.CategoryID || len(attrsJSON) == 0 {
					attrsJSON = datatypes.JSON([]byte("{}"))
				}
				now := time.Now().UTC()
				row := product.ProductPlatformPublishConfig{ProductID: pid, Platform: "ozon", ShopID: &shopID, CategoryID: g.CategoryID, CategoryPath: categoryPath, PlatformAttributes: attrsJSON, SourceCategoryKey: derivedKey, SourceCategoryName: derivedName, SchemaHash: hash, SchemaConfirmedAt: &now}
				if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "product_id"}, {Name: "platform"}}, DoUpdates: clause.AssignmentColumns([]string{"shop_id", "category_id", "category_path", "platform_attributes", "source_category_key", "source_category_name", "schema_hash", "schema_confirmed_at", "updated_at"})}).Create(&row).Error; err != nil {
					return err
				}
				productStatus, productMessage := ozonConfiguredProductStatus(attrs, attrsJSON)
				if productStatus != "ready" {
					groupOut.Status = "needs_work"
					groupOut.StatusLabel = "类目已确认，属性待补充"
				}
				groupOut.Products = append(groupOut.Products, OzonCategoryProductResult{ProductID: pid.String(), Status: productStatus, Message: productMessage})
			}
			if req.SaveMappings {
				now := time.Now().UTC()
				mapping := shop.OzonCategoryMapping{TenantID: tenantID, ShopID: &shopID, ScopeKey: shopID.String(), SourceCategoryKey: derivedKey, SourceCategoryName: derivedName, CategoryID: g.CategoryID, CategoryPath: categoryPath, Status: shop.OzonMappingActive, SchemaHash: hash, ConfirmedAt: &now, ConfirmedBy: adminID}
				if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "scope_key"}, {Name: "source_category_key"}}, DoUpdates: clause.AssignmentColumns([]string{"shop_id", "source_category_name", "category_id", "category_path", "status", "schema_hash", "confirmed_at", "confirmed_by", "updated_at"})}).Create(&mapping).Error; err != nil {
					return err
				}
			}
			out = append(out, groupOut)
		}
		return nil
	})
	return out, err
}

func (s *Service) validateOzonCategoryGroupProducts(groups []OzonCategoryGroupConfirm) ([][]uuid.UUID, error) {
	if len(groups) > s.batchMaxProducts() {
		return nil, batchLimitExceeded()
	}
	parsed := make([][]uuid.UUID, len(groups))
	seen := map[uuid.UUID]struct{}{}
	for index, group := range groups {
		ids, err := s.parseBatchProductIDs(group.ProductIDs)
		if err != nil {
			return nil, err
		}
		for _, productID := range ids {
			if _, exists := seen[productID]; exists {
				return nil, fmt.Errorf("product %s appears in more than one category group", productID)
			}
			seen[productID] = struct{}{}
			if len(seen) > s.batchMaxProducts() {
				return nil, batchLimitExceeded()
			}
		}
		parsed[index] = ids
	}
	return parsed, nil
}

func (s *Service) validateOzonCategoryGroupShop(ctx context.Context, tenantID int64, raw string) (uuid.UUID, error) {
	shopID, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil || shopID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid shopId")
	}
	var row shop.Shop
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ? AND platform = ? AND status = ? AND auth_status = ?", shopID, tenantID, "ozon", shop.StatusActive, shop.AuthAuthorized).First(&row).Error; err != nil {
		return uuid.Nil, err
	}
	return shopID, nil
}

func (s *Service) isActiveOzonLeaf(ctx context.Context, categoryID string) bool {
	var count int64
	err := s.DB.WithContext(ctx).Model(&shop.PlatformCategory{}).
		Where("platform = ? AND category_id = ? AND is_leaf = ? AND status = ?", "ozon", categoryID, true, "active").
		Count(&count).Error
	return err == nil && count == 1
}

func ozonConfiguredProductStatus(attrs []shop.PlatformCategoryAttribute, raw datatypes.JSON) (string, string) {
	values := map[string]any{}
	_ = json.Unmarshal(raw, &values)
	missing := make([]string, 0)
	for _, attr := range attrs {
		if !attr.Required {
			continue
		}
		value, ok := values[attr.AttrID]
		if !ok || !ozonGroupAttributePresent(value) {
			missing = append(missing, attr.Name)
		}
	}
	if len(missing) > 0 {
		return "needs_work", "需补充必填属性：" + strings.Join(missing, "、")
	}
	return "ready", ""
}

func ozonGroupAttributePresent(value any) bool {
	if nested, ok := value.(map[string]any); ok {
		value = nested["value"]
	}
	return strings.TrimSpace(fmt.Sprint(value)) != ""
}
func firstNonEmptyOzonPath(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

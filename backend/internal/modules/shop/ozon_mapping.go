package shop

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm/clause"
)

const (
	OzonMappingActive      = "active"
	OzonMappingNeedsReview = "needs_review"
	OzonMappingInactive    = "inactive"
)

type OzonCategoryMappingDTO struct {
	ID                 uuid.UUID  `json:"id"`
	ShopID             *uuid.UUID `json:"shopId,omitempty"`
	SourceCategoryKey  string     `json:"sourceCategoryKey"`
	SourceCategoryName string     `json:"sourceCategoryName,omitempty"`
	CategoryID         string     `json:"categoryId"`
	CategoryPath       string     `json:"categoryPath,omitempty"`
	Status             string     `json:"status"`
	SchemaHash         string     `json:"schemaHash,omitempty"`
	ConfirmedAt        *time.Time `json:"confirmedAt,omitempty"`
}

type PutOzonCategoryMappingBody struct {
	ShopID             string `json:"shopId"`
	SourceCategoryKey  string `json:"sourceCategoryKey"`
	SourceCategoryName string `json:"sourceCategoryName"`
	CategoryID         string `json:"categoryId"`
	CategoryPath       string `json:"categoryPath"`
	Status             string `json:"status"`
}

type RecommendOzonCategoryMappingBody struct {
	ShopID             string `json:"shopId"`
	SourceCategoryKey  string `json:"sourceCategoryKey"`
	SourceCategoryName string `json:"sourceCategoryName"`
}

func normalizeOzonSourceCategory(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func (s *Service) ListOzonCategoryMappings(ctx context.Context, tenantID int64, shopID *uuid.UUID) ([]OzonCategoryMappingDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	q := s.DB.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if shopID != nil {
		q = q.Where("shop_id IS NULL OR shop_id = ?", *shopID)
	} else {
		q = q.Where("shop_id IS NULL")
	}
	var rows []OzonCategoryMapping
	if err := q.Order("source_category_key ASC").
		Clauses(clause.OrderBy{Expression: clause.Expr{SQL: "CASE WHEN shop_id IS NULL THEN 1 ELSE 0 END", WithoutParentheses: true}}).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]OzonCategoryMappingDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, ozonMappingDTO(row))
	}
	return out, nil
}

func (s *Service) RecommendOzonCategoryMapping(ctx context.Context, tenantID int64, body RecommendOzonCategoryMappingBody) (*OzonCategoryMappingDTO, error) {
	key := normalizeOzonSourceCategory(firstNonEmptyOzon(body.SourceCategoryKey, body.SourceCategoryName))
	if key == "" {
		return nil, fmt.Errorf("sourceCategoryKey is required")
	}
	if body.ShopID != "" {
		if _, err := s.ensureOzonMappingShop(ctx, tenantID, body.ShopID); err != nil {
			return nil, err
		}
	}
	var rows []PlatformCategory
	if err := s.DB.WithContext(ctx).Where("platform = ? AND is_leaf = ? AND status = ?", ozonPlatform, true, "active").Order("name ASC, category_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ozonCategoryErr(OzonCategoryEmpty, fmt.Errorf("no active ozon leaf categories in cache"))
	}
	sort.SliceStable(rows, func(i, j int) bool { return categoryScore(rows[i].Name, key) > categoryScore(rows[j].Name, key) })
	best := rows[0]
	if categoryScore(best.Name, key) < 2 {
		return nil, nil
	}
	return &OzonCategoryMappingDTO{SourceCategoryKey: key, SourceCategoryName: strings.TrimSpace(body.SourceCategoryName), CategoryID: best.CategoryID, CategoryPath: CanonicalOzonCategoryPath(ctx, s.DB, best), Status: OzonMappingNeedsReview}, nil
}

func categoryScore(name, key string) int {
	n := normalizeOzonSourceCategory(name)
	if n == "" || key == "" {
		return 0
	}
	if n == key {
		return 1000
	}
	if strings.Contains(n, key) || strings.Contains(key, n) {
		return 500
	}
	score := 0
	for _, r := range key {
		if strings.ContainsRune(n, r) {
			score++
		}
	}
	return score
}

func (s *Service) PutOzonCategoryMapping(ctx context.Context, tenantID int64, body PutOzonCategoryMappingBody, adminID *uuid.UUID) (*OzonCategoryMappingDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	key := normalizeOzonSourceCategory(firstNonEmptyOzon(body.SourceCategoryKey, body.SourceCategoryName))
	if key == "" {
		return nil, fmt.Errorf("sourceCategoryKey is required")
	}
	var sid *uuid.UUID
	if strings.TrimSpace(body.ShopID) != "" {
		id, err := s.ensureOzonMappingShop(ctx, tenantID, body.ShopID)
		if err != nil {
			return nil, err
		}
		sid = &id
	}
	catID := strings.TrimSpace(body.CategoryID)
	var cat PlatformCategory
	if err := s.DB.WithContext(ctx).Where("platform = ? AND category_id = ?", ozonPlatform, catID).First(&cat).Error; err != nil {
		return nil, err
	}
	if !cat.IsLeaf || cat.Status != "active" {
		return nil, fmt.Errorf("ozon category must be an active leaf")
	}
	status := strings.TrimSpace(body.Status)
	if status == "" {
		status = OzonMappingActive
	}
	if status != OzonMappingActive && status != OzonMappingNeedsReview && status != OzonMappingInactive {
		return nil, fmt.Errorf("invalid mapping status")
	}
	var attrs []PlatformCategoryAttribute
	if err := s.DB.WithContext(ctx).Where("platform = ? AND category_id = ?", ozonPlatform, catID).Find(&attrs).Error; err != nil {
		return nil, err
	}
	scopeKey := ozonMappingScopeKey(sid)
	row := OzonCategoryMapping{TenantID: tenantID, ShopID: sid, ScopeKey: scopeKey, SourceCategoryKey: key, SourceCategoryName: strings.TrimSpace(body.SourceCategoryName), CategoryID: catID, CategoryPath: CanonicalOzonCategoryPath(ctx, s.DB, cat), Status: status, SchemaHash: OzonCategorySchemaHash(attrs)}
	if status == OzonMappingActive {
		now := time.Now().UTC()
		row.ConfirmedAt = &now
		row.ConfirmedBy = adminID
	}
	if err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "scope_key"}, {Name: "source_category_key"}}, DoUpdates: clause.AssignmentColumns([]string{"shop_id", "source_category_name", "category_id", "category_path", "status", "schema_hash", "confirmed_at", "confirmed_by", "updated_at"})}).Create(&row).Error; err != nil {
		return nil, err
	}
	var saved OzonCategoryMapping
	q := s.DB.WithContext(ctx).Where("tenant_id = ? AND source_category_key = ?", tenantID, key)
	if sid == nil {
		q = q.Where("shop_id IS NULL")
	} else {
		q = q.Where("shop_id = ?", *sid)
	}
	if err := q.First(&saved).Error; err != nil {
		return nil, err
	}
	out := ozonMappingDTO(saved)
	return &out, nil
}

func ozonMappingScopeKey(shopID *uuid.UUID) string {
	if shopID == nil || *shopID == uuid.Nil {
		return "tenant"
	}
	return shopID.String()
}

func (s *Service) ensureOzonMappingShop(ctx context.Context, tenantID int64, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid shopId")
	}
	var row Shop
	if err := s.DB.WithContext(security.WithTenantContext(ctx, &security.TenantContext{TenantID: tenantID})).Where("id = ? AND tenant_id = ? AND platform = ? AND status = ? AND auth_status = ?", id, tenantID, ozonPlatform, StatusActive, AuthAuthorized).First(&row).Error; err != nil {
		return uuid.Nil, err
	}
	return id, nil
}
func ozonMappingDTO(row OzonCategoryMapping) OzonCategoryMappingDTO {
	return OzonCategoryMappingDTO{ID: row.ID, ShopID: row.ShopID, SourceCategoryKey: row.SourceCategoryKey, SourceCategoryName: row.SourceCategoryName, CategoryID: row.CategoryID, CategoryPath: row.CategoryPath, Status: row.Status, SchemaHash: row.SchemaHash, ConfirmedAt: row.ConfirmedAt}
}
func firstNonEmptyOzon(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

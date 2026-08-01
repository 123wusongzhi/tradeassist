package shop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformozon "github.com/trademind-ai/trademind/backend/internal/providers/platform/ozon"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	OzonCategorySyncFailed      = "OZON_CATEGORY_SYNC_FAILED"
	OzonCategoryAttrSyncFailed  = "OZON_CATEGORY_ATTR_SYNC_FAILED"
	OzonCategoryEmpty           = "OZON_CATEGORY_EMPTY"
	OzonCategoryNotLeaf         = "OZON_CATEGORY_NOT_LEAF"
	OzonShopRequired            = "OZON_SHOP_REQUIRED"
	OzonAttributeMappingInvalid = "OZON_ATTRIBUTE_MAPPING_INVALID"
	OzonCategoryCacheTTL        = 24 * time.Hour
	ozonPlatform                = "ozon"
	ozonMaxCategoryRows         = 20000
	ozonMaxDictionaryAttrFetch  = 30
)

type OzonCategoryError struct {
	Code    string
	Message string
	Err     error
}

func (e *OzonCategoryError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func (e *OzonCategoryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ozonCategoryErr(code string, err error) *OzonCategoryError {
	return &OzonCategoryError{Code: code, Message: code, Err: err}
}

// OzonCategoryNodeDTO is one cached Ozon category row (level 1 or leaf).
// Leaf rows carry both descriptionCategoryId and typeId.
type OzonCategoryNodeDTO struct {
	ID                    uuid.UUID  `json:"id"`
	CategoryID            string     `json:"categoryId"` // leaf: "<descId>:<typeId>"
	DescriptionCategoryID string     `json:"descriptionCategoryId,omitempty"`
	TypeID                string     `json:"typeId,omitempty"`
	ParentID              string     `json:"parentId"`
	Name                  string     `json:"name"`
	Level                 int        `json:"level"`
	IsLeaf                bool       `json:"isLeaf"`
	SyncedAt              *time.Time `json:"syncedAt,omitempty"`
}

type OzonCategoryListResult struct {
	List      []OzonCategoryNodeDTO `json:"list"`
	Total     int                   `json:"total"`
	LeafCount int                   `json:"leafCount"`
}

type OzonCategoryStats struct {
	Count        int64      `json:"count"`
	LeafCount    int64      `json:"leafCount"`
	LastSyncedAt *time.Time `json:"lastSyncedAt,omitempty"`
}

type OzonAttributeDTO struct {
	ID           uuid.UUID       `json:"id"`
	CategoryID   string          `json:"categoryId"`
	AttrID       string          `json:"attrId"`
	Name         string          `json:"name"`
	Required     bool            `json:"required"`
	ValueType    string          `json:"valueType,omitempty"`
	DictionaryID string          `json:"dictionaryId,omitempty"`
	Options      json.RawMessage `json:"options,omitempty"`
	SyncedAt     *time.Time      `json:"syncedAt,omitempty"`
	CacheStale   bool            `json:"cacheStale"`
}

type OzonAttributeMappingDTO struct {
	AttributeID   string `json:"attributeId"`
	AttributeName string `json:"attributeName,omitempty"`
	LocalField    string `json:"localField,omitempty"`
	Enabled       bool   `json:"enabled"`
	SortOrder     int    `json:"sortOrder"`
}

type PutOzonAttributeMappingsBody struct {
	Items []OzonAttributeMappingDTO `json:"items"`
}

// resolveOzonShop returns an authorized Ozon shop's plain auth. When shopID is
// zero it resolves the first authorized Ozon shop (settings-page convenience).
func (s *Service) resolveOzonShop(ctx context.Context, shopID uuid.UUID) (platformp.TestConnectionRequest, error) {
	if shopID == uuid.Nil {
		var row Shop
		if err := s.DB.WithContext(ctx).
			Where("platform = ? AND auth_status = ?", ozonPlatform, AuthAuthorized).
			Order("updated_at DESC").First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return platformp.TestConnectionRequest{}, ozonCategoryErr(OzonShopRequired, fmt.Errorf("no authorized ozon shop found, please authorize one first"))
			}
			return platformp.TestConnectionRequest{}, err
		}
		shopID = row.ID
	}
	shopRow, plainAuth, err := s.PlainAuthForProviderCtx(ctx, shopID)
	if err != nil {
		return platformp.TestConnectionRequest{}, ozonCategoryErr(OzonShopRequired, fmt.Errorf("ozon shop not found: %w", err))
	}
	if shopRow == nil {
		return platformp.TestConnectionRequest{}, ozonCategoryErr(OzonShopRequired, fmt.Errorf("ozon shop not found"))
	}
	if strings.TrimSpace(shopRow.Platform) != ozonPlatform {
		return platformp.TestConnectionRequest{}, ozonCategoryErr(OzonShopRequired, fmt.Errorf("shop %s is not an ozon shop", shopID.String()))
	}
	if strings.TrimSpace(shopRow.AuthStatus) != AuthAuthorized {
		return platformp.TestConnectionRequest{}, ozonCategoryErr(OzonShopRequired, fmt.Errorf("ozon shop is not authorized"))
	}
	return plainAuth, nil
}

// SyncOzonCategories downloads the category tree and upserts the cache.
func (s *Service) SyncOzonCategories(ctx context.Context, shopID uuid.UUID) (*OzonCategoryStats, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	auth, err := s.resolveOzonShop(ctx, shopID)
	if err != nil {
		return nil, err
	}
	client, err := platformozon.NewClient(auth)
	if err != nil {
		return nil, ozonCategoryErr(OzonCategorySyncFailed, err)
	}
	nodes, err := client.FetchCategoryTree(ctx)
	if err != nil {
		return nil, ozonCategoryErr(OzonCategorySyncFailed, err)
	}
	if len(nodes) == 0 {
		return nil, ozonCategoryErr(OzonCategoryEmpty, fmt.Errorf("ozon category tree is empty"))
	}
	now := time.Now().UTC()
	rows := make([]PlatformCategory, 0, len(nodes))
	for _, n := range nodes {
		row := PlatformCategory{
			Platform:   ozonPlatform,
			CategoryID: n.DescriptionCategoryID,
			ParentID:   n.ParentID,
			Name:       n.Name,
			Level:      n.Level,
			IsLeaf:     n.IsLeaf,
			Status:     "active",
			SyncedAt:   &now,
		}
		if n.IsLeaf {
			row.CategoryID = ozonLeafCategoryID(n.DescriptionCategoryID, n.TypeID)
			raw := map[string]any{
				"description_category_id": n.DescriptionCategoryID,
				"type_id":                 n.TypeID,
				"type_name":               n.Name,
			}
			if b, err := json.Marshal(raw); err == nil {
				row.Raw = datatypes.JSON(b)
			}
		}
		rows = append(rows, row)
		if len(rows) >= ozonMaxCategoryRows {
			break
		}
	}
	upsert := clause.OnConflict{
		Columns: []clause.Column{{Name: "platform"}, {Name: "category_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"parent_id", "name", "level", "is_leaf", "status", "raw", "synced_at", "updated_at",
		}),
	}
	if err := s.DB.WithContext(ctx).Clauses(upsert).CreateInBatches(rows, 500).Error; err != nil {
		return nil, ozonCategoryErr(OzonCategorySyncFailed, err)
	}
	stats, err := s.OzonCategoryStats(ctx)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func ozonLeafCategoryID(descID, typeID string) string {
	return descID + ":" + typeID
}

// OzonCategoryListQuery filters cached Ozon categories.
type OzonCategoryListQuery struct {
	Keyword  string
	OnlyLeaf bool
	Limit    int
}

func (s *Service) ListOzonCategories(ctx context.Context, q OzonCategoryListQuery) (*OzonCategoryListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	qb := s.DB.WithContext(ctx).Where("platform = ?", ozonPlatform)
	if strings.TrimSpace(q.Keyword) != "" {
		kw := "%" + strings.TrimSpace(q.Keyword) + "%"
		qb = qb.Where("name ILIKE ? OR category_id ILIKE ?", kw, kw)
	}
	if q.OnlyLeaf {
		qb = qb.Where("is_leaf = ?", true)
	}
	var rows []PlatformCategory
	limit := q.Limit
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	if err := qb.Order("level ASC, name ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	list := make([]OzonCategoryNodeDTO, 0, len(rows))
	for _, r := range rows {
		row := OzonCategoryNodeDTO{
			ID:         r.ID,
			CategoryID: r.CategoryID,
			ParentID:   r.ParentID,
			Name:       r.Name,
			Level:      r.Level,
			IsLeaf:     r.IsLeaf,
			SyncedAt:   r.SyncedAt,
		}
		if r.IsLeaf {
			descID, typeID := splitOzonLeafCategoryID(r.CategoryID)
			row.DescriptionCategoryID = descID
			row.TypeID = typeID
		}
		list = append(list, row)
	}
	var cnt int64
	var leafCnt int64
	_ = s.DB.WithContext(ctx).Model(&PlatformCategory{}).Where("platform = ?", ozonPlatform).Count(&cnt).Error
	_ = s.DB.WithContext(ctx).Model(&PlatformCategory{}).Where("platform = ? AND is_leaf = ?", ozonPlatform, true).Count(&leafCnt).Error
	return &OzonCategoryListResult{List: list, Total: int(cnt), LeafCount: int(leafCnt)}, nil
}

func splitOzonLeafCategoryID(id string) (descID, typeID string) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return id, ""
}

func (s *Service) OzonCategoryStats(ctx context.Context) (*OzonCategoryStats, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	var cnt, leafCnt int64
	if err := s.DB.WithContext(ctx).Model(&PlatformCategory{}).Where("platform = ?", ozonPlatform).Count(&cnt).Error; err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(ctx).Model(&PlatformCategory{}).Where("platform = ? AND is_leaf = ?", ozonPlatform, true).Count(&leafCnt).Error; err != nil {
		return nil, err
	}
	var lastSynced time.Time
	lerr := s.DB.WithContext(ctx).Model(&PlatformCategory{}).
		Where("platform = ? AND synced_at IS NOT NULL", ozonPlatform).
		Select("synced_at").Order("synced_at DESC").Limit(1).Scan(&lastSynced).Error
	if lerr != nil && !errors.Is(lerr, gorm.ErrRecordNotFound) {
		return nil, lerr
	}
	var p *time.Time
	if !lastSynced.IsZero() && lastSynced.Year() > 1970 {
		p = &lastSynced
	}
	return &OzonCategoryStats{Count: cnt, LeafCount: leafCnt, LastSyncedAt: p}, nil
}

// SyncOzonCategoryAttributes fetches the attribute template for one leaf
// category and refreshes the 24h cache (dictionary values are prefetched for
// dictionary attributes to speed up mapping).
func (s *Service) SyncOzonCategoryAttributes(ctx context.Context, categoryID, shopID uuid.UUID) (*OzonCategoryStats, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	var cat PlatformCategory
	if err := s.DB.WithContext(ctx).Where("id = ? AND platform = ?", categoryID, ozonPlatform).First(&cat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ozonCategoryErr(OzonCategoryNotLeaf, fmt.Errorf("ozon category not found"))
		}
		return nil, err
	}
	if !cat.IsLeaf {
		return nil, ozonCategoryErr(OzonCategoryNotLeaf, fmt.Errorf("please select a leaf category (type)"))
	}
	descID, typeID := splitOzonLeafCategoryID(cat.CategoryID)
	if descID == "" || typeID == "" {
		return nil, ozonCategoryErr(OzonCategoryNotLeaf, fmt.Errorf("leaf category missing type_id"))
	}
	auth, err := s.resolveOzonShop(ctx, shopID)
	if err != nil {
		return nil, err
	}
	client, err := platformozon.NewClient(auth)
	if err != nil {
		return nil, ozonCategoryErr(OzonCategoryAttrSyncFailed, err)
	}
	attrs, err := client.FetchCategoryAttributes(ctx, descID, typeID)
	if err != nil {
		return nil, ozonCategoryErr(OzonCategoryAttrSyncFailed, err)
	}
	now := time.Now().UTC()
	rows := make([]PlatformCategoryAttribute, 0, len(attrs))
	dictFetchCount := 0
	for _, a := range attrs {
		row := PlatformCategoryAttribute{
			Platform:   ozonPlatform,
			CategoryID: cat.CategoryID,
			AttrID:     a.ID,
			Name:       a.Name,
			Required:   a.Required,
			ValueType:  a.ValueType,
			SyncedAt:   &now,
		}
		if a.DictionaryID != "" {
			raw := map[string]any{"dictionary_id": a.DictionaryID}
			if b, jerr := json.Marshal(raw); jerr == nil {
				row.Raw = datatypes.JSON(b)
			}
		}
		if a.DictionaryID != "" && dictFetchCount < ozonMaxDictionaryAttrFetch {
			if vals, verr := client.FetchDictionaryValues(ctx, descID, typeID, a.ID); verr == nil && len(vals) > 0 {
				if b, jerr := json.Marshal(vals); jerr == nil {
					row.Options = datatypes.JSON(b)
				}
				dictFetchCount++
			}
		}
		rows = append(rows, row)
	}
	attrUpsert := clause.OnConflict{
		Columns: []clause.Column{{Name: "platform"}, {Name: "category_id"}, {Name: "attr_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "required", "value_type", "options", "raw", "synced_at", "updated_at",
		}),
	}
	if len(rows) > 0 {
		if err := s.DB.WithContext(ctx).Clauses(attrUpsert).CreateInBatches(rows, 200).Error; err != nil {
			return nil, ozonCategoryErr(OzonCategoryAttrSyncFailed, err)
		}
	}
	return &OzonCategoryStats{Count: int64(len(rows))}, nil
}

func (s *Service) ListOzonCategoryAttributes(ctx context.Context, categoryID uuid.UUID) ([]OzonAttributeDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	var cat PlatformCategory
	if err := s.DB.WithContext(ctx).Where("id = ? AND platform = ?", categoryID, ozonPlatform).First(&cat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ozonCategoryErr(OzonCategoryNotLeaf, fmt.Errorf("ozon category not found"))
		}
		return nil, err
	}
	var rows []PlatformCategoryAttribute
	if err := s.DB.WithContext(ctx).
		Where("platform = ? AND category_id = ?", ozonPlatform, cat.CategoryID).
		Order("required DESC, attr_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	cacheStale := false
	if cat.SyncedAt == nil || time.Since(*cat.SyncedAt) > OzonCategoryCacheTTL {
		cacheStale = true
	}
	out := make([]OzonAttributeDTO, 0, len(rows))
	for _, r := range rows {
		dto := OzonAttributeDTO{
			ID:         r.ID,
			CategoryID: r.CategoryID,
			AttrID:     r.AttrID,
			Name:       r.Name,
			Required:   r.Required,
			ValueType:  r.ValueType,
			SyncedAt:   r.SyncedAt,
			CacheStale: cacheStale,
		}
		dto.DictionaryID = dictionaryIDFromRaw(r.Raw)
		if r.Options != nil && len(r.Options) > 0 {
			dto.Options = json.RawMessage(r.Options)
		}
		out = append(out, dto)
	}
	return out, nil
}

func dictionaryIDFromRaw(raw datatypes.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	if v, ok := m["dictionary_id"].(string); ok {
		return strings.TrimSpace(v)
	}
	if v, ok := m["dictionary_id"].(float64); ok {
		return fmt.Sprintf("%.0f", v)
	}
	return ""
}

func (s *Service) GetOzonAttributeMappings(ctx context.Context, categoryID uuid.UUID) ([]OzonAttributeMappingDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	var cat PlatformCategory
	if err := s.DB.WithContext(ctx).Where("id = ? AND platform = ?", categoryID, ozonPlatform).First(&cat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ozonCategoryErr(OzonCategoryNotLeaf, fmt.Errorf("ozon category not found"))
		}
		return nil, err
	}
	var rows []PlatformCategoryAttributeMapping
	if err := s.DB.WithContext(ctx).
		Where("platform = ? AND category_id = ?", ozonPlatform, cat.CategoryID).
		Order("sort_order ASC, attribute_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]OzonAttributeMappingDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, OzonAttributeMappingDTO{
			AttributeID:   r.AttributeID,
			AttributeName: r.AttributeName,
			LocalField:    r.LocalField,
			Enabled:       r.Enabled,
			SortOrder:     r.SortOrder,
		})
	}
	return out, nil
}

func (s *Service) PutOzonAttributeMappings(ctx context.Context, categoryID uuid.UUID, body PutOzonAttributeMappingsBody) ([]OzonAttributeMappingDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	var cat PlatformCategory
	if err := s.DB.WithContext(ctx).Where("id = ? AND platform = ?", categoryID, ozonPlatform).First(&cat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ozonCategoryErr(OzonCategoryNotLeaf, fmt.Errorf("ozon category not found"))
		}
		return nil, err
	}
	seen := map[string]bool{}
	rows := make([]PlatformCategoryAttributeMapping, 0, len(body.Items))
	for i, item := range body.Items {
		attrID := strings.TrimSpace(item.AttributeID)
		if attrID == "" {
			return nil, ozonCategoryErr(OzonAttributeMappingInvalid, fmt.Errorf("attributeId is required"))
		}
		if seen[attrID] {
			return nil, ozonCategoryErr(OzonAttributeMappingInvalid, fmt.Errorf("duplicate attributeId %s", attrID))
		}
		seen[attrID] = true
		rows = append(rows, PlatformCategoryAttributeMapping{
			Platform:      ozonPlatform,
			CategoryID:    cat.CategoryID,
			AttributeID:   attrID,
			AttributeName: strings.TrimSpace(item.AttributeName),
			LocalField:    strings.TrimSpace(item.LocalField),
			Enabled:       item.Enabled,
			SortOrder:     i,
		})
	}
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("platform = ? AND category_id = ?", ozonPlatform, cat.CategoryID).Delete(&PlatformCategoryAttributeMapping{}).Error; err != nil {
			return err
		}
		if len(rows) > 0 {
			return tx.Create(&rows).Error
		}
		return nil
	}); err != nil {
		return nil, ozonCategoryErr(OzonAttributeMappingInvalid, err)
	}
	return s.GetOzonAttributeMappings(ctx, categoryID)
}

package shop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
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
	OzonCategorySyncQueueName   = "ozon:category:sync"
	OzonCategorySyncPending     = "pending"
	OzonCategorySyncRunning     = "running"
	OzonCategorySyncSucceeded   = "succeeded"
	OzonCategorySyncPartial     = "partial"
	OzonCategorySyncFailedState = "failed"
)

const ozonCredentialInvalidMessage = "Ozon 店铺授权已失效或 API Key 已停用，请前往平台设置更新凭证后重试"

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
	message := code
	if errors.Is(err, platformp.ErrPlatformProductPublishPermissionDenied) {
		message = ozonCredentialInvalidMessage
	}
	return &OzonCategoryError{Code: code, Message: message, Err: err}
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
	Status                string     `json:"status"`
	SyncedAt              *time.Time `json:"syncedAt,omitempty"`
}

type OzonCategoryListResult struct {
	List      []OzonCategoryNodeDTO `json:"list"`
	Total     int                   `json:"total"`
	LeafCount int                   `json:"leafCount"`
}

type OzonCategoryStats struct {
	Count         int64                `json:"count"`
	LeafCount     int64                `json:"leafCount"`
	ActiveCount   int64                `json:"activeCount"`
	InactiveCount int64                `json:"inactiveCount"`
	LastSyncedAt  *time.Time           `json:"lastSyncedAt,omitempty"`
	LastRun       *OzonCategorySyncRun `json:"lastRun,omitempty"`
}

type OzonCategoryChangesQuery struct {
	Limit      int
	ChangeType string
}

// OzonCategoryChangeDTO is the stable API representation for the change
// center. It intentionally does not expose the persistence model directly.
type OzonCategoryChangeDTO struct {
	ID           uuid.UUID       `json:"id"`
	SyncRunID    uuid.UUID       `json:"syncRunId"`
	ShopID       uuid.UUID       `json:"shopId"`
	CategoryID   string          `json:"categoryId"`
	CategoryName string          `json:"categoryName,omitempty"`
	ChangeType   string          `json:"changeType"`
	OccurredAt   time.Time       `json:"occurredAt"`
	Detail       string          `json:"detail"`
	Before       json.RawMessage `json:"before,omitempty"`
	After        json.RawMessage `json:"after,omitempty"`
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

// OzonCategorySchemaHash intentionally excludes database IDs and timestamps.
// It changes only when Ozon's listing template semantics change.
func OzonCategorySchemaHash(attrs []PlatformCategoryAttribute) string {
	rows := make([]platformozon.CategoryAttr, 0, len(attrs))
	for _, a := range attrs {
		meta := ozonAttributeSchemaMeta(a.Raw)
		rows = append(rows, platformozon.CategoryAttr{
			ID: a.AttrID, Name: a.Name, ValueType: a.ValueType, DictionaryID: meta.DictionaryID, Required: a.Required,
			IsCollection: meta.IsCollection, AttributeComplexID: meta.AttributeComplexID, MaxValueCount: meta.MaxValueCount,
			ComplexIsCollection: meta.ComplexIsCollection, CategoryDependent: meta.CategoryDependent,
		})
	}
	return platformozon.CategorySchemaHash(rows)
}

type PutOzonAttributeMappingsBody struct {
	Items []OzonAttributeMappingDTO `json:"items"`
}

// resolveOzonShop returns an authorized Ozon shop's plain auth. When shopID is
// zero it resolves the first authorized Ozon shop (settings-page convenience).
func (s *Service) resolveOzonShopAndAuth(ctx context.Context, tenantID int64, shopID uuid.UUID) (uuid.UUID, platformp.TestConnectionRequest, error) {
	if shopID == uuid.Nil {
		var row Shop
		if err := s.DB.WithContext(ctx).
			Where("tenant_id = ? AND platform = ? AND status = ? AND auth_status = ?", tenantID, ozonPlatform, StatusActive, AuthAuthorized).
			Order("updated_at DESC").First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return uuid.Nil, platformp.TestConnectionRequest{}, ozonCategoryErr(OzonShopRequired, fmt.Errorf("no authorized ozon shop found, please authorize one first"))
			}
			return uuid.Nil, platformp.TestConnectionRequest{}, err
		}
		shopID = row.ID
	}
	shopRow, plainAuth, err := s.PlainAuthForProviderCtx(ctx, tenantID, shopID)
	if err != nil {
		return uuid.Nil, platformp.TestConnectionRequest{}, ozonCategoryErr(OzonShopRequired, fmt.Errorf("ozon shop not found: %w", err))
	}
	if shopRow == nil {
		return uuid.Nil, platformp.TestConnectionRequest{}, ozonCategoryErr(OzonShopRequired, fmt.Errorf("ozon shop not found"))
	}
	if strings.TrimSpace(shopRow.Platform) != ozonPlatform {
		return uuid.Nil, platformp.TestConnectionRequest{}, ozonCategoryErr(OzonShopRequired, fmt.Errorf("shop %s is not an ozon shop", shopID.String()))
	}
	if strings.TrimSpace(shopRow.AuthStatus) != AuthAuthorized {
		return uuid.Nil, platformp.TestConnectionRequest{}, ozonCategoryErr(OzonShopRequired, fmt.Errorf("ozon shop is not authorized"))
	}
	if strings.TrimSpace(shopRow.Status) != StatusActive {
		return uuid.Nil, platformp.TestConnectionRequest{}, ozonCategoryErr(OzonShopRequired, fmt.Errorf("ozon shop is not active"))
	}
	if _, err := platformozon.NewClient(plainAuth); err != nil {
		return uuid.Nil, platformp.TestConnectionRequest{}, ozonCategoryErr(OzonShopRequired, err)
	}
	return shopID, plainAuth, nil
}

func (s *Service) resolveOzonShop(ctx context.Context, tenantID int64, shopID uuid.UUID) (platformp.TestConnectionRequest, error) {
	_, auth, err := s.resolveOzonShopAndAuth(ctx, tenantID, shopID)
	return auth, err
}

// SyncOzonCategories downloads the category tree and upserts the cache.
func (s *Service) SyncOzonCategories(ctx context.Context, tenantID int64, shopID uuid.UUID) (*OzonCategoryStats, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	ctx = security.WithTenantContext(ctx, &security.TenantContext{TenantID: tenantID})
	resolvedShopID, auth, err := s.resolveOzonShopAndAuth(ctx, tenantID, shopID)
	if err != nil {
		return nil, err
	}
	shopID = resolvedShopID
	now := time.Now().UTC()
	run := OzonCategorySyncRun{TenantID: tenantID, ShopID: shopID, Status: OzonCategorySyncRunning, StartedAt: &now}
	trackRun := s.DB.Migrator().HasTable(&OzonCategorySyncRun{}) && s.DB.Migrator().HasTable(&OzonCategoryChange{})
	if trackRun {
		if err := s.DB.WithContext(ctx).Create(&run).Error; err != nil {
			return nil, err
		}
	}
	var runPtr *OzonCategorySyncRun
	if trackRun {
		runPtr = &run
	}
	stats, syncErr := s.syncOzonCategoriesRun(ctx, auth, runPtr)
	if syncErr != nil {
		if trackRun {
			finished := time.Now().UTC()
			_ = s.DB.WithContext(ctx).Model(&OzonCategorySyncRun{}).Where("id = ? AND tenant_id = ?", run.ID, tenantID).Updates(map[string]any{"status": OzonCategorySyncFailedState, "finished_at": &finished, "error_code": OzonCategorySyncFailed, "error_message": syncErr.Error()}).Error
		}
		return nil, syncErr
	}
	return stats, nil
}

func (s *Service) syncOzonCategoriesRun(ctx context.Context, auth platformp.TestConnectionRequest, run *OzonCategorySyncRun) (*OzonCategoryStats, error) {
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
	truncated := len(nodes) >= ozonMaxCategoryRows
	rows := make([]PlatformCategory, 0, len(nodes))
	seen := make(map[string]PlatformCategory, len(nodes))
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
		if n.Disabled {
			row.Status = "inactive"
		}
		rows = append(rows, row)
		seen[row.CategoryID] = row
		if len(rows) >= ozonMaxCategoryRows {
			break
		}
	}
	changeCounts := map[string]int{"added": 0, "changed": 0, "deactivated": 0, "reactivated": 0}
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var oldRows []PlatformCategory
		if err := tx.Where("platform = ?", ozonPlatform).Find(&oldRows).Error; err != nil {
			return err
		}
		oldByID := make(map[string]PlatformCategory, len(oldRows))
		for _, old := range oldRows {
			oldByID[old.CategoryID] = old
		}
		changes := make([]OzonCategoryChange, 0)
		for _, row := range rows {
			old, exists := oldByID[row.CategoryID]
			change := ""
			if !exists {
				change = "added"
			} else if old.Status == "inactive" && row.Status == "active" {
				change = "reactivated"
			} else if old.Status == "active" && row.Status == "inactive" {
				change = "deactivated"
			} else if old.Name != row.Name || old.ParentID != row.ParentID || old.Level != row.Level || old.IsLeaf != row.IsLeaf || old.Status != row.Status {
				change = "changed"
			}
			if change != "" {
				before, _ := json.Marshal(old)
				after, _ := json.Marshal(row)
				if run != nil {
					changes = append(changes, OzonCategoryChange{TenantID: run.TenantID, ShopID: run.ShopID, SyncRunID: run.ID, CategoryID: row.CategoryID, ChangeType: change, Before: datatypes.JSON(before), After: datatypes.JSON(after)})
				}
				changeCounts[change]++
			}
		}
		// A complete response allows us to deactivate previously cached rows that
		// disappeared from Ozon. Never delete them: product configs need the history.
		for _, old := range oldRows {
			if _, ok := seen[old.CategoryID]; !truncated && !ok && old.Status != "inactive" {
				before, _ := json.Marshal(old)
				after, _ := json.Marshal(map[string]any{"status": "inactive"})
				if run != nil {
					changes = append(changes, OzonCategoryChange{TenantID: run.TenantID, ShopID: run.ShopID, SyncRunID: run.ID, CategoryID: old.CategoryID, ChangeType: "deactivated", Before: datatypes.JSON(before), After: datatypes.JSON(after)})
				}
				changeCounts["deactivated"]++
				if err := tx.Model(&PlatformCategory{}).Where("id = ?", old.ID).Updates(map[string]any{"status": "inactive", "updated_at": now}).Error; err != nil {
					return err
				}
			}
		}
		upsert := clause.OnConflict{Columns: []clause.Column{{Name: "platform"}, {Name: "category_id"}}, DoUpdates: clause.AssignmentColumns([]string{"parent_id", "name", "level", "is_leaf", "status", "raw", "synced_at", "updated_at"})}
		if err := tx.Clauses(upsert).CreateInBatches(rows, 500).Error; err != nil {
			return err
		}
		if run != nil && len(changes) > 0 {
			if err := tx.CreateInBatches(changes, 500).Error; err != nil {
				return err
			}
		}
		if run != nil {
			summary, _ := json.Marshal(map[string]any{
				"total":       len(rows),
				"added":       changeCounts["added"],
				"changed":     changeCounts["changed"],
				"deactivated": changeCounts["deactivated"],
				"reactivated": changeCounts["reactivated"],
				"truncated":   truncated,
			})
			finished := time.Now().UTC()
			status := OzonCategorySyncSucceeded
			if truncated {
				status = OzonCategorySyncPartial
			}
			return tx.Model(&OzonCategorySyncRun{}).Where("id = ? AND tenant_id = ?", run.ID, run.TenantID).Updates(map[string]any{"status": status, "finished_at": &finished, "summary": datatypes.JSON(summary), "error_code": "", "error_message": ""}).Error
		}
		return nil
	}); err != nil {
		return nil, ozonCategoryErr(OzonCategorySyncFailed, err)
	}
	if run != nil {
		stats, err := s.OzonCategoryStats(ctx, run.TenantID)
		if err != nil {
			return nil, err
		}
		return stats, nil
	}
	stats, err := s.OzonCategoryStats(ctx)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func (s *Service) ListOzonCategorySyncRuns(ctx context.Context, tenantID int64, limit int) ([]OzonCategorySyncRun, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var out []OzonCategorySyncRun
	err := s.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Limit(limit).Find(&out).Error
	return out, err
}

func (s *Service) GetOzonCategorySyncRun(ctx context.Context, tenantID int64, id uuid.UUID) (*OzonCategorySyncRun, error) {
	var out OzonCategorySyncRun
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).First(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) ListOzonCategoryChanges(ctx context.Context, tenantID int64, q OzonCategoryChangesQuery) ([]OzonCategoryChangeDTO, error) {
	if q.Limit < 1 || q.Limit > 500 {
		q.Limit = 100
	}
	db := s.DB.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if strings.TrimSpace(q.ChangeType) != "" {
		db = db.Where("change_type = ?", strings.TrimSpace(q.ChangeType))
	}
	var rows []OzonCategoryChange
	if err := db.Order("created_at DESC").Limit(q.Limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]OzonCategoryChangeDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, ozonCategoryChangeDTO(row))
	}
	return out, nil
}

func ozonCategoryChangeDTO(row OzonCategoryChange) OzonCategoryChangeDTO {
	before := json.RawMessage(row.Before)
	after := json.RawMessage(row.After)
	name := ozonCategoryNameFromChange(after)
	if name == "" {
		name = ozonCategoryNameFromChange(before)
	}
	detail := map[string]string{
		"added":       "发现新增类目",
		"changed":     "类目名称或层级已变化",
		"deactivated": "类目已停用",
		"reactivated": "类目已恢复启用",
	}[strings.TrimSpace(row.ChangeType)]
	if detail == "" {
		detail = "类目发生变化"
	}
	if name != "" {
		detail += "：" + name
	}
	return OzonCategoryChangeDTO{ID: row.ID, SyncRunID: row.SyncRunID, ShopID: row.ShopID, CategoryID: row.CategoryID, CategoryName: name, ChangeType: row.ChangeType, OccurredAt: row.CreatedAt, Detail: detail, Before: before, After: after}
}

func ozonCategoryNameFromChange(raw json.RawMessage) string {
	if len(raw) == 0 || !json.Valid(raw) {
		return ""
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	for _, key := range []string{"name", "category_name", "type_name"} {
		if text, ok := value[key].(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func ozonLeafCategoryID(descID, typeID string) string {
	return descID + ":" + typeID
}

// OzonCategoryListQuery filters cached Ozon categories.
type OzonCategoryListQuery struct {
	Keyword    string
	OnlyLeaf   bool
	ActiveOnly bool
	Limit      int
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
	if q.ActiveOnly {
		qb = qb.Where("status = ?", "active")
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
			Status:     r.Status,
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
	if err := s.DB.WithContext(ctx).Model(&PlatformCategory{}).Where("platform = ?", ozonPlatform).Count(&cnt).Error; err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(ctx).Model(&PlatformCategory{}).Where("platform = ? AND is_leaf = ?", ozonPlatform, true).Count(&leafCnt).Error; err != nil {
		return nil, err
	}
	return &OzonCategoryListResult{List: list, Total: int(cnt), LeafCount: int(leafCnt)}, nil
}

func splitOzonLeafCategoryID(id string) (descID, typeID string) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return id, ""
}

// CanonicalOzonCategoryPath derives a display path exclusively from the cached
// Ozon hierarchy; client-supplied paths are never trusted for saved configs.
func CanonicalOzonCategoryPath(ctx context.Context, db *gorm.DB, category PlatformCategory) string {
	if db == nil {
		return strings.TrimSpace(category.Name)
	}
	names := make([]string, 0, 4)
	current := category
	seen := map[string]bool{}
	for depth := 0; depth < 10; depth++ {
		if name := strings.TrimSpace(current.Name); name != "" {
			names = append(names, name)
		}
		parentID := strings.TrimSpace(current.ParentID)
		if parentID == "" || seen[parentID] {
			break
		}
		seen[parentID] = true
		var parent PlatformCategory
		if err := db.WithContext(ctx).Where("platform = ? AND category_id = ?", ozonPlatform, parentID).First(&parent).Error; err != nil {
			break
		}
		current = parent
	}
	for left, right := 0, len(names)-1; left < right; left, right = left+1, right-1 {
		names[left], names[right] = names[right], names[left]
	}
	return strings.Join(names, " / ")
}

func (s *Service) OzonCategoryStats(ctx context.Context, tenantIDs ...int64) (*OzonCategoryStats, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	var cnt, leafCnt int64
	if err := s.DB.WithContext(ctx).Model(&PlatformCategory{}).Where("platform = ?", ozonPlatform).Count(&cnt).Error; err != nil {
		return nil, err
	}
	var activeCnt, inactiveCnt int64
	if err := s.DB.WithContext(ctx).Model(&PlatformCategory{}).Where("platform = ? AND status = ?", ozonPlatform, "active").Count(&activeCnt).Error; err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(ctx).Model(&PlatformCategory{}).Where("platform = ? AND status = ?", ozonPlatform, "inactive").Count(&inactiveCnt).Error; err != nil {
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
	stats := &OzonCategoryStats{Count: cnt, LeafCount: leafCnt, ActiveCount: activeCnt, InactiveCount: inactiveCnt, LastSyncedAt: p}
	if len(tenantIDs) > 0 && tenantIDs[0] >= 0 && s.DB.Migrator().HasTable(&OzonCategorySyncRun{}) {
		var last OzonCategorySyncRun
		if s.DB.WithContext(ctx).Where("tenant_id = ?", tenantIDs[0]).Order("created_at DESC").First(&last).Error == nil {
			stats.LastRun = &last
		}
	}
	return stats, nil
}

// SyncOzonCategoryAttributes fetches the attribute template for one leaf
// category and refreshes the 24h cache (dictionary values are prefetched for
// dictionary attributes to speed up mapping).
func (s *Service) SyncOzonCategoryAttributes(ctx context.Context, tenantID int64, categoryID string, shopID uuid.UUID) (*OzonCategoryStats, error) {
	return s.syncOzonCategoryAttributes(ctx, tenantID, categoryID, shopID, true)
}

// RefreshOzonCategoryAttributeTemplate refreshes only template semantics. It
// preserves cached option lists and is used by live preflight, which validates
// each configured dictionary value through Ozon's search API instead of
// downloading every possible value.
func (s *Service) RefreshOzonCategoryAttributeTemplate(ctx context.Context, tenantID int64, categoryID string, shopID uuid.UUID) (*OzonCategoryStats, error) {
	return s.syncOzonCategoryAttributes(ctx, tenantID, categoryID, shopID, false)
}

func (s *Service) syncOzonCategoryAttributes(ctx context.Context, tenantID int64, categoryID string, shopID uuid.UUID, includeDictionaryValues bool) (*OzonCategoryStats, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	ctx = security.WithTenantContext(ctx, &security.TenantContext{TenantID: tenantID})
	cat, err := s.resolveOzonCategoryIdentifier(ctx, categoryID)
	if err != nil {
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
	auth, err := s.resolveOzonShop(ctx, tenantID, shopID)
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
	// Count attempts, not successful responses: otherwise a sequence of empty
	// or failing dictionary endpoints could make this sync issue unbounded HTTP
	// requests. Each attempt is itself bounded by the Ozon client's timeout and
	// retry limit.
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
		raw := map[string]any{
			"dictionary_id":         a.DictionaryID,
			"is_collection":         a.IsCollection,
			"attribute_complex_id":  a.AttributeComplexID,
			"max_value_count":       a.MaxValueCount,
			"complex_is_collection": a.ComplexIsCollection,
			"category_dependent":    a.CategoryDependent,
		}
		if b, jerr := json.Marshal(raw); jerr == nil {
			row.Raw = datatypes.JSON(b)
		}
		if includeDictionaryValues && isOzonDictionaryID(a.DictionaryID) && dictFetchCount < ozonMaxDictionaryAttrFetch {
			dictFetchCount++
			if vals, verr := client.FetchDictionaryValuesLimited(ctx, descID, typeID, a.ID, 200); verr == nil && len(vals) > 0 {
				if b, jerr := json.Marshal(vals); jerr == nil {
					row.Options = datatypes.JSON(b)
				}
			}
		}
		rows = append(rows, row)
	}
	updateColumns := []string{"name", "required", "value_type", "raw", "synced_at", "updated_at"}
	if includeDictionaryValues {
		updateColumns = append(updateColumns, "options")
	}
	attrUpsert := clause.OnConflict{
		Columns:   []clause.Column{{Name: "platform"}, {Name: "category_id"}, {Name: "attr_id"}},
		DoUpdates: clause.AssignmentColumns(updateColumns),
	}
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(rows) == 0 {
			return tx.Unscoped().Where("platform = ? AND category_id = ?", ozonPlatform, cat.CategoryID).
				Delete(&PlatformCategoryAttribute{}).Error
		}
		attrIDs := make([]string, 0, len(rows))
		for _, row := range rows {
			attrIDs = append(attrIDs, row.AttrID)
		}
		if err := tx.Unscoped().Where("platform = ? AND category_id = ? AND attr_id NOT IN ?", ozonPlatform, cat.CategoryID, attrIDs).
			Delete(&PlatformCategoryAttribute{}).Error; err != nil {
			return err
		}
		return tx.Clauses(attrUpsert).CreateInBatches(rows, 200).Error
	}); err != nil {
		return nil, ozonCategoryErr(OzonCategoryAttrSyncFailed, err)
	}
	return &OzonCategoryStats{Count: int64(len(rows))}, nil
}

func (s *Service) ListOzonCategoryAttributes(ctx context.Context, categoryID string) ([]OzonAttributeDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	cat, err := s.resolveOzonCategoryIdentifier(ctx, categoryID)
	if err != nil {
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

func (s *Service) SearchOzonDictionaryValues(ctx context.Context, tenantID int64, categoryID, attrID string, shopID uuid.UUID, keyword string) ([]platformozon.DictionaryValue, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	cat, err := s.resolveOzonCategoryIdentifier(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	if !cat.IsLeaf || cat.Status == "inactive" {
		return nil, ozonCategoryErr(OzonCategoryNotLeaf, fmt.Errorf("ozon category must be an active leaf"))
	}
	descID, typeID := splitOzonLeafCategoryID(cat.CategoryID)
	if descID == "" || typeID == "" {
		return nil, ozonCategoryErr(OzonCategoryNotLeaf, fmt.Errorf("leaf category missing type_id"))
	}
	var attr PlatformCategoryAttribute
	if err := s.DB.WithContext(ctx).Where("platform = ? AND category_id = ? AND attr_id = ?", ozonPlatform, cat.CategoryID, strings.TrimSpace(attrID)).First(&attr).Error; err != nil {
		return nil, err
	}
	if dictionaryIDFromRaw(attr.Raw) == "" {
		return nil, fmt.Errorf("ozon attribute is not a dictionary")
	}
	auth, err := s.resolveOzonShop(ctx, tenantID, shopID)
	if err != nil {
		return nil, err
	}
	client, err := platformozon.NewClient(auth)
	if err != nil {
		return nil, err
	}
	return client.SearchDictionaryValues(ctx, descID, typeID, attr.AttrID, keyword)
}

func dictionaryIDFromRaw(raw datatypes.JSON) string {
	return ozonAttributeSchemaMeta(raw).DictionaryID
}

func isOzonDictionaryID(raw string) bool {
	value := strings.TrimSpace(raw)
	return value != "" && value != "0"
}

type ozonAttributeSchemaMetadata struct {
	DictionaryID        string
	IsCollection        bool
	AttributeComplexID  int64
	MaxValueCount       int64
	ComplexIsCollection bool
	CategoryDependent   bool
}

func ozonAttributeSchemaMeta(raw datatypes.JSON) ozonAttributeSchemaMetadata {
	if len(raw) == 0 {
		return ozonAttributeSchemaMetadata{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ozonAttributeSchemaMetadata{}
	}
	out := ozonAttributeSchemaMetadata{
		IsCollection:        ozonBool(m["is_collection"]),
		AttributeComplexID:  ozonInt64(m["attribute_complex_id"]),
		MaxValueCount:       ozonInt64(m["max_value_count"]),
		ComplexIsCollection: ozonBool(m["complex_is_collection"]),
		CategoryDependent:   ozonBool(m["category_dependent"]),
	}
	if v, ok := m["dictionary_id"].(string); ok {
		out.DictionaryID = strings.TrimSpace(v)
	}
	if v, ok := m["dictionary_id"].(float64); ok {
		out.DictionaryID = fmt.Sprintf("%.0f", v)
	}
	if !isOzonDictionaryID(out.DictionaryID) {
		out.DictionaryID = ""
	}
	return out
}

func ozonBool(value any) bool {
	v, _ := value.(bool)
	return v
}

func ozonInt64(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}

func (s *Service) GetOzonAttributeMappings(ctx context.Context, categoryID string) ([]OzonAttributeMappingDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	cat, err := s.resolveOzonCategoryIdentifier(ctx, categoryID)
	if err != nil {
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

func (s *Service) PutOzonAttributeMappings(ctx context.Context, categoryID string, body PutOzonAttributeMappingsBody) ([]OzonAttributeMappingDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	cat, err := s.resolveOzonCategoryIdentifier(ctx, categoryID)
	if err != nil {
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

func (s *Service) resolveOzonCategoryIdentifier(ctx context.Context, identifier string) (*PlatformCategory, error) {
	value := strings.TrimSpace(identifier)
	if value == "" || len(value) > 128 {
		return nil, gorm.ErrRecordNotFound
	}
	query := s.DB.WithContext(ctx).Where("platform = ?", ozonPlatform)
	if id, err := uuid.Parse(value); err == nil && id != uuid.Nil {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("category_id = ?", value)
	}
	var category PlatformCategory
	if err := query.First(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

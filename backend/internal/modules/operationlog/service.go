package operationlog

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"gorm.io/gorm"
)

// WriteOpts is a single audit row to append.
type WriteOpts struct {
	TenantID    int64
	AdminUserID *uuid.UUID
	SessionID   *uuid.UUID
	AdminRole   string
	Username    string
	Action      string
	Resource    string
	ResourceID  string
	ShopID      *uuid.UUID
	Platform    string
	Permission  string
	Status      string
	Message     string
}

// Service persists operation logs.
type Service struct {
	DB *gorm.DB
}

// Write inserts one log row from the HTTP context plus overrides in opts.
func (s *Service) Write(c *gin.Context, opts WriteOpts) error {
	if s == nil || s.DB == nil || c == nil {
		return nil
	}
	reqID, _ := c.Get(ctxkey.TraceID)
	rid, _ := reqID.(string)

	adminID := opts.AdminUserID
	if adminID == nil {
		if idStr, ok := c.Get(ctxkey.AdminID); ok {
			if sub, ok := idStr.(string); ok {
				if u, err := uuid.Parse(sub); err == nil {
					adminID = &u
				}
			}
		}
	}
	username := strings.TrimSpace(opts.Username)
	if username == "" {
		if u, ok := c.Get(ctxkey.AdminUsername); ok {
			username, _ = u.(string)
			username = strings.TrimSpace(username)
		}
	}

	path := c.Request.URL.Path
	if fp := c.FullPath(); fp != "" {
		path = fp
	}

	row := &OperationLog{
		TenantID:         opts.TenantID,
		AdminUserID:      adminID,
		SessionID:        opts.SessionID,
		AdminRole:        strings.TrimSpace(opts.AdminRole),
		Username:         username,
		Action:           strings.TrimSpace(opts.Action),
		Resource:         strings.TrimSpace(opts.Resource),
		ResourceID:       strings.TrimSpace(opts.ResourceID),
		ShopID:           opts.ShopID,
		Platform:         strings.TrimSpace(opts.Platform),
		Permission:       strings.TrimSpace(opts.Permission),
		Method:           c.Request.Method,
		Path:             path,
		IPHash:           authutil.HashIP(c.ClientIP()),
		UserAgentSummary: authutil.SummarizeUserAgent(c.Request.UserAgent()),
		RequestID:        rid,
		Status:           strings.TrimSpace(opts.Status),
		Message:          truncateRunes(opts.Message, 2000),
		CreatedAt:        time.Now().UTC(),
	}
	if row.TenantID == 0 {
		if tid, ok := c.Get(ctxkey.TenantID); ok {
			if v, ok := tid.(int64); ok {
				row.TenantID = v
			}
		}
	}
	if row.SessionID == nil {
		if sid, ok := c.Get(ctxkey.SessionID); ok {
			if s, ok := sid.(string); ok {
				if u, err := uuid.Parse(s); err == nil {
					row.SessionID = &u
				}
			}
		}
	}
	if row.AdminRole == "" && s.DB != nil {
		if p, err := adminperm.LoadPrincipal(c, s.DB); err == nil && p != nil {
			row.AdminRole = p.Role
		}
	}
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := s.appendHashChain(tx, row); err != nil {
			return err
		}
		return tx.Create(row).Error
	})
}

// WriteBackground inserts one log row without an HTTP request (workers, cron).
func (s *Service) WriteBackground(ctx context.Context, opts WriteOpts) error {
	if s == nil || s.DB == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	adminID := opts.AdminUserID
	username := strings.TrimSpace(opts.Username)

	row := &OperationLog{
		AdminUserID: adminID,
		Username:    username,
		TenantID:    opts.TenantID,
		Action:      strings.TrimSpace(opts.Action),
		Resource:    strings.TrimSpace(opts.Resource),
		ResourceID:  strings.TrimSpace(opts.ResourceID),
		Method:      "INTERNAL",
		Path:        "/internal/worker",
		RequestID:   "",
		Status:      strings.TrimSpace(opts.Status),
		Message:     truncateRunes(opts.Message, 2000),
		CreatedAt:   time.Now().UTC(),
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.appendHashChain(tx, row); err != nil {
			return err
		}
		return tx.Create(row).Error
	})
}

// ListQuery binds query params for listing operation logs.
type ListQuery struct {
	Page      int
	PageSize  int
	Cursor    string
	Limit     int
	UseCursor bool
	Action    string
	Username  string
	Resource  string
	ShopID    *uuid.UUID
	Start     *time.Time
	End       *time.Time
}

// ListResult is a paginated slice of logs.
type ListResult struct {
	Items      []OperationLog
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
	Limit      int
	NextCursor string
	HasMore    bool
}

func operationLogCursorScope(c *gin.Context, db *gorm.DB, q ListQuery, tenantID int64) (string, string) {
	shopScope := ""
	if q.ShopID != nil && *q.ShopID != uuid.Nil {
		shopScope = q.ShopID.String()
	}
	allowed := []string{}
	if p, err := adminperm.LoadPrincipal(c, db); err == nil && p != nil {
		for _, id := range p.AllowedStoreIDs() {
			allowed = append(allowed, id.String())
		}
		sort.Strings(allowed)
	}
	return pagination.Fingerprint(map[string]any{
		"tenantId":       tenantID,
		"shopId":         shopScope,
		"allowedShopIds": allowed,
		"action":         q.Action,
		"username":       q.Username,
		"resource":       q.Resource,
		"start":          q.Start,
		"end":            q.End,
		"sort":           "created_at_desc_id_desc",
	}), shopScope
}

// List returns a paginated list with optional filters.
func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("operationlog: no db")
	}
	if q.UseCursor && q.Limit > 0 {
		q.PageSize = q.Limit
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	ps := q.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}

	tx := s.DB.WithContext(c.Request.Context()).Model(&OperationLog{})
	var tenantID int64
	if scoped, tid, err := adminperm.ApplyTenantScope(c, tx); err != nil {
		return nil, err
	} else {
		tx = scoped
		tenantID = tid
	}
	if scoped, err := adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id"); err != nil {
		return nil, err
	} else {
		tx = scoped
	}
	if v := strings.TrimSpace(q.Action); v != "" {
		tx = tx.Where("action = ?", v)
	}
	if v := strings.TrimSpace(q.Username); v != "" {
		pat := "%" + strings.ToLower(v) + "%"
		tx = tx.Where("LOWER(username) LIKE ?", pat)
	}
	if v := strings.TrimSpace(q.Resource); v != "" {
		tx = tx.Where("resource = ?", v)
	}
	if q.ShopID != nil && *q.ShopID != uuid.Nil {
		tx = tx.Where("shop_id = ?", *q.ShopID)
	}
	if q.Start != nil {
		tx = tx.Where("created_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("created_at <= ?", *q.End)
	}
	scopeHash, cursorShopID := operationLogCursorScope(c, s.DB, q, tenantID)
	if q.UseCursor && strings.TrimSpace(q.Cursor) != "" {
		cur, err := pagination.DecodeCursor(q.Cursor, tenantID, cursorShopID, scopeHash)
		if err != nil {
			return nil, err
		}
		next, err := pagination.ApplyDescKeyset(tx, "created_at", "id", cur)
		if err != nil {
			return nil, err
		}
		tx = next
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}

	var items []OperationLog
	query := tx.Order("created_at DESC, id DESC")
	limit := ps
	if q.UseCursor {
		limit = ps + 1
	} else {
		offset := (page - 1) * ps
		query = query.Offset(offset)
	}
	if err := query.Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	hasMore := q.UseCursor && len(items) > ps
	if hasMore {
		items = items[:ps]
	}
	nextCursor := ""
	if q.UseCursor && hasMore && len(items) > 0 {
		last := items[len(items)-1]
		var err error
		nextCursor, err = pagination.BuildNextCursor(true, tenantID, cursorShopID, scopeHash, "created_at", last.CreatedAt, last.ID.String())
		if err != nil {
			return nil, err
		}
	}

	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}

	return &ListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pages,
		Limit:      ps,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max])
	}
	return s
}

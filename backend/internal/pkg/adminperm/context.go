package adminperm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

const ctxPrincipalKey = "adminperm.principal"

// LoadPrincipal resolves role, permissions and store grants for the current admin.
func LoadPrincipal(c *gin.Context, db *gorm.DB) (*Principal, error) {
	if c == nil {
		return &Principal{Role: RoleReadonly, Disabled: true}, nil
	}
	if cached, ok := c.Get(ctxPrincipalKey); ok {
		if p, ok := cached.(*Principal); ok && p != nil {
			return p, nil
		}
	}
	if db == nil {
		p := &Principal{Role: RoleReadonly, Disabled: true}
		c.Set(ctxPrincipalKey, p)
		return p, nil
	}
	idStr, ok := c.Get(ctxkey.AdminID)
	if !ok {
		// Authenticated HTTP routes must always carry an admin id. Missing context
		// is a middleware/configuration failure, never implicit administrator scope.
		p := &Principal{Role: RoleReadonly, Disabled: true}
		c.Set(ctxPrincipalKey, p)
		return p, nil
	}
	s, _ := idStr.(string)
	uid, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil || uid == uuid.Nil {
		p := &Principal{Role: RoleReadonly, Permissions: PermissionsForRole(RoleReadonly)}
		c.Set(ctxPrincipalKey, p)
		return p, nil
	}

	var row admin.AdminUser
	if err := db.WithContext(c.Request.Context()).Select("id", "role", "status", "tenant_id", "token_version").First(&row, "id = ?", uid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			p := &Principal{UserID: uid, Role: RoleReadonly, Disabled: true}
			c.Set(ctxPrincipalKey, p)
			return p, nil
		}
		return nil, err
	}
	role := roleForTenant(row.Role, row.TenantID)
	cacheKey := permissionCacheKey(row.TenantID, uid, row.TokenVersion, row.Status, role, "")
	if cached, ok := getCachedPrincipal(cacheKey); ok {
		c.Set(ctxPrincipalKey, cached)
		return cached, nil
	}
	p := &Principal{
		UserID:      uid,
		TenantID:    row.TenantID,
		Role:        role,
		Permissions: PermissionsForRole(role),
	}
	if !strings.EqualFold(strings.TrimSpace(row.Status), admin.StatusActive) {
		p.Disabled = true
		p.Permissions = nil
		c.Set(ctxPrincipalKey, p)
		putCachedPrincipal(cacheKey, p)
		return p, nil
	}
	if !p.IsAdmin() && !p.IsTenantAdmin() {
		var grants []admin.UserStorePermission
		_ = db.WithContext(c.Request.Context()).
			Where("user_id = ?", uid).
			Order("created_at ASC").
			Find(&grants).Error
		p.StoreGrants = make([]StoreGrant, 0, len(grants))
		for _, g := range grants {
			p.StoreGrants = append(p.StoreGrants, StoreGrant{
				StoreID:         g.StoreID,
				Platform:        strings.TrimSpace(g.Platform),
				PermissionScope: admin.NormalizeStorePermScope(g.PermissionScope),
			})
		}
	}
	cacheKey = permissionCacheKey(row.TenantID, uid, row.TokenVersion, row.Status, role, storeGrantsFingerprint(p.StoreGrants))
	if cached, ok := getCachedPrincipal(cacheKey); ok {
		c.Set(ctxPrincipalKey, cached)
		return cached, nil
	}
	c.Set(ctxPrincipalKey, p)
	putCachedPrincipal(cacheKey, p)
	return p, nil
}

func storeGrantsFingerprint(grants []StoreGrant) string {
	if len(grants) == 0 {
		return ""
	}
	type row struct {
		StoreID string `json:"storeId"`
		Scope   string `json:"scope"`
		Plat    string `json:"platform"`
	}
	rows := make([]row, 0, len(grants))
	for _, g := range grants {
		rows = append(rows, row{
			StoreID: g.StoreID.String(),
			Scope:   strings.TrimSpace(strings.ToLower(g.PermissionScope)),
			Plat:    strings.TrimSpace(strings.ToLower(g.Platform)),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StoreID == rows[j].StoreID {
			return rows[i].Scope < rows[j].Scope
		}
		return rows[i].StoreID < rows[j].StoreID
	})
	raw, _ := json.Marshal(rows)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

// RoleFromContext loads admin_users.role for the authenticated admin.
func RoleFromContext(c *gin.Context, db *gorm.DB) string {
	p, _ := LoadPrincipal(c, db)
	if p == nil {
		return RoleReadonly
	}
	return p.Role
}

// ApplyStoreScope restricts query to allowed stores for non-admin principals.
// column is the SQL column name, e.g. "shop_id".
func ApplyStoreScope(c *gin.Context, db *gorm.DB, tx *gorm.DB, column string) (*gorm.DB, error) {
	if tx == nil {
		return tx, nil
	}
	p, err := LoadPrincipal(c, db)
	if err != nil {
		return nil, err
	}
	if p.IsAdmin() {
		return tx, nil
	}
	col := strings.TrimSpace(column)
	if col == "" {
		col = "shop_id"
	}
	if p.IsTenantAdmin() {
		return tx.Where(col+" IN (SELECT id FROM shops WHERE tenant_id = ?)", p.TenantID), nil
	}
	ids := p.AllowedStoreIDs()
	if len(ids) == 0 {
		return tx.Where("1 = 0"), nil
	}
	return tx.Where(col+" IN ?", ids), nil
}

// RequireStoreView denies with 404 when store is inaccessible (no existence leak).
func RequireStoreView(c *gin.Context, db *gorm.DB, storeID uuid.UUID) bool {
	p, err := LoadPrincipal(c, db)
	if err != nil {
		response.HandleError(c, err)
		return false
	}
	if p.IsTenantAdmin() {
		if tenantStoreVisible(c, db, p.TenantID, storeID) {
			return true
		}
	} else if p.CanViewStore(storeID) {
		return true
	}
	response.Fail(c, 404, response.CodeNotFound, "资源不存在")
	return false
}

// EnsureStoreVisible returns gorm.ErrRecordNotFound when shop is out of scope.
func EnsureStoreVisible(c *gin.Context, db *gorm.DB, shopID *uuid.UUID) error {
	if shopID == nil || *shopID == uuid.Nil {
		return nil
	}
	p, err := LoadPrincipal(c, db)
	if err != nil {
		return err
	}
	if p.IsTenantAdmin() {
		if tenantStoreVisible(c, db, p.TenantID, *shopID) {
			return nil
		}
		return gorm.ErrRecordNotFound
	}
	if p.CanViewStore(*shopID) {
		return nil
	}
	return gorm.ErrRecordNotFound
}

// RequireStoreOperate denies when store write is not allowed.
func RequireStoreOperate(c *gin.Context, db *gorm.DB, storeID uuid.UUID) bool {
	p, err := LoadPrincipal(c, db)
	if err != nil {
		response.HandleError(c, err)
		return false
	}
	if p.IsTenantAdmin() {
		if tenantStoreVisible(c, db, p.TenantID, storeID) {
			return true
		}
	} else if p.CanOperateStore(storeID) {
		return true
	}
	if storeID != uuid.Nil && !p.CanViewStore(storeID) {
		response.Fail(c, 404, response.CodeNotFound, "资源不存在")
		return false
	}
	DenyStorePermission(c)
	return false
}

// EnsureStoreOperate returns 404 for stores outside view scope and a safe 403
// error when the store is visible but the caller lacks write authority.
func EnsureStoreOperate(c *gin.Context, db *gorm.DB, storeID uuid.UUID) error {
	if storeID == uuid.Nil {
		return gorm.ErrRecordNotFound
	}
	p, err := LoadPrincipal(c, db)
	if err != nil {
		return err
	}
	trustedTenantID, err := TenantIDFromGin(c)
	if err != nil || trustedTenantID < 0 || !tenantStoreVisible(c, db, trustedTenantID, storeID) {
		return gorm.ErrRecordNotFound
	}
	if p.TenantID != trustedTenantID {
		if p.IsAdmin() {
			return crossTenantOperationError()
		}
		return gorm.ErrRecordNotFound
	}
	if p.IsAdmin() || p.IsTenantAdmin() || p.CanOperateStore(storeID) {
		return nil
	}
	if p.CanViewStore(storeID) {
		return storeOperationError()
	}
	return gorm.ErrRecordNotFound
}

func tenantStoreVisible(c *gin.Context, db *gorm.DB, tenantID int64, storeID uuid.UUID) bool {
	if db == nil || tenantID <= 0 || storeID == uuid.Nil {
		return false
	}
	var count int64
	if err := db.WithContext(c.Request.Context()).Table("shops").Where("id = ? AND tenant_id = ?", storeID, tenantID).Count(&count).Error; err != nil {
		return false
	}
	return count == 1
}

// TenantStoreIDs resolves all stores belonging to a tenant for cursor scopes
// and callers that cannot express the scope as a SQL subquery.
func TenantStoreIDs(c *gin.Context, db *gorm.DB, tenantID int64) ([]uuid.UUID, error) {
	if db == nil || tenantID <= 0 {
		return []uuid.UUID{}, nil
	}
	var ids []uuid.UUID
	if err := db.WithContext(c.Request.Context()).Table("shops").Where("tenant_id = ?", tenantID).Order("id ASC").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

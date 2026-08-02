package operationdashboard

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// Scope carries RBAC store filters for dashboard aggregation.
type Scope struct {
	AllowedShopIDs []uuid.UUID // nil = admin (all stores)
	TenantID       int64
	IsAdmin        bool
}

func scopeFromContext(c *gin.Context, db *gorm.DB) Scope {
	if c == nil {
		return Scope{IsAdmin: true}
	}
	p, _ := adminperm.LoadPrincipal(c, db)
	// Only the system tenant's global admin has deliberate cross-tenant semantics.
	// Missing or malformed principals must never turn an aggregate into all tenants.
	if p == nil {
		return Scope{}
	}
	if p.IsAdmin() && p.TenantID == 0 {
		return Scope{IsAdmin: true}
	}
	if p.IsTenantAdmin() {
		return Scope{TenantID: p.TenantID}
	}
	return Scope{TenantID: p.TenantID, AllowedShopIDs: p.AllowedStoreIDs()}
}

func (sc Scope) applyTenant(tx *gorm.DB, column string) *gorm.DB {
	if tx == nil || sc.IsAdmin {
		return tx
	}
	if sc.TenantID <= 0 {
		return tx.Where("1 = 0")
	}
	return tx.Where(column+" = ?", sc.TenantID)
}

func (sc Scope) applyShopColumn(tx *gorm.DB, column string) *gorm.DB {
	if tx == nil || sc.IsAdmin {
		return tx
	}
	col := column
	if col == "" {
		col = "shop_id"
	}
	if len(sc.AllowedShopIDs) == 0 {
		return tx.Where(col+" IN (SELECT id FROM shops WHERE tenant_id = ?)", sc.TenantID)
	}
	return tx.Where(col+" IN (SELECT id FROM shops WHERE tenant_id = ? AND id IN ?)", sc.TenantID, sc.AllowedShopIDs)
}

func (sc Scope) applyProductScope(tx *gorm.DB) *gorm.DB {
	if tx == nil || sc.IsAdmin {
		return tx
	}
	if len(sc.AllowedShopIDs) == 0 {
		if sc.TenantID > 0 {
			return tx.Where("products.tenant_id = ?", sc.TenantID)
		}
		return tx.Where("1 = 0")
	}
	return tx.Where(`products.id IN (
		SELECT DISTINCT product_id FROM product_platform_publish_configs WHERE shop_id IN ?
		UNION
		SELECT DISTINCT product_id FROM product_publications WHERE shop_id IN ? AND deleted_at IS NULL
	)`, sc.AllowedShopIDs, sc.AllowedShopIDs)
}

func (sc Scope) shopIDStrings() []string {
	if len(sc.AllowedShopIDs) == 0 {
		return nil
	}
	out := make([]string, 0, len(sc.AllowedShopIDs))
	for _, id := range sc.AllowedShopIDs {
		out = append(out, id.String())
	}
	return out
}

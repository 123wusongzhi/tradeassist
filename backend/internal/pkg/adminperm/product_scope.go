package adminperm

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ApplyProductScope restricts product queries to authorized stores for non-admin principals.
// Products are visible when linked via product_platform_publish_configs.shop_id or
// product_publications.shop_id. Unassigned drafts (no shop link) are admin-only.
func ApplyProductScope(c *gin.Context, db *gorm.DB, tx *gorm.DB) (*gorm.DB, error) {
	return ApplyProductScopeAlias(c, db, tx, "products")
}

// ApplyProductScopeAlias applies the read scope when products are joined under
// a SQL alias.
func ApplyProductScopeAlias(c *gin.Context, db *gorm.DB, tx *gorm.DB, productAlias string) (*gorm.DB, error) {
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
	if p.IsTenantAdmin() {
		// Tenant administrators own every product in their tenant, including new
		// drafts that have not been linked to a shop yet. The mandatory tenant
		// predicate is narrower than the global-admin bypass and cannot expose a
		// different tenant's unassigned drafts.
		return tx.Where(productAlias+".tenant_id = ?", p.TenantID), nil
	}
	ids := p.AllowedStoreIDs()
	if len(ids) == 0 {
		return tx.Where("1 = 0"), nil
	}
	return tx.Where(productAlias+`.id IN (
		SELECT DISTINCT product_id FROM product_platform_publish_configs
		WHERE shop_id IN ?
		UNION
		SELECT DISTINCT product_id FROM product_publications
		WHERE shop_id IN ? AND deleted_at IS NULL
	)`, ids, ids), nil
}

// EnsureProductVisible returns gorm.ErrRecordNotFound when product is out of scope.
func EnsureProductVisible(c *gin.Context, db *gorm.DB, productID uuid.UUID) error {
	if productID == uuid.Nil {
		return gorm.ErrRecordNotFound
	}
	p, err := LoadPrincipal(c, db)
	if err != nil {
		return err
	}
	if p.IsAdmin() {
		return nil
	}
	if p.IsTenantAdmin() {
		var count int64
		err = db.WithContext(c.Request.Context()).Table("products").
			Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", productID, p.TenantID).
			Count(&count).Error
		if err != nil || count == 0 {
			if err != nil {
				return err
			}
			return gorm.ErrRecordNotFound
		}
		return nil
	}
	ids := p.AllowedStoreIDs()
	if len(ids) == 0 {
		return gorm.ErrRecordNotFound
	}
	var count int64
	err = db.WithContext(c.Request.Context()).Raw(`
SELECT COUNT(*) FROM (
	SELECT product_id FROM product_platform_publish_configs WHERE product_id = ? AND shop_id IN ?
	UNION
	SELECT product_id FROM product_publications WHERE product_id = ? AND shop_id IN ? AND deleted_at IS NULL
) scoped`, productID, ids, productID, ids).Scan(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// EnsureProductOperate requires write authority for every shop linked to a
// product. A scoped user must not mutate a shared product through only one of
// its shop grants; unassigned drafts remain tenant-admin/global-admin only.
func EnsureProductOperate(c *gin.Context, db *gorm.DB, productID uuid.UUID) error {
	if productID == uuid.Nil {
		return gorm.ErrRecordNotFound
	}
	p, err := LoadPrincipal(c, db)
	if err != nil {
		return err
	}
	trustedTenantID, err := TenantIDFromGin(c)
	if err != nil || trustedTenantID < 0 {
		return gorm.ErrRecordNotFound
	}
	var count int64
	q := db.WithContext(c.Request.Context()).Table("products").Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", productID, trustedTenantID)
	if err := q.Count(&count).Error; err != nil || count != 1 {
		if err != nil {
			return err
		}
		return gorm.ErrRecordNotFound
	}
	if p.TenantID != trustedTenantID {
		if p.IsAdmin() {
			return crossTenantOperationError()
		}
		return gorm.ErrRecordNotFound
	}
	if p.IsAdmin() {
		return nil
	}
	if p.IsTenantAdmin() {
		return nil
	}
	var shopIDs []uuid.UUID
	if err := db.WithContext(c.Request.Context()).Raw(`
SELECT DISTINCT shop_id FROM product_platform_publish_configs WHERE product_id = ? AND shop_id IS NOT NULL
UNION
SELECT DISTINCT shop_id FROM product_publications WHERE product_id = ? AND shop_id IS NOT NULL AND deleted_at IS NULL`, productID, productID).Scan(&shopIDs).Error; err != nil {
		return err
	}
	if len(shopIDs) == 0 {
		return gorm.ErrRecordNotFound
	}
	for _, shopID := range shopIDs {
		if !p.CanOperateStore(shopID) {
			if p.CanViewStore(shopID) {
				return productOperationError()
			}
			return gorm.ErrRecordNotFound
		}
	}
	return nil
}

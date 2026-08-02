package product

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// ProductSKUSearchHit is a trimmed row for admin SKU picker.
type ProductSKUSearchHit struct {
	ProductID    string `json:"productId"`
	ProductTitle string `json:"productTitle"`
	ProductSKUID string `json:"productSkuId"`
	SKUCode      string `json:"skuCode"`
	SKUName      string `json:"skuName"`
	Stock        *int   `json:"stock,omitempty"`
	Attrs        any    `json:"attrs,omitempty"`
}

// SearchSKUsQuery GET /product-skus/search
type SearchSKUsQuery struct {
	Keyword   string
	ProductID *string
	Limit     int
}

// SearchSKUs runs keyword search across sku_code, sku_name, product.title
func (s *Service) SearchSKUs(c *gin.Context, q SearchSKUsQuery) ([]ProductSKUSearchHit, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	kw := strings.TrimSpace(q.Keyword)
	lim := q.Limit
	if lim <= 0 {
		lim = 20
	}
	if lim > 50 {
		lim = 50
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil || tenantID < 0 {
		return nil, fmt.Errorf("tenant context required")
	}
	tx := s.DB.WithContext(c.Request.Context()).
		Table("products").
		Select(`sk.id AS sku_id, sk.product_id AS product_id, sk.sku_code AS sku_code, sk.sku_name AS sku_name, sk.stock AS stock, sk.attrs AS attrs,
			products.title AS product_title`).
		Joins("JOIN product_skus AS sk ON sk.product_id = products.id").
		Where("products.deleted_at IS NULL AND products.tenant_id = ?", tenantID)
	if scoped, scopeErr := adminperm.ApplyProductScope(c, s.DB, tx); scopeErr != nil {
		return nil, scopeErr
	} else {
		tx = scoped
	}
	if q.ProductID != nil && strings.TrimSpace(*q.ProductID) != "" {
		tx = tx.Where("products.id = ?", strings.TrimSpace(*q.ProductID))
	}
	if kw != "" {
		pat := "%" + strings.ToLower(kw) + "%"
		tx = tx.Where(`(LOWER(sk.sku_code) LIKE ? OR LOWER(sk.sku_name) LIKE ? OR LOWER(p.title) LIKE ?)`, pat, pat, pat)
	}
	type row struct {
		SKUID        string `gorm:"column:sku_id"`
		ProductID    string `gorm:"column:product_id"`
		SKUCode      string `gorm:"column:sku_code"`
		SKUName      string `gorm:"column:sku_name"`
		Stock        *int   `gorm:"column:stock"`
		Attrs        any    `gorm:"column:attrs"`
		ProductTitle string `gorm:"column:product_title"`
	}
	var rows []row
	if err := tx.Order("products.updated_at DESC, sk.created_at ASC").Limit(lim).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ProductSKUSearchHit, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProductSKUSearchHit{
			ProductID:    r.ProductID,
			ProductTitle: r.ProductTitle,
			ProductSKUID: r.SKUID,
			SKUCode:      r.SKUCode,
			SKUName:      r.SKUName,
			Stock:        r.Stock,
			Attrs:        r.Attrs,
		})
	}
	return out, nil
}

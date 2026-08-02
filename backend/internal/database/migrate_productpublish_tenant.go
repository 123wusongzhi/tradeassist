package database

import (
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"gorm.io/gorm"
)

// migrateProductPublishTenantScope backfills only relationships whose product and
// shop agree on a non-zero tenant. Ambiguous legacy rows deliberately stay at 0.
func migrateProductPublishTenantScope(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("product publish tenant migration: nil db")
	}
	if err := db.AutoMigrate(&productpublish.ProductPublishTask{}, &productpublish.ProductPublishBatch{}, &productpublish.ProductPublication{}); err != nil {
		return err
	}
	postgres := strings.EqualFold(db.Dialector.Name(), "postgres")
	for _, table := range []string{"product_publish_tasks", "product_publications"} {
		// PostgreSQL does not allow the UPDATE target alias to be referenced from
		// a JOIN ... ON inside the FROM clause. Keep all correlations to x in the
		// WHERE clause so a fresh PostgreSQL migration is valid.
		stmt := "UPDATE " + table + " AS x SET tenant_id = p.tenant_id FROM products p, shops s WHERE x.product_id = p.id AND x.shop_id = s.id AND x.tenant_id = 0 AND p.tenant_id = s.tenant_id AND p.tenant_id <> 0"
		if !postgres {
			stmt = "UPDATE " + table + " SET tenant_id = (SELECT p.tenant_id FROM products p JOIN shops s ON s.id = " + table + ".shop_id WHERE p.id = " + table + ".product_id AND p.tenant_id = s.tenant_id AND p.tenant_id <> 0) WHERE tenant_id = 0 AND EXISTS (SELECT 1 FROM products p JOIN shops s ON s.id = " + table + ".shop_id WHERE p.id = " + table + ".product_id AND p.tenant_id = s.tenant_id AND p.tenant_id <> 0)"
		}
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("backfill %s tenant: %w", table, err)
		}
	}
	// A zero tenant is a valid system tenant, so it cannot be rewritten as a
	// marker. Rows whose product and shop disagree are deliberately left at zero
	// and reported for operator remediation instead of being guessed from either
	// side.
	for _, table := range []string{"product_publish_tasks", "product_publications"} {
		var count int64
		if err := db.Table(table + " AS x").Joins("JOIN products p ON p.id = x.product_id").Joins("JOIN shops s ON s.id = x.shop_id").Where("x.tenant_id = 0 AND p.tenant_id <> s.tenant_id").Count(&count).Error; err != nil {
			return fmt.Errorf("count quarantined %s tenant rows: %w", table, err)
		}
		if count > 0 {
			db.Logger.Warn(nil, "product publish tenant backfill left mismatched rows unassigned", "table", table, "count", count)
		}
	}
	batchStmt := "UPDATE product_publish_batches AS b SET tenant_id = src.tenant_id FROM (SELECT batch_id, MIN(tenant_id) AS tenant_id FROM product_publish_tasks WHERE batch_id IS NOT NULL AND tenant_id <> 0 GROUP BY batch_id HAVING COUNT(DISTINCT tenant_id) = 1) src WHERE b.id = src.batch_id AND b.tenant_id = 0"
	if !postgres {
		batchStmt = "UPDATE product_publish_batches SET tenant_id = (SELECT MIN(t.tenant_id) FROM product_publish_tasks t WHERE t.batch_id = product_publish_batches.id AND t.tenant_id <> 0 GROUP BY t.batch_id HAVING COUNT(DISTINCT t.tenant_id) = 1) WHERE tenant_id = 0 AND EXISTS (SELECT 1 FROM product_publish_tasks t WHERE t.batch_id = product_publish_batches.id AND t.tenant_id <> 0 GROUP BY t.batch_id HAVING COUNT(DISTINCT t.tenant_id) = 1)"
	}
	if err := db.Exec(batchStmt).Error; err != nil {
		return fmt.Errorf("backfill publish batches tenant: %w", err)
	}
	return nil
}

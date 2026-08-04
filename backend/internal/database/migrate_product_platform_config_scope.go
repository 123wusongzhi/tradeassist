package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

// migrateProductPlatformConfigShopScope upgrades the historical
// (product_id, platform) uniqueness to a scope-aware key. Existing Ozon rows
// with shop_id are assigned to that shop; every other legacy row keeps its
// former singleton scope. This is additive and does not duplicate or rewrite
// the saved JSON payloads.
func migrateProductPlatformConfigShopScope(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&product.ProductPlatformPublishConfig{}) {
		return nil
	}
	model := &product.ProductPlatformPublishConfig{}
	if !db.Migrator().HasColumn(model, "config_scope_key") {
		if err := db.Migrator().AddColumn(model, "ConfigScopeKey"); err != nil {
			return fmt.Errorf("add product platform config scope key: %w", err)
		}
	}
	shopCast := "CAST(shop_id AS TEXT)"
	if db.Dialector.Name() == "mysql" {
		shopCast = "CAST(shop_id AS CHAR)"
	}
	if err := db.Exec(`UPDATE product_platform_publish_configs
		SET config_scope_key = CASE
			WHEN LOWER(platform) = 'ozon'
				AND shop_id IS NOT NULL
				AND LOWER(TRIM(` + shopCast + `)) <> '00000000-0000-0000-0000-000000000000'
			THEN ` + shopCast + `
			ELSE 'legacy'
		END
		WHERE config_scope_key IS NULL OR config_scope_key = '' OR config_scope_key = 'legacy'`).Error; err != nil {
		return fmt.Errorf("backfill product platform config scope key: %w", err)
	}
	if db.Migrator().HasIndex(model, "idx_product_platform_publish_config") {
		if err := db.Migrator().DropIndex(model, "idx_product_platform_publish_config"); err != nil {
			return fmt.Errorf("drop legacy product platform config unique index: %w", err)
		}
	}
	if !db.Migrator().HasIndex(model, "ux_product_platform_publish_config_scope") {
		if err := db.Migrator().CreateIndex(model, "ux_product_platform_publish_config_scope"); err != nil {
			return fmt.Errorf("create product platform config scope index: %w", err)
		}
	}
	return nil
}

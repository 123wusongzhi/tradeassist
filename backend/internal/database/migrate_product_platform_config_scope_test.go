package database

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

func TestMigrateProductPlatformConfigShopScopePreservesLegacyRowsAndAllowsIndependentOzonStores(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:platform_scope_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE product_platform_publish_configs (
		id text primary key,
		product_id text not null,
		platform text not null,
		shop_id text
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX idx_product_platform_publish_config ON product_platform_publish_configs(product_id, platform)`).Error; err != nil {
		t.Fatal(err)
	}
	productID, nilShopProductID, firstShop, secondShop := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := db.Exec(`INSERT INTO product_platform_publish_configs(id, product_id, platform, shop_id) VALUES (?, ?, 'ozon', ?)`, uuid.NewString(), productID.String(), firstShop.String()).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO product_platform_publish_configs(id, product_id, platform, shop_id) VALUES (?, ?, 'ozon', ?)`, uuid.NewString(), nilShopProductID.String(), uuid.Nil.String()).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateProductPlatformConfigShopScope(db); err != nil {
		t.Fatal(err)
	}
	var firstScope string
	if err := db.Raw(`SELECT config_scope_key FROM product_platform_publish_configs WHERE product_id = ? AND platform = 'ozon'`, productID.String()).Scan(&firstScope).Error; err != nil {
		t.Fatal(err)
	}
	if firstScope != firstShop.String() {
		t.Fatalf("legacy Ozon row scope=%q want %q", firstScope, firstShop.String())
	}
	var nilShopScope string
	if err := db.Raw(`SELECT config_scope_key FROM product_platform_publish_configs WHERE product_id = ? AND platform = 'ozon'`, nilShopProductID.String()).Scan(&nilShopScope).Error; err != nil {
		t.Fatal(err)
	}
	if nilShopScope != product.PlatformConfigLegacyScope {
		t.Fatalf("nil UUID Ozon row scope=%q want legacy", nilShopScope)
	}
	if err := db.Exec(`INSERT INTO product_platform_publish_configs(id, product_id, platform, shop_id, config_scope_key) VALUES (?, ?, 'ozon', ?, ?)`, uuid.NewString(), productID.String(), secondShop.String(), secondShop.String()).Error; err != nil {
		t.Fatalf("new store scope should not overwrite first store: %v", err)
	}
	var count int64
	if err := db.Table(product.ProductPlatformPublishConfig{}.TableName()).Where("product_id = ? AND platform = ?", productID, "ozon").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("rows=%d err=%v", count, err)
	}
}

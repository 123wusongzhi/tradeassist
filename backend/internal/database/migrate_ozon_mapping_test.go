package database

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

func TestMigrateOzonCategoryMappingScopeBackfillsShopRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:migrate_ozon_mapping?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&shop.OzonCategoryMapping{}); err != nil {
		t.Fatal(err)
	}
	shopA, shopB := uuid.New(), uuid.New()
	for _, shopID := range []uuid.UUID{shopA, shopB} {
		row := shop.OzonCategoryMapping{TenantID: 7, ShopID: &shopID, SourceCategoryKey: "мебель", CategoryID: "100:200", Status: shop.OzonMappingActive}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := migrateOzonCategoryMappingScope(db); err != nil {
		t.Fatal(err)
	}
	var rows []shop.OzonCategoryMapping
	if err := db.Order("scope_key ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ScopeKey == "tenant" || rows[1].ScopeKey == "tenant" || rows[0].ScopeKey == rows[1].ScopeKey {
		t.Fatalf("shop scopes were not backfilled: %+v", rows)
	}
	duplicate := shop.OzonCategoryMapping{TenantID: 7, ShopID: &shopA, ScopeKey: shopA.String(), SourceCategoryKey: "мебель", CategoryID: "100:201", Status: shop.OzonMappingActive}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("expected unique tenant/scope/source constraint")
	}
}

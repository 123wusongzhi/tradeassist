package productcheck

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestOzonCategoryConfirmationRequiresActiveMatchingMapping(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&shop.OzonCategoryMapping{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	shopID := uuid.New()
	prod := product.Product{TenantID: 7}
	base := product.ProductPlatformPublishConfig{
		ShopID: &shopID, CategoryID: "100:200", CategoryPath: "电子产品 / 手机",
		SourceCategoryKey: "source-phone", SchemaConfirmedAt: &now,
	}
	svc := &Service{DB: db}

	if checks := svc.checkOzonCategoryConfirmation(context.Background(), prod, base, "schema-v1"); !hasOzonConfirmationCode(checks, "OZON_CATEGORY_MAPPING_UNCONFIRMED") {
		t.Fatalf("missing mapping must block, got %+v", checks)
	}
	mapping := shop.OzonCategoryMapping{
		TenantID: 7, ShopID: &shopID, ScopeKey: shopID.String(), SourceCategoryKey: "source-phone",
		CategoryID: "300:400", CategoryPath: "电子产品 / 3D 眼镜", Status: shop.OzonMappingActive,
		SchemaHash: "schema-v1", SelectionMethod: "manual", ConfirmationReason: "商品用途和规格与手机叶子类目一致", ConfirmedAt: &now,
	}
	if err := db.Create(&mapping).Error; err != nil {
		t.Fatal(err)
	}
	checks := svc.checkOzonCategoryConfirmation(context.Background(), prod, base, "schema-v1")
	if !hasOzonConfirmationCode(checks, "OZON_CATEGORY_MAPPING_CONFLICT") {
		t.Fatalf("conflicting mapping must block, got %+v", checks)
	}
	if err := db.Model(&mapping).Updates(map[string]any{"category_id": base.CategoryID, "category_path": base.CategoryPath}).Error; err != nil {
		t.Fatal(err)
	}
	if checks := svc.checkOzonCategoryConfirmation(context.Background(), prod, base, "schema-v1"); len(checks) != 0 {
		t.Fatalf("matching confirmed mapping should pass, got %+v", checks)
	}
}

func TestOzonCategoryConfirmationRequiresHumanEvidenceReason(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&shop.OzonCategoryMapping{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	shopID := uuid.New()
	prod := product.Product{TenantID: 11}
	cfg := product.ProductPlatformPublishConfig{
		ShopID: &shopID, CategoryID: "100:200", CategoryPath: "住宅和花园 / 收纳 / 储物箱",
		SourceCategoryKey: "storage-box", SchemaConfirmedAt: &now,
	}
	if err := db.Create(&shop.OzonCategoryMapping{
		TenantID: 11, ShopID: &shopID, ScopeKey: shopID.String(), SourceCategoryKey: cfg.SourceCategoryKey,
		CategoryID: cfg.CategoryID, CategoryPath: cfg.CategoryPath, Status: shop.OzonMappingActive,
		SchemaHash: "schema-v1", ConfirmedAt: &now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	checks := (&Service{DB: db}).checkOzonCategoryConfirmation(context.Background(), prod, cfg, "schema-v1")
	if !hasOzonConfirmationCode(checks, "OZON_CATEGORY_MAPPING_EVIDENCE_INCOMPLETE") {
		t.Fatalf("missing human reason must block, got %+v", checks)
	}
}

func TestOzonCategoryConfirmationRequiresProductConfirmationAndCurrentSchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&shop.OzonCategoryMapping{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	prod := product.Product{TenantID: 9}
	cfg := product.ProductPlatformPublishConfig{CategoryID: "100:200", SourceCategoryKey: "chair"}
	if err := db.Create(&shop.OzonCategoryMapping{
		TenantID: 9, ScopeKey: "tenant", SourceCategoryKey: "chair", CategoryID: cfg.CategoryID,
		CategoryPath: "家具 / 椅子", Status: shop.OzonMappingActive, SchemaHash: "old-schema", ConfirmedAt: &now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	checks := (&Service{DB: db}).checkOzonCategoryConfirmation(context.Background(), prod, cfg, "new-schema")
	for _, code := range []string{"OZON_CATEGORY_NOT_CONFIRMED", "OZON_CATEGORY_PATH_MISSING", "OZON_CATEGORY_MAPPING_SCHEMA_CHANGED"} {
		if !hasOzonConfirmationCode(checks, code) {
			t.Fatalf("expected %s, got %+v", code, checks)
		}
	}
}

func TestOzonSKUStockFailsClosedForUnknownAndWarnsForZero(t *testing.T) {
	zero, positive, negative := 0, 3, -1
	checks := checkOzonSKUStock([]product.ProductSKU{
		{SKUName: "unknown"},
		{SKUName: "zero", Stock: &zero},
		{SKUName: "positive", Stock: &positive},
		{SKUName: "negative", Stock: &negative},
	})
	if len(checks) != 2 || checks[0].Code != "OZON_SKU_STOCK_UNCONFIRMED" || checks[1].Code != "OZON_SKU_STOCK_ZERO" {
		t.Fatalf("unexpected Ozon stock checks: %+v", checks)
	}
	if checks[0].Level != levelError || checks[1].Level != levelWarning {
		t.Fatalf("unexpected Ozon stock levels: %+v", checks)
	}
}

func TestOzonContractCurrencyCheckBlocksMismatch(t *testing.T) {
	resolved := product.OzonResolvedListingDTO{Currency: product.OzonResolvedString{Value: "RUB"}}
	check := ozonContractCurrencyCheck(resolved, "cny")
	if check == nil || check.Code != "OZON_CURRENCY_CONTRACT_MISMATCH" || check.Level != levelError {
		t.Fatalf("expected contract currency blocker, got %+v", check)
	}
	resolved.Currency.Value = "CNY"
	if check := ozonContractCurrencyCheck(resolved, "cny"); check != nil {
		t.Fatalf("matching contract currency must pass: %+v", check)
	}
}

func hasOzonConfirmationCode(checks []CheckItem, code string) bool {
	for _, check := range checks {
		if check.Code == code {
			return true
		}
	}
	return false
}

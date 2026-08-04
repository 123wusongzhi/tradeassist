package product

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

func TestResolveOzonListingUsesShopOverridesAndNeverCopiesInventory(t *testing.T) {
	firstPrice, secondPrice := 10.0, 20.0
	firstStock, secondStock := 3, 7
	first := ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUCode: "ONE", SKUName: "One", Price: &firstPrice, Stock: &firstStock, ImageURL: "https://img.example/one.jpg"}
	second := ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUCode: "TWO", SKUName: "Two", Price: &secondPrice, Stock: &secondStock, ImageURL: "https://img.example/two.jpg"}
	productRow := Product{Base: model.Base{ID: uuid.New()}, Title: "Local title", Description: "Local description", SKUs: []ProductSKU{first, second}}
	listingRaw, err := json.Marshal(OzonListingConfigInput{
		Version: OzonListingConfigVersion, TitleOverride: "Ozon title", CurrencyCode: "RUB",
		SKUPriceOverrides: map[string]float64{first.ID.String(): 12.5},
		Package:           OzonPackageConfigInput{WarehouseID: "123", VAT: "0.1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	shopID := uuid.New()
	cfg := ProductPlatformPublishConfig{ShopID: &shopID, CategoryID: "100:200", CategoryPath: "Furniture", SchemaHash: "schema-v1", ListingConfig: datatypes.JSON(listingRaw)}
	resolved := ResolveOzonListing(productRow, &cfg, map[string]string{
		"default_weight": "100", "default_width": "200", "default_height": "300", "default_depth": "400",
	}, "CNY")
	if !resolved.CanSubmit || resolved.Title.Value != "Ozon title" || resolved.Title.Source != OzonValueSourceShopConfig || resolved.Currency.Value != "RUB" {
		t.Fatalf("unexpected resolved listing: %+v", resolved)
	}
	bySKU := map[uuid.UUID]OzonResolvedSKUListingDTO{}
	for _, sku := range resolved.SKUs {
		bySKU[sku.SKUID] = sku
	}
	if bySKU[first.ID].Price.Value != 12.5 || bySKU[first.ID].Price.Source != OzonValueSourceShopConfig {
		t.Fatalf("first SKU override missing: %+v", bySKU[first.ID])
	}
	if bySKU[second.ID].Price.Value != secondPrice || bySKU[second.ID].Price.Source != OzonValueSourceProduct {
		t.Fatalf("second SKU local fallback missing: %+v", bySKU[second.ID])
	}
	if bySKU[first.ID].LocalStock != firstStock || bySKU[second.ID].LocalStock != secondStock || bySKU[first.ID].StockSource != OzonValueSourceLocalStock {
		t.Fatalf("inventory must remain local source: %+v", resolved.SKUs)
	}
	decoded, _, err := DecodeOzonListingConfig(cfg.ListingConfig)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(decoded)
	if string(encoded) == "" || len(decoded.SKUPriceOverrides) != 1 {
		t.Fatalf("listing config unexpectedly contains derived inventory: %s", encoded)
	}
}

func TestResolveOzonListingBlocksMissingPackageAfterPresetFallback(t *testing.T) {
	price, stock := 9.9, 1
	sku := ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUName: "Default", Price: &price, Stock: &stock, ImageURL: "https://img.example/main.jpg"}
	p := Product{Base: model.Base{ID: uuid.New()}, Title: "Title", Description: "Description", SKUs: []ProductSKU{sku}}
	resolved := ResolveOzonListing(p, &ProductPlatformPublishConfig{}, map[string]string{"currency_code": "RUB", "warehouse_id": "10"}, "")
	if resolved.CanSubmit || resolved.ErrorCount != 6 {
		t.Fatalf("missing category, schema and four dimensions must block: %+v", resolved)
	}
}

func TestNormalizeOzonListingConfigRejectsNonAlphabeticCurrency(t *testing.T) {
	_, err := NormalizeOzonListingConfigInput(Product{}, OzonListingConfigInput{CurrencyCode: "123"})
	if err == nil {
		t.Fatal("numeric currency code must be rejected")
	}
}

func TestHydrateOzonListingDTORejectsCorruptSavedConfig(t *testing.T) {
	service := &Service{}
	out := &PlatformPublishConfigDTO{}
	err := service.hydrateOzonListingDTO(
		context.Background(),
		Product{},
		&ProductPlatformPublishConfig{ListingConfig: datatypes.JSON([]byte(`{"version":`))},
		out,
	)
	if err == nil {
		t.Fatal("corrupt saved listing config must not be replaced with editable defaults")
	}
}

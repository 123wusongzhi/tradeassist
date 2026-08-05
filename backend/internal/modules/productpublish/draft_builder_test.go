package productpublish

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

func TestBuildPlatformDraftForPlatformMapsSavedOzonImagesPerSKU(t *testing.T) {
	red := product.ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUCode: "RED", SKUName: "红色", ImageURL: "https://img.example/red.jpg"}
	blue := product.ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUCode: "BLUE", SKUName: "蓝色", ImageURL: "https://img.example/blue.jpg"}
	sharedA := product.ProductImage{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, ImageType: product.ImageTypeMain, PublicURL: "https://img.example/shared-a.jpg", SortOrder: 1}
	sharedB := product.ProductImage{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, ImageType: product.ImageTypeDetail, PublicURL: "https://img.example/shared-b.jpg", SortOrder: 2}
	p := product.Product{Base: model.Base{ID: uuid.New()}, TenantID: 7, Title: "测试商品", Currency: "RUB", Images: []product.ProductImage{sharedA, sharedB}, SKUs: []product.ProductSKU{red, blue}}
	raw, err := product.MarshalOzonImageConfig(p, product.OzonImageConfigInput{SKUSelections: []product.OzonSKUImageSelection{
		{SKUID: red.ID, AdditionalImageIDs: []uuid.UUID{sharedA.ID}},
		{SKUID: blue.ID, AdditionalImageIDs: []uuid.UUID{sharedB.ID}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	draft, err := BuildPlatformDraftForPlatform(p, "ozon", &product.ProductPlatformPublishConfig{MappedImages: datatypes.JSON(raw)})
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.SKUs) != 2 {
		t.Fatalf("SKU count = %d", len(draft.SKUs))
	}
	byCode := map[string][]string{}
	for _, sku := range draft.SKUs {
		for _, image := range sku.Images {
			byCode[sku.SKUCode] = append(byCode[sku.SKUCode], image.URL)
		}
	}
	assertStringSlice(t, byCode["RED"], red.ImageURL, sharedA.PublicURL)
	assertStringSlice(t, byCode["BLUE"], blue.ImageURL, sharedB.PublicURL)
}

func TestBuildPlatformDraftForPlatformBlocksNamedOzonSKUWithoutMainImage(t *testing.T) {
	sku := product.ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUCode: "BLUE-L", SKUName: "蓝色 / L"}
	p := product.Product{Base: model.Base{ID: uuid.New()}, Title: "测试商品", SKUs: []product.ProductSKU{sku}}
	_, err := BuildPlatformDraftForPlatform(p, "ozon", nil)
	if err == nil || !strings.Contains(err.Error(), "蓝色 / L") || !strings.Contains(err.Error(), "缺少原始主图") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildOzonPlatformDraftFromResolvedUsesSameEffectiveSKUValues(t *testing.T) {
	price, stock := 10.0, 6
	const largeComplexID int64 = 9007199254740993
	sku := product.ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUCode: "SKU-1", SKUName: "Blue", Price: &price, Stock: &stock, ImageURL: "https://img.example/blue.jpg"}
	p := product.Product{Base: model.Base{ID: uuid.New()}, TenantID: 7, Title: "Local", Description: "Local description", Currency: "CNY", SKUs: []product.ProductSKU{sku}}
	resolved := product.OzonResolvedListingDTO{
		ProductID:   p.ID,
		Title:       product.OzonResolvedString{Value: "Ozon title", Source: product.OzonValueSourceShopConfig},
		Description: product.OzonResolvedString{Value: "Ozon description", Source: product.OzonValueSourceShopConfig},
		Currency:    product.OzonResolvedString{Value: "RUB", Source: product.OzonValueSourceShopConfig},
		SKUs: []product.OzonResolvedSKUListingDTO{{
			SKUID: sku.ID, SKUCode: sku.SKUCode, SKUName: sku.SKUName,
			Price:      product.OzonResolvedFloat{Value: 12.5, Source: product.OzonValueSourceShopConfig},
			LocalStock: stock, StockSource: product.OzonValueSourceLocalStock,
			Images: []product.OzonResolvedImageDTO{{URL: sku.ImageURL, Source: product.OzonImageSourceSKUOriginal, ImageType: product.ImageTypeMain, Position: 1}},
			PlatformAttributes: product.OzonEffectiveAttributePayload{
				Version:                product.OzonPlatformAttributesVersion,
				Attributes:             map[string][]product.OzonAttributeSelection{"10096": {{Value: "Blue"}}},
				ComplexGroups:          []product.OzonComplexAttributeGroup{{ComplexID: largeComplexID, Attributes: map[string][]product.OzonAttributeSelection{"200": {{Value: "Cotton"}}}}},
				SKUVariantAttributeIDs: []string{"10096"},
			},
			CanSubmit: true,
		}},
		CanSubmit: true,
	}
	draft, err := BuildOzonPlatformDraftFromResolved(p, resolved)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Title != "Ozon title" || draft.Description != "Ozon description" || draft.Currency != "RUB" || len(draft.SKUs) != 1 || draft.SKUs[0].Price != 12.5 || draft.SKUs[0].Stock != stock || len(draft.SKUs[0].Images) != 1 {
		t.Fatalf("draft diverged from resolved preview: %+v", draft)
	}
	if version, ok := draft.SKUs[0].PlatformAttributes["version"].(json.Number); !ok || version.String() != "3" {
		t.Fatalf("resolved SKU attributes were not copied to immutable draft: %+v", draft.SKUs[0].PlatformAttributes)
	}
	groups, ok := draft.SKUs[0].PlatformAttributes["complexGroups"].([]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("resolved complex groups were not copied: %+v", draft.SKUs[0].PlatformAttributes)
	}
	group, ok := groups[0].(map[string]any)
	if !ok {
		t.Fatalf("resolved complex group has unexpected type: %+v", groups[0])
	}
	complexID, ok := group["complexId"].(json.Number)
	if !ok || complexID.String() != "9007199254740993" {
		t.Fatalf("large Ozon complex ID lost precision: %#v", group["complexId"])
	}
}

func assertStringSlice(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

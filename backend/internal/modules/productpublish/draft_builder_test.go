package productpublish

import (
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

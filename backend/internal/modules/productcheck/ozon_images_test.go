package productcheck

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

func TestCheckOzonSKUImagesBlocksAndIdentifiesMissingSKU(t *testing.T) {
	missing := product.ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUCode: "BLUE-L", SKUName: "蓝色 / L"}
	p := product.Product{SKUs: []product.ProductSKU{missing}}
	checks := checkOzonSKUImages(p, nil)
	if len(checks) != 1 {
		t.Fatalf("checks = %+v", checks)
	}
	check := checks[0]
	if check.Code != "OZON_SKU_MAIN_IMAGE_MISSING" || check.Level != levelError || check.RelatedResourceID != missing.ID.String() {
		t.Fatalf("check = %+v", check)
	}
	if !strings.Contains(check.Message, missing.SKUName) {
		t.Fatalf("message does not identify SKU: %q", check.Message)
	}
}

func TestCheckOzonSKUImagesAcceptsExplicitSavedFallback(t *testing.T) {
	sku := product.ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUCode: "NO-ORIGINAL"}
	image := product.ProductImage{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, ImageType: product.ImageTypeMain, PublicURL: "https://cdn.example.test/fallback.jpg"}
	p := product.Product{Images: []product.ProductImage{image}, SKUs: []product.ProductSKU{sku}}
	raw, err := product.MarshalOzonImageConfig(p, product.OzonImageConfigInput{SKUSelections: []product.OzonSKUImageSelection{{SKUID: sku.ID, FallbackMainImageID: &image.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	if checks := checkOzonSKUImages(p, raw); len(checks) != 0 {
		t.Fatalf("checks = %+v", checks)
	}
}

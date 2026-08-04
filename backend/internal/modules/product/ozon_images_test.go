package product

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

func TestResolveOzonImageConfigUsesPerSKUImagesInStableDeduplicatedOrder(t *testing.T) {
	redSKU := ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUCode: "RED", SKUName: "红色", ImageURL: "https://img.example/red.jpg"}
	blueSKU := ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUCode: "BLUE", SKUName: "蓝色", ImageURL: "https://img.example/blue.jpg"}
	fallbackSKU := ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUCode: "NO-IMAGE", SKUName: "缺图"}
	shared := ProductImage{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, ImageType: ImageTypeMain, PublicURL: "https://img.example/shared.jpg", SortOrder: 1}
	duplicateRed := ProductImage{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, ImageType: ImageTypeDetail, PublicURL: redSKU.ImageURL, SortOrder: 2}
	detail := ProductImage{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, ImageType: ImageTypeDetail, PublicURL: "https://img.example/detail.jpg", SortOrder: 3}
	p := Product{Images: []ProductImage{detail, duplicateRed, shared}, SKUs: []ProductSKU{redSKU, blueSKU, fallbackSKU}}

	raw, err := MarshalOzonImageConfig(p, OzonImageConfigInput{
		Version: OzonImageConfigVersion,
		SKUSelections: []OzonSKUImageSelection{
			{SKUID: redSKU.ID, AdditionalImageIDs: []uuid.UUID{shared.ID, duplicateRed.ID, shared.ID, detail.ID}},
			{SKUID: blueSKU.ID, AdditionalImageIDs: []uuid.UUID{detail.ID}},
			{SKUID: fallbackSKU.ID, FallbackMainImageID: &shared.ID, AdditionalImageIDs: []uuid.UUID{shared.ID, detail.ID}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	view := ResolveOzonImageConfig(p, raw)
	if !view.Configured || view.ErrorCount != 0 || len(view.SKUs) != 3 {
		t.Fatalf("unexpected resolved config: %+v", view)
	}
	byID := make(map[uuid.UUID]OzonSKUImageDTO, len(view.SKUs))
	for _, sku := range view.SKUs {
		byID[sku.SKUID] = sku
	}
	assertOzonImageURLs(t, byID[redSKU.ID].FinalImages, redSKU.ImageURL, shared.PublicURL, detail.PublicURL)
	assertOzonImageURLs(t, byID[blueSKU.ID].FinalImages, blueSKU.ImageURL, detail.PublicURL)
	assertOzonImageURLs(t, byID[fallbackSKU.ID].FinalImages, shared.PublicURL, detail.PublicURL)
	if byID[fallbackSKU.ID].FinalImages[0].Source != OzonImageSourceManualFallback {
		t.Fatalf("fallback source = %q", byID[fallbackSKU.ID].FinalImages[0].Source)
	}
}

func TestResolveOzonImageConfigLegacyDefaultUsesOnlySKUOriginalAndNamesMissingSKU(t *testing.T) {
	withImage := ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUCode: "WITH", SKUName: "有图", ImageURL: "https://img.example/own.jpg"}
	missing := ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUCode: "MISSING", SKUName: "蓝色 / L"}
	p := Product{
		Images: []ProductImage{{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, ImageType: ImageTypeMain, PublicURL: "https://img.example/product.jpg"}},
		SKUs:   []ProductSKU{withImage, missing},
	}
	view := ResolveOzonImageConfig(p, nil)
	if view.Configured || view.CompatibilityMode != "sku_original_only" {
		t.Fatalf("legacy mode = %+v", view)
	}
	byID := map[uuid.UUID]OzonSKUImageDTO{}
	for _, row := range view.SKUs {
		byID[row.SKUID] = row
	}
	assertOzonImageURLs(t, byID[withImage.ID].FinalImages, withImage.ImageURL)
	missingView := byID[missing.ID]
	if view.ErrorCount != 1 || len(missingView.Issues) != 1 || missingView.Issues[0].Code != "OZON_SKU_MAIN_IMAGE_MISSING" {
		t.Fatalf("missing SKU issues = %+v", missingView.Issues)
	}
	if got := missingView.Issues[0].Message; got == "" || !containsAll(got, "蓝色 / L", "缺少原始主图") {
		t.Fatalf("missing SKU message = %q", got)
	}
}

func TestNormalizeOzonImageConfigRejectsUntraceableReferences(t *testing.T) {
	sku := ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUName: "SKU"}
	p := Product{SKUs: []ProductSKU{sku}}
	unknown := uuid.New()
	_, err := NormalizeOzonImageConfigInput(p, OzonImageConfigInput{
		Version:       OzonImageConfigVersion,
		SKUSelections: []OzonSKUImageSelection{{SKUID: sku.ID, FallbackMainImageID: &unknown}},
	})
	if err == nil {
		t.Fatal("expected unknown fallback image to be rejected")
	}
	_, err = NormalizeOzonImageConfigInput(p, OzonImageConfigInput{
		Version:       OzonImageConfigVersion,
		SKUSelections: []OzonSKUImageSelection{{SKUID: sku.ID, AdditionalImageIDs: []uuid.UUID{uuid.Nil}}},
	})
	if err == nil {
		t.Fatal("expected empty additional image reference to be rejected")
	}
}

func TestOzonImageConfigJSONRoundTripKeepsExplicitFallback(t *testing.T) {
	sku := ProductSKU{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, SKUName: "SKU"}
	image := ProductImage{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, ImageType: ImageTypeMain, PublicURL: "https://img.example/fallback.jpg"}
	p := Product{Images: []ProductImage{image}, SKUs: []ProductSKU{sku}}
	raw, err := MarshalOzonImageConfig(p, OzonImageConfigInput{SKUSelections: []OzonSKUImageSelection{{SKUID: sku.ID, FallbackMainImageID: &image.ID, AdditionalImageIDs: []uuid.UUID{}}}})
	if err != nil {
		t.Fatal(err)
	}
	var persisted OzonImageConfigInput
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Version != OzonImageConfigVersion || len(persisted.SKUSelections) != 1 || persisted.SKUSelections[0].FallbackMainImageID == nil || *persisted.SKUSelections[0].FallbackMainImageID != image.ID {
		t.Fatalf("persisted config = %+v", persisted)
	}
	view := ResolveOzonImageConfig(p, raw)
	assertOzonImageURLs(t, view.SKUs[0].FinalImages, image.PublicURL)
}

func assertOzonImageURLs(t *testing.T, images []OzonResolvedImageDTO, want ...string) {
	t.Helper()
	if len(images) != len(want) {
		t.Fatalf("image count = %d, want %d: %+v", len(images), len(want), images)
	}
	for i := range want {
		if images[i].URL != want[i] || images[i].Position != i+1 {
			t.Fatalf("image %d = %+v, want URL %q at position %d", i, images[i], want[i], i+1)
		}
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}

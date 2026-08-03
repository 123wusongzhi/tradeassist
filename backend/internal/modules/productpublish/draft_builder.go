package productpublish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/gorm"
)

// BuildPlatformDraftFromProduct maps a hydrated product.Product into a provider-neutral listing draft (no encryption).
func BuildPlatformDraftFromProduct(p product.Product) (platformp.PlatformProductDraft, error) {
	return buildPlatformDraftFromProduct(p, true)
}

// BuildPlatformDraftForPlatform applies platform-owned product preparation
// without changing category, pricing or inventory semantics. Ozon resolves a
// separate image list for every SKU from the saved product-level configuration.
func BuildPlatformDraftForPlatform(p product.Product, platform string, cfg *product.ProductPlatformPublishConfig) (platformp.PlatformProductDraft, error) {
	if !strings.EqualFold(strings.TrimSpace(platform), "ozon") {
		return BuildPlatformDraftFromProduct(p)
	}
	draft, err := buildPlatformDraftFromProduct(p, false)
	if err != nil {
		return platformp.PlatformProductDraft{}, err
	}
	var rawImages []byte
	if cfg != nil {
		rawImages = cfg.MappedImages
	}
	resolved := product.ResolveOzonImageConfig(p, rawImages)
	if err := resolved.ValidationError(); err != nil {
		return platformp.PlatformProductDraft{}, err
	}
	bySKU := make(map[uuid.UUID]product.OzonSKUImageDTO, len(resolved.SKUs))
	for _, sku := range resolved.SKUs {
		bySKU[sku.SKUID] = sku
	}
	for i := range draft.SKUs {
		plan, ok := bySKU[draft.SKUs[i].LocalSKUID]
		if !ok {
			return platformp.PlatformProductDraft{}, fmt.Errorf("Ozon image plan missing SKU %s", draft.SKUs[i].LocalSKUID)
		}
		draft.SKUs[i].Images = make([]platformp.PlatformProductImage, 0, len(plan.FinalImages))
		for _, image := range plan.FinalImages {
			draft.SKUs[i].Images = append(draft.SKUs[i].Images, platformp.PlatformProductImage{
				TenantID:  p.TenantID,
				URL:       image.URL,
				ObjectKey: image.ObjectKey,
				Type:      image.ImageType,
				SortOrder: image.Position,
			})
		}
	}
	return draft, nil
}

func buildPlatformDraftFromProduct(p product.Product, requireProductMain bool) (platformp.PlatformProductDraft, error) {
	title := strings.TrimSpace(p.Title)
	if title == "" {
		title = strings.TrimSpace(p.AITitle)
	}
	if title == "" {
		title = strings.TrimSpace(p.OriginalTitle)
	}
	desc := strings.TrimSpace(p.Description)
	if desc == "" {
		desc = strings.TrimSpace(p.AIDescription)
	}
	curr := strings.TrimSpace(p.Currency)
	if curr == "" {
		curr = "USD"
	}
	if title == "" {
		return platformp.PlatformProductDraft{}, fmt.Errorf("product title is required for publish")
	}
	imgs := append([]product.ProductImage(nil), p.Images...)
	if len(imgs) == 0 && requireProductMain {
		return platformp.PlatformProductDraft{}, fmt.Errorf("product main image required for publish")
	}
	sort.SliceStable(imgs, func(i, j int) bool {
		if imgs[i].SortOrder == imgs[j].SortOrder {
			return imgs[i].CreatedAt.Before(imgs[j].CreatedAt)
		}
		return imgs[i].SortOrder < imgs[j].SortOrder
	})

	hasMain := false
	plImgs := make([]platformp.PlatformProductImage, 0, len(imgs))
	for _, im := range imgs {
		imgType := strings.TrimSpace(strings.ToLower(im.ImageType))
		if imgType == product.ImageTypeDescription {
			imgType = product.ImageTypeDetail
		}
		url := strings.TrimSpace(im.PublicURL)
		if url == "" {
			url = strings.TrimSpace(im.OriginURL)
		}
		if url == "" {
			continue
		}
		if imgType == product.ImageTypeMain {
			hasMain = true
		}
		plImgs = append(plImgs, platformp.PlatformProductImage{
			TenantID:  p.TenantID,
			URL:       url,
			ObjectKey: strings.TrimSpace(im.ObjectKey),
			Type:      imgType,
			SortOrder: im.SortOrder,
		})
	}
	if !hasMain {
		for i := range plImgs {
			if strings.TrimSpace(plImgs[i].Type) != product.ImageTypeSKU {
				plImgs[i].Type = product.ImageTypeMain
				hasMain = true
				break
			}
		}
	}
	if !hasMain && requireProductMain {
		return platformp.PlatformProductDraft{}, fmt.Errorf("product main image required for publish")
	}

	if len(p.SKUs) == 0 {
		return platformp.PlatformProductDraft{}, fmt.Errorf("product SKU is required for publish")
	}

	var attrs map[string]any
	if len(p.RawData) > 0 {
		var top map[string]any
		_ = json.Unmarshal(p.RawData, &top)
		if attrs == nil && top != nil {
			if raw, ok := top["attributes"].([]any); ok {
				attrs = map[string]any{"attributes": raw}
			} else if a, ok := top["attrs"].(map[string]any); ok {
				attrs = a
			}
		}
	}

	skus := make([]platformp.PlatformProductSKU, 0, len(p.SKUs))
	for _, s := range p.SKUs {
		pr := 0.0
		if s.Price != nil {
			pr = *s.Price
		}
		st := 0
		if s.Stock != nil {
			st = *s.Stock
		}
		var skuAttrs map[string]any
		if len(s.Attrs) > 0 {
			_ = json.Unmarshal(s.Attrs, &skuAttrs)
		}
		skus = append(skus, platformp.PlatformProductSKU{
			TenantID:   p.TenantID,
			LocalSKUID: s.ID,
			SKUCode:    strings.TrimSpace(s.SKUCode),
			SKUName:    strings.TrimSpace(s.SKUName),
			Attrs:      skuAttrs,
			Price:      pr,
			Stock:      st,
			ImageURL:   strings.TrimSpace(s.ImageURL),
		})
	}

	srcRow := platformp.TrimRawMap(map[string]any{
		"id":             p.ID.String(),
		"titleLen":       len([]rune(title)),
		"descriptionLen": len([]rune(desc)),
		"skuCount":       len(skus),
		"imageCount":     len(plImgs),
	}, 12, 120)

	return platformp.PlatformProductDraft{
		ProductID:        p.ID,
		Title:            title,
		Description:      desc,
		Currency:         curr,
		Images:           plImgs,
		SKUs:             skus,
		Attributes:       attrs,
		SourceProductRow: srcRow,
	}, nil
}

func (s *Service) buildPlatformDraftForProduct(ctx context.Context, p product.Product, platform string) (platformp.PlatformProductDraft, error) {
	if !strings.EqualFold(strings.TrimSpace(platform), "ozon") {
		return BuildPlatformDraftFromProduct(p)
	}
	var cfg product.ProductPlatformPublishConfig
	err := s.DB.WithContext(ctx).Where("product_id = ? AND platform = ?", p.ID, "ozon").First(&cfg).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return platformp.PlatformProductDraft{}, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return BuildPlatformDraftForPlatform(p, platform, nil)
	}
	return BuildPlatformDraftForPlatform(p, platform, &cfg)
}

func platformPayloadSnapshot(d platformp.PlatformProductDraft, merged map[string]string) map[string]any {
	price := 0.0
	stock := 0
	for i, sku := range d.SKUs {
		if i == 0 || (sku.Price > 0 && sku.Price < price) {
			price = sku.Price
		}
		stock += sku.Stock
	}
	return platformp.TrimRawMap(map[string]any{
		"platformTitle":       d.Title,
		"platformDescription": d.Description,
		"platformImages":      d.Images,
		"platformSkus":        d.SKUs,
		"platformPrice":       price,
		"platformStock":       stock,
		"platformCategory":    strings.TrimSpace(merged["category_id"]),
		"platformAttributes":  d.Attributes,
	}, 80, 500)
}

func taskImagesAndSKUsSnapshot(d platformp.PlatformProductDraft) ([]byte, []byte, *float64) {
	imgs, _ := json.Marshal(d.Images)
	skus, _ := json.Marshal(d.SKUs)
	var price *float64
	for i, sku := range d.SKUs {
		if sku.Price <= 0 {
			continue
		}
		if i == 0 || price == nil || sku.Price < *price {
			v := sku.Price
			price = &v
		}
	}
	return imgs, skus, price
}

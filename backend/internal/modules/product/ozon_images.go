package product

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const (
	OzonImageConfigVersion = 1
	OzonMaxImagesPerSKU    = 10

	OzonImageSourceSKUOriginal    = "sku_original"
	OzonImageSourceManualFallback = "manual_fallback"
	OzonImageSourceProductShared  = "product_shared"
)

// OzonImageConfigInput is the persisted, versioned operator selection. It
// deliberately stores product/SKU references instead of duplicating mutable
// image URLs.
type OzonImageConfigInput struct {
	Version       int                     `json:"version"`
	SKUSelections []OzonSKUImageSelection `json:"skuSelections"`
}

type OzonSKUImageSelection struct {
	SKUID               uuid.UUID   `json:"skuId"`
	FallbackMainImageID *uuid.UUID  `json:"fallbackMainImageId,omitempty"`
	AdditionalImageIDs  []uuid.UUID `json:"additionalImageIds"`
}

type OzonSharedImageDTO struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	ImageType string    `json:"imageType"`
	SortOrder int       `json:"sortOrder"`
}

type OzonResolvedImageDTO struct {
	ImageID   *uuid.UUID `json:"imageId,omitempty"`
	URL       string     `json:"url"`
	Source    string     `json:"source"`
	Position  int        `json:"position"`
	ImageType string     `json:"imageType,omitempty"`
	ObjectKey string     `json:"-"`
}

type OzonImageIssueDTO struct {
	Code       string     `json:"code"`
	Message    string     `json:"message"`
	Suggestion string     `json:"suggestion,omitempty"`
	SKUID      *uuid.UUID `json:"skuId,omitempty"`
}

type OzonSKUImageDTO struct {
	SKUID                uuid.UUID              `json:"skuId"`
	SKUCode              string                 `json:"skuCode,omitempty"`
	SKUName              string                 `json:"skuName,omitempty"`
	Attrs                json.RawMessage        `json:"attrs,omitempty"`
	OriginalMainImageURL string                 `json:"originalMainImageUrl,omitempty"`
	FallbackMainImageID  *uuid.UUID             `json:"fallbackMainImageId,omitempty"`
	AdditionalImageIDs   []uuid.UUID            `json:"additionalImageIds"`
	FinalImages          []OzonResolvedImageDTO `json:"finalImages"`
	CanPublish           bool                   `json:"canPublish"`
	Issues               []OzonImageIssueDTO    `json:"issues"`
}

type OzonImageConfigDTO struct {
	Version           int                  `json:"version"`
	Configured        bool                 `json:"configured"`
	CompatibilityMode string               `json:"compatibilityMode,omitempty"`
	MaxImagesPerSKU   int                  `json:"maxImagesPerSku"`
	SharedImages      []OzonSharedImageDTO `json:"sharedImages"`
	SKUs              []OzonSKUImageDTO    `json:"skus"`
	Issues            []OzonImageIssueDTO  `json:"issues"`
	ErrorCount        int                  `json:"errorCount"`
}

type ozonSharedImage struct {
	dto       OzonSharedImageDTO
	objectKey string
}

// NormalizeOzonImageConfigInput validates tenant-owned references and returns a
// stable representation suitable for mapped_images persistence. Missing SKU
// main images are allowed to be saved so the UI can display and fix them; the
// readiness check and adapter still block submission.
func NormalizeOzonImageConfigInput(p Product, input OzonImageConfigInput) (OzonImageConfigInput, error) {
	if input.Version == 0 {
		input.Version = OzonImageConfigVersion
	}
	if input.Version != OzonImageConfigVersion {
		return OzonImageConfigInput{}, fmt.Errorf("unsupported Ozon image config version: %d", input.Version)
	}

	skus := sortedOzonSKUs(p.SKUs)
	knownSKUs := make(map[uuid.UUID]ProductSKU, len(skus))
	for _, sku := range skus {
		knownSKUs[sku.ID] = sku
	}
	_, sharedByID := ozonSharedImages(p.Images)

	bySKU := make(map[uuid.UUID]OzonSKUImageSelection, len(input.SKUSelections))
	for _, selection := range input.SKUSelections {
		if selection.SKUID == uuid.Nil {
			return OzonImageConfigInput{}, fmt.Errorf("Ozon SKU image selection requires skuId")
		}
		sku, ok := knownSKUs[selection.SKUID]
		if !ok {
			return OzonImageConfigInput{}, fmt.Errorf("Ozon SKU image selection references an unknown SKU: %s", selection.SKUID)
		}
		if _, duplicate := bySKU[selection.SKUID]; duplicate {
			return OzonImageConfigInput{}, fmt.Errorf("Ozon SKU image selection is duplicated: %s", selection.SKUID)
		}
		if selection.FallbackMainImageID != nil {
			if strings.TrimSpace(sku.ImageURL) != "" {
				return OzonImageConfigInput{}, fmt.Errorf("SKU %s already has an original main image and cannot use a fallback", ozonSKUDisplayName(sku))
			}
			if _, ok := sharedByID[*selection.FallbackMainImageID]; !ok {
				return OzonImageConfigInput{}, fmt.Errorf("SKU %s fallback image is not an available product image", ozonSKUDisplayName(sku))
			}
		}
		for _, imageID := range selection.AdditionalImageIDs {
			if imageID == uuid.Nil {
				return OzonImageConfigInput{}, fmt.Errorf("SKU %s contains an empty additional image reference", ozonSKUDisplayName(sku))
			}
		}
		selection.AdditionalImageIDs = stableUniqueUUIDs(selection.AdditionalImageIDs)
		if len(selection.AdditionalImageIDs) > OzonMaxImagesPerSKU-1 {
			return OzonImageConfigInput{}, fmt.Errorf("SKU %s can select at most %d additional product images", ozonSKUDisplayName(sku), OzonMaxImagesPerSKU-1)
		}
		for _, imageID := range selection.AdditionalImageIDs {
			if _, ok := sharedByID[imageID]; !ok {
				return OzonImageConfigInput{}, fmt.Errorf("SKU %s references an unavailable product image: %s", ozonSKUDisplayName(sku), imageID)
			}
		}
		if selection.AdditionalImageIDs == nil {
			selection.AdditionalImageIDs = []uuid.UUID{}
		}
		bySKU[selection.SKUID] = selection
	}

	normalized := OzonImageConfigInput{
		Version:       OzonImageConfigVersion,
		SKUSelections: make([]OzonSKUImageSelection, 0, len(bySKU)),
	}
	for _, sku := range skus {
		if selection, ok := bySKU[sku.ID]; ok {
			normalized.SKUSelections = append(normalized.SKUSelections, selection)
		}
	}
	return normalized, nil
}

func MarshalOzonImageConfig(p Product, input OzonImageConfigInput) ([]byte, error) {
	normalized, err := NormalizeOzonImageConfigInput(p, input)
	if err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

// ResolveOzonImageConfig hydrates the saved references against the current
// product. The same resolved order is used by the Admin preview, readiness
// validation, immutable task snapshot and Ozon adapter.
func ResolveOzonImageConfig(p Product, raw []byte) OzonImageConfigDTO {
	config, configured, decodeErr := decodeOzonImageConfig(raw)
	view := OzonImageConfigDTO{
		Version:         OzonImageConfigVersion,
		Configured:      configured,
		MaxImagesPerSKU: OzonMaxImagesPerSKU,
		SharedImages:    []OzonSharedImageDTO{},
		SKUs:            []OzonSKUImageDTO{},
		Issues:          []OzonImageIssueDTO{},
	}
	if !configured {
		view.CompatibilityMode = "sku_original_only"
	}
	if decodeErr != nil {
		view.Issues = append(view.Issues, OzonImageIssueDTO{
			Code:       "OZON_IMAGE_CONFIG_INVALID",
			Message:    "已保存的 Ozon SKU 图片配置无法读取",
			Suggestion: "请在商品级 Ozon 配置中重新检查并保存 SKU 图片。",
		})
		config = OzonImageConfigInput{Version: OzonImageConfigVersion}
	}

	shared, sharedByID := ozonSharedImages(p.Images)
	for _, image := range shared {
		view.SharedImages = append(view.SharedImages, image.dto)
	}

	selections := make(map[uuid.UUID]OzonSKUImageSelection, len(config.SKUSelections))
	knownSKUs := make(map[uuid.UUID]struct{}, len(p.SKUs))
	for _, sku := range p.SKUs {
		knownSKUs[sku.ID] = struct{}{}
	}
	for _, selection := range config.SKUSelections {
		if selection.SKUID == uuid.Nil {
			view.Issues = append(view.Issues, OzonImageIssueDTO{Code: "OZON_IMAGE_CONFIG_INVALID", Message: "Ozon 图片配置包含空的 SKU 标识", Suggestion: "请重新保存 SKU 图片配置。"})
			continue
		}
		if _, exists := selections[selection.SKUID]; exists {
			skuID := selection.SKUID
			view.Issues = append(view.Issues, OzonImageIssueDTO{Code: "OZON_SKU_IMAGE_SELECTION_DUPLICATED", Message: "Ozon 图片配置包含重复的 SKU", Suggestion: "请重新保存 SKU 图片配置。", SKUID: &skuID})
			continue
		}
		if _, exists := knownSKUs[selection.SKUID]; !exists {
			skuID := selection.SKUID
			view.Issues = append(view.Issues, OzonImageIssueDTO{Code: "OZON_SKU_IMAGE_REFERENCE_STALE", Message: "Ozon 图片配置仍引用已删除的 SKU", Suggestion: "请重新保存 SKU 图片配置以移除失效引用。", SKUID: &skuID})
			continue
		}
		selections[selection.SKUID] = selection
	}

	for _, sku := range sortedOzonSKUs(p.SKUs) {
		selection := selections[sku.ID]
		resolved := OzonSKUImageDTO{
			SKUID:                sku.ID,
			SKUCode:              strings.TrimSpace(sku.SKUCode),
			SKUName:              strings.TrimSpace(sku.SKUName),
			Attrs:                json.RawMessage(sku.Attrs),
			OriginalMainImageURL: strings.TrimSpace(sku.ImageURL),
			FallbackMainImageID:  selection.FallbackMainImageID,
			AdditionalImageIDs:   append([]uuid.UUID(nil), selection.AdditionalImageIDs...),
			FinalImages:          []OzonResolvedImageDTO{},
			Issues:               []OzonImageIssueDTO{},
		}
		if resolved.AdditionalImageIDs == nil {
			resolved.AdditionalImageIDs = []uuid.UUID{}
		}
		seenURLs := map[string]struct{}{}
		hasPrimary := false
		appendImage := func(image OzonResolvedImageDTO) {
			url := strings.TrimSpace(image.URL)
			if url == "" {
				return
			}
			if _, exists := seenURLs[url]; exists {
				return
			}
			if len(resolved.FinalImages) >= OzonMaxImagesPerSKU {
				return
			}
			seenURLs[url] = struct{}{}
			image.URL = url
			image.Position = len(resolved.FinalImages) + 1
			resolved.FinalImages = append(resolved.FinalImages, image)
		}

		if resolved.OriginalMainImageURL != "" {
			appendImage(OzonResolvedImageDTO{URL: resolved.OriginalMainImageURL, Source: OzonImageSourceSKUOriginal, ImageType: ImageTypeMain})
			hasPrimary = true
			if selection.FallbackMainImageID != nil {
				resolved.Issues = append(resolved.Issues, ozonSKUImageIssue(sku, "OZON_SKU_FALLBACK_NOT_ALLOWED", "SKU 已有原始主图，不能同时保存替代主图", "请清除该 SKU 的替代主图后重新保存。"))
			}
		} else if selection.FallbackMainImageID != nil {
			if fallback, ok := sharedByID[*selection.FallbackMainImageID]; ok {
				id := fallback.dto.ID
				appendImage(OzonResolvedImageDTO{ImageID: &id, URL: fallback.dto.URL, Source: OzonImageSourceManualFallback, ImageType: ImageTypeMain, ObjectKey: fallback.objectKey})
				hasPrimary = true
			} else {
				resolved.Issues = append(resolved.Issues, ozonSKUImageIssue(sku, "OZON_SKU_FALLBACK_IMAGE_STALE", "SKU 保存的替代主图已不存在或不可用", "请为该 SKU 重新选择替代主图。"))
			}
		}

		if len(resolved.AdditionalImageIDs) > OzonMaxImagesPerSKU-1 {
			resolved.Issues = append(resolved.Issues, ozonSKUImageIssue(sku, "OZON_SKU_IMAGE_LIMIT_EXCEEDED", fmt.Sprintf("SKU 追加图片超过 Ozon 上限（最多 %d 张追加图）", OzonMaxImagesPerSKU-1), "请减少该 SKU 的商品公共图片选择。"))
		}
		for _, imageID := range stableUniqueUUIDs(resolved.AdditionalImageIDs) {
			image, ok := sharedByID[imageID]
			if !ok {
				resolved.Issues = append(resolved.Issues, ozonSKUImageIssue(sku, "OZON_SKU_SHARED_IMAGE_STALE", "SKU 选择的商品公共图片已不存在或不可用", "请重新检查并保存该 SKU 的追加图片。"))
				continue
			}
			id := image.dto.ID
			appendImage(OzonResolvedImageDTO{ImageID: &id, URL: image.dto.URL, Source: OzonImageSourceProductShared, ImageType: ImageTypeDetail, ObjectKey: image.objectKey})
		}

		if !hasPrimary {
			resolved.Issues = append(resolved.Issues, ozonSKUImageIssue(sku, "OZON_SKU_MAIN_IMAGE_MISSING", fmt.Sprintf("SKU「%s」缺少原始主图，且未指定可追溯的替代主图", ozonSKUDisplayName(sku)), "请补齐该 SKU 的采集原图，或在商品级 Ozon 配置中明确选择替代主图。"))
		}
		resolved.CanPublish = len(resolved.FinalImages) > 0 && len(resolved.Issues) == 0
		view.ErrorCount += len(resolved.Issues)
		view.SKUs = append(view.SKUs, resolved)
	}
	view.ErrorCount += len(view.Issues)
	return view
}

func (view OzonImageConfigDTO) ValidationError() error {
	if view.ErrorCount == 0 {
		return nil
	}
	if len(view.Issues) > 0 {
		return fmt.Errorf("%s", view.Issues[0].Message)
	}
	for _, sku := range view.SKUs {
		if len(sku.Issues) > 0 {
			return fmt.Errorf("%s", sku.Issues[0].Message)
		}
	}
	return fmt.Errorf("Ozon SKU image configuration is invalid")
}

func decodeOzonImageConfig(raw []byte) (OzonImageConfigInput, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("{}")) || bytes.Equal(trimmed, []byte("[]")) {
		return OzonImageConfigInput{Version: OzonImageConfigVersion}, false, nil
	}
	var config OzonImageConfigInput
	if err := json.Unmarshal(trimmed, &config); err != nil {
		return OzonImageConfigInput{Version: OzonImageConfigVersion}, true, err
	}
	if config.Version == 0 {
		config.Version = OzonImageConfigVersion
	}
	if config.Version != OzonImageConfigVersion {
		return OzonImageConfigInput{Version: OzonImageConfigVersion}, true, fmt.Errorf("unsupported Ozon image config version: %d", config.Version)
	}
	return config, true, nil
}

func ozonSharedImages(images []ProductImage) ([]ozonSharedImage, map[uuid.UUID]ozonSharedImage) {
	rows := append([]ProductImage(nil), images...)
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].SortOrder == rows[j].SortOrder {
			if rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
				return rows[i].ID.String() < rows[j].ID.String()
			}
			return rows[i].CreatedAt.Before(rows[j].CreatedAt)
		}
		return rows[i].SortOrder < rows[j].SortOrder
	})
	out := make([]ozonSharedImage, 0, len(rows))
	byID := make(map[uuid.UUID]ozonSharedImage, len(rows))
	for _, image := range rows {
		if strings.EqualFold(strings.TrimSpace(image.ImageType), ImageTypeSKU) {
			continue
		}
		url := strings.TrimSpace(image.PublicURL)
		if url == "" {
			url = strings.TrimSpace(image.OriginURL)
		}
		if image.ID == uuid.Nil || url == "" {
			continue
		}
		item := ozonSharedImage{dto: OzonSharedImageDTO{ID: image.ID, URL: url, ImageType: strings.TrimSpace(image.ImageType), SortOrder: image.SortOrder}, objectKey: strings.TrimSpace(image.ObjectKey)}
		out = append(out, item)
		byID[image.ID] = item
	}
	return out, byID
}

func sortedOzonSKUs(skus []ProductSKU) []ProductSKU {
	rows := append([]ProductSKU(nil), skus...)
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].ID.String() < rows[j].ID.String()
		}
		return rows[i].CreatedAt.Before(rows[j].CreatedAt)
	})
	return rows
}

func stableUniqueUUIDs(values []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(values))
	seen := make(map[uuid.UUID]struct{}, len(values))
	for _, value := range values {
		if value == uuid.Nil {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func ozonSKUDisplayName(sku ProductSKU) string {
	if value := strings.TrimSpace(sku.SKUName); value != "" {
		return value
	}
	if value := strings.TrimSpace(sku.SKUCode); value != "" {
		return value
	}
	return sku.ID.String()
}

func ozonSKUImageIssue(sku ProductSKU, code, message, suggestion string) OzonImageIssueDTO {
	skuID := sku.ID
	return OzonImageIssueDTO{Code: code, Message: message, Suggestion: suggestion, SKUID: &skuID}
}

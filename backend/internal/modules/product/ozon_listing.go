package product

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const (
	OzonListingConfigVersion = 1

	OzonValueSourceProduct       = "product"
	OzonValueSourceShopConfig    = "ozon_product_shop_config"
	OzonValueSourceGlobalPreset  = "global_ozon_preset"
	OzonValueSourceLocalStock    = "local_inventory"
	OzonValueSourceStoreContract = "store_contract"
	OzonValueSourceDefault       = "ozon_default"
	OzonValueSourceMissing       = "missing"
)

// OzonListingConfigInput is the product + Ozon shop scoped editable listing
// configuration. Stock is intentionally absent: product_skus.stock remains the
// only source of inventory truth and keeps the existing adjustment audit trail.
type OzonListingConfigInput struct {
	Version             int                    `json:"version"`
	TitleOverride       string                 `json:"titleOverride,omitempty"`
	DescriptionOverride string                 `json:"descriptionOverride,omitempty"`
	CurrencyCode        string                 `json:"currencyCode,omitempty"`
	SKUPriceOverrides   map[string]float64     `json:"skuPriceOverrides,omitempty"`
	Package             OzonPackageConfigInput `json:"package"`
}

type OzonPackageConfigInput struct {
	WeightG     *int64 `json:"weightG,omitempty"`
	WidthMM     *int64 `json:"widthMm,omitempty"`
	HeightMM    *int64 `json:"heightMm,omitempty"`
	DepthMM     *int64 `json:"depthMm,omitempty"`
	WarehouseID string `json:"warehouseId,omitempty"`
	VAT         string `json:"vat,omitempty"`
}

type OzonResolvedString struct {
	Value  string `json:"value,omitempty"`
	Source string `json:"source"`
}

type OzonResolvedInt64 struct {
	Value  int64  `json:"value"`
	Source string `json:"source"`
}

type OzonResolvedFloat struct {
	Value  float64 `json:"value"`
	Source string  `json:"source"`
}

type OzonListingIssueDTO struct {
	Code       string     `json:"code"`
	Message    string     `json:"message"`
	Suggestion string     `json:"suggestion,omitempty"`
	Field      string     `json:"field,omitempty"`
	SKUID      *uuid.UUID `json:"skuId,omitempty"`
}

type OzonResolvedPackageDTO struct {
	WeightG     OzonResolvedInt64  `json:"weightG"`
	WidthMM     OzonResolvedInt64  `json:"widthMm"`
	HeightMM    OzonResolvedInt64  `json:"heightMm"`
	DepthMM     OzonResolvedInt64  `json:"depthMm"`
	WarehouseID OzonResolvedString `json:"warehouseId"`
	VAT         OzonResolvedString `json:"vat"`
}

type OzonResolvedSKUListingDTO struct {
	SKUID       uuid.UUID              `json:"skuId"`
	SKUCode     string                 `json:"skuCode,omitempty"`
	SKUName     string                 `json:"skuName,omitempty"`
	Price       OzonResolvedFloat      `json:"price"`
	LocalStock  int                    `json:"localStock"`
	StockSource string                 `json:"stockSource"`
	Images      []OzonResolvedImageDTO `json:"images"`
	CanSubmit   bool                   `json:"canSubmit"`
	Issues      []OzonListingIssueDTO  `json:"issues"`
}

// OzonResolvedListingDTO is the canonical effective-value view reused by the
// Admin preview, live preflight and immutable publish-task snapshot builder.
type OzonResolvedListingDTO struct {
	ProductID          uuid.UUID                   `json:"productId"`
	ShopID             *uuid.UUID                  `json:"shopId,omitempty"`
	CategoryID         string                      `json:"categoryId,omitempty"`
	CategoryPath       string                      `json:"categoryPath,omitempty"`
	SchemaHash         string                      `json:"schemaHash,omitempty"`
	PlatformAttributes json.RawMessage             `json:"platformAttributes,omitempty"`
	Title              OzonResolvedString          `json:"title"`
	Description        OzonResolvedString          `json:"description"`
	Currency           OzonResolvedString          `json:"currency"`
	Package            OzonResolvedPackageDTO      `json:"package"`
	SKUs               []OzonResolvedSKUListingDTO `json:"skus"`
	Issues             []OzonListingIssueDTO       `json:"issues"`
	ErrorCount         int                         `json:"errorCount"`
	CanSubmit          bool                        `json:"canSubmit"`
}

func DecodeOzonListingConfig(raw []byte) (OzonListingConfigInput, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("{}")) {
		return OzonListingConfigInput{Version: OzonListingConfigVersion, SKUPriceOverrides: map[string]float64{}}, false, nil
	}
	var config OzonListingConfigInput
	if err := json.Unmarshal(trimmed, &config); err != nil {
		return OzonListingConfigInput{}, true, err
	}
	if config.Version == 0 {
		config.Version = OzonListingConfigVersion
	}
	if config.Version != OzonListingConfigVersion {
		return OzonListingConfigInput{}, true, fmt.Errorf("unsupported Ozon listing config version: %d", config.Version)
	}
	if config.SKUPriceOverrides == nil {
		config.SKUPriceOverrides = map[string]float64{}
	}
	return config, true, nil
}

func NormalizeOzonListingConfigInput(p Product, input OzonListingConfigInput) (OzonListingConfigInput, error) {
	if input.Version == 0 {
		input.Version = OzonListingConfigVersion
	}
	if input.Version != OzonListingConfigVersion {
		return OzonListingConfigInput{}, fmt.Errorf("unsupported Ozon listing config version: %d", input.Version)
	}
	input.TitleOverride = strings.TrimSpace(input.TitleOverride)
	input.DescriptionOverride = strings.TrimSpace(input.DescriptionOverride)
	input.CurrencyCode = strings.ToUpper(strings.TrimSpace(input.CurrencyCode))
	if input.CurrencyCode != "" && !isOzonCurrencyCode(input.CurrencyCode) {
		return OzonListingConfigInput{}, fmt.Errorf("Ozon currencyCode must be a three-letter code")
	}
	knownSKUs := make(map[string]struct{}, len(p.SKUs))
	for _, sku := range p.SKUs {
		knownSKUs[sku.ID.String()] = struct{}{}
	}
	normalizedPrices := make(map[string]float64, len(input.SKUPriceOverrides))
	for rawID, price := range input.SKUPriceOverrides {
		parsed, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil || parsed == uuid.Nil {
			return OzonListingConfigInput{}, fmt.Errorf("Ozon SKU price override contains invalid skuId")
		}
		if _, ok := knownSKUs[parsed.String()]; !ok {
			return OzonListingConfigInput{}, fmt.Errorf("Ozon SKU price override references an unknown SKU: %s", parsed)
		}
		if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
			return OzonListingConfigInput{}, fmt.Errorf("Ozon SKU price override must be greater than zero: %s", parsed)
		}
		normalizedPrices[parsed.String()] = price
	}
	input.SKUPriceOverrides = normalizedPrices
	for _, field := range []struct {
		name  string
		value *int64
	}{
		{"weightG", input.Package.WeightG},
		{"widthMm", input.Package.WidthMM},
		{"heightMm", input.Package.HeightMM},
		{"depthMm", input.Package.DepthMM},
	} {
		if field.value != nil && *field.value <= 0 {
			return OzonListingConfigInput{}, fmt.Errorf("Ozon package %s must be greater than zero when set", field.name)
		}
	}
	input.Package.WarehouseID = strings.TrimSpace(input.Package.WarehouseID)
	if input.Package.WarehouseID != "" {
		value, err := strconv.ParseInt(input.Package.WarehouseID, 10, 64)
		if err != nil || value <= 0 {
			return OzonListingConfigInput{}, fmt.Errorf("Ozon warehouseId must be a positive integer")
		}
	}
	input.Package.VAT = strings.TrimSpace(input.Package.VAT)
	if input.Package.VAT != "" && input.Package.VAT != "0" && input.Package.VAT != "0.1" && input.Package.VAT != "0.2" {
		return OzonListingConfigInput{}, fmt.Errorf("Ozon VAT must be one of 0, 0.1 or 0.2")
	}
	return input, nil
}

func MarshalOzonListingConfig(p Product, input OzonListingConfigInput) ([]byte, error) {
	normalized, err := NormalizeOzonListingConfigInput(p, input)
	if err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

// ResolveOzonListing applies the product+shop configuration first, then the
// global Ozon preset. Local SKU stock is read directly and is never persisted
// into the listing configuration.
func ResolveOzonListing(p Product, cfg *ProductPlatformPublishConfig, preset map[string]string, contractCurrency string) OzonResolvedListingDTO {
	resolved := OzonResolvedListingDTO{
		ProductID: p.ID,
		SKUs:      []OzonResolvedSKUListingDTO{},
		Issues:    []OzonListingIssueDTO{},
	}
	var listing OzonListingConfigInput
	if cfg != nil {
		resolved.ShopID = cfg.ShopID
		resolved.CategoryID = strings.TrimSpace(cfg.CategoryID)
		resolved.CategoryPath = strings.TrimSpace(cfg.CategoryPath)
		resolved.SchemaHash = strings.TrimSpace(cfg.SchemaHash)
		resolved.PlatformAttributes = append(json.RawMessage(nil), cfg.PlatformAttributes...)
		decoded, _, err := DecodeOzonListingConfig(cfg.ListingConfig)
		if err != nil {
			resolved.Issues = append(resolved.Issues, OzonListingIssueDTO{
				Code: "OZON_LISTING_CONFIG_INVALID", Message: "已保存的 Ozon 刊登配置无法读取", Suggestion: "请重新检查并保存当前商品和店铺的刊登配置。",
			})
		} else {
			listing = decoded
		}
	}

	resolved.Title = resolveOzonTitle(p, listing.TitleOverride)
	resolved.Description = resolveOzonDescription(p, listing.DescriptionOverride)
	resolved.Currency = resolveOzonString(listing.CurrencyCode, OzonValueSourceShopConfig, preset["currency_code"], OzonValueSourceGlobalPreset, strings.ToUpper(strings.TrimSpace(contractCurrency)), OzonValueSourceStoreContract)
	resolved.Package = OzonResolvedPackageDTO{
		WeightG:     resolveOzonInt(listing.Package.WeightG, preset["default_weight"]),
		WidthMM:     resolveOzonInt(listing.Package.WidthMM, preset["default_width"]),
		HeightMM:    resolveOzonInt(listing.Package.HeightMM, preset["default_height"]),
		DepthMM:     resolveOzonInt(listing.Package.DepthMM, preset["default_depth"]),
		WarehouseID: resolveOzonString(listing.Package.WarehouseID, OzonValueSourceShopConfig, preset["warehouse_id"], OzonValueSourceGlobalPreset, "", OzonValueSourceMissing),
		VAT:         resolveOzonString(listing.Package.VAT, OzonValueSourceShopConfig, preset["vat"], OzonValueSourceGlobalPreset, "0", OzonValueSourceDefault),
	}
	if resolved.CategoryID == "" {
		resolved.Issues = append(resolved.Issues, OzonListingIssueDTO{Code: "OZON_CATEGORY_MISSING", Message: "Ozon 类目未选择", Suggestion: "请选择并保存当前店铺的 Ozon 类目。", Field: "categoryId"})
	}
	if resolved.SchemaHash == "" {
		resolved.Issues = append(resolved.Issues, OzonListingIssueDTO{Code: "OZON_SCHEMA_MISSING", Message: "Ozon 类目属性模板未确认", Suggestion: "请同步类目属性并重新保存当前店铺的 Ozon 配置。", Field: "platformAttributes"})
	}

	if resolved.Title.Value == "" {
		resolved.Issues = append(resolved.Issues, OzonListingIssueDTO{Code: "OZON_TITLE_MISSING", Message: "Ozon 标题未填写", Suggestion: "请填写 Ozon 标题或补齐商品标题。", Field: "title"})
	}
	if resolved.Description.Value == "" {
		resolved.Issues = append(resolved.Issues, OzonListingIssueDTO{Code: "OZON_DESCRIPTION_MISSING", Message: "Ozon 描述未填写", Suggestion: "请填写 Ozon 描述或补齐商品描述。", Field: "description"})
	}
	if resolved.Currency.Value == "" {
		resolved.Issues = append(resolved.Issues, OzonListingIssueDTO{Code: "OZON_CURRENCY_MISSING", Message: "Ozon 币种无法确定", Suggestion: "请设置商品店铺级币种、全局 Ozon 刊登预设，或检查店铺合同币种。", Field: "currencyCode"})
	}
	if resolved.Package.WarehouseID.Value == "" {
		resolved.Issues = append(resolved.Issues, OzonListingIssueDTO{Code: "OZON_WAREHOUSE_MISSING", Message: "Ozon 仓库未设置", Suggestion: "请填写商品店铺级仓库，或在 Ozon 刊登预设中设置默认仓库。", Field: "package.warehouseId"})
	}
	for _, field := range []struct {
		name  string
		label string
		value OzonResolvedInt64
	}{
		{"package.weightG", "重量", resolved.Package.WeightG},
		{"package.widthMm", "宽度", resolved.Package.WidthMM},
		{"package.heightMm", "高度", resolved.Package.HeightMM},
		{"package.depthMm", "深度", resolved.Package.DepthMM},
	} {
		if field.value.Value <= 0 {
			resolved.Issues = append(resolved.Issues, OzonListingIssueDTO{Code: "OZON_PACKAGE_VALUE_MISSING", Message: "Ozon 商品" + field.label + "未设置", Suggestion: "请填写商品级值，或在 Ozon 刊登预设中设置默认值。", Field: field.name})
		}
	}

	var imageView OzonImageConfigDTO
	if cfg != nil {
		imageView = ResolveOzonImageConfig(p, cfg.MappedImages)
	} else {
		imageView = ResolveOzonImageConfig(p, nil)
	}
	imagesBySKU := make(map[uuid.UUID]OzonSKUImageDTO, len(imageView.SKUs))
	for _, imageSKU := range imageView.SKUs {
		imagesBySKU[imageSKU.SKUID] = imageSKU
	}
	for _, sku := range sortedOzonSKUs(p.SKUs) {
		price := 0.0
		priceSource := OzonValueSourceProduct
		if override, ok := listing.SKUPriceOverrides[sku.ID.String()]; ok {
			price = override
			priceSource = OzonValueSourceShopConfig
		} else if sku.Price != nil {
			price = *sku.Price
		}
		stock := 0
		if sku.Stock != nil {
			stock = *sku.Stock
		}
		row := OzonResolvedSKUListingDTO{
			SKUID: sku.ID, SKUCode: strings.TrimSpace(sku.SKUCode), SKUName: strings.TrimSpace(sku.SKUName),
			Price: OzonResolvedFloat{Value: price, Source: priceSource}, LocalStock: stock, StockSource: OzonValueSourceLocalStock,
			Images: []OzonResolvedImageDTO{}, Issues: []OzonListingIssueDTO{},
		}
		if imageSKU, ok := imagesBySKU[sku.ID]; ok {
			row.Images = append(row.Images, imageSKU.FinalImages...)
			for _, issue := range imageSKU.Issues {
				skuID := sku.ID
				row.Issues = append(row.Issues, OzonListingIssueDTO{Code: issue.Code, Message: issue.Message, Suggestion: issue.Suggestion, Field: "skuImages." + sku.ID.String(), SKUID: &skuID})
			}
		}
		if price <= 0 {
			skuID := sku.ID
			row.Issues = append(row.Issues, OzonListingIssueDTO{Code: "OZON_SKU_PRICE_MISSING", Message: fmt.Sprintf("SKU「%s」缺少有效的 Ozon 售价", ozonSKUDisplayName(sku)), Suggestion: "请设置 Ozon 专属售价，或补齐本地 SKU 销售价。", Field: "skuPrices." + sku.ID.String(), SKUID: &skuID})
		}
		if stock < 0 {
			skuID := sku.ID
			row.Issues = append(row.Issues, OzonListingIssueDTO{Code: "OZON_SKU_STOCK_INVALID", Message: fmt.Sprintf("SKU「%s」的本地库存不能为负数", ozonSKUDisplayName(sku)), Suggestion: "请通过库存调整功能修正本地库存。", Field: "skuStock." + sku.ID.String(), SKUID: &skuID})
		}
		row.CanSubmit = len(row.Issues) == 0
		resolved.ErrorCount += len(row.Issues)
		resolved.SKUs = append(resolved.SKUs, row)
	}
	resolved.ErrorCount += len(resolved.Issues)
	resolved.CanSubmit = resolved.ErrorCount == 0
	return resolved
}

func isOzonCurrencyCode(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, char := range value {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}

func resolveOzonTitle(p Product, override string) OzonResolvedString {
	if value := strings.TrimSpace(override); value != "" {
		return OzonResolvedString{Value: value, Source: OzonValueSourceShopConfig}
	}
	for _, value := range []string{p.Title, p.AITitle, p.OriginalTitle} {
		if value = strings.TrimSpace(value); value != "" {
			return OzonResolvedString{Value: value, Source: OzonValueSourceProduct}
		}
	}
	return OzonResolvedString{Source: OzonValueSourceMissing}
}

func resolveOzonDescription(p Product, override string) OzonResolvedString {
	if value := strings.TrimSpace(override); value != "" {
		return OzonResolvedString{Value: value, Source: OzonValueSourceShopConfig}
	}
	for _, value := range []string{p.Description, p.AIDescription} {
		if value = strings.TrimSpace(value); value != "" {
			return OzonResolvedString{Value: value, Source: OzonValueSourceProduct}
		}
	}
	return OzonResolvedString{Source: OzonValueSourceMissing}
}

func resolveOzonString(primary, primarySource, secondary, secondarySource, fallback, fallbackSource string) OzonResolvedString {
	if value := strings.TrimSpace(primary); value != "" {
		return OzonResolvedString{Value: value, Source: primarySource}
	}
	if value := strings.TrimSpace(secondary); value != "" {
		return OzonResolvedString{Value: value, Source: secondarySource}
	}
	if value := strings.TrimSpace(fallback); value != "" {
		return OzonResolvedString{Value: value, Source: fallbackSource}
	}
	return OzonResolvedString{Source: OzonValueSourceMissing}
}

func resolveOzonInt(override *int64, preset string) OzonResolvedInt64 {
	if override != nil && *override > 0 {
		return OzonResolvedInt64{Value: *override, Source: OzonValueSourceShopConfig}
	}
	value, _ := strconv.ParseInt(strings.TrimSpace(preset), 10, 64)
	if value > 0 {
		return OzonResolvedInt64{Value: value, Source: OzonValueSourceGlobalPreset}
	}
	return OzonResolvedInt64{Source: OzonValueSourceMissing}
}

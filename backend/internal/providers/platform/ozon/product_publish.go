package ozon

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const (
	pathProductImport = "/v3/product/import"
	pathImportInfo    = "/v1/product/import/info"
	pathStocks        = "/v2/products/stocks"
)

type ozonPublishMerged struct {
	DescriptionCategoryID int64
	TypeID                int64
	WarehouseID           int64
	VAT                   string
	CurrencyCode          string
	DefaultBrand          string
	DefaultCountry        string
	DefaultManufacturer   string
	DefaultType           string
	AutoFillAttributes    bool
	AutoFillAI            bool
	WeightG               int64
	WidthMM               int64
	HeightMM              int64
	DepthMM               int64
	AttributeDefaults     map[string]string
}

func mergeOzonPublish(pub map[string]any, opt map[string]any) ozonPublishMerged {
	base := loweredScalarMap(pub)
	over := loweredScalarMap(opt)
	merged := overlayStringMaps(base, over)

	out := ozonPublishMerged{
		DescriptionCategoryID: parseInt64Merged(merged["description_category_id"]),
		TypeID:                parseInt64Merged(merged["type_id"]),
		WarehouseID:           parseInt64Merged(merged["warehouse_id"]),
		VAT:                   strings.TrimSpace(merged["vat"]),
		CurrencyCode:          strings.TrimSpace(merged["currency_code"]),
		DefaultBrand:          strings.TrimSpace(merged["default_brand"]),
		DefaultCountry:        strings.TrimSpace(merged["default_country"]),
		DefaultManufacturer:   strings.TrimSpace(merged["default_manufacturer"]),
		DefaultType:           strings.TrimSpace(merged["default_type"]),
		AutoFillAttributes:    truthyString(merged["auto_fill_attributes"]),
		AutoFillAI:            truthyString(merged["ai_auto_fill"]),
		WeightG:               parseInt64Merged(merged["default_weight"]),
		WidthMM:               parseInt64Merged(merged["default_width"]),
		HeightMM:              parseInt64Merged(merged["default_height"]),
		DepthMM:               parseInt64Merged(merged["default_depth"]),
		AttributeDefaults:     parseJSONStringMap(merged["default_attributes"]),
	}
	if out.VAT == "" {
		out.VAT = "0"
	}
	return out
}

func validateOzonPublishMerged(m ozonPublishMerged) error {
	if m.DescriptionCategoryID <= 0 {
		return fmt.Errorf("ozon publish requires description_category_id in publish settings")
	}
	if m.TypeID <= 0 {
		return fmt.Errorf("ozon publish requires type_id in publish settings")
	}
	for _, field := range []struct {
		name  string
		value int64
	}{
		{"default_weight", m.WeightG},
		{"default_width", m.WidthMM},
		{"default_height", m.HeightMM},
		{"default_depth", m.DepthMM},
	} {
		if field.value <= 0 {
			return fmt.Errorf("ozon publish requires %s greater than zero in publish settings", field.name)
		}
	}
	return nil
}

type ozonImportItem struct {
	Attributes            []ozonAttributeValue        `json:"attributes,omitempty"`
	ComplexAttributes     []ozonComplexAttributeGroup `json:"complex_attributes,omitempty"`
	DescriptionCategoryID int64                       `json:"description_category_id"`
	TypeID                int64                       `json:"type_id"`
	Name                  string                      `json:"name"`
	OfferID               string                      `json:"offer_id"`
	CurrencyCode          string                      `json:"currency_code"`
	Price                 string                      `json:"price"`
	VAT                   string                      `json:"vat"`
	Images                []string                    `json:"images,omitempty"`
	PrimaryImage          string                      `json:"primary_image,omitempty"`
	Weight                int64                       `json:"weight"`
	Width                 int64                       `json:"width"`
	Height                int64                       `json:"height"`
	Depth                 int64                       `json:"depth"`
	WeightUnit            string                      `json:"weight_unit,omitempty"`
	DimensionUnit         string                      `json:"dimension_unit,omitempty"`
	IsPreorder            bool                        `json:"is_preorder"`
	AutoRenew             string                      `json:"auto_renew,omitempty"`
}

// ozonComplexAttributeGroup represents one complex-attribute instance. The
// current product UI supplies one value per attribute, so one group is emitted
// for each non-zero complex ID; repeated complex instances are deliberately
// not inferred here.
type ozonComplexAttributeGroup struct {
	Attributes []ozonAttributeValue `json:"attributes"`
}

func partitionOzonImportAttributes(values []ozonAttributeValue) ([]ozonAttributeValue, []ozonComplexAttributeGroup) {
	ordinary := make([]ozonAttributeValue, 0, len(values))
	byComplexID := make(map[int64][]ozonAttributeValue)
	complexIDs := make([]int64, 0)
	for _, value := range values {
		if value.ComplexID <= 0 {
			ordinary = append(ordinary, value)
			continue
		}
		if _, exists := byComplexID[value.ComplexID]; !exists {
			complexIDs = append(complexIDs, value.ComplexID)
		}
		byComplexID[value.ComplexID] = append(byComplexID[value.ComplexID], value)
	}
	sort.Slice(complexIDs, func(i, j int) bool { return complexIDs[i] < complexIDs[j] })
	complex := make([]ozonComplexAttributeGroup, 0, len(complexIDs))
	for _, complexID := range complexIDs {
		complex = append(complex, ozonComplexAttributeGroup{Attributes: byComplexID[complexID]})
	}
	return ordinary, complex
}

func (ozonProvider) PublishProduct(ctx context.Context, req platformp.PublishProductRequest) (*platformp.PublishProductResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}
	cfg, err := ResolveRuntime(req.Auth)
	if err != nil {
		return nil, fmt.Errorf("ozon publish: %w", err)
	}
	merged := mergeOzonPublish(req.PublishConfig, req.Options)
	if err := validateOzonPublishMerged(merged); err != nil {
		return nil, err
	}

	d := req.Product
	title := strings.TrimSpace(d.Title)
	if title == "" {
		return nil, fmt.Errorf("product title is required for publish")
	}
	if len(d.SKUs) == 0 {
		return nil, fmt.Errorf("product SKU is required for publish")
	}
	skuImagePlans := make([]ozonSKUImagePlan, len(d.SKUs))
	for i, sku := range d.SKUs {
		plan, imageErr := resolveOzonSKUImagePlan(sku)
		if imageErr != nil {
			return nil, imageErr
		}
		skuImagePlans[i] = plan
	}

	cctx, cancel := context.WithTimeout(ctx, cfg.Timeout+2*time.Minute)
	defer cancel()
	client := newClient(cfg)

	explicitAttrs, err := parseExplicitOzonAttributes(req.Options)
	if err != nil {
		return nil, err
	}
	localAttrs := map[string]string{}
	if merged.AutoFillAttributes {
		localAttrs = localAttributeMap(d)
		if desc := strings.TrimSpace(d.Description); desc != "" {
			localAttrs[normalizeText("аннотация")] = desc
		}
	}
	// Always refresh the live category template immediately before import. This
	// keeps required-attribute and dictionary validation authoritative even when
	// automatic filling is disabled and the UI cache is stale.
	liveAttrs, err := client.getCategoryAttributes(cctx, merged.DescriptionCategoryID, merged.TypeID)
	if err != nil {
		return nil, mapOzonPublishError(err)
	}
	liveSchemaHash := CategorySchemaHash(categoryAttrsForHash(liveAttrs))
	if rawExpectedHash, ok := req.Options["ozon_schema_hash"]; ok && rawExpectedHash != nil {
		if expectedHash := strings.TrimSpace(fmt.Sprint(rawExpectedHash)); expectedHash != "" && expectedHash != liveSchemaHash {
			return nil, fmt.Errorf("ozon category schema changed after task creation; rerun preflight")
		}
	}
	attrPayload, missingAttrs, missingDefs, err := client.buildCategoryAttributesForPublish(
		cctx,
		merged.DescriptionCategoryID,
		merged.TypeID,
		localAttrs,
		merged,
		explicitAttrs,
		merged.AutoFillAttributes,
		liveAttrs,
	)
	if err != nil {
		return nil, mapOzonPublishError(err)
	}

	aiFillUsed := false
	aiFillFailed := false
	if merged.AutoFillAI && len(missingDefs) > 0 && boundChat != nil {
		suggestions, aiErr := fillMissingAttributesWithAI(cctx, boundChat, d, missingDefs)
		if aiErr != nil {
			aiFillFailed = true
		} else if len(suggestions) > 0 {
			extra, stillMissing := client.applySuggestedAttributes(cctx, merged.DescriptionCategoryID, merged.TypeID, missingDefs, suggestions)
			if len(extra) > 0 {
				attrPayload = append(attrPayload, extra...)
				aiFillUsed = true
			}
			missingAttrs = stillMissing
		}
	}
	if len(missingAttrs) > 0 {
		return nil, fmt.Errorf("ozon publish missing required category attributes: %s", strings.Join(missingAttrs, ", "))
	}
	ordinaryAttrs, complexAttrs := partitionOzonImportAttributes(attrPayload)

	currencyCode := strings.TrimSpace(merged.CurrencyCode)
	if currencyCode == "" {
		info, infoErr := client.getSellerInfo(cctx)
		if infoErr != nil {
			return nil, mapOzonPublishError(fmt.Errorf("ozon publish: resolve contract currency: %w", infoErr))
		}
		currencyCode = strings.TrimSpace(info.Company.Currency)
		if currencyCode == "" {
			return nil, fmt.Errorf("ozon publish: cannot resolve contract currency, please set currency_code in publish settings")
		}
	}

	items := make([]ozonImportItem, 0, len(d.SKUs))
	for i, sku := range d.SKUs {
		imagePlan := skuImagePlans[i]
		name := title
		if len(d.SKUs) > 1 && strings.TrimSpace(sku.SKUName) != "" {
			name = title + " / " + strings.TrimSpace(sku.SKUName)
		}
		item := ozonImportItem{
			Attributes:            ordinaryAttrs,
			ComplexAttributes:     complexAttrs,
			DescriptionCategoryID: merged.DescriptionCategoryID,
			TypeID:                merged.TypeID,
			Name:                  truncateRunes(name, maxNameRunes),
			OfferID:               buildOfferID(d, sku, i),
			CurrencyCode:          currencyCode,
			Price:                 formatOzonPrice(sku.Price),
			VAT:                   merged.VAT,
			Images:                imagePlan.Images,
			PrimaryImage:          imagePlan.Primary,
			Weight:                merged.WeightG,
			Width:                 merged.WidthMM,
			Height:                merged.HeightMM,
			Depth:                 merged.DepthMM,
			WeightUnit:            "g",
			DimensionUnit:         "mm",
			IsPreorder:            false,
			AutoRenew:             "ON",
		}
		items = append(items, item)
	}

	var importResp struct {
		Result struct {
			TaskID int64 `json:"task_id"`
		} `json:"result"`
	}
	if err := client.postJSONNoRetry(cctx, pathProductImport, map[string]any{"items": items}, &importResp); err != nil {
		return nil, mapOzonPublishError(err)
	}
	if importResp.Result.TaskID <= 0 {
		return nil, fmt.Errorf("ozon product publish: platform did not return task_id")
	}

	imported, failed, warnings, err := pollImportInfo(cctx, client, importResp.Result.TaskID)
	if err != nil {
		return nil, mapOzonPublishError(err)
	}
	if len(failed) > 0 {
		return nil, fmt.Errorf("ozon product publish failed: %s", strings.Join(failed, "; "))
	}
	if len(imported) == 0 {
		return nil, fmt.Errorf("ozon product publish: no products imported")
	}

	// Optional per-SKU stock sync (Ozon stock is per product + warehouse).
	stockWarnings := []string(nil)
	if merged.WarehouseID > 0 {
		stockWarnings = applyStocks(cctx, client, merged.WarehouseID, d, imported)
	}

	mappings := buildSKUMappings(d, imported)
	var firstProductID int64
	var firstOfferID string
	for _, row := range imported {
		if firstProductID == 0 && row.ProductID > 0 {
			firstProductID = row.ProductID
			firstOfferID = row.OfferID
		}
	}
	status := "published"
	summary := map[string]any{
		"provider":         "ozon",
		"taskId":           importResp.Result.TaskID,
		"imported":         len(imported),
		"skuMappings":      len(mappings),
		"categoryId":       merged.DescriptionCategoryID,
		"typeId":           merged.TypeID,
		"currencyCode":     merged.CurrencyCode,
		"resolvedCurrency": currencyCode,
		"autoFillAttrs":    len(attrPayload),
		"aiFillUsed":       aiFillUsed,
		"aiFillFailed":     aiFillFailed,
		"missingAttrs":     missingAttrs,
		"importWarnings":   warnings,
		"stockWarnings":    stockWarnings,
		"warehouseId":      merged.WarehouseID,
	}
	return &platformp.PublishProductResult{
		ExternalProductID: strconv.FormatInt(firstProductID, 10),
		ExternalSPUID:     firstOfferID,
		Status:            status,
		SKUMappings:       mappings,
		RawSummary:        platformp.TrimRawMap(summary, 24, 240),
	}, nil
}

func parseExplicitOzonAttributes(options map[string]any) (map[string]explicitOzonAttribute, error) {
	out := map[string]explicitOzonAttribute{}
	if options == nil {
		return out, nil
	}
	raw, ok := options["platform_attributes"]
	if !ok || raw == nil {
		return out, nil
	}
	var attrs map[string]any
	switch v := raw.(type) {
	case json.RawMessage:
		if err := json.Unmarshal(v, &attrs); err != nil {
			return nil, fmt.Errorf("ozon platform_attributes must be valid JSON: %w", err)
		}
	case []byte:
		if err := json.Unmarshal(v, &attrs); err != nil {
			return nil, fmt.Errorf("ozon platform_attributes must be valid JSON: %w", err)
		}
	case map[string]any:
		attrs = v
	default:
		return nil, fmt.Errorf("ozon platform_attributes must be an object")
	}
	for rawKey, rawValue := range attrs {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			return nil, fmt.Errorf("ozon platform_attributes contains an empty attribute key")
		}
		nested, ok := rawValue.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("ozon attribute %s must use {value,dictionaryValueId}", key)
		}
		spec := explicitOzonAttribute{}
		if value, exists := nested["value"]; exists && value != nil {
			spec.Value = strings.TrimSpace(fmt.Sprint(value))
		}
		if value, exists := nested["dictionaryValueId"]; exists && value != nil {
			switch typed := value.(type) {
			case float64:
				if typed > 0 {
					spec.DictionaryValueID = int64(typed)
				}
			case string:
				parsed, parseErr := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
				if parseErr != nil || parsed <= 0 {
					return nil, fmt.Errorf("ozon attribute %s has invalid dictionaryValueId", key)
				}
				spec.DictionaryValueID = parsed
			default:
				return nil, fmt.Errorf("ozon attribute %s has invalid dictionaryValueId", key)
			}
		}
		out[key] = spec
	}
	return out, nil
}

type importedRow struct {
	OfferID   string
	ProductID int64
}

type importInfoItemError struct {
	Code        string `json:"code"`
	Field       string `json:"field"`
	Level       string `json:"level"`
	Message     string `json:"message"`
	Description string `json:"description"`
}

func pollImportInfo(ctx context.Context, client *ozonClient, taskID int64) ([]importedRow, []string, []string, error) {
	var lastErr error
	for attempt := 0; attempt < maxPollAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, nil, nil, fmt.Errorf("ozon product import poll: %w", ctx.Err())
			case <-time.After(pollInterval * time.Second):
			}
		}
		var resp struct {
			Result struct {
				Items []struct {
					OfferID   string                `json:"offer_id"`
					ProductID int64                 `json:"product_id"`
					Status    string                `json:"status"`
					Errors    []importInfoItemError `json:"errors"`
				} `json:"items"`
			} `json:"result"`
		}
		if err := client.postJSON(ctx, pathImportInfo, map[string]any{"task_id": taskID}, &resp); err != nil {
			lastErr = err
			continue
		}
		rows := resp.Result.Items
		if len(rows) == 0 {
			lastErr = fmt.Errorf("ozon product import: empty task result")
			continue
		}
		imported := make([]importedRow, 0, len(rows))
		failed := make([]string, 0, len(rows))
		warnings := make([]string, 0, len(rows))
		pending := false
		for _, it := range rows {
			switch strings.ToLower(strings.TrimSpace(it.Status)) {
			case importStatusImported:
				if it.ProductID > 0 {
					imported = append(imported, importedRow{OfferID: it.OfferID, ProductID: it.ProductID})
				} else {
					// "imported" with product_id==0 is the transient initial state;
					// keep polling until the final status or a real product id arrives.
					pending = true
					continue
				}
				for _, e := range it.Errors {
					// error-level items on an imported product mean "cannot sell yet"
					// (missing required attributes) — report as warnings, not failure.
					if strings.EqualFold(strings.TrimSpace(e.Level), "warning") ||
						strings.EqualFold(strings.TrimSpace(e.Level), "error") {
						warnings = append(warnings, fmt.Sprintf("offer_id=%s %s", it.OfferID, firstNonEmptyErrorMsg(e)))
					}
				}
			case importStatusFailed:
				msgs := make([]string, 0, len(it.Errors))
				for _, e := range it.Errors {
					if strings.EqualFold(strings.TrimSpace(e.Level), "error") {
						msgs = append(msgs, firstNonEmptyErrorMsg(e))
					}
				}
				if len(msgs) == 0 {
					for _, e := range it.Errors {
						msgs = append(msgs, firstNonEmptyErrorMsg(e))
					}
				}
				msg := strings.TrimSpace(strings.Join(msgs, "; "))
				if msg == "" {
					msg = "unknown import error"
				}
				failed = append(failed, fmt.Sprintf("offer_id=%s %s", it.OfferID, msg))
			case importStatusSkipped:
				failed = append(failed, fmt.Sprintf("offer_id=%s import skipped (no product created)", it.OfferID))
			default:
				pending = true
			}
		}
		if !pending {
			return imported, failed, warnings, nil
		}
		lastErr = fmt.Errorf("ozon product import still processing")
	}
	return nil, nil, nil, fmt.Errorf("ozon product import timed out: %v", lastErr)
}

func firstNonEmptyErrorMsg(e importInfoItemError) string {
	return firstNonEmptyMsg(e.Code, e.Message, e.Description)
}

func firstNonEmptyMsg(code, message, description string) string {
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = strings.TrimSpace(description)
	}
	if msg == "" {
		msg = strings.TrimSpace(code)
	}
	return msg
}

func applyStocks(ctx context.Context, client *ozonClient, warehouseID int64, d platformp.PlatformProductDraft, imported []importedRow) []string {
	rows := make([]map[string]any, 0, len(imported))
	for i, s := range d.SKUs {
		offerID := buildOfferID(d, s, i)
		var productID int64
		for _, row := range imported {
			if row.OfferID == offerID {
				productID = row.ProductID
				break
			}
		}
		if productID == 0 {
			continue
		}
		stock := s.Stock
		if stock < 0 {
			stock = 0
		}
		rows = append(rows, map[string]any{
			"offer_id":     offerID,
			"product_id":   productID,
			"stock":        stock,
			"warehouse_id": warehouseID,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	var resp struct {
		Result []struct {
			OfferID string       `json:"offer_id"`
			Updated bool         `json:"updated"`
			Errors  []stockError `json:"errors"`
		} `json:"result"`
	}
	if err := client.postJSONNoRetry(ctx, pathStocks, map[string]any{"stocks": rows}, &resp); err != nil {
		return []string{fmt.Sprintf("stock sync failed: %v", err)}
	}
	var warns []string
	for _, r := range resp.Result {
		if !r.Updated || len(r.Errors) > 0 {
			msgs := make([]string, 0, len(r.Errors))
			for _, e := range r.Errors {
				msgs = append(msgs, firstNonEmptyMsg(e.Code, e.Message, ""))
			}
			warns = append(warns, fmt.Sprintf("offer_id=%s %s", r.OfferID, strings.Join(msgs, "; ")))
		}
	}
	return warns
}

type stockError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func buildSKUMappings(d platformp.PlatformProductDraft, imported []importedRow) []platformp.PlatformSKUMapping {
	out := make([]platformp.PlatformSKUMapping, 0, len(d.SKUs))
	for i, s := range d.SKUs {
		offerID := buildOfferID(d, s, i)
		extID := ""
		var productID int64
		for _, row := range imported {
			if row.OfferID == offerID {
				extID = strconv.FormatInt(row.ProductID, 10)
				productID = row.ProductID
				break
			}
		}
		if extID == "" {
			continue
		}
		pr := s.Price
		st := s.Stock
		rd := platformp.TrimRawMap(map[string]any{
			"offerId":   offerID,
			"productId": productID,
		}, 8, 120)
		out = append(out, platformp.PlatformSKUMapping{
			LocalSKUID:    s.LocalSKUID,
			ExternalSKUID: extID,
			SKUCode:       strings.TrimSpace(s.SKUCode),
			Price:         &pr,
			Stock:         &st,
			RawData:       rd,
		})
	}
	return out
}

func buildOfferID(d platformp.PlatformProductDraft, sku platformp.PlatformProductSKU, idx int) string {
	code := strings.TrimSpace(sku.SKUCode)
	if code == "" {
		code = strings.TrimSpace(sku.SKUName)
	}
	if code == "" {
		code = fmt.Sprintf("TM-%s-%d", shortProductID(d.ProductID.String()), idx+1)
	}
	return sanitizeOfferID(code)
}

func sanitizeOfferID(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.' || r == ':':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := b.String()
	out = strings.Trim(out, "-")
	if out == "" {
		out = "ozon"
	}
	return truncateRunes(out, maxOfferIDRunes)
}

func shortProductID(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}

func orderedImageURLs(imgs []platformp.PlatformProductImage) []string {
	out := make([]string, 0, maxListingImages)
	seen := make(map[string]struct{}, len(imgs))
	for _, im := range imgs {
		if len(out) >= maxListingImages {
			break
		}
		u := strings.TrimSpace(im.URL)
		if u == "" {
			continue
		}
		if _, exists := seen[u]; exists {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	return out
}

type ozonSKUImagePlan struct {
	Images  []string
	Primary string
}

func resolveOzonSKUImagePlan(sku platformp.PlatformProductSKU) (ozonSKUImagePlan, error) {
	images := append([]platformp.PlatformProductImage(nil), sku.Images...)
	if len(images) == 0 && strings.TrimSpace(sku.ImageURL) != "" {
		images = []platformp.PlatformProductImage{{URL: strings.TrimSpace(sku.ImageURL), Type: "main"}}
	}
	urls := orderedImageURLs(images)
	label := strings.TrimSpace(sku.SKUName)
	if label == "" {
		label = strings.TrimSpace(sku.SKUCode)
	}
	if label == "" {
		label = sku.LocalSKUID.String()
	}
	if len(urls) == 0 {
		return ozonSKUImagePlan{}, fmt.Errorf("Ozon SKU %q is missing its original main image and has no explicitly saved fallback", label)
	}
	firstType := ""
	for _, image := range images {
		if strings.TrimSpace(image.URL) != "" {
			firstType = strings.ToLower(strings.TrimSpace(image.Type))
			break
		}
	}
	if firstType != "main" {
		return ozonSKUImagePlan{}, fmt.Errorf("Ozon SKU %q does not have an explicit main image in the first position", label)
	}
	if original := strings.TrimSpace(sku.ImageURL); original != "" && urls[0] != original {
		return ozonSKUImagePlan{}, fmt.Errorf("Ozon SKU %q original main image must be the first image", label)
	}
	return ozonSKUImagePlan{Images: urls, Primary: urls[0]}, nil
}

func formatOzonPrice(p float64) string {
	if p < 0 {
		p = 0
	}
	s := strconv.FormatFloat(p, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func mapOzonPublishError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "permission") ||
		strings.Contains(strings.ToLower(err.Error()), "forbidden") ||
		strings.Contains(strings.ToLower(err.Error()), "unauthorized") ||
		strings.Contains(strings.ToLower(err.Error()), "invalid api key") ||
		strings.Contains(strings.ToLower(err.Error()), "invalid client") {
		return fmt.Errorf("%w: %v", platformp.ErrPlatformProductPublishPermissionDenied, err)
	}
	return err
}

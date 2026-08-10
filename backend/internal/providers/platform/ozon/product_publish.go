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

	// MaxProductImportItemsPerRequest is TradeMind's conservative safety
	// ceiling for one synchronous Ozon import. It is an adapter guardrail, not
	// category metadata: the category APIs do not expose a per-category SKU cap.
	MaxProductImportItemsPerRequest = 100
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

type ozonSKUAttributePlan struct {
	Ordinary []ozonAttributeValue
	Complex  []ozonComplexAttributeGroup
}

// ozonComplexAttributeGroup represents one complex-attribute instance. The
// Legacy payloads are grouped once per complex ID here. Canonical v2 payloads
// provide explicit repeated groups and bypass this inference.
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
	if len(d.SKUs) > MaxProductImportItemsPerRequest {
		return nil, fmt.Errorf("ozon publish contains %d SKUs; TradeMind allows at most %d items in one import request", len(d.SKUs), MaxProductImportItemsPerRequest)
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
	ordinarySchema := make([]ozonAttribute, 0, len(liveAttrs))
	for _, attr := range liveAttrs {
		if attr.AttributeComplexID <= 0 {
			ordinarySchema = append(ordinarySchema, attr)
		}
	}
	attributePlans := make([]ozonSKUAttributePlan, len(d.SKUs))
	resolvedAttributeCount := 0
	missingAttrs := make([]string, 0)
	aiFillUsed := false
	aiFillFailed := false
	resolvedSKUCount := 0
	for _, sku := range d.SKUs {
		if len(sku.PlatformAttributes) > 0 {
			resolvedSKUCount++
		}
	}
	if resolvedSKUCount > 0 {
		if resolvedSKUCount != len(d.SKUs) {
			return nil, fmt.Errorf("ozon publish task snapshot has incomplete per-SKU attribute payloads; rerun preflight")
		}
		variantDimensions := ""
		seenVariantTuples := map[string]string{}
		for i, sku := range d.SKUs {
			resolvedAttrs, parseErr := parseExplicitOzonAttributesValue(sku.PlatformAttributes)
			if parseErr != nil {
				return nil, fmt.Errorf("ozon SKU %s attributes are invalid: %w", skuDisplayNameForPublish(sku), parseErr)
			}
			if resolvedAttrs.Legacy {
				return nil, fmt.Errorf("ozon SKU %s task snapshot does not contain canonical per-SKU attributes; rerun preflight", skuDisplayNameForPublish(sku))
			}
			dimensions := append([]string(nil), resolvedAttrs.SKUVariantAttributeIDs...)
			sort.Strings(dimensions)
			if dimensionErr := validateLiveOzonVariantDimensions(dimensions, liveAttrs); dimensionErr != nil {
				return nil, fmt.Errorf("ozon SKU %s: %w; no Ozon write was sent", skuDisplayNameForPublish(sku), dimensionErr)
			}
			joinedDimensions := strings.Join(dimensions, "\x00")
			if i == 0 {
				variantDimensions = joinedDimensions
			} else if joinedDimensions != variantDimensions {
				return nil, fmt.Errorf("ozon SKU variant dimensions changed inside the task snapshot; rerun preflight")
			}
			if len(d.SKUs) > 1 {
				if len(dimensions) == 0 {
					return nil, fmt.Errorf("ozon multi-SKU publish requires explicit SKU variant attributes; no Ozon write was sent")
				}
				tuple, tupleErr := explicitOzonSKUVariantTuple(dimensions, resolvedAttrs.Attributes)
				if tupleErr != nil {
					return nil, fmt.Errorf("ozon SKU %s: %w", skuDisplayNameForPublish(sku), tupleErr)
				}
				if previous, exists := seenVariantTuples[tuple]; exists {
					return nil, fmt.Errorf("ozon SKU %s duplicates the variant attributes of SKU %s; no Ozon write was sent", skuDisplayNameForPublish(sku), previous)
				}
				seenVariantTuples[tuple] = skuDisplayNameForPublish(sku)
			}
			// Canonical per-SKU snapshots are the final values produced by the
			// shared resolver and shown during confirmation. Applying local or AI
			// auto-fill here would make the Ozon write diverge from that immutable
			// preview, so auto-fill remains a legacy single-SKU compatibility path.
			attrPayload, skuMissing, _, buildErr := client.buildCategoryAttributesForPublish(
				cctx, merged.DescriptionCategoryID, merged.TypeID, nil, merged,
				resolvedAttrs.Attributes, false, ordinarySchema,
			)
			if buildErr != nil {
				return nil, mapOzonPublishError(fmt.Errorf("ozon SKU %s: %w", skuDisplayNameForPublish(sku), buildErr))
			}
			explicitComplex, complexMissing, complexErr := client.buildExplicitOzonComplexGroups(
				cctx, merged.DescriptionCategoryID, merged.TypeID, liveAttrs, resolvedAttrs,
			)
			if complexErr != nil {
				return nil, mapOzonPublishError(fmt.Errorf("ozon SKU %s: %w", skuDisplayNameForPublish(sku), complexErr))
			}
			skuMissing = append(skuMissing, complexMissing...)
			if len(skuMissing) > 0 {
				return nil, fmt.Errorf("ozon SKU %s missing required category attributes: %s", skuDisplayNameForPublish(sku), strings.Join(skuMissing, ", "))
			}
			ordinary, complex := partitionOzonImportAttributes(attrPayload)
			complex = append(complex, explicitComplex...)
			attributePlans[i] = ozonSKUAttributePlan{Ordinary: ordinary, Complex: complex}
			resolvedAttributeCount += len(ordinary) + len(complex)
		}
	} else {
		// Old single-SKU task snapshots remain readable. Multi-SKU snapshots
		// without explicit variant mapping are stopped before product/import.
		if len(d.SKUs) > 1 {
			return nil, fmt.Errorf("ozon multi-SKU publish task has no per-SKU variant mapping; rerun preflight before submitting")
		}
		legacySchema := ordinarySchema
		if explicitAttrs.Legacy {
			legacySchema = liveAttrs
		}
		attrPayload, legacyMissing, missingDefs, buildErr := client.buildCategoryAttributesForPublish(
			cctx, merged.DescriptionCategoryID, merged.TypeID, localAttrs, merged,
			explicitAttrs.Attributes, merged.AutoFillAttributes, legacySchema,
		)
		if buildErr != nil {
			return nil, mapOzonPublishError(buildErr)
		}
		missingAttrs = legacyMissing
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
		explicitComplex, complexMissing, complexErr := client.buildExplicitOzonComplexGroups(cctx, merged.DescriptionCategoryID, merged.TypeID, liveAttrs, explicitAttrs)
		if complexErr != nil {
			return nil, mapOzonPublishError(complexErr)
		}
		missingAttrs = append(missingAttrs, complexMissing...)
		if len(missingAttrs) > 0 {
			return nil, fmt.Errorf("ozon publish missing required category attributes: %s", strings.Join(missingAttrs, ", "))
		}
		ordinary, complex := partitionOzonImportAttributes(attrPayload)
		complex = append(complex, explicitComplex...)
		attributePlans[0] = ozonSKUAttributePlan{Ordinary: ordinary, Complex: complex}
		resolvedAttributeCount = len(ordinary) + len(complex)
	}

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
		attributePlan := attributePlans[i]
		name := title
		if len(d.SKUs) > 1 && strings.TrimSpace(sku.SKUName) != "" {
			name = title + " / " + strings.TrimSpace(sku.SKUName)
		}
		item := ozonImportItem{
			Attributes:            attributePlan.Ordinary,
			ComplexAttributes:     attributePlan.Complex,
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

	imported, failed, importWarnings, err := pollImportInfo(cctx, client, importResp.Result.TaskID)
	if err != nil {
		return nil, mapOzonPublishError(err)
	}
	if len(failed) > 0 {
		return nil, fmt.Errorf("ozon product publish failed: %s", strings.Join(failed, "; "))
	}
	if len(imported) == 0 {
		return nil, fmt.Errorf("ozon product publish: no products imported")
	}
	importWarnings = append(importWarnings, missingImportResultWarnings(d, imported)...)

	// Optional per-SKU stock sync (Ozon stock is per product + warehouse).
	stockWarnings := []platformp.PublishWarning(nil)
	stockSyncStatus := "not_requested"
	if allImportedSKUsHaveExplicitZeroStock(d, imported) {
		// A newly imported offer already has no sellable stock. Sending an
		// immediate zero-stock mutation is unnecessary and can race Ozon's
		// product materialization, producing a misleading "Product is not
		// created" response. Only exact imported-offer/SKU matches qualify.
		stockSyncStatus = "zero_stock_not_required"
	} else if merged.WarehouseID > 0 {
		stockWarnings = applyStocks(cctx, client, merged.WarehouseID, d, imported)
		stockSyncStatus = "synced"
		if len(stockWarnings) > 0 {
			stockSyncStatus = "failed"
		}
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
	warnings := make([]platformp.PublishWarning, 0, len(importWarnings)+len(stockWarnings))
	warnings = append(warnings, importWarnings...)
	warnings = append(warnings, stockWarnings...)
	status := platformp.PublishStatusImported
	if len(warnings) > 0 {
		status = platformp.PublishStatusNeedsAction
	}
	summary := map[string]any{
		"provider":           "ozon",
		"taskId":             importResp.Result.TaskID,
		"imported":           len(imported),
		"platformStatus":     importStatusImported,
		"verificationStatus": status,
		"sellableVerified":   false,
		"needsAction":        status == platformp.PublishStatusNeedsAction,
		"warningCount":       len(warnings),
		"stockSyncStatus":    stockSyncStatus,
		"skuMappings":        len(mappings),
		"categoryId":         merged.DescriptionCategoryID,
		"typeId":             merged.TypeID,
		"currencyCode":       merged.CurrencyCode,
		"resolvedCurrency":   currencyCode,
		"resolvedAttrs":      resolvedAttributeCount,
		"resolvedSKUCount":   len(d.SKUs),
		// Kept for task-summary compatibility; resolvedAttrs is the precise
		// name for canonical multi-SKU snapshots.
		"autoFillAttrs":  resolvedAttributeCount,
		"aiFillUsed":     aiFillUsed,
		"aiFillFailed":   aiFillFailed,
		"missingAttrs":   missingAttrs,
		"importWarnings": publishWarningMessages(importWarnings),
		"stockWarnings":  publishWarningMessages(stockWarnings),
		"warehouseId":    merged.WarehouseID,
	}
	return &platformp.PublishProductResult{
		ExternalProductID: strconv.FormatInt(firstProductID, 10),
		ExternalSPUID:     firstOfferID,
		Status:            status,
		SKUMappings:       mappings,
		Warnings:          warnings,
		RawSummary:        platformp.TrimRawMap(summary, 28, 240),
	}, nil
}

func validateLiveOzonVariantDimensions(dimensions []string, schema []ozonAttribute) error {
	byID := make(map[string]ozonAttribute, len(schema))
	eligibleCount := 0
	for _, attr := range schema {
		id := strconv.FormatInt(attr.ID, 10)
		byID[id] = attr
		if attr.AttributeComplexID <= 0 && attr.IsAspect != nil && *attr.IsAspect {
			eligibleCount++
		}
	}
	seen := make(map[string]struct{}, len(dimensions))
	for _, rawID := range dimensions {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return fmt.Errorf("variant dimension contains an empty attribute id")
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("variant dimension attribute %s is duplicated", id)
		}
		seen[id] = struct{}{}
		attr, exists := byID[id]
		if !exists {
			return fmt.Errorf("live category template no longer contains variant attribute %s", id)
		}
		if attr.AttributeComplexID > 0 {
			return fmt.Errorf("complex attribute %s cannot be a SKU variant dimension", attributeDisplayName(attr))
		}
		if attr.IsAspect == nil {
			return fmt.Errorf("attribute %s has no live is_aspect eligibility evidence", attributeDisplayName(attr))
		}
		if !*attr.IsAspect {
			return fmt.Errorf("attribute %s is not eligible as a SKU variant dimension", attributeDisplayName(attr))
		}
	}
	if len(dimensions) > eligibleCount {
		return fmt.Errorf("variant dimension count %d exceeds the %d dimensions confirmed by the live category template", len(dimensions), eligibleCount)
	}
	return nil
}

type explicitOzonComplexGroup struct {
	ComplexID  int64
	Attributes map[string][]explicitOzonAttribute
}

type explicitOzonAttributesPayload struct {
	Version                int
	Legacy                 bool
	Attributes             map[string][]explicitOzonAttribute
	ComplexGroups          []explicitOzonComplexGroup
	SKUVariantAttributeIDs []string
}

func parseExplicitOzonAttributes(options map[string]any) (explicitOzonAttributesPayload, error) {
	if options == nil {
		return parseExplicitOzonAttributesValue(nil)
	}
	raw, ok := options["platform_attributes"]
	if !ok || raw == nil {
		return parseExplicitOzonAttributesValue(nil)
	}
	return parseExplicitOzonAttributesValue(raw)
}

func parseExplicitOzonAttributesValue(raw any) (explicitOzonAttributesPayload, error) {
	out := explicitOzonAttributesPayload{
		Legacy: true, Attributes: map[string][]explicitOzonAttribute{}, ComplexGroups: []explicitOzonComplexGroup{}, SKUVariantAttributeIDs: []string{},
	}
	if raw == nil {
		return out, nil
	}
	var encoded []byte
	switch v := raw.(type) {
	case json.RawMessage:
		encoded = append([]byte(nil), v...)
	case []byte:
		encoded = append([]byte(nil), v...)
	case map[string]any:
		var err error
		encoded, err = json.Marshal(v)
		if err != nil {
			return out, fmt.Errorf("ozon platform_attributes must be valid JSON: %w", err)
		}
	default:
		return out, fmt.Errorf("ozon platform_attributes must be an object")
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &probe); err != nil {
		return out, fmt.Errorf("ozon platform_attributes must be valid JSON: %w", err)
	}
	if attributesRaw, isCanonical := probe["attributes"]; isCanonical {
		out.Legacy = false
		var version int
		if rawVersion, exists := probe["version"]; exists {
			if err := json.Unmarshal(rawVersion, &version); err != nil {
				return out, fmt.Errorf("ozon platform_attributes.version must be an integer")
			}
		}
		if version != 0 && version != 2 && version != 3 {
			return out, fmt.Errorf("ozon platform_attributes version %d is unsupported", version)
		}
		out.Version = version
		if rawDimensions, exists := probe["skuVariantAttributeIds"]; exists {
			if err := json.Unmarshal(rawDimensions, &out.SKUVariantAttributeIDs); err != nil {
				return out, fmt.Errorf("ozon platform_attributes.skuVariantAttributeIds must be a string array")
			}
			seen := map[string]struct{}{}
			normalized := make([]string, 0, len(out.SKUVariantAttributeIDs))
			for _, rawID := range out.SKUVariantAttributeIDs {
				id := strings.TrimSpace(rawID)
				if id == "" {
					return out, fmt.Errorf("ozon platform_attributes.skuVariantAttributeIds contains an empty attribute id")
				}
				if _, duplicate := seen[id]; duplicate {
					continue
				}
				seen[id] = struct{}{}
				normalized = append(normalized, id)
			}
			out.SKUVariantAttributeIDs = normalized
		}
		var attrs map[string][]map[string]any
		if err := json.Unmarshal(attributesRaw, &attrs); err != nil {
			return out, fmt.Errorf("ozon platform_attributes.attributes must be an object of value arrays")
		}
		for key, values := range attrs {
			parsed, err := parseExplicitOzonAttributeList(key, values, true)
			if err != nil {
				return out, err
			}
			out.Attributes[strings.TrimSpace(key)] = parsed
		}
		var groups []struct {
			ComplexID  int64                       `json:"complexId"`
			Attributes map[string][]map[string]any `json:"attributes"`
		}
		if rawGroups, exists := probe["complexGroups"]; exists {
			if err := json.Unmarshal(rawGroups, &groups); err != nil {
				return out, fmt.Errorf("ozon platform_attributes.complexGroups must be an array")
			}
		}
		for index, group := range groups {
			if group.ComplexID <= 0 {
				return out, fmt.Errorf("ozon complex attribute group %d has invalid complexId", index+1)
			}
			parsedGroup := explicitOzonComplexGroup{ComplexID: group.ComplexID, Attributes: map[string][]explicitOzonAttribute{}}
			for key, values := range group.Attributes {
				parsed, err := parseExplicitOzonAttributeList(key, values, true)
				if err != nil {
					return out, err
				}
				parsedGroup.Attributes[strings.TrimSpace(key)] = parsed
			}
			out.ComplexGroups = append(out.ComplexGroups, parsedGroup)
		}
		return out, nil
	}

	var attrs map[string]map[string]any
	if err := json.Unmarshal(encoded, &attrs); err != nil {
		return out, fmt.Errorf("ozon legacy platform_attributes must map each attribute to {value,dictionaryValueId}")
	}
	for rawKey, rawValue := range attrs {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			return out, fmt.Errorf("ozon platform_attributes contains an empty attribute key")
		}
		parsed, err := parseExplicitOzonAttributeList(key, []map[string]any{rawValue}, false)
		if err != nil {
			return out, err
		}
		out.Attributes[key] = parsed
	}
	return out, nil
}

func parseExplicitOzonAttributeList(key string, values []map[string]any, strict bool) ([]explicitOzonAttribute, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("ozon platform_attributes contains an empty attribute key")
	}
	out := make([]explicitOzonAttribute, 0, len(values))
	for _, nested := range values {
		spec := explicitOzonAttribute{StrictDictionaryID: strict}
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
		out = append(out, spec)
	}
	return out, nil
}

func explicitOzonSKUVariantTuple(attributeIDs []string, attributes map[string][]explicitOzonAttribute) (string, error) {
	parts := make([]string, 0, len(attributeIDs))
	for _, attrID := range attributeIDs {
		values := attributes[attrID]
		if len(values) == 0 {
			return "", fmt.Errorf("missing selected variant attribute %s", attrID)
		}
		encoded := make([]string, 0, len(values))
		for _, value := range values {
			text := strings.TrimSpace(value.Value)
			if text == "" {
				return "", fmt.Errorf("variant attribute %s contains an empty value", attrID)
			}
			encoded = append(encoded, strconv.FormatInt(value.DictionaryValueID, 10)+"\x00"+text)
		}
		sort.Strings(encoded)
		parts = append(parts, attrID+"="+strings.Join(encoded, "\x01"))
	}
	return strings.Join(parts, "\x02"), nil
}

func skuDisplayNameForPublish(sku platformp.PlatformProductSKU) string {
	for _, value := range []string{sku.SKUName, sku.SKUCode, sku.LocalSKUID.String()} {
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return "unknown"
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

func pollImportInfo(ctx context.Context, client *ozonClient, taskID int64) ([]importedRow, []string, []platformp.PublishWarning, error) {
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
		warnings := make([]platformp.PublishWarning, 0, len(rows))
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
					// (missing required attributes). Preserve the platform-created product,
					// but mark the result as needs_action instead of published/sellable.
					severity := strings.ToLower(strings.TrimSpace(e.Level))
					if severity == "warning" || severity == "error" {
						msg := firstNonEmptyErrorMsg(e)
						if strings.TrimSpace(msg) == "" {
							msg = "unknown import warning"
						}
						warnings = append(warnings, platformp.PublishWarning{
							Stage:    "import",
							Severity: severity,
							Code:     strings.TrimSpace(e.Code),
							Field:    strings.TrimSpace(e.Field),
							OfferID:  strings.TrimSpace(it.OfferID),
							Message:  msg,
						})
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

func applyStocks(ctx context.Context, client *ozonClient, warehouseID int64, d platformp.PlatformProductDraft, imported []importedRow) []platformp.PublishWarning {
	rows := make([]map[string]any, 0, len(imported))
	requestedOffers := make([]string, 0, len(imported))
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
		requestedOffers = append(requestedOffers, offerID)
	}
	if len(rows) == 0 {
		return []platformp.PublishWarning{{
			Stage:    "stock",
			Severity: "error",
			Code:     "stock_sync_no_matching_skus",
			Message:  "stock sync could not match any imported offer to a local SKU",
		}}
	}
	var resp struct {
		Result []struct {
			OfferID string       `json:"offer_id"`
			Updated bool         `json:"updated"`
			Errors  []stockError `json:"errors"`
		} `json:"result"`
	}
	if err := client.postJSONNoRetry(ctx, pathStocks, map[string]any{"stocks": rows}, &resp); err != nil {
		return []platformp.PublishWarning{{
			Stage:    "stock",
			Severity: "error",
			Code:     "stock_sync_request_failed",
			Message:  fmt.Sprintf("stock sync failed: %v", err),
		}}
	}
	warns := make([]platformp.PublishWarning, 0, len(resp.Result))
	returnedOffers := make(map[string]struct{}, len(resp.Result))
	for _, r := range resp.Result {
		offerID := strings.TrimSpace(r.OfferID)
		returnedOffers[offerID] = struct{}{}
		if !r.Updated || len(r.Errors) > 0 {
			if len(r.Errors) == 0 {
				warns = append(warns, platformp.PublishWarning{
					Stage:    "stock",
					Severity: "error",
					Code:     "stock_not_updated",
					OfferID:  offerID,
					Message:  "stock was not updated",
				})
				continue
			}
			for _, e := range r.Errors {
				msg := firstNonEmptyMsg(e.Code, e.Message, "")
				if strings.TrimSpace(msg) == "" {
					msg = "stock update rejected"
				}
				warns = append(warns, platformp.PublishWarning{
					Stage:    "stock",
					Severity: "error",
					Code:     strings.TrimSpace(e.Code),
					OfferID:  offerID,
					Message:  msg,
				})
			}
		}
	}
	for _, offerID := range requestedOffers {
		if _, ok := returnedOffers[offerID]; ok {
			continue
		}
		warns = append(warns, platformp.PublishWarning{
			Stage:    "stock",
			Severity: "error",
			Code:     "stock_result_missing",
			OfferID:  offerID,
			Message:  "stock sync response did not include this offer",
		})
	}
	return warns
}

func allImportedSKUsHaveExplicitZeroStock(d platformp.PlatformProductDraft, imported []importedRow) bool {
	if len(imported) == 0 || len(d.SKUs) == 0 {
		return false
	}
	stockByOffer := make(map[string]int, len(d.SKUs))
	for i, sku := range d.SKUs {
		offerID := strings.TrimSpace(buildOfferID(d, sku, i))
		if offerID == "" {
			return false
		}
		if _, duplicate := stockByOffer[offerID]; duplicate {
			return false
		}
		stockByOffer[offerID] = sku.Stock
	}
	seenImported := make(map[string]struct{}, len(imported))
	for _, row := range imported {
		offerID := strings.TrimSpace(row.OfferID)
		if offerID == "" || row.ProductID <= 0 {
			return false
		}
		if _, duplicate := seenImported[offerID]; duplicate {
			return false
		}
		seenImported[offerID] = struct{}{}
		stock, matched := stockByOffer[offerID]
		if !matched || stock != 0 {
			// Positive, negative, or unmatched/unknown stock must follow the
			// existing stock-sync/error path; none is a safe explicit zero.
			return false
		}
	}
	return true
}

func publishWarningMessages(warnings []platformp.PublishWarning) []string {
	if len(warnings) == 0 {
		return nil
	}
	out := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		prefix := ""
		if offerID := strings.TrimSpace(warning.OfferID); offerID != "" {
			prefix = "offer_id=" + offerID + " "
		}
		out = append(out, prefix+strings.TrimSpace(warning.Message))
	}
	return out
}

func missingImportResultWarnings(d platformp.PlatformProductDraft, imported []importedRow) []platformp.PublishWarning {
	seen := make(map[string]struct{}, len(imported))
	for _, row := range imported {
		if offerID := strings.TrimSpace(row.OfferID); offerID != "" {
			seen[offerID] = struct{}{}
		}
	}
	warnings := make([]platformp.PublishWarning, 0)
	for i, sku := range d.SKUs {
		offerID := strings.TrimSpace(buildOfferID(d, sku, i))
		if offerID == "" {
			continue
		}
		if _, ok := seen[offerID]; ok {
			continue
		}
		warnings = append(warnings, platformp.PublishWarning{
			Stage:    "import",
			Severity: "error",
			Code:     "import_result_missing",
			OfferID:  offerID,
			Message:  "product import response did not include this offer",
		})
	}
	return warnings
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

package product

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
)

const (
	OzonCategoryRecommendationReady              = "ready"
	OzonCategoryRecommendationPartial            = "partial"
	OzonCategoryRecommendationNoMatch            = "no_match"
	OzonCategoryRecommendationAIUnavailable      = "ai_unavailable"
	OzonCategoryRecommendationCategoryCacheEmpty = "category_cache_empty"

	OzonCategoryRecommendationInvalid     = "OZON_CATEGORY_RECOMMENDATION_INVALID"
	OzonCategoryRecommendationUnavailable = "OZON_CATEGORY_RECOMMENDATION_UNAVAILABLE"

	OzonListingStrategyGroupAll       = "group_all"
	OzonListingStrategyGroupSubset    = "group_subset"
	OzonListingStrategySplitSingleSKU = "split_single_sku"
	OzonListingStrategyManualReview   = "manual_review"

	OzonRecommendationRefreshIfMissingOrStale = "if_missing_or_stale"
	OzonRecommendationRefreshCacheOnly        = "cache_only"
)

// AIChatClient keeps the recommendation business flow behind the shared AI
// provider boundary while allowing deterministic fake clients in tests.
type AIChatClient interface {
	Chat(context.Context, aigate.ChatRequest) (*aigate.ChatResponse, error)
}

// OzonCategoryRecommendationCatalog is the narrow, read-only shop boundary
// used by product recommendation orchestration.
type OzonCategoryRecommendationCatalog interface {
	EnsureAuthorizedOzonShop(context.Context, int64, uuid.UUID) error
	ListOzonCategories(context.Context, shop.OzonCategoryListQuery) (*shop.OzonCategoryListResult, error)
	SearchOzonLeafCategories(context.Context, shop.OzonCategorySearchQuery) (*shop.OzonCategorySearchResult, error)
	ListOzonCategoryAttributes(context.Context, string) ([]shop.OzonAttributeDTO, error)
	RefreshOzonCategoryAttributeTemplate(context.Context, int64, string, uuid.UUID) (*shop.OzonCategoryStats, error)
	ListOzonCategoryMappings(context.Context, int64, *uuid.UUID) ([]shop.OzonCategoryMappingDTO, error)
}

type OzonCategoryRecommendationBody struct {
	ShopID        string   `json:"shopId"`
	SKUIDs        []string `json:"skuIds"`
	RefreshPolicy string   `json:"refreshPolicy"`
}

type OzonRecommendationSourceSummary struct {
	ProductTitle          string   `json:"productTitle"`
	SKUCount              int      `json:"skuCount"`
	SelectedSKUCount      int      `json:"selectedSkuCount"`
	SKUGroupNames         []string `json:"skuGroupNames"`
	ProductAttributeCount int      `json:"productAttributeCount"`
	PrimaryEvidence       string   `json:"primaryEvidence"`
}

type OzonRecommendationEvidence struct {
	SKUID     string `json:"skuId"`
	SKUCode   string `json:"skuCode,omitempty"`
	Source    string `json:"source"`
	SourceKey string `json:"sourceKey"`
	RawValue  string `json:"rawValue"`
}

type OzonRecommendationDifferenceDimension struct {
	Key        string                       `json:"key"`
	Name       string                       `json:"name"`
	Semantic   string                       `json:"semantic"`
	Confidence float64                      `json:"confidence"`
	Evidence   []OzonRecommendationEvidence `json:"evidence"`
}

type OzonRecommendationAnomaly struct {
	Type       string                       `json:"type"`
	Message    string                       `json:"message"`
	SKUIDs     []string                     `json:"skuIds"`
	Confidence float64                      `json:"confidence"`
	Evidence   []OzonRecommendationEvidence `json:"evidence,omitempty"`
}

type OzonRecommendationCoverage struct {
	Matched int     `json:"matched"`
	Total   int     `json:"total"`
	Ratio   float64 `json:"ratio"`
}

type OzonRecommendationMatchedDimension struct {
	SourceDimensionKey  string `json:"sourceDimensionKey"`
	SourceDimensionName string `json:"sourceDimensionName"`
	TargetAttributeID   string `json:"targetAttributeId"`
	TargetAttributeName string `json:"targetAttributeName"`
	IsAspect            bool   `json:"isAspect"`
	IsAspectKnown       bool   `json:"isAspectKnown"`
}

type OzonRecommendationUnmatchedDimension struct {
	SourceDimensionKey  string `json:"sourceDimensionKey"`
	SourceDimensionName string `json:"sourceDimensionName"`
	Reason              string `json:"reason"`
}

type OzonCategoryRecommendationCandidate struct {
	CategoryID          string                                 `json:"categoryId"`
	CategoryPath        string                                 `json:"categoryPath"`
	Score               float64                                `json:"score"`
	Confidence          float64                                `json:"confidence"`
	Approximate         bool                                   `json:"approximate"`
	VariantCoverage     OzonRecommendationCoverage             `json:"variantCoverage"`
	RequiredCoverage    OzonRecommendationCoverage             `json:"requiredCoverage"`
	MatchedDimensions   []OzonRecommendationMatchedDimension   `json:"matchedDimensions"`
	UnmatchedDimensions []OzonRecommendationUnmatchedDimension `json:"unmatchedDimensions"`
	ListingStrategy     string                                 `json:"listingStrategy"`
	Reasons             []string                               `json:"reasons"`
	Warnings            []string                               `json:"warnings"`
	SchemaHash          string                                 `json:"schemaHash"`
	TemplateSyncedAt    *time.Time                             `json:"templateSyncedAt,omitempty"`
}

type OzonCategoryRecommendationResult struct {
	Status               string                                  `json:"status"`
	TaskID               *uuid.UUID                              `json:"taskId,omitempty"`
	SourceSummary        OzonRecommendationSourceSummary         `json:"sourceSummary"`
	ProductType          string                                  `json:"productType,omitempty"`
	DifferenceDimensions []OzonRecommendationDifferenceDimension `json:"differenceDimensions"`
	Anomalies            []OzonRecommendationAnomaly             `json:"anomalies"`
	Candidates           []OzonCategoryRecommendationCandidate   `json:"candidates"`
	Warnings             []string                                `json:"warnings"`
}

type ozonRecommendationAPIError struct {
	status  int
	code    string
	message string
	err     error
}

func (e *ozonRecommendationAPIError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *ozonRecommendationAPIError) Unwrap() error       { return e.err }
func (e *ozonRecommendationAPIError) HTTPStatus() int     { return e.status }
func (e *ozonRecommendationAPIError) SafeMessage() string { return e.message }
func (e *ozonRecommendationAPIError) SafeData() any {
	return map[string]any{"errorCode": e.code}
}

func invalidOzonRecommendation(message string, err error) error {
	return &ozonRecommendationAPIError{
		status: http.StatusBadRequest, code: OzonCategoryRecommendationInvalid,
		message: message, err: err,
	}
}

func unavailableOzonRecommendation(message string, err error) error {
	return &ozonRecommendationAPIError{
		status: http.StatusServiceUnavailable, code: OzonCategoryRecommendationUnavailable,
		message: message, err: err,
	}
}

type ozonRecommendationSnapshot struct {
	ProductID         string                            `json:"productId"`
	Title             string                            `json:"title"`
	Description       string                            `json:"description,omitempty"`
	ProductAttributes map[string]string                 `json:"productAttributes,omitempty"`
	SKUGroups         []ozonRecommendationSnapshotGroup `json:"skuGroups"`
	SKUs              []ozonRecommendationSnapshotSKU   `json:"skus"`
}

type ozonRecommendationSnapshotGroup struct {
	Name    string   `json:"name"`
	Options []string `json:"options"`
}

type ozonRecommendationSnapshotSKU struct {
	ID         string            `json:"id"`
	Code       string            `json:"code,omitempty"`
	Name       string            `json:"name,omitempty"`
	Selections map[string]string `json:"selections"`
}

type ozonAIAnalysis struct {
	ProductType          string                      `json:"productType"`
	SearchTerms          []string                    `json:"searchTerms"`
	DifferenceDimensions []ozonAIDifferenceDimension `json:"differenceDimensions"`
	Anomalies            []ozonAIAnomaly             `json:"anomalies"`
}

type ozonAIDifferenceDimension struct {
	Key        string           `json:"key"`
	Name       string           `json:"name"`
	Semantic   string           `json:"semantic"`
	Confidence float64          `json:"confidence"`
	Evidence   []ozonAIEvidence `json:"evidence"`
}

type ozonAIAnomaly struct {
	Type       string           `json:"type"`
	Message    string           `json:"message"`
	SKUIDs     []string         `json:"skuIds"`
	Confidence float64          `json:"confidence"`
	Evidence   []ozonAIEvidence `json:"evidence"`
}

type ozonAIEvidence struct {
	SKUID     string `json:"skuId"`
	Source    string `json:"source"`
	SourceKey string `json:"sourceKey"`
	RawValue  string `json:"rawValue"`
}

type ozonAIRerank struct {
	Ranked []ozonAIRerankItem `json:"ranked"`
}

type ozonAIRerankItem struct {
	CandidateKey string   `json:"candidateKey"`
	Confidence   float64  `json:"confidence"`
	Approximate  bool     `json:"approximate"`
	Reasons      []string `json:"reasons"`
	Warnings     []string `json:"warnings"`
}

type ozonAIPathSelection struct {
	Selected []ozonAIPathSelectionItem `json:"selected"`
}

type ozonAIPathSelectionItem struct {
	CandidateKey string   `json:"candidateKey"`
	SubjectMatch string   `json:"subjectMatch"`
	Confidence   float64  `json:"confidence"`
	Reasons      []string `json:"reasons"`
	Warnings     []string `json:"warnings"`
}

type ozonAIFinalReview struct {
	Verdicts []ozonAIFinalReviewItem `json:"verdicts"`
}

type ozonAIFinalReviewItem struct {
	CandidateKey string   `json:"candidateKey"`
	SubjectMatch string   `json:"subjectMatch"`
	Confidence   float64  `json:"confidence"`
	Reasons      []string `json:"reasons"`
	Warnings     []string `json:"warnings"`
}

func decodeStrictOzonRecommendationJSON(content string, target any) error {
	decoder := json.NewDecoder(bytes.NewBufferString(strings.TrimSpace(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple json values")
		}
		return err
	}
	return nil
}

func parseOzonAIAnalysis(content string, snapshot ozonRecommendationSnapshot) (*ozonAIAnalysis, error) {
	var out ozonAIAnalysis
	if err := decodeStrictOzonRecommendationJSON(content, &out); err != nil {
		return nil, fmt.Errorf("invalid analysis json: %w", err)
	}
	out.ProductType = truncateRunes(strings.TrimSpace(out.ProductType), 160)
	if out.ProductType == "" {
		return nil, fmt.Errorf("productType is required")
	}
	if len(out.SearchTerms) == 0 || len(out.SearchTerms) > 12 {
		return nil, fmt.Errorf("searchTerms must contain 1 to 12 items")
	}
	seenTerms := map[string]bool{}
	terms := make([]string, 0, len(out.SearchTerms))
	for _, term := range out.SearchTerms {
		term = truncateRunes(strings.TrimSpace(term), 80)
		key := strings.ToLower(term)
		if term == "" || seenTerms[key] {
			continue
		}
		seenTerms[key] = true
		terms = append(terms, term)
	}
	if len(terms) == 0 {
		return nil, fmt.Errorf("searchTerms are empty")
	}
	out.SearchTerms = terms
	if len(out.DifferenceDimensions) > 16 || len(out.Anomalies) > 20 {
		return nil, fmt.Errorf("analysis exceeds bounded result size")
	}
	allowedSemantic := map[string]bool{
		"model": true, "control_method": true, "current": true,
		"package": true, "color": true, "size": true, "material": true, "other": true,
	}
	seenDimensions := map[string]bool{}
	for i := range out.DifferenceDimensions {
		dimension := &out.DifferenceDimensions[i]
		dimension.Key = truncateRunes(strings.TrimSpace(dimension.Key), 64)
		dimension.Name = truncateRunes(strings.TrimSpace(dimension.Name), 80)
		dimension.Semantic = strings.ToLower(strings.TrimSpace(dimension.Semantic))
		if dimension.Key == "" || dimension.Name == "" || seenDimensions[dimension.Key] {
			return nil, fmt.Errorf("difference dimension key/name must be unique and non-empty")
		}
		seenDimensions[dimension.Key] = true
		if !allowedSemantic[dimension.Semantic] {
			return nil, fmt.Errorf("unsupported dimension semantic %q", dimension.Semantic)
		}
		if dimension.Confidence < 0 || dimension.Confidence > 1 {
			return nil, fmt.Errorf("dimension confidence must be between 0 and 1")
		}
		if len(dimension.Evidence) == 0 || len(dimension.Evidence) > 200 {
			return nil, fmt.Errorf("difference dimension evidence is required and bounded")
		}
		values := map[string]bool{}
		skus := map[string]bool{}
		for j := range dimension.Evidence {
			if err := validateOzonAIEvidence(&dimension.Evidence[j], snapshot); err != nil {
				return nil, fmt.Errorf("dimension %s: %w", dimension.Key, err)
			}
			values[dimension.Evidence[j].RawValue] = true
			skus[dimension.Evidence[j].SKUID] = true
		}
		if len(snapshot.SKUs) > 1 && (len(values) < 2 || len(skus) < 2) {
			dimension.Evidence = completeOzonAIDimensionEvidence(dimension.Evidence, snapshot, 12)
			values = map[string]bool{}
			skus = map[string]bool{}
			for _, evidence := range dimension.Evidence {
				values[evidence.RawValue] = true
				skus[evidence.SKUID] = true
			}
			if len(values) < 2 || len(skus) < 2 {
				return nil, fmt.Errorf("dimension %s does not prove a cross-SKU difference", dimension.Key)
			}
		}
	}
	allowedAnomaly := map[string]bool{
		"different_product_subject": true, "mixed_product_subject": true,
		"inconsistent_selection": true, "other": true,
	}
	for i := range out.Anomalies {
		anomaly := &out.Anomalies[i]
		anomaly.Type = strings.ToLower(strings.TrimSpace(anomaly.Type))
		anomaly.Message = truncateRunes(strings.TrimSpace(anomaly.Message), 240)
		if !allowedAnomaly[anomaly.Type] || anomaly.Message == "" {
			return nil, fmt.Errorf("invalid anomaly")
		}
		if anomaly.Confidence < 0 || anomaly.Confidence > 1 {
			return nil, fmt.Errorf("anomaly confidence must be between 0 and 1")
		}
		seenSKU := map[string]bool{}
		for _, skuID := range anomaly.SKUIDs {
			if _, ok := snapshotSKUByID(snapshot, skuID); !ok {
				return nil, fmt.Errorf("anomaly references unknown skuId")
			}
			seenSKU[skuID] = true
		}
		anomaly.SKUIDs = anomaly.SKUIDs[:0]
		for skuID := range seenSKU {
			anomaly.SKUIDs = append(anomaly.SKUIDs, skuID)
		}
		sort.Strings(anomaly.SKUIDs)
		for j := range anomaly.Evidence {
			if err := validateOzonAIEvidence(&anomaly.Evidence[j], snapshot); err != nil {
				return nil, fmt.Errorf("anomaly: %w", err)
			}
			if !seenSKU[anomaly.Evidence[j].SKUID] {
				return nil, fmt.Errorf("anomaly evidence must belong to an anomaly skuId")
			}
		}
		if len(anomaly.SKUIDs) == 0 || len(anomaly.Evidence) == 0 {
			return nil, fmt.Errorf("anomaly skuIds and evidence are required")
		}
		for skuID := range seenSKU {
			foundEvidence := false
			for _, evidence := range anomaly.Evidence {
				if evidence.SKUID == skuID {
					foundEvidence = true
					break
				}
			}
			if !foundEvidence {
				return nil, fmt.Errorf("every anomaly skuId must have persisted evidence")
			}
		}
	}
	return &out, nil
}

func completeOzonAIDimensionEvidence(
	evidence []ozonAIEvidence,
	snapshot ozonRecommendationSnapshot,
	limit int,
) []ozonAIEvidence {
	if limit <= 0 || len(evidence) == 0 {
		return evidence
	}
	out := append([]ozonAIEvidence{}, evidence...)
	seenEvidence := map[string]bool{}
	seenSourceValues := map[string]bool{}
	seenSourceKeys := map[string]bool{}
	sourceKeys := make([]string, 0, len(evidence))
	for _, item := range evidence {
		fingerprint := item.SKUID + "\x00" + item.SourceKey + "\x00" + item.RawValue
		seenEvidence[fingerprint] = true
		normalizedKey := strings.ToLower(strings.TrimSpace(item.SourceKey))
		seenSourceValues[normalizedKey+"\x00"+strings.TrimSpace(item.RawValue)] = true
		if normalizedKey != "" && !seenSourceKeys[normalizedKey] {
			seenSourceKeys[normalizedKey] = true
			sourceKeys = append(sourceKeys, item.SourceKey)
		}
	}
	for _, sourceKey := range sourceKeys {
		for _, sku := range snapshot.SKUs {
			for key, value := range sku.Selections {
				if !strings.EqualFold(strings.TrimSpace(key), strings.TrimSpace(sourceKey)) {
					continue
				}
				value = strings.TrimSpace(value)
				if value == "" {
					continue
				}
				normalizedKey := strings.ToLower(strings.TrimSpace(key))
				if seenSourceValues[normalizedKey+"\x00"+value] {
					continue
				}
				fingerprint := sku.ID + "\x00" + key + "\x00" + value
				if seenEvidence[fingerprint] {
					continue
				}
				out = append(out, ozonAIEvidence{
					SKUID: sku.ID, Source: "sku.attrs", SourceKey: key, RawValue: value,
				})
				seenEvidence[fingerprint] = true
				seenSourceValues[normalizedKey+"\x00"+value] = true
				if len(out) >= limit {
					return out
				}
			}
		}
	}
	return out
}

func validateOzonAIEvidence(evidence *ozonAIEvidence, snapshot ozonRecommendationSnapshot) error {
	evidence.SKUID = strings.TrimSpace(evidence.SKUID)
	evidence.Source = strings.TrimSpace(evidence.Source)
	evidence.SourceKey = truncateRunes(strings.TrimSpace(evidence.SourceKey), 120)
	evidence.RawValue = truncateRunes(strings.TrimSpace(evidence.RawValue), 240)
	if evidence.Source != "sku.attrs" {
		return fmt.Errorf("evidence source must be sku.attrs")
	}
	sku, ok := snapshotSKUByID(snapshot, evidence.SKUID)
	if !ok {
		return fmt.Errorf("evidence references unknown skuId")
	}
	for key, value := range sku.Selections {
		if strings.EqualFold(strings.TrimSpace(key), evidence.SourceKey) && strings.TrimSpace(value) == evidence.RawValue {
			evidence.SourceKey = key
			evidence.RawValue = value
			return nil
		}
	}
	return fmt.Errorf("evidence is not an exact persisted SKU selection")
}

func snapshotSKUByID(snapshot ozonRecommendationSnapshot, id string) (ozonRecommendationSnapshotSKU, bool) {
	for _, sku := range snapshot.SKUs {
		if sku.ID == strings.TrimSpace(id) {
			return sku, true
		}
	}
	return ozonRecommendationSnapshotSKU{}, false
}

func parseOzonAIRerank(content string, allowlist map[string]bool) (*ozonAIRerank, error) {
	var out ozonAIRerank
	if err := decodeStrictOzonRecommendationJSON(content, &out); err != nil {
		return nil, fmt.Errorf("invalid rerank json: %w", err)
	}
	if len(out.Ranked) == 0 || len(out.Ranked) > 5 {
		return nil, fmt.Errorf("ranked must contain 1 to 5 candidates")
	}
	seen := map[string]bool{}
	for i := range out.Ranked {
		item := &out.Ranked[i]
		item.CandidateKey = strings.TrimSpace(item.CandidateKey)
		if !allowlist[item.CandidateKey] || seen[item.CandidateKey] {
			return nil, fmt.Errorf("candidateKey is outside the server allowlist")
		}
		seen[item.CandidateKey] = true
		if item.Confidence < 0 || item.Confidence > 1 {
			return nil, fmt.Errorf("candidate confidence must be between 0 and 1")
		}
		item.Reasons = boundedStrings(item.Reasons, 5, 240)
		item.Warnings = boundedStrings(item.Warnings, 5, 240)
	}
	return &out, nil
}

func boundedStrings(values []string, maxItems, maxRunes int) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = truncateRunes(strings.TrimSpace(value), maxRunes)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

func flatStringMap(raw json.RawMessage, onlySelectionContainers bool) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 || !json.Valid(raw) {
		return out
	}
	var decoded any
	if json.Unmarshal(raw, &decoded) != nil {
		return out
	}
	root, ok := decoded.(map[string]any)
	if !ok {
		return out
	}
	if onlySelectionContainers {
		for _, key := range []string{"properties", "attrs", "selections"} {
			if nested, exists := root[key]; exists {
				flattenStringValues(nested, "", out, 40)
			}
		}
		return out
	}
	flattenStringValues(root, "", out, 40)
	return out
}

func flattenStringValues(value any, prefix string, out map[string]string, limit int) {
	if len(out) >= limit {
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		if scalar, ok := scalarRecommendationValue(typed["value"]); ok && prefix != "" {
			addOzonRecommendationSelection(out, prefix, scalar, limit)
			return
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			next := strings.TrimSpace(key)
			if prefix != "" && next != "" {
				next = prefix + "." + next
			}
			if scalar, ok := scalarRecommendationValue(typed[key]); ok {
				addOzonRecommendationSelection(out, next, scalar, limit)
				continue
			}
			flattenStringValues(typed[key], next, out, limit)
			if len(out) >= limit {
				return
			}
		}
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if scalar, ok := scalarRecommendationValue(item); ok {
				parts = append(parts, scalar)
			}
		}
		if prefix != "" && len(parts) > 0 {
			addOzonRecommendationSelection(out, prefix, strings.Join(parts, " / "), limit)
		}
	}
}

func addOzonRecommendationSelection(out map[string]string, key, value string, limit int) {
	if len(out) >= limit {
		return
	}
	key = truncateRunes(strings.TrimSpace(key), 120)
	value = truncateRunes(strings.TrimSpace(value), 240)
	if key == "" || value == "" || !ozonRecommendationSelectionKeyAllowed(key) {
		return
	}
	lowerValue := strings.ToLower(value)
	if strings.HasPrefix(lowerValue, "http://") || strings.HasPrefix(lowerValue, "https://") || strings.HasPrefix(lowerValue, "data:") {
		return
	}
	out[key] = value
}

func ozonRecommendationSelectionKeyAllowed(key string) bool {
	normalized := normalizeOzonRecommendationText(key)
	for _, blocked := range []string{
		"image", "img", "图片", "主图", "url", "链接",
		"inventory", "stock", "库存", "warehouse", "仓库",
		"credential", "password", "secret", "token", "cookie", "authorization", "apikey", "accesskey",
		"price", "cost", "售价", "价格", "成本",
	} {
		if strings.Contains(normalized, normalizeOzonRecommendationText(blocked)) {
			return false
		}
	}
	return true
}

func scalarRecommendationValue(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		return trimmed, trimmed != ""
	case json.Number:
		return typed.String(), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(typed), true
	default:
		return "", false
	}
}

func parseRecommendationSKUGroups(raw json.RawMessage) []ozonRecommendationSnapshotGroup {
	if len(raw) == 0 || !json.Valid(raw) {
		return []ozonRecommendationSnapshotGroup{}
	}
	var rows []map[string]any
	if json.Unmarshal(raw, &rows) != nil {
		return []ozonRecommendationSnapshotGroup{}
	}
	out := make([]ozonRecommendationSnapshotGroup, 0, len(rows))
	for _, row := range rows {
		name := firstRecommendationString(row, "name", "label", "title")
		if name == "" {
			continue
		}
		options := make([]string, 0)
		if rawOptions, ok := row["options"].([]any); ok {
			for _, rawOption := range rawOptions {
				var option string
				switch typed := rawOption.(type) {
				case string:
					option = strings.TrimSpace(typed)
				case map[string]any:
					option = firstRecommendationString(typed, "label", "name", "value")
				}
				if option != "" {
					options = append(options, truncateRunes(option, 120))
				}
				if len(options) >= 80 {
					break
				}
			}
		}
		out = append(out, ozonRecommendationSnapshotGroup{Name: truncateRunes(name, 120), Options: options})
		if len(out) >= 16 {
			break
		}
	}
	return out
}

func firstRecommendationString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := scalarRecommendationValue(row[key]); ok {
			return value
		}
	}
	return ""
}

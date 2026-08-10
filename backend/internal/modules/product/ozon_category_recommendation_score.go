package product

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

var (
	ozonRecommendationModelValuePattern   = regexp.MustCompile(`(?i)\b[a-z]{2,}[a-z0-9-]*\d[a-z0-9-]*\b`)
	ozonRecommendationCurrentValuePattern = regexp.MustCompile(`(?i)(^|[^a-z0-9])\d+(?:\.\d+)?\s*a([^a-z]|$)`)
)

const ozonRecommendationMinSemanticScore = 0.25

type ozonRecommendationCandidateWork struct {
	node                     shop.OzonCategoryNodeDTO
	attrs                    []shop.OzonAttributeDTO
	semantic                 float64
	searchScore              float64
	searchTerms              []string
	searchLanes              []string
	history                  float64
	key                      string
	pathConfidence           float64
	pathApproximate          bool
	pathReasons              []string
	pathWarnings             []string
	aiConfidence             float64
	aiApproximate            bool
	aiReasons                []string
	aiWarnings               []string
	result                   OzonCategoryRecommendationCandidate
	templateStale            bool
	templateRefreshAttempted bool
}

func normalizeOzonRecommendationText(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func ozonRecommendationSemanticScore(node shop.OzonCategoryNodeDTO, productType string, terms []string, productTitle string) float64 {
	path := normalizeOzonRecommendationText(node.Path)
	if path == "" {
		path = normalizeOzonRecommendationText(node.Name)
	}
	name := normalizeOzonRecommendationText(node.Name)
	inputs := append([]string{productType}, terms...)
	inputs = append(inputs, productTitle)
	best := 0.0
	total := 0.0
	matched := 0
	for _, input := range inputs {
		term := normalizeOzonRecommendationText(input)
		if term == "" {
			continue
		}
		nameScore := recommendationTextSimilarity(name, term)
		pathScore := recommendationTextSimilarity(path, term)
		score := nameScore*0.75 + math.Min(pathScore, 0.45)*0.25
		if name == "" {
			score = pathScore
		}
		if score >= 0.2 {
			total += score
			matched++
		}
		if score > best {
			best = score
		}
	}
	if matched == 0 {
		return 0
	}
	average := total / float64(matched)
	return clampRecommendationRatio(best*0.7 + average*0.3)
}

func recommendationTextSimilarity(candidate, input string) float64 {
	if candidate == "" || input == "" {
		return 0
	}
	candidateRunes := []rune(candidate)
	inputRunes := []rune(input)
	switch {
	case candidate == input:
		return 1
	case len(inputRunes) >= 2 && strings.Contains(candidate, input):
		return 0.92
	case len(candidateRunes) >= 2 && strings.Contains(input, candidate):
		return 0.84
	default:
		return recommendationRuneOverlap(candidate, input)
	}
}

func recommendationRuneOverlap(left, right string) float64 {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	if len(leftRunes) < 2 || len(rightRunes) < 2 {
		return 0
	}
	leftSet := map[string]bool{}
	for index := 0; index+1 < len(leftRunes); index++ {
		leftSet[string(leftRunes[index:index+2])] = true
	}
	common := 0
	rightSet := map[string]bool{}
	for index := 0; index+1 < len(rightRunes); index++ {
		rightSet[string(rightRunes[index:index+2])] = true
	}
	seen := map[string]bool{}
	for pair := range rightSet {
		if leftSet[pair] && !seen[pair] {
			common++
			seen[pair] = true
		}
	}
	denominator := math.Min(float64(len(leftSet)), float64(len(rightSet)))
	if denominator == 0 {
		return 0
	}
	ratio := float64(common) / denominator
	if ratio < 0.45 {
		return 0
	}
	return ratio * 0.70
}

func scoreOzonRecommendationCandidate(
	work *ozonRecommendationCandidateWork,
	snapshot ozonRecommendationSnapshot,
	dimensions []OzonRecommendationDifferenceDimension,
	anomalies []OzonRecommendationAnomaly,
) {
	matched, unmatched := matchOzonRecommendationDimensions(dimensions, work.attrs)
	variant := recommendationCoverage(len(matched), len(dimensions))
	required := requiredOzonRecommendationCoverage(snapshot, work.attrs)
	score := variant.Ratio*45 + work.semantic*30 + required.Ratio*15 + work.history*10
	strategy := ozonRecommendationListingStrategy(len(snapshot.SKUs), variant, anomalies)
	warnings := make([]string, 0, len(unmatched)+3)
	for _, dimension := range unmatched {
		warnings = append(warnings, dimension.SourceDimensionName+"："+dimension.Reason)
	}
	if work.templateStale {
		warnings = append(warnings, "属性模板缓存可能已过期，应用后请先刷新并人工核对")
	}
	if strategy == OzonListingStrategySplitSingleSKU {
		warnings = append(warnings, "检测到不同商品主体 SKU，建议拆分为单 SKU 刊登并人工复核")
	}
	if strategy == OzonListingStrategyManualReview {
		warnings = append(warnings, "当前模板不能安全承载全部 SKU 区别，请人工复核或拆分")
	}
	reasons := []string{
		"SKU 区别承载度 " + recommendationPercent(variant.Ratio),
		"商品语义匹配度 " + recommendationPercent(work.semantic),
		"必填属性预计可填写率 " + recommendationPercent(required.Ratio),
	}
	if work.history > 0 {
		reasons = append(reasons, "同租户确认/成功历史提供加分")
	}
	work.result = OzonCategoryRecommendationCandidate{
		CategoryID: work.node.CategoryID, CategoryPath: work.node.Path,
		Score: roundRecommendationScore(score), Confidence: roundRecommendationRatio(score / 100),
		Approximate:     work.semantic < 0.75 || variant.Ratio < 1 || required.Ratio < 0.5,
		VariantCoverage: variant, RequiredCoverage: required,
		MatchedDimensions: matched, UnmatchedDimensions: unmatched,
		ListingStrategy: strategy, Reasons: reasons, Warnings: boundedStrings(warnings, 10, 240),
		SchemaHash:       shop.OzonCategoryAttributeSchemaHash(work.attrs),
		TemplateSyncedAt: latestOzonTemplateSync(work.attrs),
	}
}

func matchOzonRecommendationDimensions(
	dimensions []OzonRecommendationDifferenceDimension,
	attrs []shop.OzonAttributeDTO,
) ([]OzonRecommendationMatchedDimension, []OzonRecommendationUnmatchedDimension) {
	matched := make([]OzonRecommendationMatchedDimension, 0, len(dimensions))
	unmatched := make([]OzonRecommendationUnmatchedDimension, 0, len(dimensions))
	used := map[string]bool{}
	for _, dimension := range dimensions {
		semantic := inferOzonDifferenceSemantic(dimension)
		var target *shop.OzonAttributeDTO
		for i := range attrs {
			attr := &attrs[i]
			if used[attr.AttrID] || attr.AttributeComplexID > 0 {
				continue
			}
			if !attr.SKUVariantEligibilityKnown || !attr.SKUVariantEligible {
				continue
			}
			if compatibleOzonDimensionAttribute(semantic, *attr) {
				target = attr
				break
			}
		}
		if target == nil {
			reason := "模板中没有语义相符且 is_aspect=true、资格已知的属性"
			unmatched = append(unmatched, OzonRecommendationUnmatchedDimension{
				SourceDimensionKey: dimension.Key, SourceDimensionName: dimension.Name, Reason: reason,
			})
			continue
		}
		used[target.AttrID] = true
		matched = append(matched, OzonRecommendationMatchedDimension{
			SourceDimensionKey: dimension.Key, SourceDimensionName: dimension.Name,
			TargetAttributeID: target.AttrID, TargetAttributeName: target.Name,
			IsAspect: true, IsAspectKnown: true,
		})
	}
	return matched, unmatched
}

func validatedOzonRecommendationTemplateAttributes(categoryID string, attrs []shop.OzonAttributeDTO) ([]shop.OzonAttributeDTO, bool) {
	categoryID = strings.TrimSpace(categoryID)
	out := make([]shop.OzonAttributeDTO, 0, len(attrs))
	seen := map[string]bool{}
	rejected := false
	for _, attr := range attrs {
		attrID := strings.TrimSpace(attr.AttrID)
		if categoryID == "" || strings.TrimSpace(attr.CategoryID) != categoryID || attrID == "" || seen[attrID] {
			rejected = true
			continue
		}
		attr.AttrID = attrID
		seen[attrID] = true
		out = append(out, attr)
	}
	return out, rejected
}

func inferOzonDifferenceSemantic(dimension OzonRecommendationDifferenceDimension) string {
	declared := strings.ToLower(strings.TrimSpace(dimension.Semantic))
	if declared == "" || declared == "other" {
		return ""
	}
	nameSemantic := classifyOzonRecommendationSemantic(dimension.Name)
	if nameSemantic != "other" && nameSemantic != declared {
		return ""
	}
	for _, evidence := range dimension.Evidence {
		if semantic := classifyOzonRecommendationSemantic(evidence.SourceKey); semantic != "other" && semantic != "color" && semantic != declared {
			return ""
		}
	}
	if !ozonRecommendationEvidenceSupportsSemantic(declared, dimension.Evidence) {
		return ""
	}
	return declared
}

func ozonRecommendationEvidenceSupportsSemantic(semantic string, evidence []OzonRecommendationEvidence) bool {
	for _, item := range evidence {
		value := item.RawValue
		switch semantic {
		case "model":
			if ozonRecommendationModelValuePattern.MatchString(value) {
				return true
			}
		case "current":
			if ozonRecommendationCurrentValuePattern.MatchString(value) {
				return true
			}
		case "control_method":
			if containsAnyNormalized(value, "直流", "交流", "dc", "ac", "control") {
				return true
			}
		case "package":
			if containsAnyNormalized(value, "只装", "件装", "套装", "包装", "pack") {
				return true
			}
		case "color", "size", "material":
			if ozonRecommendationCompositeSelectionValue(value) {
				continue
			}
			if classifyOzonRecommendationSemantic(item.SourceKey) == semantic || classifyOzonRecommendationSemantic(value) == semantic {
				return true
			}
		}
	}
	return false
}

func ozonRecommendationCompositeSelectionValue(value string) bool {
	return ozonRecommendationModelValuePattern.MatchString(value) ||
		ozonRecommendationCurrentValuePattern.MatchString(value) ||
		containsAnyNormalized(value, "直流", "交流", "dc", "ac", "control", "只装", "件装", "套装", "pack")
}

func compatibleOzonDimensionAttribute(semantic string, attr shop.OzonAttributeDTO) bool {
	attrSemantic := classifyOzonRecommendationSemantic(attr.Name + " " + attr.Description)
	if semantic != "" && semantic != "other" && attrSemantic == semantic {
		return true
	}
	return false
}

func classifyOzonRecommendationSemantic(value string) string {
	switch {
	case containsAnyNormalized(value, "型号", "model", "модель", "артикул"):
		return "model"
	case containsAnyNormalized(value, "控制方式", "控制类型", "controlmethod", "直流控", "交流控"):
		return "control_method"
	case containsAnyNormalized(value, "额定电流", "电流", "current", "ампер", "ток"):
		return "current"
	case containsAnyNormalized(value, "包装", "只装", "件数", "数量", "pack", "комплект"):
		return "package"
	case containsAnyNormalized(value, "内径", "innerdiameter", "внутреннийдиаметр"):
		return "inner_diameter"
	case containsAnyNormalized(value, "颜色", "color", "colour", "цвет"):
		return "color"
	case containsAnyNormalized(value, "尺码", "尺寸", "size", "размер"):
		return "size"
	case containsAnyNormalized(value, "材质", "材料", "material", "материал"):
		return "material"
	default:
		return "other"
	}
}

func containsAnyNormalized(value string, candidates ...string) bool {
	normalized := normalizeOzonRecommendationText(value)
	for _, candidate := range candidates {
		if strings.Contains(normalized, normalizeOzonRecommendationText(candidate)) {
			return true
		}
	}
	return false
}

func requiredOzonRecommendationCoverage(snapshot ozonRecommendationSnapshot, attrs []shop.OzonAttributeDTO) OzonRecommendationCoverage {
	sourceKeys := make([]string, 0, len(snapshot.ProductAttributes)+20)
	for key := range snapshot.ProductAttributes {
		sourceKeys = append(sourceKeys, key)
	}
	for _, sku := range snapshot.SKUs {
		for key := range sku.Selections {
			sourceKeys = append(sourceKeys, key)
		}
	}
	total := 0
	matched := 0
	for _, attr := range attrs {
		if !attr.Required {
			continue
		}
		total++
		if ozonRequiredAttributeCanBeFilled(attr, snapshot, sourceKeys) {
			matched++
		}
	}
	return recommendationCoverage(matched, total)
}

func ozonRequiredAttributeCanBeFilled(attr shop.OzonAttributeDTO, snapshot ozonRecommendationSnapshot, sourceKeys []string) bool {
	attrName := normalizeOzonRecommendationText(attr.Name)
	attrSemantic := classifyOzonRecommendationSemantic(attr.Name)
	for _, sourceKey := range sourceKeys {
		sourceName := normalizeOzonRecommendationText(sourceKey)
		if attrName != "" && sourceName != "" && (attrName == sourceName || strings.Contains(sourceName, attrName) || strings.Contains(attrName, sourceName)) {
			return true
		}
		if attrSemantic != "other" && classifyOzonRecommendationSemantic(sourceKey) == attrSemantic {
			return true
		}
	}
	if containsAnyNormalized(attr.Name, "商品名称", "标题", "name", "title") {
		return strings.TrimSpace(snapshot.Title) != ""
	}
	if containsAnyNormalized(attr.Name, "描述", "description") {
		return strings.TrimSpace(snapshot.Description) != ""
	}
	return false
}

func ozonRecommendationListingStrategy(
	skuCount int,
	coverage OzonRecommendationCoverage,
	anomalies []OzonRecommendationAnomaly,
) string {
	if skuCount <= 1 {
		return OzonListingStrategyGroupAll
	}
	if hasDifferentProductAnomaly(anomalies) {
		return OzonListingStrategySplitSingleSKU
	}
	if coverage.Total == 0 || coverage.Matched == coverage.Total {
		return OzonListingStrategyGroupAll
	}
	if coverage.Matched > 0 {
		return OzonListingStrategyGroupSubset
	}
	return OzonListingStrategyManualReview
}

func hasDifferentProductAnomaly(anomalies []OzonRecommendationAnomaly) bool {
	for _, anomaly := range anomalies {
		if anomaly.Type == "different_product_subject" || anomaly.Type == "mixed_product_subject" {
			return true
		}
	}
	return false
}

func recommendationCoverage(matched, total int) OzonRecommendationCoverage {
	ratio := 1.0
	if total > 0 {
		ratio = float64(matched) / float64(total)
	}
	return OzonRecommendationCoverage{Matched: matched, Total: total, Ratio: roundRecommendationRatio(ratio)}
}

func latestOzonTemplateSync(attrs []shop.OzonAttributeDTO) *time.Time {
	var latest *time.Time
	for _, attr := range attrs {
		if attr.SyncedAt == nil {
			continue
		}
		value := attr.SyncedAt.UTC()
		if latest == nil || value.After(*latest) {
			latest = &value
		}
	}
	return latest
}

func ozonRecommendationTemplateStale(attrs []shop.OzonAttributeDTO, now time.Time) bool {
	if len(attrs) == 0 {
		return true
	}
	latest := latestOzonTemplateSync(attrs)
	if latest == nil || now.Sub(*latest) > shop.OzonCategoryCacheTTL {
		return true
	}
	for _, attr := range attrs {
		if attr.CacheStale {
			return true
		}
	}
	return false
}

func sortOzonRecommendationCandidates(items []*ozonRecommendationCandidateWork) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].result.Score != items[j].result.Score {
			return items[i].result.Score > items[j].result.Score
		}
		if items[i].semantic != items[j].semantic {
			return items[i].semantic > items[j].semantic
		}
		return items[i].node.CategoryID < items[j].node.CategoryID
	})
}

func clampRecommendationRatio(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func roundRecommendationRatio(value float64) float64 {
	return math.Round(clampRecommendationRatio(value)*1000) / 1000
}

func roundRecommendationScore(value float64) float64 {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return math.Round(value*10) / 10
}

func recommendationPercent(value float64) string {
	return strconv.Itoa(int(math.Round(clampRecommendationRatio(value)*100))) + "%"
}

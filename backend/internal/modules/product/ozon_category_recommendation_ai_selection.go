package product

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
)

const (
	ozonRecommendationMaxSearchChoices = 30
	ozonRecommendationMaxPathChoices   = 8
	ozonRecommendationMaxRootChoices   = 5
)

type ozonRecommendationRootItem struct {
	Key  string
	Node shop.OzonCategoryNodeDTO
}

type ozonAIRootSelection struct {
	SelectedRootKeys []string `json:"selectedRootKeys"`
}

func (s *Service) ozonRecommendationRoots(ctx context.Context) ([]ozonRecommendationRootItem, error) {
	result, err := s.OzonCategories.ListOzonCategories(ctx, shop.OzonCategoryListQuery{
		RootOnly: true, ActiveOnly: true, AllMatches: true,
	})
	if err != nil || result == nil {
		if err == nil {
			err = fmt.Errorf("empty root category response")
		}
		return nil, err
	}
	roots := make([]ozonRecommendationRootItem, 0, len(result.List))
	for _, node := range result.List {
		if node.Status != "active" || strings.TrimSpace(node.CategoryID) == "" || strings.TrimSpace(node.Name) == "" {
			continue
		}
		roots = append(roots, ozonRecommendationRootItem{Node: node})
	}
	sort.SliceStable(roots, func(i, j int) bool {
		if roots[i].Node.Name != roots[j].Node.Name {
			return roots[i].Node.Name < roots[j].Node.Name
		}
		return roots[i].Node.CategoryID < roots[j].Node.CategoryID
	})
	for index := range roots {
		roots[index].Key = fmt.Sprintf("root_%d", index+1)
	}
	return roots, nil
}

func callOzonRecommendationRootSelection(
	ctx context.Context,
	client AIChatClient,
	snapshot ozonRecommendationSnapshot,
	productType string,
	searchTerms []string,
	roots []ozonRecommendationRootItem,
) (*aigate.ChatResponse, error) {
	type rootSummary struct {
		RootKey string `json:"rootKey"`
		Name    string `json:"name"`
	}
	summaries := make([]rootSummary, 0, len(roots))
	for _, root := range roots {
		summaries = append(summaries, rootSummary{RootKey: root.Key, Name: root.Node.Name})
	}
	payload, _ := json.Marshal(map[string]any{
		"productTitle":  truncateRunes(strings.TrimSpace(snapshot.Title), 320),
		"productType":   strings.TrimSpace(productType),
		"searchTerms":   boundedStrings(searchTerms, 12, 80),
		"maxSelections": ozonRecommendationMaxRootChoices,
		"roots":         summaries,
	})
	callCtx, cancel := context.WithTimeout(ctx, ozonRecommendationAIRequestTimeout)
	defer cancel()
	return client.Chat(callCtx, aigate.ChatRequest{
		Temperature: 0, MaxTokens: 900, ResponseFormat: &aigate.ResponseFormat{Type: "json_object"},
		Messages: []aigate.Message{
			{Role: "system", Content: "你是 Ozon 一级类目领域路由器。roots 是服务端提供的全部真实启用一级类目，只能引用其中的 rootKey，不能输出类目 ID 或新 key。根据 productTitle、productType、searchTerms 选择最多 maxSelections 个能够包含该物理商品主体的领域。不要仅因局部词重合选择强制适用对象不一致的领域；不确定时可保留多个相邻领域，完全没有合理领域时返回空数组。严格输出单个 JSON 对象，不使用 Markdown。"},
			{Role: "user", Content: "输出 {\"selectedRootKeys\":[\"root_1\"]}。输入：" + string(payload)},
		},
	})
}

func parseOzonAIRootSelection(content string, roots []ozonRecommendationRootItem) (*ozonAIRootSelection, error) {
	var out ozonAIRootSelection
	if err := decodeStrictOzonRecommendationJSON(content, &out); err != nil {
		return nil, fmt.Errorf("invalid root selection json: %w", err)
	}
	if len(out.SelectedRootKeys) > ozonRecommendationMaxRootChoices {
		return nil, fmt.Errorf("selected root list exceeds %d items", ozonRecommendationMaxRootChoices)
	}
	allowlist := make(map[string]bool, len(roots))
	for _, root := range roots {
		allowlist[root.Key] = true
	}
	seen := map[string]bool{}
	selected := make([]string, 0, len(out.SelectedRootKeys))
	for _, key := range out.SelectedRootKeys {
		key = strings.TrimSpace(key)
		if !allowlist[key] || seen[key] {
			return nil, fmt.Errorf("rootKey is outside the server allowlist")
		}
		seen[key] = true
		selected = append(selected, key)
	}
	out.SelectedRootKeys = selected
	return &out, nil
}

func filterOzonRecommendationSearchByRoots(
	search *shop.OzonCategorySearchResult,
	roots []ozonRecommendationRootItem,
	selection *ozonAIRootSelection,
) *shop.OzonCategorySearchResult {
	if search == nil || selection == nil {
		return search
	}
	rootByKey := make(map[string]ozonRecommendationRootItem, len(roots))
	for _, root := range roots {
		rootByKey[root.Key] = root
	}
	allowedIDs := map[string]bool{}
	allowedNames := map[string]bool{}
	for _, key := range selection.SelectedRootKeys {
		root := rootByKey[key]
		allowedIDs[strings.TrimSpace(root.Node.CategoryID)] = true
		allowedNames[normalizeOzonRecommendationText(root.Node.Name)] = true
	}
	filtered := *search
	filtered.Matches = make([]shop.OzonCategorySearchMatch, 0, len(search.Matches))
	for _, match := range search.Matches {
		rootID := ""
		rootName := ""
		if len(match.Node.Ancestors) > 0 {
			rootID = strings.TrimSpace(match.Node.Ancestors[0].CategoryID)
			rootName = normalizeOzonRecommendationText(match.Node.Ancestors[0].Name)
		} else if path := strings.Split(match.Node.Path, "/"); len(path) > 0 {
			rootName = normalizeOzonRecommendationText(path[0])
		}
		if allowedIDs[rootID] || allowedNames[rootName] {
			filtered.Matches = append(filtered.Matches, match)
		}
	}
	return &filtered
}

func selectedOzonRecommendationRoots(
	roots []ozonRecommendationRootItem,
	selection *ozonAIRootSelection,
) ([]string, []string) {
	if selection == nil {
		return nil, nil
	}
	byKey := make(map[string]ozonRecommendationRootItem, len(roots))
	for _, root := range roots {
		byKey[root.Key] = root
	}
	ids := make([]string, 0, len(selection.SelectedRootKeys))
	names := make([]string, 0, len(selection.SelectedRootKeys))
	for _, key := range selection.SelectedRootKeys {
		root, ok := byKey[key]
		if !ok {
			continue
		}
		if value := strings.TrimSpace(root.Node.CategoryID); value != "" {
			ids = append(ids, value)
		}
		if value := strings.TrimSpace(root.Node.Name); value != "" {
			names = append(names, value)
		}
	}
	return ids, names
}

type ozonRecommendationPathSummary struct {
	CandidateKey string   `json:"candidateKey"`
	Path         string   `json:"path"`
	LeafName     string   `json:"leafName"`
	RecallScore  float64  `json:"recallScore"`
	MatchedTerms []string `json:"matchedTerms,omitempty"`
	RecallLanes  []string `json:"recallLanes,omitempty"`
}

func ozonRecommendationWorksFromSearch(
	search *shop.OzonCategorySearchResult,
	history map[string]float64,
) []*ozonRecommendationCandidateWork {
	if search == nil {
		return nil
	}
	works := make([]*ozonRecommendationCandidateWork, 0, len(search.Matches))
	for _, match := range search.Matches {
		node := match.Node
		if !node.IsLeaf || node.Status != "active" || strings.TrimSpace(node.CategoryID) == "" {
			continue
		}
		if strings.TrimSpace(node.Path) == "" {
			node.Path = strings.TrimSpace(node.Name)
		}
		works = append(works, &ozonRecommendationCandidateWork{
			node: node, semantic: match.Score, searchScore: match.Score,
			searchTerms: append([]string{}, match.MatchedTerms...),
			searchLanes: append([]string{}, match.Lanes...), history: history[node.CategoryID],
		})
	}
	sort.SliceStable(works, func(i, j int) bool {
		left := works[i].searchScore*.9 + works[i].history*.1
		right := works[j].searchScore*.9 + works[j].history*.1
		if left != right {
			return left > right
		}
		return works[i].node.CategoryID < works[j].node.CategoryID
	})
	if len(works) > ozonRecommendationMaxSearchChoices {
		works = works[:ozonRecommendationMaxSearchChoices]
	}
	for index, work := range works {
		work.key = fmt.Sprintf("candidate_%d", index+1)
	}
	return works
}

func callOzonRecommendationPathSelection(
	ctx context.Context,
	client AIChatClient,
	snapshot ozonRecommendationSnapshot,
	productType string,
	searchTerms []string,
	works []*ozonRecommendationCandidateWork,
) (*aigate.ChatResponse, error) {
	summaries := make([]ozonRecommendationPathSummary, 0, len(works))
	for _, work := range works {
		summaries = append(summaries, ozonRecommendationPathSummary{
			CandidateKey: work.key, Path: work.node.Path, LeafName: work.node.Name,
			RecallScore:  roundRecommendationRatio(work.searchScore),
			MatchedTerms: boundedStrings(work.searchTerms, 8, 80),
			RecallLanes:  boundedStrings(work.searchLanes, 4, 40),
		})
	}
	payload, _ := json.Marshal(map[string]any{
		"productTitle":  truncateRunes(strings.TrimSpace(snapshot.Title), 320),
		"productType":   strings.TrimSpace(productType),
		"searchTerms":   boundedStrings(searchTerms, 12, 80),
		"maxSelections": ozonRecommendationMaxPathChoices,
		"candidates":    summaries,
	})
	callCtx, cancel := context.WithTimeout(ctx, ozonRecommendationAIRequestTimeout)
	defer cancel()
	return client.Chat(callCtx, aigate.ChatRequest{
		Temperature: 0, MaxTokens: 3200, ResponseFormat: &aigate.ResponseFormat{Type: "json_object"},
		Messages: []aigate.Message{
			{Role: "system", Content: "你是 Ozon 类目路径筛选器。服务端已经从完整真实叶子类目索引召回候选，你只能引用输入中的 candidateKey，不能输出或猜测任何类目 ID、属性 ID 或新 key。必须比较完整路径每一层与商品主体，不能仅凭叶子名的局部词重合选择。只有物理商品主体不同，或路径强制限定了与商品证据互斥的适用对象时，才属于硬冲突；如果叶子仍是同一核心商品类型，只是上位路径的营销渠道、使用场景或细分类别相邻，应保留为 approximate 供人工审核，不得仅因场景不完全一致淘汰。subjectMatch 只能是 exact、approximate 或 conflict。只输出要保留的最多 maxSelections 个 exact/approximate 候选，不要回传淘汰项；每个输出项的 confidence 必须大于 0 且不超过 1。没有精准类目时应保留最合理且无硬冲突的近似类目，全部物理主体都不同时才返回空数组。reasons 和 warnings 每项最多两条短句。严格输出单个 JSON 对象，不使用 Markdown。"},
			{Role: "user", Content: "输出 {\"selected\":[{\"candidateKey\":\"candidate_1\",\"subjectMatch\":\"exact\",\"confidence\":0.85,\"reasons\":[\"\"],\"warnings\":[\"\"]}]}。输入：" + string(payload)},
		},
	})
}

func parseOzonAIPathSelection(content string, allowlist map[string]bool) (*ozonAIPathSelection, error) {
	var out ozonAIPathSelection
	if err := decodeStrictOzonRecommendationJSON(content, &out); err != nil {
		return nil, fmt.Errorf("invalid path selection json: %w", err)
	}
	if len(out.Selected) > len(allowlist) {
		return nil, fmt.Errorf("selected path list exceeds the candidate allowlist")
	}
	seen := map[string]bool{}
	selected := make([]ozonAIPathSelectionItem, 0, len(out.Selected))
	for _, item := range out.Selected {
		item.CandidateKey = strings.TrimSpace(item.CandidateKey)
		item.SubjectMatch = strings.TrimSpace(strings.ToLower(item.SubjectMatch))
		if !allowlist[item.CandidateKey] || seen[item.CandidateKey] {
			return nil, fmt.Errorf("candidateKey is outside the server allowlist")
		}
		if item.SubjectMatch != "exact" && item.SubjectMatch != "approximate" && item.SubjectMatch != "conflict" {
			return nil, fmt.Errorf("subjectMatch must be exact, approximate or conflict")
		}
		if item.Confidence < 0 || item.Confidence > 1 {
			return nil, fmt.Errorf("path confidence must be between 0 and 1")
		}
		seen[item.CandidateKey] = true
		item.Reasons = boundedStrings(item.Reasons, 5, 240)
		item.Warnings = boundedStrings(item.Warnings, 5, 240)
		if item.SubjectMatch == "conflict" || item.Confidence == 0 {
			continue
		}
		selected = append(selected, item)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].Confidence != selected[j].Confidence {
			return selected[i].Confidence > selected[j].Confidence
		}
		return selected[i].CandidateKey < selected[j].CandidateKey
	})
	if len(selected) > ozonRecommendationMaxPathChoices {
		selected = selected[:ozonRecommendationMaxPathChoices]
	}
	out.Selected = selected
	return &out, nil
}

func selectOzonRecommendationPaths(
	works []*ozonRecommendationCandidateWork,
	selection *ozonAIPathSelection,
) []*ozonRecommendationCandidateWork {
	if selection == nil {
		return nil
	}
	byKey := make(map[string]*ozonRecommendationCandidateWork, len(works))
	for _, work := range works {
		byKey[work.key] = work
	}
	selected := make([]*ozonRecommendationCandidateWork, 0, len(selection.Selected))
	for _, item := range selection.Selected {
		work := byKey[item.CandidateKey]
		work.semantic = clampRecommendationRatio(work.searchScore*.35 + item.Confidence*.65)
		work.pathConfidence = item.Confidence
		work.pathApproximate = item.SubjectMatch == "approximate"
		work.pathReasons = boundedStrings(item.Reasons, 10, 240)
		work.pathWarnings = boundedStrings(item.Warnings, 10, 240)
		work.aiConfidence = item.Confidence
		work.aiApproximate = work.pathApproximate
		work.aiReasons = append([]string{}, work.pathReasons...)
		work.aiWarnings = append([]string{}, work.pathWarnings...)
		selected = append(selected, work)
	}
	return selected
}

func (s *Service) loadOzonRecommendationTemplateSummaries(
	ctx context.Context,
	works []*ozonRecommendationCandidateWork,
) ([]string, bool) {
	warnings := []string{}
	partial := false
	now := time.Now().UTC()
	for _, work := range works {
		attrs, err := s.OzonCategories.ListOzonCategoryAttributes(ctx, work.node.CategoryID)
		if err != nil {
			partial = true
			warnings = append(warnings, "候选路径的本地模板摘要读取失败，复核将按模板未知处理")
			continue
		}
		var rejected bool
		work.attrs, rejected = validatedOzonRecommendationTemplateAttributes(work.node.CategoryID, attrs)
		if rejected {
			partial = true
			warnings = append(warnings, "候选模板包含不属于该类目的属性，复核摘要已排除非法属性")
		}
		work.templateStale = ozonRecommendationTemplateStale(work.attrs, now)
	}
	return boundedStrings(warnings, 8, 240), partial
}

type ozonRecommendationTemplateSelectionSummary struct {
	CandidateKey            string   `json:"candidateKey"`
	Path                    string   `json:"path"`
	TemplateAvailable       bool     `json:"templateAvailable"`
	TemplateStale           bool     `json:"templateStale"`
	AttributeCount          int      `json:"attributeCount"`
	RequiredAttributes      []string `json:"requiredAttributes"`
	VariantAttributes       []string `json:"knownIsAspectAttributes"`
	NonVariantAttributes    []string `json:"knownNonAspectAttributes,omitempty"`
	VariantEligibilityKnown bool     `json:"variantEligibilityFullyKnown"`
	SchemaHash              string   `json:"schemaHash,omitempty"`
	HistoricalConfirmation  bool     `json:"historicalConfirmation"`
}

func callOzonRecommendationFinalReview(
	ctx context.Context,
	client AIChatClient,
	snapshot ozonRecommendationSnapshot,
	works []*ozonRecommendationCandidateWork,
	productType string,
	searchTerms []string,
	dimensions []OzonRecommendationDifferenceDimension,
	anomalies []OzonRecommendationAnomaly,
) (*aigate.ChatResponse, error) {
	type dimensionSummary struct {
		Name     string `json:"name"`
		Semantic string `json:"semantic"`
	}
	type anomalySummary struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	dimensionSummaries := make([]dimensionSummary, 0, len(dimensions))
	for _, dimension := range dimensions {
		dimensionSummaries = append(dimensionSummaries, dimensionSummary{Name: dimension.Name, Semantic: dimension.Semantic})
	}
	anomalySummaries := make([]anomalySummary, 0, len(anomalies))
	for _, anomaly := range anomalies {
		anomalySummaries = append(anomalySummaries, anomalySummary{Type: anomaly.Type, Message: anomaly.Message})
	}
	summaries := make([]ozonRecommendationTemplateSelectionSummary, 0, len(works))
	for _, work := range works {
		required := make([]string, 0)
		variants := make([]string, 0)
		nonVariants := make([]string, 0)
		fullyKnown := true
		for _, attr := range work.attrs {
			if attr.Required {
				required = append(required, attr.Name)
			}
			if !attr.SKUVariantEligibilityKnown {
				fullyKnown = false
				continue
			}
			if attr.SKUVariantEligible {
				variants = append(variants, attr.Name)
			} else {
				nonVariants = append(nonVariants, attr.Name)
			}
		}
		summaries = append(summaries, ozonRecommendationTemplateSelectionSummary{
			CandidateKey: work.key, Path: work.node.Path,
			TemplateAvailable: len(work.attrs) > 0, TemplateStale: work.templateStale,
			AttributeCount:          len(work.attrs),
			RequiredAttributes:      boundedStrings(required, 24, 100),
			VariantAttributes:       boundedStrings(variants, 24, 100),
			NonVariantAttributes:    boundedStrings(nonVariants, 24, 100),
			VariantEligibilityKnown: fullyKnown,
			SchemaHash:              shop.OzonCategoryAttributeSchemaHash(work.attrs),
			HistoricalConfirmation:  work.history > 0,
		})
	}
	payload, _ := json.Marshal(map[string]any{
		"productTitle":         truncateRunes(strings.TrimSpace(snapshot.Title), 320),
		"productType":          strings.TrimSpace(productType),
		"searchTerms":          boundedStrings(searchTerms, 12, 80),
		"differenceDimensions": dimensionSummaries,
		"anomalies":            anomalySummaries,
		"candidates":           summaries,
	})
	callCtx, cancel := context.WithTimeout(ctx, ozonRecommendationAIRequestTimeout)
	defer cancel()
	return client.Chat(callCtx, aigate.ChatRequest{
		Temperature: 0, MaxTokens: 2600, ResponseFormat: &aigate.ResponseFormat{Type: "json_object"},
		Messages: []aigate.Message{
			{Role: "system", Content: "你是 Ozon 类目最终复核器。必须只引用输入中的 candidateKey，不能输出或猜测类目 ID、属性 ID或新 key。subjectMatch 只能是 exact、approximate、conflict：exact 表示完整路径和叶子准确描述商品主体；approximate 表示叶子仍是同一核心商品类型，但上位路径的营销渠道、用途场景或细分类别相邻；conflict 只用于物理商品主体不同，或路径强制限定了与商品证据互斥的适用对象。exact/approximate 的 confidence 必须大于 0 且不超过 1；conflict 的 confidence 必须为 0。判断必须通用于任何商品，不依赖固定行业词表；没有精准类目时，不能仅因场景不完全一致而淘汰合理近似项。完整路径优先于局部词重合。模板摘要只用于判断刊登兼容性，模板不能承载全部 SKU 区别时应给出 warning 和人工审核建议，不能据此把同一商品主体改判为 conflict；不能把型号、电流等 SKU 区别解释为颜色、内径或其他无关属性，只有 knownIsAspectAttributes 可承载 SKU 变体。templateAvailable=false 或 templateStale=true 需要警告，但不能猜测模板内容。遗漏候选等同于淘汰。严格输出单个 JSON 对象，不使用 Markdown。"},
			{Role: "user", Content: "输出 {\"verdicts\":[{\"candidateKey\":\"candidate_1\",\"subjectMatch\":\"exact\",\"confidence\":0.85,\"reasons\":[\"\"],\"warnings\":[\"\"]}]}。输入：" + string(payload)},
		},
	})
}

func parseOzonAIFinalReview(content string, allowlist map[string]bool) (*ozonAIFinalReview, error) {
	var out ozonAIFinalReview
	if err := decodeStrictOzonRecommendationJSON(content, &out); err != nil {
		return nil, fmt.Errorf("invalid final review json: %w", err)
	}
	if len(out.Verdicts) > len(allowlist) {
		return nil, fmt.Errorf("final review exceeds the candidate allowlist")
	}
	seen := map[string]bool{}
	verdicts := make([]ozonAIFinalReviewItem, 0, len(out.Verdicts))
	for index := range out.Verdicts {
		item := &out.Verdicts[index]
		item.CandidateKey = strings.TrimSpace(item.CandidateKey)
		item.SubjectMatch = strings.TrimSpace(strings.ToLower(item.SubjectMatch))
		if !allowlist[item.CandidateKey] || seen[item.CandidateKey] {
			return nil, fmt.Errorf("candidateKey is outside the server allowlist")
		}
		if item.SubjectMatch != "exact" && item.SubjectMatch != "approximate" && item.SubjectMatch != "conflict" {
			return nil, fmt.Errorf("subjectMatch must be exact, approximate or conflict")
		}
		if item.Confidence < 0 || item.Confidence > 1 {
			return nil, fmt.Errorf("final confidence must be between 0 and 1")
		}
		seen[item.CandidateKey] = true
		item.Reasons = boundedStrings(item.Reasons, 6, 240)
		item.Warnings = boundedStrings(item.Warnings, 6, 240)
		if item.SubjectMatch != "conflict" && item.Confidence == 0 {
			// A zero-confidence non-conflict verdict is internally contradictory.
			// Reject only that allowlisted verdict so one malformed score cannot
			// erase independent valid candidates from the same strict JSON reply.
			continue
		}
		verdicts = append(verdicts, *item)
	}
	out.Verdicts = verdicts
	return &out, nil
}

func applyOzonRecommendationFinalReview(
	works []*ozonRecommendationCandidateWork,
	review *ozonAIFinalReview,
) []*ozonRecommendationCandidateWork {
	if review == nil {
		return nil
	}
	byKey := make(map[string]*ozonRecommendationCandidateWork, len(works))
	for _, work := range works {
		byKey[work.key] = work
	}
	selected := make([]*ozonRecommendationCandidateWork, 0, len(works))
	for _, verdict := range review.Verdicts {
		if verdict.SubjectMatch == "conflict" || verdict.Confidence == 0 {
			continue
		}
		work := byKey[verdict.CandidateKey]
		work.semantic = clampRecommendationRatio(work.searchScore*.2 + verdict.Confidence*.8)
		work.aiConfidence = verdict.Confidence
		work.aiApproximate = verdict.SubjectMatch == "approximate"
		work.aiReasons = boundedStrings(append(work.aiReasons, verdict.Reasons...), 10, 240)
		work.aiWarnings = boundedStrings(append(work.aiWarnings, verdict.Warnings...), 10, 240)
		selected = append(selected, work)
	}
	if len(selected) == 0 {
		// Root routing and path screening are independent, whitelist-constrained
		// semantic checks. A template review is allowed to explain compatibility,
		// but should not erase their only shared near-match merely because the
		// template cannot carry every SKU distinction. Keep at most one strongly
		// supported path as an explicitly low-confidence manual-review candidate.
		fallbacks := append([]*ozonRecommendationCandidateWork{}, works...)
		sort.SliceStable(fallbacks, func(i, j int) bool {
			left := fallbacks[i].pathConfidence*.65 + fallbacks[i].searchScore*.35
			right := fallbacks[j].pathConfidence*.65 + fallbacks[j].searchScore*.35
			if left != right {
				return left > right
			}
			return fallbacks[i].node.CategoryID < fallbacks[j].node.CategoryID
		})
		for _, fallback := range fallbacks {
			if fallback.pathConfidence <= 0 || (fallback.pathConfidence < 0.5 && fallback.searchScore < ozonRecommendationMinSemanticScore) {
				continue
			}
			confidence := fallback.pathConfidence
			if confidence > 0.45 {
				confidence = 0.45
			}
			fallback.semantic = clampRecommendationRatio(fallback.searchScore*.35 + confidence*.65)
			fallback.aiConfidence = confidence
			fallback.aiApproximate = true
			fallback.aiReasons = boundedStrings(append(fallback.aiReasons,
				"一级领域与完整路径筛选均保留该候选，按近似类目供人工复核",
			), 10, 240)
			fallback.aiWarnings = boundedStrings(append(fallback.aiWarnings,
				"最新模板终审与路径筛选结论不一致；已降低置信度，必须人工确认",
			), 10, 240)
			selected = append(selected, fallback)
			break
		}
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].semantic != selected[j].semantic {
			return selected[i].semantic > selected[j].semantic
		}
		return selected[i].node.CategoryID < selected[j].node.CategoryID
	})
	if len(selected) > 5 {
		selected = selected[:5]
	}
	return selected
}

func resetOzonRecommendationAISelectionMetadata(works []*ozonRecommendationCandidateWork) {
	for _, work := range works {
		work.semantic = clampRecommendationRatio(work.searchScore*.35 + work.pathConfidence*.65)
		work.aiConfidence = work.pathConfidence
		work.aiApproximate = work.pathApproximate
		work.aiReasons = append([]string{}, work.pathReasons...)
		work.aiWarnings = append([]string{}, work.pathWarnings...)
	}
}

func applyOzonRecommendationAISelection(work *ozonRecommendationCandidateWork) {
	if work == nil {
		return
	}
	work.result.Confidence = roundRecommendationRatio(work.aiConfidence)
	work.result.Approximate = work.result.Approximate || work.aiApproximate
	work.result.Reasons = boundedStrings(append(work.result.Reasons, work.aiReasons...), 10, 240)
	work.result.Warnings = boundedStrings(append(work.result.Warnings, work.aiWarnings...), 10, 240)
}

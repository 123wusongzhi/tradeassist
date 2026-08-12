package product

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	platformozon "github.com/trademind-ai/trademind/backend/internal/providers/platform/ozon"
)

type ozonAttributePipelineResult struct {
	Suggestions     []OzonAttributeSuggestion
	Skipped         []OzonAttributeSuggestionSkipped
	Warnings        []string
	InputTokens     int
	OutputTokens    int
	Models          []string
	BatchCount      int
	RepairCount     int
	FactRepairCount int
	FactCount       int
}

type ozonAttributeFactAICandidate struct {
	Name       string   `json:"name"`
	Value      string   `json:"value"`
	Evidence   string   `json:"evidence"`
	SourceRefs []string `json:"sourceRefs"`
}

type ozonAttributeFactAIOutput struct {
	Facts []ozonAttributeFactAICandidate `json:"facts"`
}

type ozonAttributeBatchWork struct {
	Candidates []ozonAttributeSuggestionCandidate
	Prompt     []ozonAttributePromptCandidate
	Feedback   map[string]string
}

type ozonAttributeBatchResult struct {
	Work         ozonAttributeBatchWork
	Output       ozonAttributeAIOutput
	Response     *aigate.ChatResponse
	TransportErr error
	ParseErr     error
}

type ozonAttributeCandidateValidation struct {
	Suggestion *OzonAttributeSuggestion
	Kind       string
	Reason     string
}

type ozonAttributeDictionarySearcher interface {
	SearchOzonDictionaryValues(context.Context, int64, string, string, uuid.UUID, string) ([]platformozon.DictionaryValue, error)
}

func (s *Service) runOzonAttributeSuggestionPipeline(
	ctx context.Context,
	client AIChatClient,
	factPrompt aiprompt.AIPrompt,
	suggestionPrompt aiprompt.AIPrompt,
	promptContext ozonAttributePromptContext,
	imageURLs []string,
	candidates []ozonAttributeSuggestionCandidate,
	promptCandidates []ozonAttributePromptCandidate,
	tenantID int64,
	shopID uuid.UUID,
	categoryID string,
) (*ozonAttributePipelineResult, error) {
	result := &ozonAttributePipelineResult{
		Suggestions: []OzonAttributeSuggestion{}, Skipped: []OzonAttributeSuggestionSkipped{}, Warnings: []string{},
	}
	facts, factResponses, factRepairCount, err := callOzonAttributeFactExtraction(ctx, client, factPrompt, promptContext, imageURLs, promptCandidates)
	for _, response := range factResponses {
		result.addResponse(response)
	}
	result.FactRepairCount = factRepairCount
	if err != nil {
		return result, fmt.Errorf("fact extraction failed: %w", err)
	}
	promptContext.Facts = facts
	result.FactCount = len(facts)

	promptByKey := make(map[string]ozonAttributePromptCandidate, len(promptCandidates))
	for _, candidate := range promptCandidates {
		promptByKey[candidate.AttributeKey] = candidate
	}
	initialWorks := buildOzonAttributeBatchWorks(candidates, promptByKey, ozonAttributeSuggestionMaxBatchSize, nil)
	initialResults := callOzonAttributeBatches(ctx, client, suggestionPrompt, promptContext, initialWorks)
	result.BatchCount = len(initialResults)
	outputsByKey := map[string][]ozonAttributeAICandidate{}
	failedKeys := map[string]bool{}
	formatFailedKeys := map[string]bool{}
	providerSuccesses := 0
	parsedSuccesses := 0
	var firstTransportErr error
	for _, batch := range initialResults {
		result.addResponse(batch.Response)
		if batch.TransportErr != nil {
			if firstTransportErr == nil {
				firstTransportErr = batch.TransportErr
			}
			for _, candidate := range batch.Work.Candidates {
				failedKeys[candidate.key] = true
			}
			continue
		}
		providerSuccesses++
		if batch.ParseErr != nil {
			result.Warnings = append(result.Warnings, "一个 AI 属性批次返回了无效 JSON，相关字段已进入补全阶段")
			for _, candidate := range batch.Work.Candidates {
				formatFailedKeys[candidate.key] = true
			}
			continue
		}
		parsedSuccesses++
		mergeOzonAttributeBatchOutput(outputsByKey, batch.Work.Candidates, batch.Output, &result.Warnings)
	}
	if providerSuccesses == 0 {
		if firstTransportErr != nil {
			return result, fmt.Errorf("all attribute batches failed: %w", firstTransportErr)
		}
		return result, fmt.Errorf("all attribute batches failed")
	}

	result.Warnings = append(result.Warnings, s.resolveOzonAttributeDictionaryFallbacks(
		ctx, tenantID, shopID, categoryID, candidates, outputsByKey,
	)...)
	validations := validateOzonAttributeCandidateOutputs(candidates, outputsByKey, promptContext)
	repairCandidates := make([]ozonAttributeSuggestionCandidate, 0)
	repairFeedback := map[string]string{}
	for _, candidate := range candidates {
		if failedKeys[candidate.key] {
			continue
		}
		validation := validations[candidate.key]
		if validation.Suggestion == nil {
			repairCandidates = append(repairCandidates, candidate)
			if formatFailedKeys[candidate.key] {
				repairFeedback[candidate.key] = "上一次整个批次不是合法的严格 JSON；values 的每个元素必须是 JSON 字符串，包括 Integer、Decimal 和 Boolean"
			} else {
				repairFeedback[candidate.key] = validation.Reason
			}
		}
	}
	if len(repairCandidates) > 0 {
		repairWorks := buildOzonAttributeBatchWorks(repairCandidates, promptByKey, ozonAttributeSuggestionMaxRepairBatch, repairFeedback)
		repairResults := callOzonAttributeBatches(ctx, client, suggestionPrompt, promptContext, repairWorks)
		result.RepairCount = len(repairResults)
		for _, batch := range repairResults {
			result.addResponse(batch.Response)
			if batch.TransportErr != nil {
				result.Warnings = append(result.Warnings, "一个补全批次调用失败，已保留首次校验结论且未自动重试")
				continue
			}
			if batch.ParseErr != nil {
				continue
			}
			parsedSuccesses++
			replaceOzonAttributeBatchOutput(outputsByKey, batch.Work.Candidates, batch.Output, &result.Warnings)
		}
		result.Warnings = append(result.Warnings, s.resolveOzonAttributeDictionaryFallbacks(
			ctx, tenantID, shopID, categoryID, candidates, outputsByKey,
		)...)
		validations = validateOzonAttributeCandidateOutputs(candidates, outputsByKey, promptContext)
	}
	if parsedSuccesses == 0 {
		return result, fmt.Errorf("all attribute batches returned invalid output")
	}

	for _, candidate := range candidates {
		name := strings.TrimSpace(candidate.attr.Name)
		if name == "" {
			name = candidate.attr.AttrID
		}
		if failedKeys[candidate.key] {
			result.Skipped = append(result.Skipped, OzonAttributeSuggestionSkipped{
				AttributeID: candidate.attr.AttrID, AttributeName: name, Kind: ozonAttributeSkipBatch,
				Reason: "该 AI 并发批次调用失败，未自动重试，已留空",
			})
			continue
		}
		validation := validations[candidate.key]
		if validation.Suggestion != nil {
			result.Suggestions = append(result.Suggestions, *validation.Suggestion)
			continue
		}
		kind := validation.Kind
		if kind == "" {
			kind = ozonAttributeSkipValidation
		}
		result.Skipped = append(result.Skipped, OzonAttributeSuggestionSkipped{
			AttributeID: candidate.attr.AttrID, AttributeName: name, Kind: kind,
			Reason: firstNonEmptyProduct(validation.Reason, "AI 建议未通过校验，已留空"),
		})
	}
	result.Warnings = boundedStrings(result.Warnings, 10, 240)
	result.Models = boundedStrings(result.Models, 8, 128)
	return result, nil
}

func (r *ozonAttributePipelineResult) addResponse(resp *aigate.ChatResponse) {
	if r == nil || resp == nil {
		return
	}
	r.InputTokens += resp.InputTokens
	r.OutputTokens += resp.OutputTokens
	if model := strings.TrimSpace(resp.Model); model != "" {
		for _, existing := range r.Models {
			if existing == model {
				return
			}
		}
		r.Models = append(r.Models, model)
	}
}

func callOzonAttributeFactExtraction(
	ctx context.Context,
	client AIChatClient,
	promptRow aiprompt.AIPrompt,
	promptContext ozonAttributePromptContext,
	imageURLs []string,
	attributes []ozonAttributePromptCandidate,
) ([]ozonAttributePromptFact, []*aigate.ChatResponse, int, error) {
	wishlist := make([]ozonAttributePromptCandidate, 0, len(attributes))
	for _, attribute := range attributes {
		attribute.DictionaryOptions = nil
		attribute.DictionaryOptionsTruncated = false
		attribute.SemanticHint = ""
		wishlist = append(wishlist, attribute)
	}
	contextJSON, _ := json.Marshal(promptContext)
	attributesJSON, _ := json.Marshal(wishlist)
	sourceRefsJSON, _ := json.Marshal(promptContext.AllowedSourceRefs)
	vars := map[string]string{
		"context": string(contextJSON), "evidence": string(contextJSON),
		"attributes": string(attributesJSON), "sourceRefs": string(sourceRefsJSON),
	}
	maxTokens := promptRow.MaxTokens
	if maxTokens < 800 {
		maxTokens = 800
	}
	if maxTokens > 1400 {
		maxTokens = 1400
	}
	req := aigate.ChatRequest{
		Model: strings.TrimSpace(promptRow.Model),
		Messages: []aigate.Message{
			{Role: "system", Content: aiprompt.ReplaceVariables(promptRow.SystemPrompt, vars) + "\n\n" + aiprompt.OzonAttributeFactsRuntimePolicy()},
			{Role: "user", Content: aiprompt.ReplaceVariables(promptRow.UserPrompt, vars), ImageURLs: imageURLs},
		},
		Temperature: promptRow.Temperature, MaxTokens: maxTokens,
		ResponseFormat: &aigate.ResponseFormat{Type: "json_object"},
	}
	resp, err := client.Chat(ctx, req)
	if err != nil || resp == nil {
		return nil, compactOzonAttributeResponses(resp), 0, firstNonNilOzonAttributeError(err, fmt.Errorf("empty fact response"))
	}
	responses := []*aigate.ChatResponse{resp}
	var output ozonAttributeFactAIOutput
	if err := decodeStrictOzonRecommendationJSON(resp.Content, &output); err != nil {
		repairResp, repairErr := repairOzonAttributeFactJSON(ctx, client, req.Model, maxTokens, resp.Content)
		if repairResp != nil {
			responses = append(responses, repairResp)
		}
		if repairErr != nil || repairResp == nil {
			return nil, responses, 1, fmt.Errorf("fact json repair request failed: %w", firstNonNilOzonAttributeError(repairErr, fmt.Errorf("empty fact repair response")))
		}
		if repairErr := decodeStrictOzonRecommendationJSON(repairResp.Content, &output); repairErr != nil {
			return nil, responses, 1, fmt.Errorf("fact repair invalid json: %w", repairErr)
		}
	}
	facts, err := validateOzonAttributeFacts(output, promptContext)
	if err != nil {
		return nil, responses, len(responses) - 1, err
	}
	return facts, responses, len(responses) - 1, nil
}

func repairOzonAttributeFactJSON(
	ctx context.Context,
	client AIChatClient,
	model string,
	maxTokens int,
	invalidContent string,
) (*aigate.ChatResponse, error) {
	return client.Chat(ctx, aigate.ChatRequest{
		Model: strings.TrimSpace(model),
		Messages: []aigate.Message{
			{Role: "system", Content: `你是商品事实 JSON 契约修复器。只修复用户提供内容的 JSON 语法和字段结构，不得增加、删除、改写或猜测商品事实。只输出一个 JSON 对象，顶层只能有 facts；每项只能有 name、value、evidence、sourceRefs。格式示例：{"facts":[{"name":"名称","value":"值","evidence":"依据","sourceRefs":["product.title"]}]}`},
			{Role: "user", Content: "请将以下模型输出修复为规定 JSON；不要输出 Markdown 或说明。\n\n" + invalidContent},
		},
		Temperature: 0, MaxTokens: maxTokens,
		ResponseFormat: &aigate.ResponseFormat{Type: "json_object"},
	})
}

func compactOzonAttributeResponses(responses ...*aigate.ChatResponse) []*aigate.ChatResponse {
	out := make([]*aigate.ChatResponse, 0, len(responses))
	for _, response := range responses {
		if response != nil {
			out = append(out, response)
		}
	}
	return out
}

func validateOzonAttributeFacts(output ozonAttributeFactAIOutput, promptContext ozonAttributePromptContext) ([]ozonAttributePromptFact, error) {
	if len(output.Facts) > ozonAttributeSuggestionMaxAttributes*2 {
		return nil, fmt.Errorf("too many product facts")
	}
	allowed := make(map[string]bool, len(promptContext.AllowedSourceRefs))
	for _, ref := range promptContext.AllowedSourceRefs {
		allowed[ref] = true
	}
	out := make([]ozonAttributePromptFact, 0, len(output.Facts))
	seen := map[string]bool{}
	for _, raw := range output.Facts {
		name := truncateRunes(sanitizeOzonAttributeSuggestionEvidenceValue(raw.Name), 120)
		value := truncateRunes(sanitizeOzonAttributeSuggestionEvidenceValue(raw.Value), 240)
		evidence := truncateRunes(sanitizeOzonAttributeSuggestionEvidenceValue(raw.Evidence), 240)
		refs := boundedStrings(raw.SourceRefs, 4, 80)
		if name == "" || value == "" || evidence == "" || len(refs) == 0 {
			continue
		}
		valid := true
		matchedEvidence := false
		hasImageRef := false
		hasTextRef := false
		for _, ref := range refs {
			if !allowed[ref] || ref == "common_knowledge" || strings.HasPrefix(ref, "category.") {
				valid = false
				break
			}
			if strings.HasPrefix(ref, "image.") {
				hasImageRef = true
				matchedEvidence = true
				continue
			}
			hasTextRef = true
			sourceText := ozonAttributeSourceRefText(promptContext, ref)
			normalizedEvidence := normalizeOzonRecommendationText(evidence)
			if normalizedEvidence != "" && strings.Contains(normalizeOzonRecommendationText(sourceText), normalizedEvidence) {
				matchedEvidence = true
			}
		}
		if !valid || !matchedEvidence {
			continue
		}
		if hasImageRef && !hasTextRef && !isOzonAttributeObservableVisualFact(name, value, evidence) {
			continue
		}
		if ozonAttributeSingleSKUVariantMention(value, promptContext.SKUVariations) != "" {
			continue
		}
		key := normalizeOzonRecommendationText(name + "\n" + value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ozonAttributePromptFact{
			FactKey: fmt.Sprintf("fact_%d", len(out)+1), Name: name, Value: value,
			Evidence: evidence, SourceRefs: refs,
		})
	}
	return out, nil
}

func isOzonAttributeObservableVisualFact(name, value, evidence string) bool {
	nameText := normalizeOzonRecommendationText(name)
	valueText := normalizeOzonRecommendationText(value)
	evidenceText := normalizeOzonRecommendationText(evidence)
	if valueText == "" || evidenceText == "" || !strings.Contains(evidenceText, valueText) {
		return false
	}
	if containsAnyOzonAttributePolicy(
		nameText,
		"品牌", "商标", "logo", "brand", "бренд", "логотип",
		"颜色", "色彩", "color", "colour", "цвет",
		"外观", "形态", "结构", "机身类型", "design", "appearance", "formfactor", "shape", "конструкц", "формфактор",
		"图案", "pattern", "узор",
	) {
		return true
	}
	return containsAnyOzonAttributePolicy(
		valueText,
		"翻盖", "折叠式", "直板", "滑盖", "flip", "clamshell", "monoblock", "slider", "расклад", "моноблок", "слайдер",
	)
}

func ozonAttributeSourceRefText(promptContext ozonAttributePromptContext, ref string) string {
	switch ref {
	case "product.title":
		return promptContext.ProductTitle
	case "product.description":
		return promptContext.ProductDescription
	case "product.attributes":
		raw, _ := json.Marshal(promptContext.ProductAttributes)
		return string(raw)
	}
	for _, sku := range promptContext.RepresentativeSKUs {
		if sku.SourceRef == ref {
			raw, _ := json.Marshal(sku)
			return string(raw)
		}
	}
	return ""
}

func buildOzonAttributeBatchWorks(
	candidates []ozonAttributeSuggestionCandidate,
	promptByKey map[string]ozonAttributePromptCandidate,
	batchSize int,
	feedback map[string]string,
) []ozonAttributeBatchWork {
	if batchSize < 1 {
		batchSize = 1
	}
	works := make([]ozonAttributeBatchWork, 0, (len(candidates)+batchSize-1)/batchSize)
	work := ozonAttributeBatchWork{Prompt: []ozonAttributePromptCandidate{}, Feedback: map[string]string{}}
	workBytes := 0
	flush := func() {
		if len(work.Candidates) == 0 {
			return
		}
		works = append(works, work)
		work = ozonAttributeBatchWork{Prompt: []ozonAttributePromptCandidate{}, Feedback: map[string]string{}}
		workBytes = 0
	}
	for _, candidate := range candidates {
		promptCandidate := promptByKey[candidate.key]
		encoded, _ := json.Marshal(promptCandidate)
		candidateBytes := len(encoded) + 1
		if len(work.Candidates) > 0 && (len(work.Candidates) >= batchSize || workBytes+candidateBytes > ozonAttributeSuggestionMaxBatchBytes) {
			flush()
		}
		work.Candidates = append(work.Candidates, candidate)
		work.Prompt = append(work.Prompt, promptCandidate)
		workBytes += candidateBytes
		if reason := strings.TrimSpace(feedback[candidate.key]); reason != "" {
			work.Feedback[candidate.key] = truncateRunes(reason, 220)
		}
	}
	flush()
	return works
}

func callOzonAttributeBatches(
	ctx context.Context,
	client AIChatClient,
	promptRow aiprompt.AIPrompt,
	promptContext ozonAttributePromptContext,
	works []ozonAttributeBatchWork,
) []ozonAttributeBatchResult {
	results := make([]ozonAttributeBatchResult, len(works))
	semaphore := make(chan struct{}, ozonAttributeSuggestionMaxConcurrency)
	var group sync.WaitGroup
	for index := range works {
		index := index
		group.Add(1)
		go func() {
			defer group.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results[index] = ozonAttributeBatchResult{Work: works[index], TransportErr: ctx.Err()}
				return
			}
			results[index] = callOzonAttributeBatch(ctx, client, promptRow, promptContext, works[index])
		}()
	}
	group.Wait()
	return results
}

func callOzonAttributeBatch(
	ctx context.Context,
	client AIChatClient,
	promptRow aiprompt.AIPrompt,
	promptContext ozonAttributePromptContext,
	work ozonAttributeBatchWork,
) ozonAttributeBatchResult {
	contextJSON, _ := json.Marshal(promptContext)
	attributesJSON, _ := json.Marshal(work.Prompt)
	sourceRefsJSON, _ := json.Marshal(promptContext.AllowedSourceRefs)
	vars := map[string]string{
		"evidence": string(contextJSON), "context": string(contextJSON),
		"attributes": string(attributesJSON), "sourceRefs": string(sourceRefsJSON),
	}
	userPrompt := aiprompt.ReplaceVariables(promptRow.UserPrompt, vars)
	if len(work.Feedback) > 0 {
		feedbackJSON, _ := json.Marshal(work.Feedback)
		userPrompt += "\n\n上一次输出未通过校验；请按以下服务端反馈修正，并仍为每个 attributeKey 返回一项：\n" + string(feedbackJSON)
	}
	maxTokens := 1000 + len(work.Candidates)*250
	if maxTokens < 1200 {
		maxTokens = 1200
	}
	if maxTokens > 5000 {
		maxTokens = 5000
	}
	request := aigate.ChatRequest{
		Model: strings.TrimSpace(promptRow.Model),
		Messages: []aigate.Message{
			{Role: "system", Content: aiprompt.ReplaceVariables(promptRow.SystemPrompt, vars) + "\n\n" + aiprompt.OzonAttributeSuggestionRuntimePolicy()},
			{Role: "user", Content: userPrompt},
		},
		Temperature: promptRow.Temperature, MaxTokens: maxTokens,
		ResponseFormat: &aigate.ResponseFormat{Type: "json_object"},
	}
	resp, err := client.Chat(ctx, request)
	result := ozonAttributeBatchResult{Work: work, Response: resp, TransportErr: err}
	if err != nil || resp == nil {
		if err == nil {
			result.TransportErr = fmt.Errorf("empty attribute batch response")
		}
		return result
	}
	if err := decodeStrictOzonRecommendationJSON(resp.Content, &result.Output); err != nil || len(result.Output.Suggestions) > len(work.Candidates)*2 {
		if err == nil {
			err = fmt.Errorf("attribute batch returned too many suggestions")
		}
		result.ParseErr = err
	}
	return result
}

func mergeOzonAttributeBatchOutput(
	outputsByKey map[string][]ozonAttributeAICandidate,
	candidates []ozonAttributeSuggestionCandidate,
	output ozonAttributeAIOutput,
	warnings *[]string,
) {
	allowed := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		allowed[candidate.key] = true
	}
	for _, item := range output.Suggestions {
		key := strings.TrimSpace(item.AttributeKey)
		if !allowed[key] {
			*warnings = append(*warnings, "AI 返回了当前批次之外的属性引用，已丢弃")
			continue
		}
		outputsByKey[key] = append(outputsByKey[key], item)
	}
}

func replaceOzonAttributeBatchOutput(
	outputsByKey map[string][]ozonAttributeAICandidate,
	candidates []ozonAttributeSuggestionCandidate,
	output ozonAttributeAIOutput,
	warnings *[]string,
) {
	replacements := map[string][]ozonAttributeAICandidate{}
	mergeOzonAttributeBatchOutput(replacements, candidates, output, warnings)
	for _, candidate := range candidates {
		if items, exists := replacements[candidate.key]; exists {
			outputsByKey[candidate.key] = items
		}
	}
}

func validateOzonAttributeCandidateOutputs(
	candidates []ozonAttributeSuggestionCandidate,
	outputsByKey map[string][]ozonAttributeAICandidate,
	promptContext ozonAttributePromptContext,
) map[string]ozonAttributeCandidateValidation {
	validations := make(map[string]ozonAttributeCandidateValidation, len(candidates))
	knownSourceRefs := make(map[string]bool, len(promptContext.AllowedSourceRefs))
	for _, ref := range promptContext.AllowedSourceRefs {
		knownSourceRefs[ref] = true
	}
	knownFactRefs := make(map[string]bool, len(promptContext.Facts))
	for _, fact := range promptContext.Facts {
		knownFactRefs[fact.FactKey] = true
	}
	for _, candidate := range candidates {
		validations[candidate.key] = validateOzonAttributeCandidateOutput(
			candidate, outputsByKey[candidate.key], knownSourceRefs, knownFactRefs, promptContext,
		)
	}
	applyOzonAttributeCrossFieldValidation(candidates, validations)
	return validations
}

func applyOzonAttributeCrossFieldValidation(
	candidates []ozonAttributeSuggestionCandidate,
	validations map[string]ozonAttributeCandidateValidation,
) {
	keysByAttributeID := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		keysByAttributeID[strings.TrimSpace(candidate.attr.AttrID)] = candidate.key
	}
	physicalSIM := validations[keysByAttributeID["4407"]].Suggestion
	if physicalSIM == nil || len(physicalSIM.Values) != 1 || strings.TrimSpace(physicalSIM.Values[0].Value) != "1" {
		return
	}
	multipleSIMKey := keysByAttributeID["12128"]
	if validation := validations[multipleSIMKey]; validation.Suggestion != nil {
		validations[multipleSIMKey] = ozonAttributeCandidateValidation{
			Kind:   ozonAttributeSkipValidation,
			Reason: "物理 SIM 卡数量建议为 1，多 SIM 卡操作属性与之矛盾且不适用，已留空",
		}
	}
}

func validateOzonAttributeCandidateOutput(
	candidate ozonAttributeSuggestionCandidate,
	items []ozonAttributeAICandidate,
	knownSourceRefs map[string]bool,
	knownFactRefs map[string]bool,
	promptContext ozonAttributePromptContext,
) ozonAttributeCandidateValidation {
	if len(items) == 0 {
		return ozonAttributeCandidateValidation{Kind: ozonAttributeSkipOmitted, Reason: "AI 未返回该属性，补全后仍无可用建议，已留空"}
	}
	if len(items) > 1 {
		return ozonAttributeCandidateValidation{Kind: ozonAttributeSkipValidation, Reason: "AI 对同一属性返回冲突建议，已留空"}
	}
	item := items[0]
	sourceRefs := item.SourceRefs
	if len(sourceRefs) == 0 {
		sourceRefs = item.EvidenceKeys
	}
	sourceRefs = boundedStrings(sourceRefs, 12, 80)
	if len(sourceRefs) == 0 {
		return ozonAttributeCandidateValidation{Kind: ozonAttributeSkipValidation, Reason: "AI 建议缺少来源引用，已留空"}
	}
	for _, ref := range sourceRefs {
		if !knownSourceRefs[ref] {
			return ozonAttributeCandidateValidation{Kind: ozonAttributeSkipValidation, Reason: "AI 建议引用了未知来源，已留空"}
		}
	}
	factRefs := boundedStrings(item.FactRefs, 12, 80)
	for _, ref := range factRefs {
		if !knownFactRefs[ref] {
			return ozonAttributeCandidateValidation{Kind: ozonAttributeSkipValidation, Reason: "AI 建议引用了未知商品事实，已留空"}
		}
	}
	rawReason := strings.TrimSpace(item.Reason)
	if rawReason == "" {
		return ozonAttributeCandidateValidation{Kind: ozonAttributeSkipValidation, Reason: "AI 建议缺少可审核的推断理由，已留空"}
	}
	reason := truncateRunes(sanitizeOzonAttributeSuggestionEvidenceValue(rawReason), 240)
	if reason == "" {
		reason = "推断依据包含敏感内容并已脱敏，请人工核对"
	}
	values := make([]string, 0, len(item.Values))
	seen := map[string]bool{}
	for _, raw := range item.Values {
		value := truncateRunes(strings.TrimSpace(raw), 500)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	if len(values) == 0 {
		return ozonAttributeCandidateValidation{Kind: ozonAttributeSkipValidation, Reason: "AI 未返回可用值，已留空"}
	}
	if conflict := ozonAttributeKnownSemanticConflict(candidate.attr, values, rawReason, promptContext); conflict != "" {
		return ozonAttributeCandidateValidation{Kind: ozonAttributeSkipValidation, Reason: conflict}
	}
	if isOzonAttributeProductCopyField(candidate.attr) {
		if dimension := ozonAttributeSingleSKUVariantMention(strings.Join(values, " "), promptContext.SKUVariations); dimension != "" {
			return ozonAttributeCandidateValidation{
				Kind:   ozonAttributeSkipValidation,
				Reason: "建议包含仅适用于部分代表 SKU 的“" + truncateRunes(dimension, 60) + "”规格；请改为不含颜色、尺寸、材质或型号变体的整商品中性文案",
			}
		}
	}
	if isOzonAttributeProductNameField(candidate.attr) {
		if token := ozonAttributeUngroundedLatinNameToken(values, promptContext); token != "" {
			return ozonAttributeCandidateValidation{
				Kind:   ozonAttributeSkipValidation,
				Reason: "名称中的拉丁词“" + truncateRunes(token, 40) + "”未出现在商品证据、类目或事实表，疑似臆造品牌/型号；请删除或改用有依据的中性名称",
			}
		}
	}
	basis := deriveOzonAttributeInferenceBasis(item, sourceRefs, factRefs, values, promptContext)
	selections, err := validatedOzonAttributeSuggestionSelections(candidate, values)
	if err != nil {
		kind := ozonAttributeSkipValidation
		if candidate.attr.DictionaryID != "" {
			kind = ozonAttributeSkipDictionary
		}
		return ozonAttributeCandidateValidation{Kind: kind, Reason: "AI 值未通过当前模板校验：" + truncateRunes(err.Error(), 220)}
	}
	confidence, level, review := 0.3, "low", true
	switch basis {
	case ozonAttributeBasisDirect:
		confidence, level, review = 0.9, "high", false
	case ozonAttributeBasisStandard:
		confidence, level = 0.7, "medium"
	}
	name := strings.TrimSpace(candidate.attr.Name)
	if name == "" {
		name = candidate.attr.AttrID
	}
	return ozonAttributeCandidateValidation{Suggestion: &OzonAttributeSuggestion{
		AttributeID: candidate.attr.AttrID, AttributeName: name, Values: selections,
		Confidence: confidence, ConfidenceLevel: level, InferenceBasis: basis,
		RequiresReview: review, Reason: reason, SourceRefs: sourceRefs,
	}}
}

func deriveOzonAttributeInferenceBasis(
	item ozonAttributeAICandidate,
	sourceRefs []string,
	factRefs []string,
	values []string,
	promptContext ozonAttributePromptContext,
) string {
	hasDirectRef := len(factRefs) > 0
	hasCategory := false
	hasCommon := false
	for _, ref := range sourceRefs {
		switch {
		case strings.HasPrefix(ref, "product."), strings.HasPrefix(ref, "sku."), strings.HasPrefix(ref, "image."):
			hasDirectRef = true
		case strings.HasPrefix(ref, "category."):
			hasCategory = true
		case ref == "common_knowledge":
			hasCommon = true
		}
	}
	basis := strings.TrimSpace(item.InferenceBasis)
	if basis == "" {
		// Compatibility for custom persisted v2 prompts: retain their numeric
		// signal, but never let it override the source-based downgrade below.
		switch {
		case item.Confidence >= ozonAttributeSuggestionHighThreshold:
			basis = ozonAttributeBasisDirect
		case item.Confidence >= ozonAttributeSuggestionMediumThreshold:
			basis = ozonAttributeBasisStandard
		default:
			basis = ozonAttributeBasisFallback
		}
	}
	switch basis {
	case ozonAttributeBasisDirect:
		if hasVerifiedOzonAttributeDirectEvidence(values, sourceRefs, factRefs, promptContext) && !hasCommon {
			return ozonAttributeBasisDirect
		}
		if hasDirectRef || hasCategory {
			return ozonAttributeBasisStandard
		}
		return ozonAttributeBasisFallback
	case ozonAttributeBasisStandard:
		if hasDirectRef || hasCategory {
			return ozonAttributeBasisStandard
		}
		return ozonAttributeBasisFallback
	default:
		return ozonAttributeBasisFallback
	}
}

func hasVerifiedOzonAttributeDirectEvidence(
	values []string,
	sourceRefs []string,
	factRefs []string,
	promptContext ozonAttributePromptContext,
) bool {
	directTexts := make([]string, 0, len(sourceRefs)+len(factRefs))
	for _, ref := range sourceRefs {
		if strings.HasPrefix(ref, "product.") || strings.HasPrefix(ref, "sku.") {
			if sourceText := ozonAttributeSourceRefText(promptContext, ref); strings.TrimSpace(sourceText) != "" {
				directTexts = append(directTexts, sourceText)
			}
		}
	}
	requestedFacts := map[string]bool{}
	for _, ref := range factRefs {
		requestedFacts[ref] = true
	}
	for _, fact := range promptContext.Facts {
		if requestedFacts[fact.FactKey] {
			directTexts = append(directTexts, fact.Value+" "+fact.Evidence)
		}
	}
	if len(directTexts) == 0 || len(values) == 0 {
		return false
	}
	for _, value := range values {
		valueKey := normalizeOzonRecommendationText(value)
		if valueKey == "" {
			return false
		}
		matched := false
		for _, directText := range directTexts {
			if strings.Contains(normalizeOzonRecommendationText(directText), valueKey) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

type ozonAttributeDictionaryLookup struct {
	CandidateIndex int
	Semantic       string
	Result         *ozonAttributeDictionaryOption
	Err            error
}

func (s *Service) resolveOzonAttributeDictionaryFallbacks(
	ctx context.Context,
	tenantID int64,
	shopID uuid.UUID,
	categoryID string,
	candidates []ozonAttributeSuggestionCandidate,
	outputsByKey map[string][]ozonAttributeAICandidate,
) []string {
	searcher, ok := s.OzonCategories.(ozonAttributeDictionarySearcher)
	lookups := make([]ozonAttributeDictionaryLookup, 0)
	seen := map[string]bool{}
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.attr.DictionaryID == "" {
			continue
		}
		for _, item := range outputsByKey[candidate.key] {
			for _, semantic := range item.Values {
				semantic = strings.TrimSpace(semantic)
				if semantic == "" || len(dictionarySemanticMatches(candidate.options, semantic)) != 0 {
					continue
				}
				if looksLikeOzonDictionaryID(candidate.options, semantic) {
					continue
				}
				key := candidate.key + "\n" + normalizeOzonRecommendationText(semantic)
				if key == candidate.key+"\n" || seen[key] {
					continue
				}
				seen[key] = true
				lookups = append(lookups, ozonAttributeDictionaryLookup{CandidateIndex: index, Semantic: semantic})
			}
		}
	}
	if len(lookups) == 0 {
		return nil
	}
	if !ok {
		return []string{"Ozon 官方词典只读搜索暂不可用；缓存外语义值已留空"}
	}
	semaphore := make(chan struct{}, ozonAttributeSuggestionDictConcurrency)
	var group sync.WaitGroup
	for index := range lookups {
		index := index
		group.Add(1)
		go func() {
			defer group.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				lookups[index].Err = ctx.Err()
				return
			}
			candidate := candidates[lookups[index].CandidateIndex]
			values, err := searcher.SearchOzonDictionaryValues(
				ctx, tenantID, categoryID, candidate.attr.AttrID, shopID, lookups[index].Semantic,
			)
			if err != nil {
				lookups[index].Err = err
				return
			}
			matches := make([]ozonAttributeDictionaryOption, 0)
			seenIDs := map[string]bool{}
			semanticKey := normalizeOzonRecommendationText(lookups[index].Semantic)
			for _, value := range values {
				id := strings.TrimSpace(value.ID)
				label := strings.TrimSpace(value.Value)
				if id == "" || label == "" || normalizeOzonRecommendationText(label) != semanticKey || seenIDs[id] {
					continue
				}
				seenIDs[id] = true
				matches = append(matches, ozonAttributeDictionaryOption{ID: id, Value: label})
			}
			if len(matches) == 1 {
				lookups[index].Result = &matches[0]
			}
		}()
	}
	group.Wait()
	sort.SliceStable(lookups, func(i, j int) bool {
		if lookups[i].CandidateIndex != lookups[j].CandidateIndex {
			return lookups[i].CandidateIndex < lookups[j].CandidateIndex
		}
		return lookups[i].Semantic < lookups[j].Semantic
	})
	warnings := []string{}
	for _, lookup := range lookups {
		candidate := &candidates[lookup.CandidateIndex]
		if lookup.Err != nil {
			warnings = append(warnings, "属性 "+candidate.attr.Name+" 的 Ozon 官方词典只读搜索失败，已留空")
			continue
		}
		if lookup.Result == nil {
			continue
		}
		duplicateID := false
		for _, existing := range candidate.options {
			if existing.ID == lookup.Result.ID {
				duplicateID = true
				break
			}
		}
		if !duplicateID {
			candidate.options = append(candidate.options, *lookup.Result)
		}
	}
	return warnings
}

func looksLikeOzonDictionaryID(options []ozonAttributeDictionaryOption, semantic string) bool {
	semantic = strings.TrimSpace(semantic)
	if semantic == "" {
		return false
	}
	allDigits := true
	for _, char := range semantic {
		if char < '0' || char > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		return true
	}
	for _, option := range options {
		if semantic == strings.TrimSpace(option.ID) && normalizeOzonRecommendationText(semantic) != normalizeOzonRecommendationText(option.Value) {
			return true
		}
	}
	return false
}

func dictionarySemanticMatches(options []ozonAttributeDictionaryOption, semantic string) []ozonAttributeDictionaryOption {
	key := normalizeOzonRecommendationText(semantic)
	if key == "" {
		return nil
	}
	matches := make([]ozonAttributeDictionaryOption, 0)
	for _, option := range options {
		if normalizeOzonRecommendationText(option.Value) == key {
			matches = append(matches, option)
		}
	}
	return matches
}

func firstNonNilOzonAttributeError(err error, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}

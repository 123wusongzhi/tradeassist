package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	ozonRecommendationPromptCode       = "ozon_category_recommendation_v2"
	ozonRecommendationAIRequestTimeout = 30 * time.Second
	ozonRecommendationRefreshTimeout   = 12 * time.Second
	ozonRecommendationMaxRecall        = 50
	ozonRecommendationMaxTemplates     = 8
	ozonRecommendationMaxRefresh       = 5
)

// RecommendOzonCategories analyzes persisted SKU selections and ranks only
// active leaf categories returned by the shop cache. It never mutates product,
// SKU, mapping or publish configuration rows.
func (s *Service) RecommendOzonCategories(
	c *gin.Context,
	productID uuid.UUID,
	body OzonCategoryRecommendationBody,
	adminID *uuid.UUID,
) (*OzonCategoryRecommendationResult, error) {
	if s == nil || s.DB == nil {
		return nil, unavailableOzonRecommendation("类目推荐服务暂不可用", fmt.Errorf("product: no db"))
	}
	if s.OzonCategories == nil {
		return nil, unavailableOzonRecommendation("Ozon 类目缓存服务暂不可用", fmt.Errorf("catalog not configured"))
	}
	if s.AITasks == nil {
		return nil, unavailableOzonRecommendation("AI 审计服务暂不可用", fmt.Errorf("ai task audit not configured"))
	}
	if err := adminperm.EnsureProductOperate(c, s.DB, productID); err != nil {
		return nil, err
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	shopID, err := uuid.Parse(strings.TrimSpace(body.ShopID))
	if err != nil || shopID == uuid.Nil {
		return nil, invalidOzonRecommendation("shopId 必须是有效的 Ozon 店铺 ID", err)
	}
	principal, err := adminperm.LoadPrincipal(c, s.DB)
	if err != nil {
		return nil, err
	}
	if principal == nil || (!principal.IsTenantAdmin() && !principal.CanOperateStore(shopID)) {
		if principal != nil && principal.CanViewStore(shopID) {
			return nil, &ozonRecommendationAPIError{status: 403, code: OzonCategoryRecommendationInvalid, message: "当前账号无权操作该 Ozon 店铺"}
		}
		return nil, gorm.ErrRecordNotFound
	}
	if err := s.OzonCategories.EnsureAuthorizedOzonShop(c.Request.Context(), tenantID, shopID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, invalidOzonRecommendation("Ozon 店铺不存在、未启用或未授权", err)
	}
	refreshPolicy := strings.TrimSpace(strings.ToLower(body.RefreshPolicy))
	if refreshPolicy == "" {
		refreshPolicy = OzonRecommendationRefreshIfMissingOrStale
	}
	if refreshPolicy != OzonRecommendationRefreshIfMissingOrStale && refreshPolicy != OzonRecommendationRefreshCacheOnly {
		return nil, invalidOzonRecommendation("refreshPolicy 仅支持 if_missing_or_stale 或 cache_only", nil)
	}

	productRow, err := s.findTenantProduct(c, productID, "SKUs")
	if err != nil {
		return nil, err
	}
	snapshot, err := buildOzonRecommendationSnapshot(*productRow, body.SKUIDs)
	if err != nil {
		return nil, invalidOzonRecommendation(err.Error(), err)
	}
	result := &OzonCategoryRecommendationResult{
		Status:               OzonCategoryRecommendationPartial,
		SourceSummary:        ozonRecommendationSourceSummary(snapshot, len(productRow.SKUs)),
		DifferenceDimensions: []OzonRecommendationDifferenceDimension{},
		Anomalies:            []OzonRecommendationAnomaly{}, Candidates: []OzonCategoryRecommendationCandidate{}, Warnings: []string{},
	}

	auditInput, _ := json.Marshal(map[string]any{
		"productId": productID.String(), "shopId": shopID.String(),
		"selectedSkuIds": selectedRecommendationSKUIDs(snapshot),
		"skuCount":       len(snapshot.SKUs), "skuGroupNames": result.SourceSummary.SKUGroupNames,
		"refreshPolicy": refreshPolicy,
	})
	task := &aitask.AITask{
		TenantID: tenantID, TaskType: "ozon_category_recommendation",
		Provider: s.providerNameFromSettings(c), PromptCode: ozonRecommendationPromptCode,
		Input: datatypes.JSON(auditInput), ProductID: &productID, CreatedBy: adminID,
	}
	if err := s.AITasks.Create(c.Request.Context(), task); err != nil {
		return nil, unavailableOzonRecommendation("无法建立 AI 推荐审计记录", err)
	}
	result.TaskID = &task.ID

	client := s.OzonCategoryAI
	if client == nil && s.AIGateway != nil {
		client = s.AIGateway
	}
	if client == nil {
		_ = s.AITasks.MarkFailed(c.Request.Context(), task.ID, "analysis client unavailable")
		result.Status = OzonCategoryRecommendationAIUnavailable
		result.Warnings = append(result.Warnings, "AI 分析暂不可用，可继续手动选择 Ozon 类目")
		return result, nil
	}

	analysisResponse, err := callOzonRecommendationAnalysis(c.Request.Context(), client, snapshot)
	if err != nil || analysisResponse == nil {
		_ = s.AITasks.MarkFailed(c.Request.Context(), task.ID, "analysis call failed")
		result.Status = OzonCategoryRecommendationAIUnavailable
		result.Warnings = append(result.Warnings, "AI 分析暂不可用，可继续手动选择 Ozon 类目")
		return result, nil
	}
	analysis, err := parseOzonAIAnalysis(analysisResponse.Content, snapshot)
	if err != nil {
		_ = s.AITasks.MarkFailedWithMeta(c.Request.Context(), task.ID, "analysis response rejected: "+err.Error(), nil, analysisResponse.InputTokens, analysisResponse.OutputTokens, analysisResponse.Model)
		result.Status = OzonCategoryRecommendationAIUnavailable
		result.Warnings = append(result.Warnings, "AI 返回的 SKU 分析未通过证据校验，已拒绝；可继续手动选择类目")
		return result, nil
	}
	result.ProductType = analysis.ProductType
	result.DifferenceDimensions = publicOzonRecommendationDimensions(analysis.DifferenceDimensions, snapshot)
	result.Anomalies = publicOzonRecommendationAnomalies(analysis.Anomalies, snapshot)
	tokenInput := analysisResponse.InputTokens
	tokenOutput := analysisResponse.OutputTokens
	usedModel := strings.TrimSpace(analysisResponse.Model)

	roots, rootsErr := s.ozonRecommendationRoots(c.Request.Context())
	if rootsErr != nil {
		result.Status = OzonCategoryRecommendationPartial
		result.Warnings = append(result.Warnings, "Ozon 一级类目缓存读取失败，暂不能形成安全候选；人工导航仍可继续使用")
		s.finishOzonRecommendationAudit(c.Request.Context(), task.ID, result, tokenInput, tokenOutput, usedModel)
		return result, nil
	}
	if len(roots) == 0 {
		result.Status = OzonCategoryRecommendationCategoryCacheEmpty
		result.Warnings = append(result.Warnings, "Ozon 启用一级类目缓存为空，请先同步类目缓存；人工导航仍可继续使用")
		s.finishOzonRecommendationAudit(c.Request.Context(), task.ID, result, tokenInput, tokenOutput, usedModel)
		return result, nil
	}
	rootResponse, rootErr := callOzonRecommendationRootSelection(
		c.Request.Context(), client, snapshot, analysis.ProductType, analysis.SearchTerms, roots,
	)
	if rootErr != nil || rootResponse == nil {
		_ = s.AITasks.MarkFailedWithMeta(c.Request.Context(), task.ID, "root category routing failed", nil, tokenInput, tokenOutput, usedModel)
		result.Status = OzonCategoryRecommendationAIUnavailable
		result.Warnings = append(result.Warnings, "AI 一级类目领域筛选暂不可用，可继续手动选择类目")
		return result, nil
	}
	tokenInput += rootResponse.InputTokens
	tokenOutput += rootResponse.OutputTokens
	if strings.TrimSpace(rootResponse.Model) != "" {
		usedModel = strings.TrimSpace(rootResponse.Model)
	}
	rootSelection, rootParseErr := parseOzonAIRootSelection(rootResponse.Content, roots)
	if rootParseErr != nil {
		_ = s.AITasks.MarkFailedWithMeta(c.Request.Context(), task.ID, "root category routing response rejected: "+rootParseErr.Error(), nil, tokenInput, tokenOutput, usedModel)
		result.Status = OzonCategoryRecommendationAIUnavailable
		result.Warnings = append(result.Warnings, "AI 一级类目领域筛选引用了未知 rootKey 或返回非法 JSON，已拒绝；可继续手动选择类目")
		return result, nil
	}
	if len(rootSelection.SelectedRootKeys) == 0 {
		result.Status = OzonCategoryRecommendationNoMatch
		result.Warnings = append(result.Warnings, "AI 查看真实一级类目后未找到能容纳该商品主体的领域，可继续手动搜索")
		s.finishOzonRecommendationAudit(c.Request.Context(), task.ID, result, tokenInput, tokenOutput, usedModel)
		return result, nil
	}

	history, historyWarnings := s.ozonRecommendationHistory(c.Request.Context(), tenantID, shopID)
	result.Warnings = append(result.Warnings, historyWarnings...)
	partial := len(historyWarnings) > 0
	rootIDs, rootNames := selectedOzonRecommendationRoots(roots, rootSelection)
	searchResult, searchErr := s.OzonCategories.SearchOzonLeafCategories(c.Request.Context(), shop.OzonCategorySearchQuery{
		ProductType: analysis.ProductType, SearchTerms: analysis.SearchTerms,
		ProductTitle: snapshot.Title, AllowedRootIDs: rootIDs, AllowedRootNames: rootNames,
		Limit: ozonRecommendationMaxSearchChoices,
	})
	var works []*ozonRecommendationCandidateWork
	if searchErr != nil || searchResult == nil {
		fallback, cacheEmpty, fallbackPartial, fallbackWarnings := s.recallOzonRecommendationCandidates(
			c.Request.Context(), tenantID, shopID, analysis.ProductType, analysis.SearchTerms, snapshot.Title,
		)
		result.Warnings = append(result.Warnings, "Ozon 类目搜索索引暂不可用，已使用本地兼容召回")
		result.Warnings = append(result.Warnings, fallbackWarnings...)
		partial = true
		if cacheEmpty {
			result.Status = OzonCategoryRecommendationCategoryCacheEmpty
			result.Warnings = append(result.Warnings, "Ozon 启用叶子类目缓存为空，请先在高级类目维护中同步缓存")
			s.finishOzonRecommendationAudit(c.Request.Context(), task.ID, result, tokenInput, tokenOutput, usedModel)
			return result, nil
		}
		partial = partial || fallbackPartial
		works = fallback
		for index, work := range works {
			work.key = fmt.Sprintf("candidate_%d", index+1)
			work.searchScore = work.semantic
		}
	} else {
		if searchResult.IndexedLeafCount == 0 {
			result.Status = OzonCategoryRecommendationCategoryCacheEmpty
			result.Warnings = append(result.Warnings, "Ozon 启用叶子类目缓存为空，请先在高级类目维护中同步缓存")
			s.finishOzonRecommendationAudit(c.Request.Context(), task.ID, result, tokenInput, tokenOutput, usedModel)
			return result, nil
		}
		if searchResult.CacheStale {
			partial = true
			result.Warnings = append(result.Warnings, "Ozon 类目路径缓存已过新鲜期，候选仍来自真实启用叶子类目；建议稍后同步缓存")
		}
		searchResult = filterOzonRecommendationSearchByRoots(searchResult, roots, rootSelection)
		works = ozonRecommendationWorksFromSearch(searchResult, history)
	}
	if len(works) == 0 {
		if partial {
			result.Status = OzonCategoryRecommendationPartial
			result.Warnings = append(result.Warnings, "类目索引读取不完整，暂未形成安全候选；可继续手动搜索")
		} else {
			result.Status = OzonCategoryRecommendationNoMatch
			result.Warnings = append(result.Warnings, "本地完整叶子路径索引未召回相关候选，可继续手动搜索")
		}
		s.finishOzonRecommendationAudit(c.Request.Context(), task.ID, result, tokenInput, tokenOutput, usedModel)
		return result, nil
	}

	pathResponse, pathErr := callOzonRecommendationPathSelection(
		c.Request.Context(), client, snapshot, analysis.ProductType, analysis.SearchTerms, works,
	)
	if pathErr != nil || pathResponse == nil {
		_ = s.AITasks.MarkFailedWithMeta(c.Request.Context(), task.ID, "path screening failed", nil, tokenInput, tokenOutput, usedModel)
		result.Status = OzonCategoryRecommendationAIUnavailable
		result.Warnings = append(result.Warnings, "AI 完整路径筛选暂不可用，可继续手动选择类目")
		return result, nil
	}
	tokenInput += pathResponse.InputTokens
	tokenOutput += pathResponse.OutputTokens
	if strings.TrimSpace(pathResponse.Model) != "" {
		usedModel = strings.TrimSpace(pathResponse.Model)
	}
	allowlist := make(map[string]bool, len(works))
	for _, work := range works {
		allowlist[work.key] = true
	}
	pathSelection, pathParseErr := parseOzonAIPathSelection(pathResponse.Content, allowlist)
	if pathParseErr != nil {
		_ = s.AITasks.MarkFailedWithMeta(c.Request.Context(), task.ID, "path screening response rejected: "+pathParseErr.Error(), nil, tokenInput, tokenOutput, usedModel)
		result.Status = OzonCategoryRecommendationAIUnavailable
		result.Warnings = append(result.Warnings, "AI 完整路径筛选返回了未知 candidateKey 或非法 JSON，已拒绝；可继续手动选择类目")
		return result, nil
	}
	works = selectOzonRecommendationPaths(works, pathSelection)
	if len(works) == 0 {
		result.Status = OzonCategoryRecommendationNoMatch
		result.Warnings = append(result.Warnings, "AI 对真实完整路径复核后未找到匹配候选，可继续手动搜索")
		result.Warnings = boundedStrings(result.Warnings, 20, 240)
		s.finishOzonRecommendationAudit(c.Request.Context(), task.ID, result, tokenInput, tokenOutput, usedModel)
		return result, nil
	}

	templateWarnings, templatePartial := s.loadOzonRecommendationTemplates(
		c.Request.Context(), tenantID, shopID, works, refreshPolicy,
	)
	partial = partial || templatePartial
	result.Warnings = append(result.Warnings, templateWarnings...)

	finalReviewResponse, finalReviewErr := callOzonRecommendationFinalReview(
		c.Request.Context(), client, snapshot, works, analysis.ProductType, analysis.SearchTerms,
		result.DifferenceDimensions, result.Anomalies,
	)
	if finalReviewErr != nil || finalReviewResponse == nil {
		_ = s.AITasks.MarkFailedWithMeta(c.Request.Context(), task.ID, "final template review failed", nil, tokenInput, tokenOutput, usedModel)
		result.Status = OzonCategoryRecommendationAIUnavailable
		result.Warnings = append(result.Warnings, "AI 最新模板复核暂不可用，未输出未经复核的候选；可继续手动选择类目")
		return result, nil
	}
	tokenInput += finalReviewResponse.InputTokens
	tokenOutput += finalReviewResponse.OutputTokens
	if strings.TrimSpace(finalReviewResponse.Model) != "" {
		usedModel = strings.TrimSpace(finalReviewResponse.Model)
	}
	finalAllowlist := make(map[string]bool, len(works))
	for _, work := range works {
		finalAllowlist[work.key] = true
	}
	finalReview, finalParseErr := parseOzonAIFinalReview(finalReviewResponse.Content, finalAllowlist)
	if finalParseErr != nil {
		_ = s.AITasks.MarkFailedWithMeta(c.Request.Context(), task.ID, "final template review response rejected: "+finalParseErr.Error(), nil, tokenInput, tokenOutput, usedModel)
		result.Status = OzonCategoryRecommendationAIUnavailable
		result.Warnings = append(result.Warnings, "AI 最新模板复核引用了未知 candidateKey 或返回非法 JSON，已拒绝；可继续手动选择类目")
		return result, nil
	}
	reviewedPaths := make([]string, 0, 3)
	for _, work := range works {
		if len(reviewedPaths) >= 3 {
			break
		}
		reviewedPaths = append(reviewedPaths, truncateRunes(strings.TrimSpace(work.node.Path), 160))
	}
	works = applyOzonRecommendationFinalReview(works, finalReview)
	if len(works) == 0 {
		result.Status = OzonCategoryRecommendationNoMatch
		result.Warnings = append(result.Warnings, "AI 结合完整路径与最新真实模板后未找到可用候选，可继续手动搜索")
		if len(reviewedPaths) > 0 {
			result.Warnings = append(result.Warnings, "最终复核未保留的候选路径："+strings.Join(reviewedPaths, "；"))
		}
		result.Warnings = boundedStrings(result.Warnings, 20, 240)
		s.finishOzonRecommendationAudit(c.Request.Context(), task.ID, result, tokenInput, tokenOutput, usedModel)
		return result, nil
	}

	additionalWarnings, additionalPartial, additionalRefreshed := s.refreshUnattemptedOzonRecommendationTemplates(
		c.Request.Context(), tenantID, shopID, works, refreshPolicy, ozonRecommendationMaxTemplates-ozonRecommendationMaxRefresh,
	)
	partial = partial || additionalPartial
	result.Warnings = append(result.Warnings, additionalWarnings...)
	if additionalRefreshed {
		resetOzonRecommendationAISelectionMetadata(works)
		reviewResponse, reviewErr := callOzonRecommendationFinalReview(
			c.Request.Context(), client, snapshot, works, analysis.ProductType, analysis.SearchTerms,
			result.DifferenceDimensions, result.Anomalies,
		)
		if reviewErr != nil || reviewResponse == nil {
			result.Status = OzonCategoryRecommendationAIUnavailable
			result.Warnings = append(result.Warnings, "补充模板刷新后的 AI 复核失败，未输出未经最新复核的候选；可继续手动选择类目")
			return result, nil
		}
		tokenInput += reviewResponse.InputTokens
		tokenOutput += reviewResponse.OutputTokens
		if strings.TrimSpace(reviewResponse.Model) != "" {
			usedModel = strings.TrimSpace(reviewResponse.Model)
		}
		allowlist = make(map[string]bool, len(works))
		for _, work := range works {
			allowlist[work.key] = true
		}
		review, reviewParseErr := parseOzonAIFinalReview(reviewResponse.Content, allowlist)
		if reviewParseErr != nil {
			result.Status = OzonCategoryRecommendationAIUnavailable
			result.Warnings = append(result.Warnings, "补充模板刷新后的 AI 复核返回非法结果，已拒绝；可继续手动选择类目")
			return result, nil
		}
		works = applyOzonRecommendationFinalReview(works, review)
	}

	viable := make([]*ozonRecommendationCandidateWork, 0, len(works))
	for _, work := range works {
		if len(work.attrs) == 0 {
			continue
		}
		scoreOzonRecommendationCandidate(work, snapshot, result.DifferenceDimensions, result.Anomalies)
		applyOzonRecommendationAISelection(work)
		viable = append(viable, work)
	}
	if len(viable) == 0 {
		if partial {
			result.Status = OzonCategoryRecommendationPartial
			result.Warnings = append(result.Warnings, "候选模板读取不完整，暂未生成可安全核对的候选")
		} else {
			result.Status = OzonCategoryRecommendationNoMatch
			result.Warnings = append(result.Warnings, "真实候选模板与商品语义不足以形成推荐")
		}
		s.finishOzonRecommendationAudit(c.Request.Context(), task.ID, result, tokenInput, tokenOutput, usedModel)
		return result, nil
	}
	limit := 3
	if len(viable) < limit {
		limit = len(viable)
	}
	for _, work := range viable[:limit] {
		result.Candidates = append(result.Candidates, work.result)
	}
	if partial {
		result.Status = OzonCategoryRecommendationPartial
	} else {
		result.Status = OzonCategoryRecommendationReady
	}
	result.Warnings = boundedStrings(result.Warnings, 20, 240)
	s.finishOzonRecommendationAudit(c.Request.Context(), task.ID, result, tokenInput, tokenOutput, usedModel)
	return result, nil
}

func buildOzonRecommendationSnapshot(productRow Product, requestedIDs []string) (ozonRecommendationSnapshot, error) {
	requested := map[uuid.UUID]bool{}
	for _, rawID := range requestedIDs {
		id, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil || id == uuid.Nil {
			return ozonRecommendationSnapshot{}, fmt.Errorf("skuIds 包含无效 ID")
		}
		requested[id] = true
	}
	attrsRaw, skuGroupsRaw := rawDraftDebugFields(json.RawMessage(productRow.RawData))
	title := strings.TrimSpace(productRow.Title)
	if title == "" {
		title = strings.TrimSpace(productRow.OriginalTitle)
	}
	description := strings.TrimSpace(productRow.Description)
	if description == "" {
		description = strings.TrimSpace(productRow.AIDescription)
	}
	snapshot := ozonRecommendationSnapshot{
		ProductID: productRow.ID.String(), Title: truncateRunes(title, 320),
		Description:       truncateRunes(description, 500),
		ProductAttributes: flatStringMap(attrsRaw, false),
		SKUGroups:         parseRecommendationSKUGroups(skuGroupsRaw),
		SKUs:              []ozonRecommendationSnapshotSKU{},
	}
	found := map[uuid.UUID]bool{}
	for _, sku := range productRow.SKUs {
		if len(requested) > 0 && !requested[sku.ID] {
			continue
		}
		selections := flatStringMap(json.RawMessage(sku.Attrs), false)
		for key, value := range flatStringMap(json.RawMessage(sku.RawData), true) {
			if _, exists := selections[key]; !exists {
				selections[key] = value
			}
		}
		snapshot.SKUs = append(snapshot.SKUs, ozonRecommendationSnapshotSKU{
			ID: sku.ID.String(), Code: truncateRunes(strings.TrimSpace(sku.SKUCode), 128),
			Name: truncateRunes(strings.TrimSpace(sku.SKUName), 240), Selections: selections,
		})
		found[sku.ID] = true
	}
	if len(requested) > 0 && len(found) != len(requested) {
		return ozonRecommendationSnapshot{}, fmt.Errorf("skuIds 包含不属于当前商品的 SKU")
	}
	if len(snapshot.SKUs) == 0 {
		return ozonRecommendationSnapshot{}, fmt.Errorf("当前商品没有可分析的 SKU")
	}
	return snapshot, nil
}

func ozonRecommendationSourceSummary(snapshot ozonRecommendationSnapshot, totalSKUCount int) OzonRecommendationSourceSummary {
	groupNames := make([]string, 0, len(snapshot.SKUGroups))
	for _, group := range snapshot.SKUGroups {
		groupNames = append(groupNames, group.Name)
	}
	return OzonRecommendationSourceSummary{
		ProductTitle: snapshot.Title, SKUCount: totalSKUCount,
		SelectedSKUCount: len(snapshot.SKUs), SKUGroupNames: groupNames,
		ProductAttributeCount: len(snapshot.ProductAttributes),
		PrimaryEvidence:       "persisted_sku_classification_attributes",
	}
}

func selectedRecommendationSKUIDs(snapshot ozonRecommendationSnapshot) []string {
	ids := make([]string, 0, len(snapshot.SKUs))
	for _, sku := range snapshot.SKUs {
		ids = append(ids, sku.ID)
	}
	return ids
}

func callOzonRecommendationAnalysis(ctx context.Context, client AIChatClient, snapshot ozonRecommendationSnapshot) (*aigate.ChatResponse, error) {
	payload, _ := json.Marshal(snapshot)
	callCtx, cancel := context.WithTimeout(ctx, ozonRecommendationAIRequestTimeout)
	defer cancel()
	return client.Chat(callCtx, aigate.ChatRequest{
		Temperature: 0, MaxTokens: 3200, ResponseFormat: &aigate.ResponseFormat{Type: "json_object"},
		Messages: []aigate.Message{
			{Role: "system", Content: "你是商品 SKU 结构分析器。只分析输入中已持久化的商品文本、skuGroups 与每个 SKU 的 selections；不得输出或猜测 Ozon 类目 ID。严格输出单个 JSON 对象，不使用 Markdown。productType 必须概括物理商品主体、核心功能和主要适用对象，不能只抄标题末尾的宽泛名词。searchTerms 必须给出 3 至 8 个可在类目树中搜索的短商品主体名，覆盖常见同义名、标准上位类目名和核心功能名；当标题使用特定物种、人群、行业俗称或营销名称时，应同时给出更通用的目录表达，不含年份、营销词或完整标题。differenceDimensions 中每条 evidence 必须逐字引用某个 SKU selections 的 key/value，source 固定为 sku.attrs。即使来源分组名是‘颜色分类’，也要从复合文本临时拆解真实区别；如果某个 SKU 是配件或其他商品主体，必须在 anomalies 中标记 different_product_subject。semantic 只能是 model/control_method/current/package/color/size/material/other。"},
			{Role: "user", Content: "按以下结构输出：{\"productType\":\"\",\"searchTerms\":[\"\"],\"differenceDimensions\":[{\"key\":\"\",\"name\":\"\",\"semantic\":\"model\",\"confidence\":0.0,\"evidence\":[{\"skuId\":\"\",\"source\":\"sku.attrs\",\"sourceKey\":\"\",\"rawValue\":\"\"}]}],\"anomalies\":[{\"type\":\"different_product_subject\",\"message\":\"\",\"skuIds\":[\"\"],\"confidence\":0.0,\"evidence\":[]}]}。输入：" + string(payload)},
		},
	})
}

func publicOzonRecommendationDimensions(items []ozonAIDifferenceDimension, snapshot ozonRecommendationSnapshot) []OzonRecommendationDifferenceDimension {
	out := make([]OzonRecommendationDifferenceDimension, 0, len(items))
	for _, item := range items {
		evidence := make([]OzonRecommendationEvidence, 0, len(item.Evidence))
		for _, source := range item.Evidence {
			sku, _ := snapshotSKUByID(snapshot, source.SKUID)
			evidence = append(evidence, OzonRecommendationEvidence{
				SKUID: source.SKUID, SKUCode: sku.Code, Source: source.Source,
				SourceKey: source.SourceKey, RawValue: source.RawValue,
			})
		}
		out = append(out, OzonRecommendationDifferenceDimension{
			Key: item.Key, Name: item.Name, Semantic: item.Semantic,
			Confidence: roundRecommendationRatio(item.Confidence), Evidence: evidence,
		})
	}
	return out
}

func publicOzonRecommendationAnomalies(items []ozonAIAnomaly, snapshot ozonRecommendationSnapshot) []OzonRecommendationAnomaly {
	out := make([]OzonRecommendationAnomaly, 0, len(items))
	for _, item := range items {
		evidence := make([]OzonRecommendationEvidence, 0, len(item.Evidence))
		for _, source := range item.Evidence {
			sku, _ := snapshotSKUByID(snapshot, source.SKUID)
			evidence = append(evidence, OzonRecommendationEvidence{
				SKUID: source.SKUID, SKUCode: sku.Code, Source: source.Source,
				SourceKey: source.SourceKey, RawValue: source.RawValue,
			})
		}
		out = append(out, OzonRecommendationAnomaly{
			Type: item.Type, Message: item.Message, SKUIDs: append([]string{}, item.SKUIDs...),
			Confidence: roundRecommendationRatio(item.Confidence), Evidence: evidence,
		})
	}
	return out
}

func (s *Service) recallOzonRecommendationCandidates(
	ctx context.Context,
	tenantID int64,
	shopID uuid.UUID,
	productType string,
	terms []string,
	productTitle string,
) ([]*ozonRecommendationCandidateWork, bool, bool, []string) {
	warnings := []string{}
	pool, err := s.OzonCategories.ListOzonCategories(ctx, shop.OzonCategoryListQuery{
		OnlyLeaf: true, ActiveOnly: true, AllMatches: true,
	})
	if err != nil || pool == nil {
		return nil, false, true, []string{"Ozon 类目缓存读取失败，可继续手动选择类目"}
	}
	if len(pool.List) == 0 {
		return nil, true, false, warnings
	}
	history, historyWarnings := s.ozonRecommendationHistory(ctx, tenantID, shopID)
	warnings = append(warnings, historyWarnings...)
	works := make([]*ozonRecommendationCandidateWork, 0, len(pool.List))
	for _, node := range pool.List {
		if !node.IsLeaf || node.Status != "active" || strings.TrimSpace(node.CategoryID) == "" {
			continue
		}
		if node.Path == "" {
			node.Path = node.Name
		}
		semantic := ozonRecommendationSemanticScore(node, productType, terms, productTitle)
		if semantic < ozonRecommendationMinSemanticScore {
			continue
		}
		works = append(works, &ozonRecommendationCandidateWork{
			node: node, semantic: semantic, history: history[node.CategoryID],
		})
	}
	sort.SliceStable(works, func(i, j int) bool {
		left := works[i].semantic*0.75 + works[i].history*0.25
		right := works[j].semantic*0.75 + works[j].history*0.25
		if left != right {
			return left > right
		}
		return works[i].node.CategoryID < works[j].node.CategoryID
	})
	if len(works) > ozonRecommendationMaxRecall {
		works = works[:ozonRecommendationMaxRecall]
	}
	if len(works) > ozonRecommendationMaxTemplates {
		works = works[:ozonRecommendationMaxTemplates]
	}
	return works, false, false, boundedStrings(warnings, 8, 240)
}

func (s *Service) ozonRecommendationHistory(ctx context.Context, tenantID int64, shopID uuid.UUID) (map[string]float64, []string) {
	confirmed := map[string]bool{}
	mappings, err := s.OzonCategories.ListOzonCategoryMappings(ctx, tenantID, &shopID)
	warnings := []string{}
	if err != nil {
		warnings = append(warnings, "历史确认映射暂不可用，已按零加分处理")
	} else {
		for _, mapping := range mappings {
			if mapping.Status == shop.OzonMappingActive {
				confirmed[mapping.CategoryID] = true
			}
		}
	}
	type historyRow struct {
		CategoryID string `gorm:"column:category_id"`
		Count      int64  `gorm:"column:count"`
	}
	rows := []historyRow{}
	if s.DB.Migrator().HasTable(&ProductPlatformPublishConfig{}) && s.DB.Migrator().HasTable(&Product{}) {
		queryErr := s.DB.WithContext(ctx).Table("product_platform_publish_configs AS cfg").
			Select("cfg.category_id AS category_id, COUNT(*) AS count").
			Joins("JOIN products AS p ON p.id = cfg.product_id").
			Where("p.tenant_id = ? AND p.status = ? AND cfg.platform = ? AND cfg.shop_id = ? AND cfg.category_id <> ?", tenantID, StatusPublished, "ozon", shopID, "").
			Group("cfg.category_id").Scan(&rows).Error
		if queryErr != nil {
			warnings = append(warnings, "历史成功刊登统计暂不可用，已按零加分处理")
			rows = nil
		}
	}
	maxCount := int64(0)
	for _, row := range rows {
		if row.Count > maxCount {
			maxCount = row.Count
		}
	}
	scores := map[string]float64{}
	for categoryID := range confirmed {
		scores[categoryID] = 0.5
	}
	for _, row := range rows {
		success := 0.0
		if maxCount > 0 {
			success = float64(row.Count) / float64(maxCount) * 0.5
		}
		scores[row.CategoryID] = clampRecommendationRatio(scores[row.CategoryID] + success)
	}
	return scores, warnings
}

func (s *Service) loadOzonRecommendationTemplates(
	ctx context.Context,
	tenantID int64,
	shopID uuid.UUID,
	works []*ozonRecommendationCandidateWork,
	refreshPolicy string,
) ([]string, bool) {
	warnings := []string{}
	partial := false
	now := time.Now().UTC()
	stale := make([]*ozonRecommendationCandidateWork, 0, len(works))
	for _, work := range works {
		attrs, err := s.OzonCategories.ListOzonCategoryAttributes(ctx, work.node.CategoryID)
		if err != nil {
			partial = true
			warnings = append(warnings, "类目 "+work.node.Path+" 的属性模板读取失败")
			continue
		}
		var rejected bool
		work.attrs, rejected = validatedOzonRecommendationTemplateAttributes(work.node.CategoryID, attrs)
		if rejected {
			partial = true
			warnings = append(warnings, "类目 "+work.node.Path+" 的模板包含不属于该类目的属性，已拒绝")
		}
		work.templateStale = ozonRecommendationTemplateStale(work.attrs, now)
		if work.templateStale {
			stale = append(stale, work)
		}
	}
	if refreshPolicy == OzonRecommendationRefreshCacheOnly {
		if len(stale) > 0 {
			partial = true
			warnings = append(warnings, "部分候选模板缺失或过期；cache_only 未调用 Ozon 只读刷新")
		}
		return boundedStrings(warnings, 12, 240), partial
	}
	staleCount := len(stale)
	if staleCount > ozonRecommendationMaxRefresh {
		stale = stale[:ozonRecommendationMaxRefresh]
	}
	for _, work := range stale {
		work.templateRefreshAttempted = true
	}
	refreshErrors := refreshOzonRecommendationTemplates(ctx, s.OzonCategories, tenantID, shopID, stale)
	for _, work := range stale {
		if refreshErr := refreshErrors[work.node.CategoryID]; refreshErr != nil {
			partial = true
			warnings = append(warnings, "类目 "+work.node.Path+" 的只读模板刷新失败，已使用可用缓存")
			continue
		}
		attrs, err := s.OzonCategories.ListOzonCategoryAttributes(ctx, work.node.CategoryID)
		if err != nil {
			partial = true
			warnings = append(warnings, "类目 "+work.node.Path+" 刷新后模板读取失败")
			continue
		}
		var rejected bool
		work.attrs, rejected = validatedOzonRecommendationTemplateAttributes(work.node.CategoryID, attrs)
		if rejected {
			partial = true
			warnings = append(warnings, "类目 "+work.node.Path+" 刷新后的模板包含非法属性，已拒绝")
		}
		work.templateStale = ozonRecommendationTemplateStale(work.attrs, time.Now().UTC())
		if work.templateStale {
			partial = true
			warnings = append(warnings, "类目 "+work.node.Path+" 刷新后仍缺少新鲜模板证据")
		}
	}
	if staleCount > ozonRecommendationMaxRefresh {
		partial = true
		warnings = append(warnings, "已优先刷新最相关的 5 个模板；只有最终候选仍缺少模板证据时才继续刷新其余候选")
	}
	return boundedStrings(warnings, 12, 240), partial
}

func (s *Service) refreshUnattemptedOzonRecommendationTemplates(
	ctx context.Context,
	tenantID int64,
	shopID uuid.UUID,
	works []*ozonRecommendationCandidateWork,
	refreshPolicy string,
	limit int,
) ([]string, bool, bool) {
	if refreshPolicy == OzonRecommendationRefreshCacheOnly || limit <= 0 {
		return nil, false, false
	}
	selected := make([]*ozonRecommendationCandidateWork, 0, limit)
	for _, work := range works {
		if !work.templateStale || work.templateRefreshAttempted {
			continue
		}
		work.templateRefreshAttempted = true
		selected = append(selected, work)
		if len(selected) >= limit {
			break
		}
	}
	if len(selected) == 0 {
		return nil, false, false
	}
	warnings := []string{"最终候选仍缺少新鲜模板证据，已在调用上限内补充只读刷新"}
	partial := false
	refreshErrors := refreshOzonRecommendationTemplates(ctx, s.OzonCategories, tenantID, shopID, selected)
	for _, work := range selected {
		if refreshErr := refreshErrors[work.node.CategoryID]; refreshErr != nil {
			partial = true
			warnings = append(warnings, "类目 "+work.node.Path+" 的补充只读模板刷新失败")
			continue
		}
		attrs, err := s.OzonCategories.ListOzonCategoryAttributes(ctx, work.node.CategoryID)
		if err != nil {
			partial = true
			warnings = append(warnings, "类目 "+work.node.Path+" 补充刷新后模板读取失败")
			continue
		}
		var rejected bool
		work.attrs, rejected = validatedOzonRecommendationTemplateAttributes(work.node.CategoryID, attrs)
		if rejected {
			partial = true
			warnings = append(warnings, "类目 "+work.node.Path+" 补充刷新后的模板包含非法属性，已拒绝")
		}
		work.templateStale = ozonRecommendationTemplateStale(work.attrs, time.Now().UTC())
		if work.templateStale {
			partial = true
			warnings = append(warnings, "类目 "+work.node.Path+" 补充刷新后仍缺少新鲜模板证据")
		}
	}
	return boundedStrings(warnings, 10, 240), partial, true
}

func refreshOzonRecommendationTemplates(
	ctx context.Context,
	catalog OzonCategoryRecommendationCatalog,
	tenantID int64,
	shopID uuid.UUID,
	works []*ozonRecommendationCandidateWork,
) map[string]error {
	results := make(map[string]error, len(works))
	var mutex sync.Mutex
	semaphore := make(chan struct{}, 2)
	var group sync.WaitGroup
	for _, work := range works {
		work := work
		group.Add(1)
		go func() {
			defer group.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				mutex.Lock()
				results[work.node.CategoryID] = ctx.Err()
				mutex.Unlock()
				return
			}
			var err error
			for attempt := 0; attempt < 2; attempt++ {
				callCtx, cancel := context.WithTimeout(ctx, ozonRecommendationRefreshTimeout)
				_, err = catalog.RefreshOzonCategoryAttributeTemplate(callCtx, tenantID, work.node.CategoryID, shopID)
				cancel()
				if err == nil || ctx.Err() != nil {
					break
				}
			}
			mutex.Lock()
			results[work.node.CategoryID] = err
			mutex.Unlock()
		}()
	}
	group.Wait()
	return results
}

func callOzonRecommendationRerank(
	ctx context.Context,
	client AIChatClient,
	works []*ozonRecommendationCandidateWork,
	productType string,
	searchTerms []string,
) (*aigate.ChatResponse, error) {
	type summary struct {
		CandidateKey       string                                 `json:"candidateKey"`
		Path               string                                 `json:"path"`
		RuleScore          float64                                `json:"ruleScore"`
		VariantCoverage    OzonRecommendationCoverage             `json:"variantCoverage"`
		RequiredCoverage   OzonRecommendationCoverage             `json:"requiredCoverage"`
		ListingStrategy    string                                 `json:"listingStrategy"`
		MatchedDimensions  []OzonRecommendationMatchedDimension   `json:"matchedDimensions"`
		UnmatchedDimension []OzonRecommendationUnmatchedDimension `json:"unmatchedDimensions"`
	}
	summaries := make([]summary, 0, len(works))
	for index, work := range works {
		work.key = fmt.Sprintf("candidate_%d", index+1)
		summaries = append(summaries, summary{
			CandidateKey: work.key, Path: work.result.CategoryPath, RuleScore: work.result.Score,
			VariantCoverage: work.result.VariantCoverage, RequiredCoverage: work.result.RequiredCoverage,
			ListingStrategy: work.result.ListingStrategy, MatchedDimensions: work.result.MatchedDimensions,
			UnmatchedDimension: work.result.UnmatchedDimensions,
		})
	}
	payload, _ := json.Marshal(map[string]any{
		"productType": strings.TrimSpace(productType),
		"searchTerms": boundedStrings(searchTerms, 8, 80),
		"candidates":  summaries,
	})
	callCtx, cancel := context.WithTimeout(ctx, ozonRecommendationAIRequestTimeout)
	defer cancel()
	return client.Chat(callCtx, aigate.ChatRequest{
		Temperature: 0, MaxTokens: 1600, ResponseFormat: &aigate.ResponseFormat{Type: "json_object"},
		Messages: []aigate.Message{
			{Role: "system", Content: "你只负责按输入中的 productType、searchTerms 重排服务端提供的 Ozon 候选摘要。只能引用输入中的 candidateKey；不得输出 categoryId、description_category_id、type_id 或新 candidateKey。商品主体明显不相关时 confidence 必须为 0；类目语义仅部分一致可标记 approximate。不要把型号、电流等解释为颜色或内径。严格输出单个 JSON 对象，不使用 Markdown。"},
			{Role: "user", Content: "输出 {\"ranked\":[{\"candidateKey\":\"candidate_1\",\"confidence\":0.0,\"approximate\":false,\"reasons\":[\"\"],\"warnings\":[\"\"]}]}。候选摘要：" + string(payload)},
		},
	})
}

func applyOzonRecommendationRerank(works []*ozonRecommendationCandidateWork, rerank *ozonAIRerank) []*ozonRecommendationCandidateWork {
	byKey := make(map[string]*ozonRecommendationCandidateWork, len(works))
	for _, work := range works {
		byKey[work.key] = work
	}
	out := make([]*ozonRecommendationCandidateWork, 0, len(works))
	seen := map[string]bool{}
	for _, ranked := range rerank.Ranked {
		work := byKey[ranked.CandidateKey]
		seen[work.key] = true
		if ranked.Confidence == 0 {
			continue
		}
		work.result.Confidence = roundRecommendationRatio(ranked.Confidence)
		work.result.Approximate = work.result.Approximate || ranked.Approximate
		work.result.Reasons = boundedStrings(append(work.result.Reasons, ranked.Reasons...), 10, 240)
		work.result.Warnings = boundedStrings(append(work.result.Warnings, ranked.Warnings...), 10, 240)
		out = append(out, work)
	}
	for _, work := range works {
		if !seen[work.key] {
			out = append(out, work)
		}
	}
	return out
}

func (s *Service) finishOzonRecommendationAudit(
	ctx context.Context,
	taskID uuid.UUID,
	result *OzonCategoryRecommendationResult,
	inputTokens int,
	outputTokens int,
	model string,
) {
	output, err := json.Marshal(result)
	if err != nil {
		_ = s.AITasks.MarkFailed(ctx, taskID, "recommendation output serialization failed")
		return
	}
	// Provider raw responses are intentionally not persisted for this task: the
	// validated, bounded output is sufficient for audit and avoids retaining
	// unnecessary provider metadata or reasoning text.
	_ = s.AITasks.MarkSuccess(ctx, taskID, output, nil, inputTokens, outputTokens, model)
}

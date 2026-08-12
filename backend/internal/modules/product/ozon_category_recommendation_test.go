package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	platformozon "github.com/trademind-ai/trademind/backend/internal/providers/platform/ozon"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type fakeOzonRecommendationAI struct {
	mu             sync.Mutex
	responses      []*aigate.ChatResponse
	errors         []error
	requests       []aigate.ChatRequest
	factResponse   *aigate.ChatResponse
	factError      error
	nonFactCallNum int
}

func (f *fakeOzonRecommendationAI) Chat(_ context.Context, request aigate.ChatRequest) (*aigate.ChatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, request)
	if isOzonAttributeFactRequest(request) {
		if f.factError != nil {
			return nil, f.factError
		}
		if f.factResponse != nil {
			return f.factResponse, nil
		}
		return &aigate.ChatResponse{Content: `{"facts":[]}`, Model: "fake-facts"}, nil
	}
	index := f.nonFactCallNum
	f.nonFactCallNum++
	if index < len(f.errors) && f.errors[index] != nil {
		return nil, f.errors[index]
	}
	if index >= len(f.responses) {
		return nil, errors.New("unexpected ai call")
	}
	return f.responses[index], nil
}

func isOzonAttributeFactRequest(request aigate.ChatRequest) bool {
	for _, message := range request.Messages {
		if strings.Contains(message.Content, "商品图文事实提炼器") || strings.Contains(message.Content, `"facts":[{"name"`) {
			return true
		}
	}
	return false
}

type fakeOzonRecommendationCatalog struct {
	mu               sync.Mutex
	allowedShop      uuid.UUID
	ensureErr        error
	categories       []shop.OzonCategoryNodeDTO
	categoryErr      error
	searchErr        error
	attrs            map[string][]shop.OzonAttributeDTO
	attrErrors       map[string]error
	refreshErrors    map[string]error
	refreshCalls     []string
	mappings         []shop.OzonCategoryMappingDTO
	mappingErr       error
	dictionaryValues map[string][]platformozon.DictionaryValue
	dictionaryErrors map[string]error
	dictionaryCalls  []string
}

func (f *fakeOzonRecommendationCatalog) SearchOzonDictionaryValues(
	_ context.Context,
	_ int64,
	categoryID string,
	attrID string,
	_ uuid.UUID,
	keyword string,
) ([]platformozon.DictionaryValue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := categoryID + "\n" + attrID + "\n" + keyword
	f.dictionaryCalls = append(f.dictionaryCalls, key)
	if err := f.dictionaryErrors[key]; err != nil {
		return nil, err
	}
	return append([]platformozon.DictionaryValue(nil), f.dictionaryValues[key]...), nil
}

func (f *fakeOzonRecommendationCatalog) EnsureAuthorizedOzonShop(_ context.Context, _ int64, shopID uuid.UUID) error {
	if f.ensureErr != nil {
		return f.ensureErr
	}
	if f.allowedShop != uuid.Nil && shopID != f.allowedShop {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (f *fakeOzonRecommendationCatalog) ListOzonCategories(_ context.Context, query shop.OzonCategoryListQuery) (*shop.OzonCategoryListResult, error) {
	if f.categoryErr != nil {
		return nil, f.categoryErr
	}
	if query.RootOnly {
		byName := map[string]shop.OzonCategoryNodeDTO{}
		for _, category := range f.categories {
			name := ""
			if len(category.Ancestors) > 0 {
				name = strings.TrimSpace(category.Ancestors[0].Name)
			} else if parts := strings.Split(category.Path, "/"); len(parts) > 0 {
				name = strings.TrimSpace(parts[0])
			}
			if name == "" {
				continue
			}
			byName[name] = shop.OzonCategoryNodeDTO{CategoryID: "root:" + normalizeOzonRecommendationText(name), Name: name, Path: name, Level: 1, Status: "active"}
		}
		roots := make([]shop.OzonCategoryNodeDTO, 0, len(byName))
		for _, root := range byName {
			roots = append(roots, root)
		}
		sort.SliceStable(roots, func(i, j int) bool { return roots[i].Name < roots[j].Name })
		return &shop.OzonCategoryListResult{List: roots, Total: len(roots), MatchedTotal: len(roots)}, nil
	}
	list := make([]shop.OzonCategoryNodeDTO, 0, len(f.categories))
	keyword := normalizeOzonRecommendationText(query.Keyword)
	for _, category := range f.categories {
		if keyword != "" && !strings.Contains(normalizeOzonRecommendationText(category.Path+category.Name), keyword) {
			continue
		}
		list = append(list, category)
	}
	return &shop.OzonCategoryListResult{List: list, Total: len(f.categories), LeafCount: len(f.categories), MatchedTotal: len(list)}, nil
}

func (f *fakeOzonRecommendationCatalog) SearchOzonLeafCategories(_ context.Context, query shop.OzonCategorySearchQuery) (*shop.OzonCategorySearchResult, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	if f.categoryErr != nil {
		return nil, f.categoryErr
	}
	matches := make([]shop.OzonCategorySearchMatch, 0, len(f.categories))
	for _, category := range f.categories {
		if !category.IsLeaf || category.Status != "active" {
			continue
		}
		score := ozonRecommendationSemanticScore(category, query.ProductType, query.SearchTerms, query.ProductTitle)
		if score == 0 {
			continue
		}
		matches = append(matches, shop.OzonCategorySearchMatch{Node: category, Score: score, MatchedTerms: append([]string{query.ProductType}, query.SearchTerms...), Lanes: []string{"fake_index"}})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Node.CategoryID < matches[j].Node.CategoryID
	})
	if query.Limit > 0 && len(matches) > query.Limit {
		matches = matches[:query.Limit]
	}
	return &shop.OzonCategorySearchResult{
		Matches: matches, IndexVersion: shop.OzonCategorySearchIndexVersion,
		IndexedLeafCount: len(f.categories), BuiltAt: time.Now().UTC(),
	}, nil
}

func (f *fakeOzonRecommendationCatalog) ListOzonCategoryAttributes(_ context.Context, categoryID string) ([]shop.OzonAttributeDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.attrErrors[categoryID]; err != nil {
		return nil, err
	}
	return append([]shop.OzonAttributeDTO{}, f.attrs[categoryID]...), nil
}

func (f *fakeOzonRecommendationCatalog) RefreshOzonCategoryAttributeTemplate(_ context.Context, _ int64, categoryID string, _ uuid.UUID) (*shop.OzonCategoryStats, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshCalls = append(f.refreshCalls, categoryID)
	if err := f.refreshErrors[categoryID]; err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	rows := f.attrs[categoryID]
	for index := range rows {
		rows[index].SyncedAt = &now
		rows[index].CacheStale = false
	}
	f.attrs[categoryID] = rows
	return &shop.OzonCategoryStats{Count: int64(len(rows))}, nil
}

func (f *fakeOzonRecommendationCatalog) ListOzonCategoryMappings(_ context.Context, _ int64, _ *uuid.UUID) ([]shop.OzonCategoryMappingDTO, error) {
	return append([]shop.OzonCategoryMappingDTO{}, f.mappings...), f.mappingErr
}

func jsonBytes(t *testing.T, value any) datatypes.JSON {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return datatypes.JSON(raw)
}

func setupOzonRecommendationFixture(t *testing.T) (*Service, *Product, []ProductSKU, uuid.UUID, *fakeOzonRecommendationCatalog) {
	t.Helper()
	svc, productRow := tenantProductFixture(t, 1)
	require.NoError(t, svc.DB.AutoMigrate(&aitask.AITask{}))
	productRow.Title = "SSK 系列固态继电器控制模块"
	productRow.Description = "用于工业控制的固态继电器，包含多种型号、电流和包装。"
	productRow.RawData = jsonBytes(t, map[string]any{
		"attributes": map[string]any{"品牌": "TestRelay", "用途": "工业控制"},
		"skuGroups": []map[string]any{{
			"name": "颜色分类",
			"options": []map[string]any{
				{"label": "SSK3D 直流控直流 3A 带底座 10只装"},
				{"label": "SSK4D 直流控直流 4A 带底座 10只装"},
				{"label": "SSK10D 直流控直流 10A 带底座 10只装"},
				{"label": "SSK4A 直流控交流 4A 带底座 10只装"},
				{"label": "SSK4K 交直流两用 4A 带底座 10只装"},
				{"label": "【1只装】短接线"},
			},
		}},
	})
	require.NoError(t, svc.DB.Save(productRow).Error)
	values := []string{
		"SSK3D 直流控直流 3A 带底座 10只装",
		"SSK4D 直流控直流 4A 带底座 10只装",
		"SSK10D 直流控直流 10A 带底座 10只装",
		"SSK4A 直流控交流 4A 带底座 10只装",
		"SSK4K 交直流两用 4A 带底座 10只装",
		"【1只装】短接线",
	}
	skus := make([]ProductSKU, 0, len(values))
	for index, value := range values {
		sku := ProductSKU{
			ProductID: productRow.ID, SKUCode: "SSK-" + string(rune('1'+index)), SKUName: value,
			Attrs: jsonBytes(t, map[string]any{"颜色分类": value}),
			RawData: jsonBytes(t, map[string]any{
				"properties": map[string]any{
					"安装方式": "带底座", "主图": "https://private.example.test/nested.jpg",
					"库存": 888, "apiToken": "TEST_ONLY_NESTED_TOKEN",
				},
				"inventoryQuantity":    999,
				"originalMainImageUrl": "https://private.example.test/sku.jpg",
				"credential":           "TEST_ONLY_DO_NOT_SEND",
			}),
		}
		require.NoError(t, svc.DB.Create(&sku).Error)
		skus = append(skus, sku)
	}
	shopID := uuid.New()
	now := time.Now().UTC()
	catalog := &fakeOzonRecommendationCatalog{
		allowedShop: shopID,
		categories: []shop.OzonCategoryNodeDTO{{
			CategoryID: "100:200", Name: "Solid State Relays", Path: "Electronics / Solid State Relays",
			IsLeaf: true, Status: "active",
		}},
		attrs: map[string][]shop.OzonAttributeDTO{
			"100:200": {
				{CategoryID: "100:200", AttrID: "brand", Name: "品牌", Required: true, SyncedAt: &now, SKUVariantEligibilityKnown: true},
				{CategoryID: "100:200", AttrID: "inner", Name: "内径", SyncedAt: &now, SKUVariantEligible: true, SKUVariantEligibilityKnown: true},
			},
		},
		attrErrors: map[string]error{}, refreshErrors: map[string]error{},
	}
	svc.AITasks = &aitask.Service{DB: svc.DB}
	svc.OzonCategories = catalog
	return svc, productRow, skus, shopID, catalog
}

func analysisJSON(t *testing.T, skus []ProductSKU) string {
	t.Helper()
	evidence := func(indices ...int) []map[string]any {
		out := make([]map[string]any, 0, len(indices))
		for _, index := range indices {
			var attrs map[string]string
			require.NoError(t, json.Unmarshal(skus[index].Attrs, &attrs))
			out = append(out, map[string]any{
				"skuId": skus[index].ID.String(), "source": "sku.attrs",
				"sourceKey": "颜色分类", "rawValue": attrs["颜色分类"],
			})
		}
		return out
	}
	payload := map[string]any{
		"productType": "solid state relay",
		"searchTerms": []string{"solid state relay", "industrial relay"},
		"differenceDimensions": []map[string]any{
			{"key": "model", "name": "型号", "semantic": "model", "confidence": 0.99, "evidence": evidence(0, 1, 2, 3, 4)},
			{"key": "control", "name": "控制方式", "semantic": "control_method", "confidence": 0.96, "evidence": evidence(0, 3, 4)},
			{"key": "current", "name": "电流", "semantic": "current", "confidence": 0.98, "evidence": evidence(0, 1, 2)},
			{"key": "package", "name": "包装配置", "semantic": "package", "confidence": 0.91, "evidence": evidence(0, 5)},
		},
		"anomalies": []map[string]any{{
			"type": "different_product_subject", "message": "短接线是配件，不是固态继电器主体",
			"skuIds": []string{skus[5].ID.String()}, "confidence": 0.99, "evidence": evidence(5),
		}},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	return string(raw)
}

func TestOzonCategoryRecommendationUsesPersistedSKUEvidenceAndRejectsUnrelatedAspect(t *testing.T) {
	svc, productRow, skus, shopID, _ := setupOzonRecommendationFixture(t)
	fakeAI := &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{
		{Content: analysisJSON(t, skus), Model: "fake-analysis", InputTokens: 100, OutputTokens: 80},
		{Content: `{"selectedRootKeys":["root_1"]}`, Model: "fake-root-screen", InputTokens: 20, OutputTokens: 10},
		{Content: `{"selected":[{"candidateKey":"candidate_1","subjectMatch":"exact","confidence":0.94,"reasons":["完整路径对应固态继电器"],"warnings":[]}]}`, Model: "fake-path-screen", InputTokens: 50, OutputTokens: 20},
		{Content: `{"verdicts":[{"candidateKey":"candidate_1","subjectMatch":"approximate","confidence":0.84,"reasons":["最新模板复核后仍是最接近类目"],"warnings":["模板无法承载型号和电流"]}]}`, Model: "fake-template-review", InputTokens: 40, OutputTokens: 30},
	}}
	svc.OzonCategoryAI = fakeAI
	c := tenantProductAdminContext(t, svc, 1)

	result, err := svc.RecommendOzonCategories(c, productRow.ID, OzonCategoryRecommendationBody{
		ShopID: shopID.String(), RefreshPolicy: OzonRecommendationRefreshIfMissingOrStale,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, OzonCategoryRecommendationReady, result.Status)
	require.Len(t, result.DifferenceDimensions, 4)
	require.Equal(t, []string{"model", "control_method", "current", "package"}, []string{
		result.DifferenceDimensions[0].Semantic,
		result.DifferenceDimensions[1].Semantic,
		result.DifferenceDimensions[2].Semantic,
		result.DifferenceDimensions[3].Semantic,
	})
	for _, dimension := range result.DifferenceDimensions {
		require.NotEmpty(t, dimension.Evidence)
		for _, evidence := range dimension.Evidence {
			require.Equal(t, "sku.attrs", evidence.Source)
			matched := false
			for _, sku := range skus {
				if sku.ID.String() != evidence.SKUID {
					continue
				}
				var attrs map[string]string
				require.NoError(t, json.Unmarshal(sku.Attrs, &attrs))
				matched = attrs[evidence.SourceKey] == evidence.RawValue
			}
			require.True(t, matched, "evidence must be an exact persisted SKU attribute")
		}
	}
	require.Len(t, result.Anomalies, 1)
	require.Equal(t, "different_product_subject", result.Anomalies[0].Type)
	require.Contains(t, result.Anomalies[0].SKUIDs, skus[5].ID.String())
	require.Len(t, result.Candidates, 1)
	candidate := result.Candidates[0]
	require.Equal(t, "100:200", candidate.CategoryID)
	require.Zero(t, candidate.VariantCoverage.Matched)
	require.Equal(t, 4, candidate.VariantCoverage.Total)
	require.Empty(t, candidate.MatchedDimensions)
	require.Len(t, candidate.UnmatchedDimensions, 4)
	require.Equal(t, OzonListingStrategySplitSingleSKU, candidate.ListingStrategy)
	require.True(t, candidate.Approximate)
	require.NotEmpty(t, candidate.SchemaHash)
	require.Len(t, fakeAI.requests, 4)
	analysisPrompt := fakeAI.requests[0].Messages[1].Content
	require.Contains(t, analysisPrompt, "安装方式", "whitelisted SKU properties should be included")
	require.NotContains(t, analysisPrompt, "inventoryQuantity")
	require.NotContains(t, analysisPrompt, "originalMainImageUrl")
	require.NotContains(t, analysisPrompt, "TEST_ONLY_DO_NOT_SEND")
	require.NotContains(t, analysisPrompt, "TEST_ONLY_NESTED_TOKEN")
	require.NotContains(t, analysisPrompt, "private.example.test")
	require.NotContains(t, analysisPrompt, "库存")
	require.NotContains(t, fakeAI.requests[1].Messages[1].Content, "root:electronics", "AI root screening must not receive a category id")
	require.Contains(t, fakeAI.requests[1].Messages[1].Content, "Electronics")
	require.NotContains(t, fakeAI.requests[2].Messages[1].Content, "100:200", "AI path screening must not receive a category id")
	require.Contains(t, fakeAI.requests[2].Messages[1].Content, "Electronics / Solid State Relays")
	require.NotContains(t, fakeAI.requests[3].Messages[1].Content, "100:200", "AI final review must not receive a category id")
	require.Contains(t, fakeAI.requests[3].Messages[1].Content, `"productType":"solid state relay"`)
	require.Contains(t, fakeAI.requests[3].Messages[1].Content, `"requiredAttributes":["品牌"]`)
	require.Contains(t, fakeAI.requests[3].Messages[1].Content, `"knownIsAspectAttributes":["内径"]`)

	var task aitask.AITask
	require.NoError(t, svc.DB.First(&task, "id = ?", *result.TaskID).Error)
	require.Equal(t, "ozon_category_recommendation", task.TaskType)
	require.Equal(t, aitask.StatusSuccess, task.Status)
	require.Equal(t, 210, task.TokenInput)
	require.Equal(t, 140, task.TokenOutput)
	require.Equal(t, "fake-template-review", task.Model)
	require.NotContains(t, string(task.Input), "SSK3D")
	require.Empty(t, task.RawResponse)

	var persisted []ProductSKU
	require.NoError(t, svc.DB.Where("product_id = ?", productRow.ID).Order("created_at ASC").Find(&persisted).Error)
	require.Len(t, persisted, len(skus))
	for index := range persisted {
		require.JSONEq(t, string(skus[index].Attrs), string(persisted[index].Attrs))
	}
}

func TestOzonRecommendationValidationRejectsFabricatedEvidenceAndCandidateKeys(t *testing.T) {
	_, productRow, skus, _, _ := setupOzonRecommendationFixture(t)
	snapshot, err := buildOzonRecommendationSnapshot(Product{Base: productRow.Base, SKUs: skus}, nil)
	require.NoError(t, err)
	fabricated := analysisJSON(t, skus)
	fabricated = strings.Replace(fabricated, "SSK3D 直流控直流 3A 带底座 10只装", "AI 编造值", 1)
	_, err = parseOzonAIAnalysis(fabricated, snapshot)
	require.ErrorContains(t, err, "exact persisted SKU selection")

	var sparseEvidence map[string]any
	require.NoError(t, json.Unmarshal([]byte(analysisJSON(t, skus)), &sparseEvidence))
	dimensions := sparseEvidence["differenceDimensions"].([]any)
	firstDimension := dimensions[0].(map[string]any)
	firstDimension["evidence"] = firstDimension["evidence"].([]any)[:1]
	sparseRaw, sparseMarshalErr := json.Marshal(sparseEvidence)
	require.NoError(t, sparseMarshalErr)
	completedAnalysis, err := parseOzonAIAnalysis(string(sparseRaw), snapshot)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(completedAnalysis.DifferenceDimensions[0].Evidence), 2)
	require.NotEqual(t,
		completedAnalysis.DifferenceDimensions[0].Evidence[0].RawValue,
		completedAnalysis.DifferenceDimensions[0].Evidence[1].RawValue,
		"server-completed evidence must prove a persisted cross-SKU difference",
	)

	_, err = parseOzonAIPathSelection(`{"selected":[{"candidateKey":"invented","subjectMatch":"exact","confidence":0.9,"reasons":[],"warnings":[]}]}`, map[string]bool{"candidate_1": true})
	require.ErrorContains(t, err, "outside the server allowlist")
	_, err = parseOzonAIRootSelection(`{"selectedRootKeys":["invented"]}`, []ozonRecommendationRootItem{{
		Key: "root_1", Node: shop.OzonCategoryNodeDTO{CategoryID: "root-real", Name: "真实一级类目"},
	}})
	require.ErrorContains(t, err, "outside the server allowlist")
	_, err = parseOzonAIPathSelection(`{"selected":[{"candidateKey":"candidate_1","categoryId":"999:999","subjectMatch":"exact","confidence":0.9,"reasons":[],"warnings":[]}]}`, map[string]bool{"candidate_1": true})
	require.ErrorContains(t, err, "unknown field")
	pathSelection, err := parseOzonAIPathSelection(`{"selected":[{"candidateKey":"candidate_1","subjectMatch":"exact","confidence":0.8,"reasons":[],"warnings":[]},{"candidateKey":"candidate_2","subjectMatch":"conflict","confidence":0,"reasons":[],"warnings":[]}]}`, map[string]bool{"candidate_1": true, "candidate_2": true})
	require.NoError(t, err)
	require.Len(t, pathSelection.Selected, 1, "explicit conflicts must fail closed")
	_, err = parseOzonAIFinalReview(`{"verdicts":[{"candidateKey":"invented","subjectMatch":"exact","confidence":0.9,"reasons":[],"warnings":[]}]}`, map[string]bool{"candidate_1": true})
	require.ErrorContains(t, err, "outside the server allowlist")
	partialReview, err := parseOzonAIFinalReview(`{"verdicts":[{"candidateKey":"candidate_1","subjectMatch":"exact","confidence":0.9,"reasons":[],"warnings":[]}]}`, map[string]bool{"candidate_1": true, "candidate_2": true})
	require.NoError(t, err)
	selected := applyOzonRecommendationFinalReview([]*ozonRecommendationCandidateWork{
		{key: "candidate_1", node: shop.OzonCategoryNodeDTO{CategoryID: "kept"}},
		{key: "candidate_2", node: shop.OzonCategoryNodeDTO{CategoryID: "omitted"}},
	}, partialReview)
	require.Len(t, selected, 1, "omitted allowlisted candidates must fail closed")
	require.Equal(t, "kept", selected[0].node.CategoryID)

	zeroConfidenceReview, err := parseOzonAIFinalReview(`{"verdicts":[{"candidateKey":"candidate_1","subjectMatch":"approximate","confidence":0,"reasons":[],"warnings":[]},{"candidateKey":"candidate_2","subjectMatch":"approximate","confidence":0.72,"reasons":[],"warnings":[]}]}`, map[string]bool{"candidate_1": true, "candidate_2": true})
	require.NoError(t, err)
	require.Len(t, zeroConfidenceReview.Verdicts, 1, "a contradictory zero-confidence verdict must not erase independent valid candidates")
	require.Equal(t, "candidate_2", zeroConfidenceReview.Verdicts[0].CandidateKey)

	consensusFallback := applyOzonRecommendationFinalReview([]*ozonRecommendationCandidateWork{{
		key: "candidate_1", node: shop.OzonCategoryNodeDTO{CategoryID: "nearest"},
		searchScore: 0.66, semantic: 0.78, pathConfidence: 0.81, pathApproximate: true,
		aiConfidence: 0.81, aiApproximate: true,
	}}, &ozonAIFinalReview{Verdicts: []ozonAIFinalReviewItem{{
		CandidateKey: "candidate_1", SubjectMatch: "conflict", Confidence: 0,
	}}})
	require.Len(t, consensusFallback, 1, "a final-review disagreement must retain one independently supported near-match")
	require.Equal(t, "nearest", consensusFallback[0].node.CategoryID)
	require.True(t, consensusFallback[0].aiApproximate)
	require.Equal(t, 0.45, consensusFallback[0].aiConfidence)
	require.Contains(t, strings.Join(consensusFallback[0].aiWarnings, " "), "必须人工确认")

	var mismatchedAnomaly map[string]any
	require.NoError(t, json.Unmarshal([]byte(analysisJSON(t, skus)), &mismatchedAnomaly))
	anomalies := mismatchedAnomaly["anomalies"].([]any)
	anomalies[0].(map[string]any)["skuIds"] = []string{skus[4].ID.String()}
	rawMismatch, marshalErr := json.Marshal(mismatchedAnomaly)
	require.NoError(t, marshalErr)
	_, err = parseOzonAIAnalysis(string(rawMismatch), snapshot)
	require.ErrorContains(t, err, "anomaly evidence must belong")
}

func TestOzonRecommendationSemanticIndexUsesAIExpandedTerms(t *testing.T) {
	svc, productRow, skus, shopID, catalog := setupOzonRecommendationFixture(t)
	now := time.Now().UTC()
	catalog.categories = []shop.OzonCategoryNodeDTO{{
		CategoryID: "777:888", Name: "工业电子控制元件", Path: "电子产品 / 工业控制 / 工业电子控制元件",
		IsLeaf: true, Status: "active",
	}}
	catalog.attrs = map[string][]shop.OzonAttributeDTO{
		"777:888": {{
			CategoryID: "777:888", AttrID: "brand", Name: "品牌", Required: true, SyncedAt: &now,
		}},
	}
	var expandedAnalysis map[string]any
	require.NoError(t, json.Unmarshal([]byte(analysisJSON(t, skus)), &expandedAnalysis))
	expandedAnalysis["searchTerms"] = []string{"工业电子控制元件", "固态继电器"}
	expandedRaw, err := json.Marshal(expandedAnalysis)
	require.NoError(t, err)
	svc.OzonCategoryAI = &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{
		{Content: string(expandedRaw), Model: "fake-analysis"},
		{Content: `{"selectedRootKeys":["root_1"]}`, Model: "fake-root-screen"},
		{Content: `{"selected":[{"candidateKey":"candidate_1","subjectMatch":"approximate","confidence":0.88,"reasons":["完整路径属于工业控制元件"],"warnings":[]}]}`, Model: "fake-path-screen"},
		{Content: `{"verdicts":[{"candidateKey":"candidate_1","subjectMatch":"approximate","confidence":0.79,"reasons":["最新模板复核仍保留工业控制元件"],"warnings":[]}]}`, Model: "fake-template-review"},
	}}
	c := tenantProductAdminContext(t, svc, 1)
	result, err := svc.RecommendOzonCategories(c, productRow.ID, OzonCategoryRecommendationBody{
		ShopID: shopID.String(), RefreshPolicy: OzonRecommendationRefreshCacheOnly,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, OzonCategoryRecommendationReady, result.Status)
	require.Len(t, result.Candidates, 1)
	require.Equal(t, "777:888", result.Candidates[0].CategoryID)
	require.Equal(t, 0.79, result.Candidates[0].Confidence)
}

func TestOzonRecommendationFinalReviewDropsGenericPathDomainConflicts(t *testing.T) {
	svc, productRow, skus, shopID, catalog := setupOzonRecommendationFixture(t)
	now := time.Now().UTC()
	catalog.categories = []shop.OzonCategoryNodeDTO{
		{CategoryID: "24692739:972188833", Name: "智能继电器", Path: "电子产品 / 智能家居 / 智能继电器", IsLeaf: true, Status: "active"},
		{CategoryID: "17028756:971047417", Name: "汽车通用继电器", Path: "汽车用品 / 乘用车配件 / 汽车通用继电器", IsLeaf: true, Status: "active"},
		{CategoryID: "17028756:98911", Name: "继电器调节器", Path: "汽车用品 / 乘用车配件 / 继电器调节器", IsLeaf: true, Status: "active"},
	}
	catalog.attrs = map[string][]shop.OzonAttributeDTO{}
	for _, category := range catalog.categories {
		catalog.attrs[category.CategoryID] = []shop.OzonAttributeDTO{{
			CategoryID: category.CategoryID, AttrID: "brand", Name: "品牌", Required: true, SyncedAt: &now,
		}}
	}
	var analysis map[string]any
	require.NoError(t, json.Unmarshal([]byte(analysisJSON(t, skus)), &analysis))
	analysis["productType"] = "固态继电器模组"
	analysis["searchTerms"] = []string{"固态继电器", "智能继电器", "继电器"}
	raw, err := json.Marshal(analysis)
	require.NoError(t, err)
	fakeAI := &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{
		{Content: string(raw), Model: "fake-analysis"},
		{Content: `{"selectedRootKeys":["root_1","root_2"]}`, Model: "fake-root-screen"},
		{Content: `{"selected":[{"candidateKey":"candidate_1","subjectMatch":"approximate","confidence":0.88,"reasons":[],"warnings":[]},{"candidateKey":"candidate_2","subjectMatch":"approximate","confidence":0.62,"reasons":[],"warnings":[]},{"candidateKey":"candidate_3","subjectMatch":"approximate","confidence":0.6,"reasons":[],"warnings":[]}]}`, Model: "fake-path-screen"},
		{Content: `{"verdicts":[{"candidateKey":"candidate_1","subjectMatch":"approximate","confidence":0.76,"reasons":["类目主体接近但并非明确工业固态继电器"],"warnings":["需要人工确认用途"]},{"candidateKey":"candidate_2","subjectMatch":"conflict","confidence":0,"reasons":["完整路径限定了不同适用领域"],"warnings":[]},{"candidateKey":"candidate_3","subjectMatch":"conflict","confidence":0,"reasons":["完整路径限定了不同适用领域"],"warnings":[]}]}`, Model: "fake-template-review"},
	}}
	svc.OzonCategoryAI = fakeAI
	c := tenantProductAdminContext(t, svc, 1)
	result, err := svc.RecommendOzonCategories(c, productRow.ID, OzonCategoryRecommendationBody{
		ShopID: shopID.String(), RefreshPolicy: OzonRecommendationRefreshCacheOnly,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, OzonCategoryRecommendationReady, result.Status)
	require.Len(t, result.Candidates, 1)
	require.Equal(t, "24692739:972188833", result.Candidates[0].CategoryID)
	require.True(t, result.Candidates[0].Approximate)
	require.NotContains(t, fakeAI.requests[2].Messages[0].Content, "汽车", "path screening rules must be product-agnostic")
	require.NotContains(t, fakeAI.requests[3].Messages[0].Content, "汽车", "final review rules must be product-agnostic")
}

func TestOzonRecommendationAspectMatchingFailsClosed(t *testing.T) {
	dimensions := []OzonRecommendationDifferenceDimension{
		{Key: "model", Name: "型号", Semantic: "model", Evidence: []OzonRecommendationEvidence{{SourceKey: "颜色分类", RawValue: "SSK4D"}}},
		{Key: "current", Name: "电流", Semantic: "current", Evidence: []OzonRecommendationEvidence{{SourceKey: "颜色分类", RawValue: "4A"}}},
	}
	attrs := []shop.OzonAttributeDTO{
		{AttrID: "inner", Name: "内径", SKUVariantEligible: true, SKUVariantEligibilityKnown: true},
		{AttrID: "model-unknown", Name: "型号", SKUVariantEligible: true, SKUVariantEligibilityKnown: false},
		{AttrID: "color", Name: "颜色", SKUVariantEligible: true, SKUVariantEligibilityKnown: true},
	}
	matched, unmatched := matchOzonRecommendationDimensions(dimensions, attrs)
	require.Empty(t, matched)
	require.Len(t, unmatched, 2)

	attrs = append(attrs, shop.OzonAttributeDTO{AttrID: "model", Name: "型号", SKUVariantEligible: true, SKUVariantEligibilityKnown: true})
	matched, unmatched = matchOzonRecommendationDimensions(dimensions, attrs)
	require.Len(t, matched, 1)
	require.Equal(t, "model", matched[0].TargetAttributeID)
	require.True(t, matched[0].IsAspect)
	require.True(t, matched[0].IsAspectKnown)
	require.Len(t, unmatched, 1)
	require.Equal(t, OzonListingStrategyGroupAll, ozonRecommendationListingStrategy(1, recommendationCoverage(0, 2), nil))

	unsafeDimensions := []OzonRecommendationDifferenceDimension{
		{Key: "current-as-inner", Name: "内径", Semantic: "current", Evidence: []OzonRecommendationEvidence{{SourceKey: "颜色分类", RawValue: "SSK4A 直流控交流 4A 带底座"}}},
		{Key: "composite-as-color", Name: "颜色", Semantic: "color", Evidence: []OzonRecommendationEvidence{{SourceKey: "颜色分类", RawValue: "SSK4A 直流控交流 4A 带底座"}}},
	}
	unsafeAttrs := []shop.OzonAttributeDTO{
		{AttrID: "inner", Name: "内径", SKUVariantEligible: true, SKUVariantEligibilityKnown: true},
		{AttrID: "color", Name: "颜色", SKUVariantEligible: true, SKUVariantEligibilityKnown: true},
	}
	matched, unmatched = matchOzonRecommendationDimensions(unsafeDimensions, unsafeAttrs)
	require.Empty(t, matched)
	require.Len(t, unmatched, 2)

	validated, rejected := validatedOzonRecommendationTemplateAttributes("100:200", []shop.OzonAttributeDTO{
		{CategoryID: "999:999", AttrID: "model", Name: "型号", SKUVariantEligible: true, SKUVariantEligibilityKnown: true},
		{CategoryID: "100:200", AttrID: "", Name: "空属性"},
		{CategoryID: "100:200", AttrID: "current", Name: "电流"},
		{CategoryID: "100:200", AttrID: "current", Name: "重复电流"},
	})
	require.True(t, rejected)
	require.Len(t, validated, 1)
	require.Equal(t, "current", validated[0].AttrID)
}

func TestOzonRecommendationSemanticRecallRejectsSingleCJKCharacterCollision(t *testing.T) {
	title := "男鞋2025新款四季百搭一脚蹬网面软底运动休闲鞋透气潮鞋"
	maleVoice := shop.OzonCategoryNodeDTO{
		CategoryID: "18262713:96815", Name: "男高音", Path: "乐器 / 管乐器 / 男高音",
		IsLeaf: true, Status: "active",
	}
	sneaker := shop.OzonCategoryNodeDTO{
		CategoryID: "15621048:91248", Name: "运动鞋", Path: "鞋类 / 休闲鞋 / 运动鞋",
		IsLeaf: true, Status: "active",
	}
	require.Zero(t, ozonRecommendationSemanticScore(maleVoice, "男鞋", []string{"男鞋"}, title))
	require.GreaterOrEqual(t, ozonRecommendationSemanticScore(sneaker, "男鞋", []string{"男鞋"}, title), ozonRecommendationMinSemanticScore)

	svc, _, _, shopID, catalog := setupOzonRecommendationFixture(t)
	catalog.categories = []shop.OzonCategoryNodeDTO{maleVoice, sneaker, {
		CategoryID: "15621048:91270", Name: "懒人鞋", Path: "鞋类 / 休闲鞋 / 懒人鞋",
		IsLeaf: true, Status: "active",
	}}
	for index := 0; index < 140; index++ {
		catalog.categories = append(catalog.categories, shop.OzonCategoryNodeDTO{
			CategoryID: fmt.Sprintf("irrelevant:%03d", index), Name: fmt.Sprintf("无关器材%03d", index),
			Path: "工业器材 / 无关器材", IsLeaf: true, Status: "active",
		})
	}
	works, cacheEmpty, partial, warnings := svc.recallOzonRecommendationCandidates(
		context.Background(), 1, shopID, "男鞋", []string{"男士一脚蹬运动休闲鞋"}, title,
	)
	require.False(t, cacheEmpty)
	require.False(t, partial)
	require.Empty(t, warnings)
	require.Len(t, works, 1)
	for _, work := range works {
		require.NotContains(t, work.node.Path, "乐器")
		require.GreaterOrEqual(t, work.semantic, ozonRecommendationMinSemanticScore)
	}
	require.Equal(t, "15621048:91248", works[0].node.CategoryID)
}

func TestOzonRecommendationRerankDropsExplicitZeroConfidenceCandidate(t *testing.T) {
	first := &ozonRecommendationCandidateWork{key: "candidate_1", result: OzonCategoryRecommendationCandidate{CategoryID: "wrong"}}
	second := &ozonRecommendationCandidateWork{key: "candidate_2", result: OzonCategoryRecommendationCandidate{CategoryID: "right"}}
	out := applyOzonRecommendationRerank([]*ozonRecommendationCandidateWork{first, second}, &ozonAIRerank{Ranked: []ozonAIRerankItem{
		{CandidateKey: "candidate_1", Confidence: 0},
		{CandidateKey: "candidate_2", Confidence: 0.8},
	}})
	require.Len(t, out, 1)
	require.Equal(t, "right", out[0].result.CategoryID)
}

func TestOzonRecommendationTimeoutAndCatalogDegradationRemainBusinessStatuses(t *testing.T) {
	svc, productRow, _, shopID, catalog := setupOzonRecommendationFixture(t)
	svc.OzonCategoryAI = &fakeOzonRecommendationAI{errors: []error{context.DeadlineExceeded}}
	c := tenantProductAdminContext(t, svc, 1)
	result, err := svc.RecommendOzonCategories(c, productRow.ID, OzonCategoryRecommendationBody{ShopID: shopID.String()}, nil)
	require.NoError(t, err)
	require.Equal(t, OzonCategoryRecommendationAIUnavailable, result.Status)
	require.Empty(t, result.Candidates)

	svc2, productRow2, skus2, shopID2, catalog2 := setupOzonRecommendationFixture(t)
	catalog2.categories = nil
	svc2.OzonCategoryAI = &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{{Content: analysisJSON(t, skus2)}}}
	c2 := tenantProductAdminContext(t, svc2, 1)
	result, err = svc2.RecommendOzonCategories(c2, productRow2.ID, OzonCategoryRecommendationBody{ShopID: shopID2.String()}, nil)
	require.NoError(t, err)
	require.Equal(t, OzonCategoryRecommendationCategoryCacheEmpty, result.Status)
	require.Empty(t, catalog.refreshCalls)

	svc3, productRow3, skus3, shopID3, catalog3 := setupOzonRecommendationFixture(t)
	catalog3.categoryErr = errors.New("cache temporarily unavailable")
	svc3.OzonCategoryAI = &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{{Content: analysisJSON(t, skus3)}}}
	c3 := tenantProductAdminContext(t, svc3, 1)
	result, err = svc3.RecommendOzonCategories(c3, productRow3.ID, OzonCategoryRecommendationBody{ShopID: shopID3.String()}, nil)
	require.NoError(t, err)
	require.Equal(t, OzonCategoryRecommendationPartial, result.Status)
	require.Empty(t, result.Candidates)
}

func TestOzonRecommendationHistoryCannotRecallAnUnrelatedCategory(t *testing.T) {
	svc, _, _, shopID, catalog := setupOzonRecommendationFixture(t)
	catalog.categories = []shop.OzonCategoryNodeDTO{{
		CategoryID: "300:400", Name: "餐桌", Path: "家具 / 餐厅家具 / 餐桌",
		IsLeaf: true, Status: "active",
	}}
	catalog.mappings = []shop.OzonCategoryMappingDTO{{
		CategoryID: "300:400", Status: shop.OzonMappingActive,
	}}
	works, cacheEmpty, partial, warnings := svc.recallOzonRecommendationCandidates(
		context.Background(), 1, shopID, "固态继电器", []string{"工业控制继电器"}, "工业控制固态继电器",
	)
	require.False(t, cacheEmpty)
	require.False(t, partial)
	require.Empty(t, warnings)
	require.Empty(t, works, "history is a bonus and must not create a semantically unrelated candidate")
}

func TestOzonRecommendationRefreshIsBoundedAndMissingTemplatesArePartial(t *testing.T) {
	catalog := &fakeOzonRecommendationCatalog{attrs: map[string][]shop.OzonAttributeDTO{}, attrErrors: map[string]error{}, refreshErrors: map[string]error{}}
	works := make([]*ozonRecommendationCandidateWork, 0, 8)
	for index := 0; index < 8; index++ {
		categoryID := "cat-" + string(rune('a'+index))
		catalog.attrs[categoryID] = []shop.OzonAttributeDTO{{CategoryID: categoryID, AttrID: "model", Name: "型号", SKUVariantEligible: true, SKUVariantEligibilityKnown: true}}
		works = append(works, &ozonRecommendationCandidateWork{node: shop.OzonCategoryNodeDTO{CategoryID: categoryID, Path: categoryID}})
	}
	svc := &Service{OzonCategories: catalog}
	warnings, partial := svc.loadOzonRecommendationTemplates(context.Background(), 1, uuid.New(), works, OzonRecommendationRefreshIfMissingOrStale)
	require.True(t, partial)
	require.NotEmpty(t, warnings)
	require.Len(t, catalog.refreshCalls, ozonRecommendationMaxRefresh)
	additionalWarnings, additionalPartial, additionalRefreshed := svc.refreshUnattemptedOzonRecommendationTemplates(
		context.Background(), 1, uuid.New(), works[ozonRecommendationMaxRefresh:],
		OzonRecommendationRefreshIfMissingOrStale, ozonRecommendationMaxTemplates-ozonRecommendationMaxRefresh,
	)
	require.True(t, additionalRefreshed)
	require.False(t, additionalPartial)
	require.NotEmpty(t, additionalWarnings)
	require.Len(t, catalog.refreshCalls, ozonRecommendationMaxTemplates)

	missing := []*ozonRecommendationCandidateWork{{node: shop.OzonCategoryNodeDTO{CategoryID: "missing", Path: "missing"}}}
	warnings, partial = svc.loadOzonRecommendationTemplates(context.Background(), 1, uuid.New(), missing, OzonRecommendationRefreshCacheOnly)
	require.True(t, partial)
	require.NotEmpty(t, warnings)
}

func TestOzonCategoryRecommendationHTTPRejectsReadonlyForeignProductAndShopBeforeAI(t *testing.T) {
	svc, productRow, _, shopID, catalog := setupOzonRecommendationFixture(t)
	fakeAI := &fakeOzonRecommendationAI{}
	svc.OzonCategoryAI = fakeAI
	handler := &Handler{Svc: svc}

	call := func(principal *adminperm.Principal, tenantID int64, productID uuid.UUID, requestedShop uuid.UUID) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/products/"+productID.String()+"/platform-configs/ozon/category-recommendations", bytes.NewBufferString(`{"shopId":"`+requestedShop.String()+`"}`))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: productID.String()}}
		c.Set(ctxkey.TenantID, tenantID)
		c.Set("adminperm.principal", principal)
		handler.RecommendOzonCategories(c)
		return w
	}

	readonly := &adminperm.Principal{TenantID: 1, Role: adminperm.RoleReadonly, Permissions: adminperm.PermissionsForRole(adminperm.RoleReadonly)}
	require.Equal(t, http.StatusForbidden, call(readonly, 1, productRow.ID, shopID).Code)

	tenantAdmin := &adminperm.Principal{TenantID: 2, Role: adminperm.RoleTenantAdmin, Permissions: adminperm.PermissionsForRole(adminperm.RoleTenantAdmin)}
	require.Equal(t, http.StatusNotFound, call(tenantAdmin, 2, productRow.ID, shopID).Code)

	catalog.ensureErr = gorm.ErrRecordNotFound
	tenantAdmin = &adminperm.Principal{TenantID: 1, Role: adminperm.RoleTenantAdmin, Permissions: adminperm.PermissionsForRole(adminperm.RoleTenantAdmin)}
	require.Equal(t, http.StatusNotFound, call(tenantAdmin, 1, productRow.ID, uuid.New()).Code)
	require.Empty(t, fakeAI.requests)
}

func TestOzonCategoryRecommendationHTTPSuccessUsesStandardEnvelope(t *testing.T) {
	svc, productRow, skus, shopID, _ := setupOzonRecommendationFixture(t)
	svc.OzonCategoryAI = &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{
		{Content: analysisJSON(t, skus), Model: "fake-analysis"},
		{Content: `{"selectedRootKeys":["root_1"]}`, Model: "fake-root-screen"},
		{Content: `{"selected":[{"candidateKey":"candidate_1","subjectMatch":"exact","confidence":0.9,"reasons":[],"warnings":[]}]}`, Model: "fake-path-screen"},
		{Content: `{"verdicts":[{"candidateKey":"candidate_1","subjectMatch":"approximate","confidence":0.8,"reasons":[],"warnings":[]}]}`, Model: "fake-template-review"},
	}}
	handler := &Handler{Svc: svc}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/products/"+productRow.ID.String()+"/platform-configs/ozon/category-recommendations",
		bytes.NewBufferString(`{"shopId":"`+shopID.String()+`","skuIds":[],"refreshPolicy":"cache_only"}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: productRow.ID.String()}}
	c.Set(ctxkey.TenantID, int64(1))
	c.Set("adminperm.principal", &adminperm.Principal{
		TenantID: 1, Role: adminperm.RoleTenantAdmin,
		Permissions: adminperm.PermissionsForRole(adminperm.RoleTenantAdmin),
	})

	handler.RecommendOzonCategories(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var envelope struct {
		Code    int                              `json:"code"`
		Message string                           `json:"message"`
		Data    OzonCategoryRecommendationResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, response.CodeOK, envelope.Code)
	require.Equal(t, "ok", envelope.Message)
	require.Equal(t, OzonCategoryRecommendationReady, envelope.Data.Status)
	require.Len(t, envelope.Data.Candidates, 1)
	require.Equal(t, "100:200", envelope.Data.Candidates[0].CategoryID)
}

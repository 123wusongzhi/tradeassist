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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	platformozon "github.com/trademind-ai/trademind/backend/internal/providers/platform/ozon"
	"gorm.io/gorm"
)

type deadlineCapturingOzonAttributeAI struct {
	response    *aigate.ChatResponse
	hasDeadline bool
}

type concurrentOzonAttributeAI struct {
	mu                 sync.Mutex
	requests           []aigate.ChatRequest
	factDone           bool
	fillStartedTooSoon bool
	active             int
	maxActive          int
	batches            [][]string
	repairBatches      [][]string
	callsByKey         map[string]int
	omitInitial        map[string]bool
	failKeys           map[string]bool
	failEveryBatch     bool
	delay              time.Duration
}

type callbackOzonAttributeAI struct {
	base     *concurrentOzonAttributeAI
	once     sync.Once
	callback func()
}

type repairingFactOzonAttributeAI struct {
	base         *concurrentOzonAttributeAI
	mu           sync.Mutex
	factRequests []aigate.ChatRequest
}

func (f *repairingFactOzonAttributeAI) Chat(ctx context.Context, request aigate.ChatRequest) (*aigate.ChatResponse, error) {
	if isOzonAttributeFactRequest(request) {
		f.mu.Lock()
		f.factRequests = append(f.factRequests, request)
		call := len(f.factRequests)
		f.mu.Unlock()
		if call == 1 {
			return &aigate.ChatResponse{Content: `{"facts":[}`, Model: "fake-vision", InputTokens: 10, OutputTokens: 3}, nil
		}
		return &aigate.ChatResponse{Content: `{"facts":[]}`, Model: "fake-text-repair", InputTokens: 4, OutputTokens: 2}, nil
	}
	return f.base.Chat(ctx, request)
}

func (f *callbackOzonAttributeAI) Chat(ctx context.Context, request aigate.ChatRequest) (*aigate.ChatResponse, error) {
	if !isOzonAttributeFactRequest(request) && f.callback != nil {
		f.once.Do(f.callback)
	}
	return f.base.Chat(ctx, request)
}

type staleTemplateOzonCatalog struct {
	*fakeOzonRecommendationCatalog
	mu        sync.Mutex
	listCalls int
}

func (f *staleTemplateOzonCatalog) ListOzonCategoryAttributes(ctx context.Context, categoryID string) ([]shop.OzonAttributeDTO, error) {
	attrs, err := f.fakeOzonRecommendationCatalog.ListOzonCategoryAttributes(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.listCalls++
	call := f.listCalls
	f.mu.Unlock()
	if call > 1 && len(attrs) > 0 {
		attrs = append([]shop.OzonAttributeDTO(nil), attrs...)
		attrs[0].Required = !attrs[0].Required
	}
	return attrs, nil
}

type revokedOzonCatalog struct {
	*fakeOzonRecommendationCatalog
	mu          sync.Mutex
	ensureCalls int
}

type concurrentDictionaryOzonCatalog struct {
	*fakeOzonRecommendationCatalog
	mu        sync.Mutex
	active    int
	maxActive int
	calls     int
	delay     time.Duration
}

func (f *concurrentDictionaryOzonCatalog) SearchOzonDictionaryValues(
	_ context.Context,
	_ int64,
	_ string,
	attrID string,
	_ uuid.UUID,
	keyword string,
) ([]platformozon.DictionaryValue, error) {
	f.mu.Lock()
	f.active++
	f.calls++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	delay := f.delay
	f.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	f.mu.Lock()
	f.active--
	f.mu.Unlock()
	return []platformozon.DictionaryValue{{ID: "official-" + attrID, Value: keyword}}, nil
}

func (f *revokedOzonCatalog) EnsureAuthorizedOzonShop(ctx context.Context, tenantID int64, shopID uuid.UUID) error {
	f.mu.Lock()
	f.ensureCalls++
	call := f.ensureCalls
	f.mu.Unlock()
	if call > 1 {
		return gorm.ErrRecordNotFound
	}
	return f.fakeOzonRecommendationCatalog.EnsureAuthorizedOzonShop(ctx, tenantID, shopID)
}

func (f *concurrentOzonAttributeAI) Chat(_ context.Context, request aigate.ChatRequest) (*aigate.ChatResponse, error) {
	f.mu.Lock()
	f.requests = append(f.requests, request)
	if isOzonAttributeFactRequest(request) {
		f.factDone = true
		f.mu.Unlock()
		return &aigate.ChatResponse{Content: `{"facts":[]}`, Model: "fake-vision", InputTokens: 10, OutputTokens: 4}, nil
	}
	keys := ozonAttributeKeysFromFillRequest(request)
	repair := strings.Contains(ozonAttributeRequestText([]aigate.ChatRequest{request}), "上一次输出未通过校验")
	if !f.factDone {
		f.fillStartedTooSoon = true
	}
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	if repair {
		f.repairBatches = append(f.repairBatches, append([]string(nil), keys...))
	} else {
		f.batches = append(f.batches, append([]string(nil), keys...))
	}
	if f.callsByKey == nil {
		f.callsByKey = map[string]int{}
	}
	for _, key := range keys {
		f.callsByKey[key]++
	}
	shouldFail := f.failEveryBatch
	for _, key := range keys {
		if f.failKeys[key] {
			shouldFail = true
		}
	}
	delay := f.delay
	f.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	f.mu.Lock()
	f.active--
	f.mu.Unlock()
	if shouldFail {
		return nil, errors.New("fake provider transport failure")
	}
	suggestions := make([]ozonAttributeAICandidate, 0, len(keys))
	for _, key := range keys {
		if !repair && f.omitInitial[key] {
			continue
		}
		suggestions = append(suggestions, ozonAttributeAICandidate{
			AttributeKey: key, Values: []string{"value-" + key}, Confidence: 0.3,
			InferenceBasis: ozonAttributeBasisFallback, Reason: "类目兜底测试值",
			SourceRefs: []string{"category.path", "common_knowledge"},
		})
	}
	content, _ := json.Marshal(ozonAttributeAIOutput{Suggestions: suggestions})
	return &aigate.ChatResponse{Content: string(content), Model: "fake-fill", InputTokens: 20, OutputTokens: 12}, nil
}

func ozonAttributeKeysFromFillRequest(request aigate.ChatRequest) []string {
	const startMarker = "本批允许建议的空白属性 JSON：\n"
	const endMarker = "\n\n允许引用的来源："
	for _, message := range request.Messages {
		start := strings.Index(message.Content, startMarker)
		if start < 0 {
			continue
		}
		remainder := message.Content[start+len(startMarker):]
		end := strings.Index(remainder, endMarker)
		if end < 0 {
			continue
		}
		var candidates []ozonAttributePromptCandidate
		if json.Unmarshal([]byte(remainder[:end]), &candidates) != nil {
			return nil
		}
		keys := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			keys = append(keys, candidate.AttributeKey)
		}
		return keys
	}
	return nil
}

func ozonAttributeFactRequests(requests []aigate.ChatRequest) []aigate.ChatRequest {
	out := make([]aigate.ChatRequest, 0)
	for _, request := range requests {
		if isOzonAttributeFactRequest(request) {
			out = append(out, request)
		}
	}
	return out
}

func ozonAttributeFillRequests(requests []aigate.ChatRequest) []aigate.ChatRequest {
	out := make([]aigate.ChatRequest, 0)
	for _, request := range requests {
		if !isOzonAttributeFactRequest(request) {
			out = append(out, request)
		}
	}
	return out
}

func ozonAttributeRequestText(requests []aigate.ChatRequest) string {
	parts := make([]string, 0, len(requests)*2)
	for _, request := range requests {
		for _, message := range request.Messages {
			parts = append(parts, message.Content)
		}
	}
	return strings.Join(parts, "\n")
}

func (f *deadlineCapturingOzonAttributeAI) Chat(ctx context.Context, request aigate.ChatRequest) (*aigate.ChatResponse, error) {
	_, f.hasDeadline = ctx.Deadline()
	if isOzonAttributeFactRequest(request) {
		return &aigate.ChatResponse{Content: `{"facts":[]}`}, nil
	}
	return f.response, nil
}

type ozonAttributeSuggestionFixture struct {
	svc     *Service
	product *Product
	shopID  uuid.UUID
	attrs   []shop.OzonAttributeDTO
	catalog *fakeOzonRecommendationCatalog
}

func setupOzonAttributeSuggestionFixture(t *testing.T) ozonAttributeSuggestionFixture {
	t.Helper()
	svc, productRow := tenantProductFixture(t, 1)
	require.NoError(t, svc.DB.AutoMigrate(
		&aitask.AITask{}, &aiprompt.AIPrompt{}, &shop.Shop{}, &shop.PlatformCategory{},
		&operationlog.OperationLog{},
	))
	require.NoError(t, aiprompt.EnsureDefaults(t.Context(), svc.DB))
	productRow.Title = "Acme 工业控制器，支持自动模式 https://private.example.test/title"
	productRow.Description = "品牌 Acme；数量 12；支持自动模式；apiToken=TEST_ONLY_DESCRIPTION_SECRET。"
	productRow.RawData = jsonBytes(t, map[string]any{
		"attributes": map[string]any{
			"品牌": "Acme", "apiToken": "TEST_ONLY_PRODUCT_TOKEN", "库存": 999,
			"参考链接": "https://private.example.test/product",
		},
	})
	require.NoError(t, svc.DB.Save(productRow).Error)
	sku := ProductSKU{
		ProductID: productRow.ID, SKUCode: "CTRL-1", SKUName: "自动模式",
		Attrs: jsonBytes(t, map[string]any{"模式": "自动"}),
		RawData: jsonBytes(t, map[string]any{
			"properties": map[string]any{"数量": "12", "credential": "TEST_ONLY_SKU_SECRET", "图片": "https://private.example.test/sku.jpg"},
		}),
	}
	require.NoError(t, svc.DB.Create(&sku).Error)
	productRow.SKUs = []ProductSKU{sku}
	shopRow := shop.Shop{TenantID: 1, Platform: "ozon", ShopName: "Test Ozon", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	require.NoError(t, svc.DB.Create(&shopRow).Error)
	require.NoError(t, svc.DB.Create(&[]shop.PlatformCategory{
		{Platform: "ozon", CategoryID: "root", Name: "Electronics", Level: 1, Status: "active"},
		{Platform: "ozon", CategoryID: "controls", ParentID: "root", Name: "Industrial automation", Level: 2, Status: "active"},
		{Platform: "ozon", CategoryID: "100:200", ParentID: "controls", Name: "Industrial controllers", Level: 3, IsLeaf: true, Status: "active"},
	}).Error)
	now := time.Now().UTC()
	attrs := []shop.OzonAttributeDTO{
		{CategoryID: "100:200", AttrID: "10-brand", Name: "品牌", Required: true, ValueType: "String", DictionaryID: "500", Options: json.RawMessage(`[{"id":"10","value":"Acme"},{"id":"11","value":"Other"}]`), SyncedAt: &now},
		{CategoryID: "100:200", AttrID: "20-quantity", Name: "数量", ValueType: "Integer", SyncedAt: &now},
		{CategoryID: "100:200", AttrID: "30-auto", Name: "自动模式", ValueType: "Boolean", SyncedAt: &now},
		{CategoryID: "100:200", AttrID: "40-complex", Name: "组合材质", ValueType: "String", AttributeComplexID: 901, SyncedAt: &now},
		{CategoryID: "100:200", AttrID: "50-url", Name: "说明链接", ValueType: "URL", SyncedAt: &now},
		{CategoryID: "100:200", AttrID: "60-filled", Name: "用户字段", ValueType: "String", SyncedAt: &now},
	}
	catalog := &fakeOzonRecommendationCatalog{
		allowedShop: shopRow.ID, attrs: map[string][]shop.OzonAttributeDTO{"100:200": attrs},
		attrErrors: map[string]error{}, refreshErrors: map[string]error{},
	}
	svc.Prompts = &aiprompt.Service{DB: svc.DB}
	svc.AITasks = &aitask.Service{DB: svc.DB}
	svc.OpLog = &operationlog.Service{DB: svc.DB}
	svc.OzonCategories = catalog
	svc.ozonAttributeImageProbe = func(_ context.Context, _ string) (ozonAttributeSuggestionImageMetadata, error) {
		return ozonAttributeSuggestionImageMetadata{width: 1200, height: 1200}, nil
	}
	return ozonAttributeSuggestionFixture{svc: svc, product: productRow, shopID: shopRow.ID, attrs: attrs, catalog: catalog}
}

func suggestionBody(fixture ozonAttributeSuggestionFixture, current map[string]json.RawMessage) OzonAttributeSuggestionBody {
	return OzonAttributeSuggestionBody{
		ShopID: fixture.shopID.String(), CategoryID: "100:200",
		TemplateFingerprint: shop.OzonCategoryAttributeSchemaHash(fixture.attrs),
		CurrentValues:       OzonAttributeSuggestionEditorValues{Attributes: current},
	}
}

func TestOzonAttributeSuggestionsFillOnlyBlankValidateTypesAndAuditPartialResult(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	fakeAI := &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{{
		Content: `{"suggestions":[` +
			`{"attributeKey":"attribute_1","values":["Acme"],"confidence":0.94,"reason":"apiToken=TEST_ONLY_MODEL_SECRET","sourceRefs":["product.attributes"]},` +
			`{"attributeKey":"attribute_2","values":["12.5"],"confidence":0.91,"reason":"数量证据","sourceRefs":["product.title"]},` +
			`{"attributeKey":"attribute_3","values":["true"],"confidence":0.22,"reason":"标题和 SKU 语义支持自动模式","sourceRefs":["product.title","sku.1"]},` +
			`{"attributeKey":"attribute_999","values":["ignored"],"confidence":0.99,"reason":"unknown","sourceRefs":["product.title"]}` +
			`]}`,
		Model: "fake-attributes", InputTokens: 41, OutputTokens: 29,
	}}}
	fixture.svc.OzonAttributeAI = fakeAI
	c := tenantProductAdminContext(t, fixture.svc, 1)
	beforeUpdatedAt := fixture.product.UpdatedAt

	result, err := fixture.svc.SuggestOzonAttributes(c, fixture.product.ID, suggestionBody(fixture, map[string]json.RawMessage{
		"60-filled": json.RawMessage(`"用户原值"`),
	}), nil)
	require.NoError(t, err)
	require.Equal(t, OzonAttributeSuggestionPartial, result.Status)
	require.Equal(t, OzonAttributeSuggestionSummary{
		Filled: 2, RequiresReview: 1, NotFound: 3, Eligible: 4,
		High: 1, Low: 1, UnsupportedSkipped: 1, ValidationSkipped: 2,
	}, result.Summary)
	require.Len(t, result.Suggestions, 2)
	require.Equal(t, "10", result.Suggestions[0].Values[0].DictionaryValueID)
	require.Equal(t, "Acme", result.Suggestions[0].Values[0].Value)
	require.Equal(t, ozonAttributeBasisDirect, result.Suggestions[0].InferenceBasis)
	require.NotContains(t, result.Suggestions[0].Reason, "TEST_ONLY_MODEL_SECRET")
	require.Equal(t, "30-auto", result.Suggestions[1].AttributeID)
	require.Equal(t, "low", result.Suggestions[1].ConfidenceLevel)
	require.Equal(t, ozonAttributeBasisFallback, result.Suggestions[1].InferenceBasis)
	require.True(t, result.Suggestions[1].RequiresReview)
	require.Equal(t, []string{"product.title", "sku.1"}, result.Suggestions[1].SourceRefs)
	require.NotEmpty(t, result.Context.Fingerprint)
	require.Equal(t, shop.OzonCategoryAttributeSchemaHash(fixture.attrs), result.Context.TemplateFingerprint)
	require.NotContains(t, result.Suggestions, OzonAttributeSuggestion{AttributeID: "60-filled"})
	require.Condition(t, func() bool {
		for _, skipped := range result.Skipped {
			if skipped.AttributeID == "20-quantity" && strings.Contains(skipped.Reason, "64 位整数") {
				return true
			}
		}
		return false
	})

	require.Len(t, ozonAttributeFactRequests(fakeAI.requests), 1)
	require.Len(t, ozonAttributeFillRequests(fakeAI.requests), 2, "invalid or omitted fields get one repair round")
	prompt := ozonAttributeRequestText(fakeAI.requests)
	require.NotContains(t, prompt, "TEST_ONLY_PRODUCT_TOKEN")
	require.NotContains(t, prompt, "TEST_ONLY_SKU_SECRET")
	require.NotContains(t, prompt, "TEST_ONLY_DESCRIPTION_SECRET")
	require.NotContains(t, prompt, "private.example.test")
	require.NotContains(t, prompt, "库存")
	require.NotContains(t, prompt, "用户原值")

	var task aitask.AITask
	require.NoError(t, fixture.svc.DB.First(&task, "id = ?", result.TaskID).Error)
	require.Equal(t, aitask.StatusSuccess, task.Status)
	require.Equal(t, "fake-facts,fake-attributes", task.Model)
	require.Equal(t, 41, task.TokenInput)
	require.Equal(t, 29, task.TokenOutput)
	require.Empty(t, task.RawResponse)
	require.NotContains(t, string(task.Output), "TEST_ONLY_MODEL_SECRET")
	require.NotContains(t, string(task.Input), "Acme 工业控制器")
	require.NotContains(t, string(task.Input), "TEST_ONLY")

	var log operationlog.OperationLog
	require.NoError(t, fixture.svc.DB.Where("action = ?", "ai.ozon_attribute_suggestions.success").First(&log).Error)
	require.Contains(t, log.Message, "filled=2")
	require.NotContains(t, log.Message, "Acme")
	require.NotContains(t, log.Message, "用户原值")

	var persisted Product
	require.NoError(t, fixture.svc.DB.First(&persisted, "id = ?", fixture.product.ID).Error)
	require.True(t, persisted.UpdatedAt.Equal(beforeUpdatedAt), "AI suggestion must not update the product row")
	require.Equal(t, fixture.product.Title, persisted.Title)
}

func TestOzonAttributeSuggestionsRejectDictionaryIDsAndInvalidValuesWithoutDroppingValidSuggestions(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	fixture.svc.OzonAttributeAI = &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{{
		Content: `{"suggestions":[` +
			`{"attributeKey":"attribute_1","values":["999999"],"confidence":0.99,"reason":"model tried an id","sourceRefs":["product.title"]},` +
			`{"attributeKey":"attribute_2","values":["12"],"confidence":0.9,"reason":"valid integer","sourceRefs":["product.title"]},` +
			`{"attributeKey":"attribute_3","values":["yes"],"confidence":0.9,"reason":"invalid boolean","sourceRefs":["product.title"]},` +
			`{"attributeKey":"attribute_4","values":["https://manufacturer.example/spec"],"confidence":0.31,"reason":"inferred official product page","sourceRefs":["common_knowledge"]}` +
			`]}`,
	}}}
	c := tenantProductAdminContext(t, fixture.svc, 1)
	result, err := fixture.svc.SuggestOzonAttributes(c, fixture.product.ID, suggestionBody(fixture, nil), nil)
	require.NoError(t, err)
	require.Equal(t, OzonAttributeSuggestionPartial, result.Status)
	require.Len(t, result.Suggestions, 2)
	require.Equal(t, "20-quantity", result.Suggestions[0].AttributeID)
	require.Equal(t, "12", result.Suggestions[0].Values[0].Value)
	require.Equal(t, "50-url", result.Suggestions[1].AttributeID)
	require.Equal(t, "low", result.Suggestions[1].ConfidenceLevel)
	require.True(t, result.Suggestions[1].RequiresReview)
	for _, suggestion := range result.Suggestions {
		require.NotEqual(t, "10-brand", suggestion.AttributeID, "a model-supplied dictionary ID must never be accepted")
	}
}

func TestOzonAttributeSuggestionsDoNotPromptOrOverwriteFilledFields(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	fakeAI := &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{{Content: `{"suggestions":[]}`}}}
	fixture.svc.OzonAttributeAI = fakeAI
	c := tenantProductAdminContext(t, fixture.svc, 1)
	body := suggestionBody(fixture, map[string]json.RawMessage{
		// The editor may select an official dictionary option through remote
		// search that is not present in the bounded cached Options list.
		"10-brand":    json.RawMessage(`"remote-option-id-not-in-template-page"`),
		"20-quantity": json.RawMessage(`"12.5"`),
	})
	// Current values only suppress overwrite. Eligibility/type validation stays
	// with save/preflight; this endpoint validates every AI-produced value.
	body.CurrentValues.SKUVariantAttributeIDs = []string{"30-auto"}
	result, err := fixture.svc.SuggestOzonAttributes(c, fixture.product.ID, body, nil)
	require.NoError(t, err)
	require.Empty(t, result.Suggestions)
	require.Len(t, ozonAttributeFactRequests(fakeAI.requests), 1)
	require.Len(t, ozonAttributeFillRequests(fakeAI.requests), 2)
	prompt := ozonAttributeRequestText(fakeAI.requests)
	require.NotContains(t, prompt, `"name":"品牌"`)
	require.NotContains(t, prompt, `"name":"数量"`)
	require.NotContains(t, prompt, `"name":"自动模式"`)
	require.NotContains(t, prompt, "remote-option-id-not-in-template-page")
	require.NotContains(t, prompt, "12.5")
}

func TestSelectRepresentativeOzonAttributeSKUsUsesFarthestDifferences(t *testing.T) {
	created := time.Date(2026, 8, 11, 1, 0, 0, 0, time.UTC)
	skus := []ProductSKU{
		{SKUCode: "MAIN", SKUName: "默认款", Attrs: jsonBytes(t, map[string]any{"颜色": "红", "尺寸": "S"})},
		{SKUCode: "NEAR", SKUName: "近似款", Attrs: jsonBytes(t, map[string]any{"颜色": "红", "尺寸": "M"})},
		{SKUCode: "FARTHEST", SKUName: "差异最大", Attrs: jsonBytes(t, map[string]any{"颜色": "蓝", "尺寸": "L", "材质": "钢"})},
		{SKUCode: "THIRD", SKUName: "第三代表", Attrs: jsonBytes(t, map[string]any{"颜色": "绿", "尺寸": "S", "材质": "塑料"})},
	}
	for index := range skus {
		skus[index].ID = uuid.New()
		skus[index].CreatedAt = created.Add(time.Duration(index) * time.Minute)
	}

	selected := selectRepresentativeOzonAttributeSKUs(skus)
	require.Len(t, selected, 3)
	require.Equal(t, []string{"MAIN", "FARTHEST", "THIRD"}, []string{selected[0].SKUCode, selected[1].SKUCode, selected[2].SKUCode})
	require.Equal(t, []string{"sku.1", "sku.2", "sku.3"}, []string{selected[0].SourceRef, selected[1].SourceRef, selected[2].SourceRef})
	variations := buildOzonAttributePromptSKUVariations(selected)
	require.Len(t, variations, 3)
	var sizeVariation *ozonAttributePromptSKUVariation
	for index := range variations {
		if variations[index].Attribute == "尺寸" {
			sizeVariation = &variations[index]
		}
	}
	require.NotNil(t, sizeVariation)
	require.ElementsMatch(t, []string{"L", "S"}, sizeVariation.Values)
	require.Contains(t, sizeVariation.Semantics, "size")
}

func TestOzonAttributeSKUVariationFactsCopyAndPhysicalFieldsFailClosed(t *testing.T) {
	representatives := []ozonAttributePromptSKU{
		{SourceRef: "sku.1", Attributes: map[string]string{"颜色分类": "黑色大号【柚木板】"}},
		{SourceRef: "sku.2", Attributes: map[string]string{"颜色分类": "白色中号【原木板】"}},
	}
	variations := buildOzonAttributePromptSKUVariations(representatives)
	require.Len(t, variations, 1)
	require.Contains(t, variations[0].Semantics, "color")
	require.Contains(t, variations[0].Semantics, "size")

	contextInfo := ozonAttributePromptContext{
		RepresentativeSKUs: representatives,
		SKUVariations:      variations,
		AllowedSourceRefs:  []string{"sku.1", "sku.2", "image.1", "category.path", "common_knowledge"},
	}
	facts, err := validateOzonAttributeFacts(ozonAttributeFactAIOutput{Facts: []ozonAttributeFactAICandidate{{
		Name: "颜色", Value: "黑色", Evidence: "图片可见黑色", SourceRefs: []string{"image.1"},
	}}}, contextInfo)
	require.NoError(t, err)
	require.Empty(t, facts, "one visual variant must not become a product-wide fact")

	nameCandidate := ozonAttributeSuggestionCandidate{key: "attribute_1", attr: shop.OzonAttributeDTO{AttrID: "4180", Name: "名称", ValueType: "String"}}
	copyValidation := validateOzonAttributeCandidateOutput(nameCandidate, []ozonAttributeAICandidate{{
		AttributeKey: "attribute_1", Values: []string{"ящик для хранения, черный"},
		InferenceBasis: ozonAttributeBasisStandard, Reason: "图片显示黑色", SourceRefs: []string{"image.1", "category.path", "common_knowledge"},
	}}, map[string]bool{"image.1": true, "category.path": true, "common_knowledge": true}, nil, contextInfo)
	require.Nil(t, copyValidation.Suggestion)
	require.Contains(t, copyValidation.Reason, "部分代表 SKU")

	neutralValidation := validateOzonAttributeCandidateOutput(nameCandidate, []ozonAttributeAICandidate{{
		AttributeKey: "attribute_1", Values: []string{"ящик для хранения под кроватью"},
		InferenceBasis: ozonAttributeBasisStandard, Reason: "使用不含黑色、白色等变体规格的中性名称", SourceRefs: []string{"category.path", "common_knowledge"},
	}}, map[string]bool{"category.path": true, "common_knowledge": true}, nil, contextInfo)
	require.NotNil(t, neutralValidation.Suggestion)

	contextInfo.ProductTitle = "床底收纳箱带滑轮"
	ungroundedBrandValidation := validateOzonAttributeCandidateOutput(nameCandidate, []ozonAttributeAICandidate{{
		AttributeKey: "attribute_1", Values: []string{"储物箱 TradeMind 带滑轮床底收纳箱"},
		InferenceBasis: ozonAttributeBasisStandard, Reason: "生成中性名称", SourceRefs: []string{"category.path", "common_knowledge"},
	}}, map[string]bool{"category.path": true, "common_knowledge": true}, nil, contextInfo)
	require.Nil(t, ungroundedBrandValidation.Suggestion)
	require.Contains(t, ungroundedBrandValidation.Reason, "TradeMind")

	contextInfo.ProductTitle = "Nokia 2660 Flip 手机"
	groundedBrandValidation := validateOzonAttributeCandidateOutput(nameCandidate, []ozonAttributeAICandidate{{
		AttributeKey: "attribute_1", Values: []string{"Nokia 2660 Flip 手机"},
		InferenceBasis: ozonAttributeBasisDirect, Reason: "名称直接来自标题", SourceRefs: []string{"product.title"},
	}}, map[string]bool{"product.title": true}, nil, contextInfo)
	require.NotNil(t, groundedBrandValidation.Suggestion)

	candidates := []ozonAttributeSuggestionCandidate{
		{key: "attribute_1", attr: shop.OzonAttributeDTO{AttrID: "4180", Name: "名称", ValueType: "String"}},
		{key: "attribute_2", attr: shop.OzonAttributeDTO{AttrID: "4383", Name: "商品重量，克", ValueType: "Decimal"}},
		{key: "attribute_3", attr: shop.OzonAttributeDTO{AttrID: "6871", Name: "节数，个", ValueType: "Integer"}},
	}
	prompts := []ozonAttributePromptCandidate{{AttributeKey: "attribute_1"}, {AttributeKey: "attribute_2"}, {AttributeKey: "attribute_3"}}
	kept, keptPrompts, skipped := excludeOzonAttributeSKUVariationConflicts(candidates, prompts, variations)
	require.Len(t, kept, 2)
	require.Len(t, keptPrompts, 2)
	require.Len(t, skipped, 1)
	require.Equal(t, "4383", skipped[0].AttributeID)
	require.Equal(t, ozonAttributeSkipUnsupported, skipped[0].Kind)

	petRepresentatives := []ozonAttributePromptSKU{
		{SourceRef: "sku.1", Attributes: map[string]string{"适用尺码": "S", "颜色分类": "老犬趴趴凳-猫抓皮奶白"}},
		{SourceRef: "sku.2", Attributes: map[string]string{"适用尺码": "3L", "颜色分类": "老犬趴趴凳-泰迪绒浅灰"}},
	}
	petVariations := buildOzonAttributePromptSKUVariations(petRepresentatives)
	require.Len(t, petVariations, 2)
	require.Contains(t, petVariations[0].Semantics, "size")
	require.Contains(t, petVariations[1].Semantics, "material")

	petCopyContext := ozonAttributePromptContext{
		RepresentativeSKUs: petRepresentatives,
		SKUVariations:      petVariations,
		AllowedSourceRefs:  []string{"category.path", "common_knowledge"},
	}
	introCandidate := ozonAttributeSuggestionCandidate{key: "attribute_intro", attr: shop.OzonAttributeDTO{AttrID: "4191", Name: "简介", ValueType: "String"}}
	variantMaterialCopy := validateOzonAttributeCandidateOutput(introCandidate, []ozonAttributeAICandidate{{
		AttributeKey: "attribute_intro", Values: []string{"утепляющий верхний слой из овчины"},
		InferenceBasis: ozonAttributeBasisStandard, Reason: "标准商品文案", SourceRefs: []string{"category.path", "common_knowledge"},
	}}, map[string]bool{"category.path": true, "common_knowledge": true}, nil, petCopyContext)
	require.Nil(t, variantMaterialCopy.Suggestion)
	require.Contains(t, variantMaterialCopy.Reason, "部分代表 SKU")

	petCandidates := []ozonAttributeSuggestionCandidate{
		{key: "pet_size", attr: shop.OzonAttributeDTO{AttrID: "pet-size", Name: "宠物尺寸", ValueType: "String"}},
		{key: "pet_material", attr: shop.OzonAttributeDTO{AttrID: "4967", Name: "材料", ValueType: "String"}},
		{key: "pet_shape", attr: shop.OzonAttributeDTO{AttrID: "shape", Name: "形状", ValueType: "String"}},
	}
	petPrompts := []ozonAttributePromptCandidate{{AttributeKey: "pet_size"}, {AttributeKey: "pet_material"}, {AttributeKey: "pet_shape"}}
	petKept, _, petSkipped := excludeOzonAttributeSKUVariationConflicts(petCandidates, petPrompts, petVariations)
	require.Len(t, petKept, 1)
	require.Equal(t, "shape", petKept[0].attr.AttrID)
	require.Len(t, petSkipped, 2)
	require.ElementsMatch(t, []string{"pet-size", "4967"}, []string{petSkipped[0].AttributeID, petSkipped[1].AttributeID})

	bundleRepresentatives := []ozonAttributePromptSKU{
		{SourceRef: "sku.1", Attributes: map[string]string{"套餐类型": "官方标配"}},
		{SourceRef: "sku.2", Attributes: map[string]string{"套餐类型": "套餐一 底座+充电头"}},
		{SourceRef: "sku.3", Attributes: map[string]string{"套餐类型": "套餐二 8G内存卡"}},
	}
	bundleVariations := buildOzonAttributePromptSKUVariations(bundleRepresentatives)
	require.Len(t, bundleVariations, 1)
	require.Contains(t, bundleVariations[0].Semantics, "bundle")
	require.Equal(t, "套餐类型", ozonAttributeSingleSKUVariantMention("底座+充电头", bundleVariations))
	bundleFacts, err := validateOzonAttributeFacts(ozonAttributeFactAIOutput{Facts: []ozonAttributeFactAICandidate{{
		Name: "配套", Value: "底座+充电头", Evidence: "套餐一 底座+充电头", SourceRefs: []string{"sku.2"},
	}}}, ozonAttributePromptContext{
		RepresentativeSKUs: bundleRepresentatives,
		SKUVariations:      bundleVariations,
		AllowedSourceRefs:  []string{"sku.1", "sku.2", "sku.3"},
	})
	require.NoError(t, err)
	require.Empty(t, bundleFacts, "one SKU package must not become a product-wide accessory fact")
	bundleCandidates := []ozonAttributeSuggestionCandidate{
		{key: "bundle", attr: shop.OzonAttributeDTO{AttrID: "4384", Name: "配套", ValueType: "String"}},
		{key: "battery", attr: shop.OzonAttributeDTO{AttrID: "4429", Name: "电池容量，mAh", ValueType: "Decimal"}},
	}
	bundlePrompts := []ozonAttributePromptCandidate{{AttributeKey: "bundle"}, {AttributeKey: "battery"}}
	bundleKept, _, bundleSkipped := excludeOzonAttributeSKUVariationConflicts(bundleCandidates, bundlePrompts, bundleVariations)
	require.Len(t, bundleKept, 1)
	require.Equal(t, "4429", bundleKept[0].attr.AttrID)
	require.Len(t, bundleSkipped, 1)
	require.Equal(t, "4384", bundleSkipped[0].AttributeID)
	require.Contains(t, bundleSkipped[0].Reason, "套餐/配件差异")
	require.Contains(t, aiprompt.OzonAttributeSuggestionRuntimePolicy(), "仅部分 SKU 包含的底座")
}

func TestOzonAttributePhoneBodyHintAndUnitConflictsAreDeterministic(t *testing.T) {
	bodyAttr := shop.OzonAttributeDTO{
		AttrID: "12126", Name: "房屋类别", ValueType: "String", DictionaryID: "76656905",
		Options: json.RawMessage(jsonBytes(t, []map[string]string{
			{"id": "970883112", "value": "一体式"},
			{"id": "970883114", "value": "滑 块"},
			{"id": "971042229", "value": "湿 陷 性"},
		})),
	}
	_, prompts, skipped := buildOzonAttributeSuggestionCandidates([]shop.OzonAttributeDTO{bodyAttr}, nil, nil, 3)
	require.Empty(t, skipped)
	require.Len(t, prompts, 1)
	require.Contains(t, prompts[0].SemanticHint, "移动电话机身形态")
	require.Contains(t, prompts[0].SemanticHint, "湿 陷 性")
	require.NotContains(t, prompts[0].SemanticHint, "971042229", "provider IDs must not enter the model prompt")

	contextInfo := ozonAttributePromptContext{
		ProductTitle:      "Nokia 2660 Flip 翻盖老年手机 4G全网通",
		ProductAttributes: map[string]string{"操作系统": "Series 30+"},
	}
	require.Contains(t, ozonAttributeKnownSemanticConflict(bodyAttr, []string{"一体式"}, "按类目猜测", contextInfo), "翻盖")
	require.Contains(t, ozonAttributeKnownSemanticConflict(bodyAttr, []string{"滑 块"}, "按类目猜测", contextInfo), "翻盖")
	require.Empty(t, ozonAttributeKnownSemanticConflict(bodyAttr, []string{"湿 陷 性"}, "按官方词典语义选择翻盖", contextInfo))

	nameAttr := shop.OzonAttributeDTO{AttrID: "4180", Name: "名称", ValueType: "String"}
	require.Contains(t,
		ozonAttributeKnownSemanticConflict(nameAttr, []string{"смартфон-раскладушка Nokia 2660 Flip"}, "生成商品名称", contextInfo),
		"功能机",
	)
	introAttr := shop.OzonAttributeDTO{AttrID: "4191", Name: "简介", ValueType: "String"}
	require.Contains(t,
		ozonAttributeKnownSemanticConflict(introAttr, []string{"Совместим со всеми российскими операторами"}, "生成商品简介", contextInfo),
		"全网通",
	)
	require.Empty(t,
		ozonAttributeKnownSemanticConflict(introAttr, []string{"Телефон-раскладушка Nokia 2660 Flip с поддержкой 4G"}, "生成商品简介", contextInfo),
	)
	require.Contains(t,
		ozonAttributeKnownSemanticConflict(introAttr, []string{"Nokia 2660 Flip，单SIM双待"}, "生成商品简介", contextInfo),
		"自相矛盾",
	)

	visualFacts, err := validateOzonAttributeFacts(ozonAttributeFactAIOutput{Facts: []ozonAttributeFactAICandidate{
		{Name: "物理 SIM 卡数量", Value: "1", Evidence: "双天线信号 | 双屏显示", SourceRefs: []string{"image.2"}},
		{Name: "机身形态", Value: "翻盖", Evidence: "图片可见翻盖结构", SourceRefs: []string{"image.2"}},
	}}, ozonAttributePromptContext{AllowedSourceRefs: []string{"image.2"}})
	require.NoError(t, err)
	require.Len(t, visualFacts, 1)
	require.Equal(t, "机身形态", visualFacts[0].Name)

	physicalSIMCandidate := ozonAttributeSuggestionCandidate{key: "physical", attr: shop.OzonAttributeDTO{AttrID: "4407", Name: "物理 SIM 卡数量", ValueType: "String"}}
	multipleSIMCandidate := ozonAttributeSuggestionCandidate{key: "multiple", attr: shop.OzonAttributeDTO{AttrID: "12128", Name: "多个SIM卡的操作", ValueType: "String"}}
	crossFieldContext := ozonAttributePromptContext{AllowedSourceRefs: []string{"category.path", "common_knowledge"}}
	crossFieldValidations := validateOzonAttributeCandidateOutputs(
		[]ozonAttributeSuggestionCandidate{physicalSIMCandidate, multipleSIMCandidate},
		map[string][]ozonAttributeAICandidate{
			"physical": {{AttributeKey: "physical", Values: []string{"1"}, InferenceBasis: ozonAttributeBasisFallback, Reason: "类目猜测", SourceRefs: []string{"category.path", "common_knowledge"}}},
			"multiple": {{AttributeKey: "multiple", Values: []string{"同时"}, InferenceBasis: ozonAttributeBasisFallback, Reason: "类目猜测", SourceRefs: []string{"category.path", "common_knowledge"}}},
		},
		crossFieldContext,
	)
	require.NotNil(t, crossFieldValidations["physical"].Suggestion)
	require.Nil(t, crossFieldValidations["multiple"].Suggestion)
	require.Contains(t, crossFieldValidations["multiple"].Reason, "不适用")

	standbyAttr := shop.OzonAttributeDTO{AttrID: "4439", Name: "待机操作，小时", ValueType: "Decimal"}
	require.Contains(t,
		ozonAttributeKnownSemanticConflict(standbyAttr, []string{"28"}, "同类商品常见待机 28 天，约 672 小时", contextInfo),
		"乘以 24",
	)
	require.Empty(t,
		ozonAttributeKnownSemanticConflict(standbyAttr, []string{"672"}, "同类商品常见待机 28 天，换算为 672 小时", contextInfo),
	)
}

func TestOzonAttributeExternalPolicyUsesStableIDsAndNarrowSemantics(t *testing.T) {
	for id := range externalOzonAttributeIDs {
		kind, reason := ozonAttributeSuggestionSkip(shop.OzonAttributeDTO{AttrID: id, Name: "ordinary name", ValueType: "String"}, false, 1)
		require.Equal(t, ozonAttributeSkipExternal, kind, id)
		require.NotEmpty(t, reason, id)
	}

	semanticExternal := []shop.OzonAttributeDTO{
		{AttrID: "new-seller-code", Name: "Код продавца", ValueType: "String"},
		{AttrID: "new-shipping", Name: "Package dimensions", ValueType: "String"},
		{AttrID: "new-hs", Name: "HS code", ValueType: "String"},
	}
	for _, attr := range semanticExternal {
		kind, _ := ozonAttributeSuggestionSkip(attr, false, 1)
		require.Equal(t, ozonAttributeSkipExternal, kind, attr.Name)
	}

	intrinsic := []shop.OzonAttributeDTO{
		{AttrID: "4383", Name: "净重", ValueType: "Decimal"},
		{AttrID: "4382", Name: "无包装尺寸", ValueType: "String"},
		{AttrID: "4384", Name: "配套", ValueType: "String"},
		{AttrID: "12209", Name: "包装内产品配件", ValueType: "String"},
		{AttrID: "4389", Name: "产地", ValueType: "String"},
		{AttrID: "10400", Name: "保修", ValueType: "String"},
		{AttrID: "new-model", Name: "型号", ValueType: "String"},
		{AttrID: "new-copy", Name: "商品文案", ValueType: "Text"},
	}
	for _, attr := range intrinsic {
		kind, reason := ozonAttributeSuggestionSkip(attr, false, 1)
		require.Empty(t, kind, attr.Name)
		require.Empty(t, reason, attr.Name)
	}
}

func TestOzonAttributeVariantEligibleRemainsProductLevelUntilExplicitlySelected(t *testing.T) {
	attrs := []shop.OzonAttributeDTO{
		{AttrID: "material", Name: "材料", ValueType: "String", SKUVariantEligible: true, SKUVariantEligibilityKnown: true},
		{AttrID: "color", Name: "颜色", ValueType: "String", SKUVariantEligible: true, SKUVariantEligibilityKnown: true},
	}
	candidates, prompts, skipped := buildOzonAttributeSuggestionCandidates(attrs, nil, map[string]bool{"color": true}, 8)
	require.Len(t, candidates, 1)
	require.Equal(t, "material", candidates[0].attr.AttrID)
	require.Len(t, prompts, 1)
	require.Equal(t, "材料", prompts[0].Name)
	require.Len(t, skipped, 1)
	require.Equal(t, "color", skipped[0].AttributeID)
	require.Equal(t, ozonAttributeSkipUnsupported, skipped[0].Kind)
}

func TestOzonAttributeFailureAuditUsesBoundedDiagnosticCodes(t *testing.T) {
	require.Equal(t, "provider request failed: rate_limited", ozonAttributeSuggestionFailureMessage(errors.New("通义千问: 请求过于频繁或额度受限")))
	require.Equal(t, "fact output rejected: too_many_facts", ozonAttributeSuggestionFailureMessage(errors.New("fact extraction failed: too many product facts")))
	require.Equal(t, "fact output rejected: invalid_json", ozonAttributeSuggestionFailureMessage(errors.New(`fact extraction failed: json: unknown field "timeout"`)))
	require.Equal(t, "batch output rejected: invalid_json", ozonAttributeSuggestionFailureMessage(errors.New("all attribute batches returned invalid output")))
	require.Equal(t, "provider request failed: all_batches_failed", ozonAttributeSuggestionFailureMessage(errors.New("all attribute batches failed: opaque provider detail")))
	require.Equal(t, "provider request failed", ozonAttributeSuggestionFailureMessage(errors.New("provider detail TEST_ONLY_PROVIDER_SECRET")))
}

func TestOzonAttributeFactTableFeedsFillStageAndSupportsVerifiedVisualEvidence(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	require.NoError(t, fixture.svc.DB.Create(&ProductImage{
		ProductID: fixture.product.ID, ImageType: ImageTypeMain,
		OriginURL: "https://cdn.example.test/controller-black.jpg", IsBestMain: true,
	}).Error)
	fakeAI := &fakeOzonRecommendationAI{
		factResponse: &aigate.ChatResponse{
			Content: `{"facts":[{"name":"品牌","value":"Acme","evidence":"Acme","sourceRefs":["product.title"]},{"name":"外观颜色","value":"黑色","evidence":"控制器外壳清晰呈黑色","sourceRefs":["image.1"]}]}`,
			Model:   "fake-vision", InputTokens: 18, OutputTokens: 12,
		},
		responses: []*aigate.ChatResponse{{
			Content: `{"suggestions":[{"attributeKey":"attribute_1","values":["Acme"],"confidence":1,"inferenceBasis":"direct_product_evidence","reason":"商品事实表明确给出品牌","sourceRefs":["product.title"],"factRefs":["fact_1"]}]}`,
			Model:   "fake-fill",
		}},
	}
	fixture.svc.OzonAttributeAI = fakeAI
	current := map[string]json.RawMessage{
		"20-quantity": json.RawMessage(`"manual"`), "30-auto": json.RawMessage(`"manual"`),
		"50-url": json.RawMessage(`"manual"`), "60-filled": json.RawMessage(`"manual"`),
	}
	result, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, current), nil,
	)
	require.NoError(t, err)
	require.Len(t, result.Suggestions, 1)
	require.Equal(t, ozonAttributeBasisDirect, result.Suggestions[0].InferenceBasis)
	require.Equal(t, 0.9, result.Suggestions[0].Confidence)
	factRequests := ozonAttributeFactRequests(fakeAI.requests)
	fillRequests := ozonAttributeFillRequests(fakeAI.requests)
	require.Len(t, factRequests, 1)
	require.Len(t, fillRequests, 1)
	require.Equal(t, []string{"https://cdn.example.test/controller-black.jpg"}, factRequests[0].Messages[1].ImageURLs)
	require.Contains(t, ozonAttributeRequestText(fillRequests), `"factKey":"fact_1"`)
	require.Contains(t, ozonAttributeRequestText(fillRequests), `"sourceRefs":["image.1"]`)
	require.Empty(t, fillRequests[0].Messages[1].ImageURLs)
}

func TestOzonAttributeBatchesAreStableBoundedAndRunAtMostThreeConcurrently(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	attrs := plainOzonAttributeSuggestionAttrs(46)
	fixture.attrs = attrs
	fixture.catalog.attrs["100:200"] = attrs
	fakeAI := &concurrentOzonAttributeAI{delay: 30 * time.Millisecond}
	fixture.svc.OzonAttributeAI = fakeAI

	result, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
	)
	require.NoError(t, err)
	require.Equal(t, OzonAttributeSuggestionReady, result.Status)
	require.Len(t, result.Suggestions, 46)
	require.Equal(t, 46, result.Summary.Low)
	require.False(t, fakeAI.fillStartedTooSoon, "fact extraction must complete before field batches start")
	require.GreaterOrEqual(t, fakeAI.maxActive, 2)
	require.LessOrEqual(t, fakeAI.maxActive, ozonAttributeSuggestionMaxConcurrency)
	require.Empty(t, fakeAI.repairBatches)
	require.Len(t, fakeAI.batches, 4)
	maxCompletionTokens := 0
	for _, request := range ozonAttributeFillRequests(fakeAI.requests) {
		if request.MaxTokens > maxCompletionTokens {
			maxCompletionTokens = request.MaxTokens
		}
		require.LessOrEqual(t, request.MaxTokens, 5000)
	}
	require.Greater(t, maxCompletionTokens, 3000, "rich 15-field JSON batches need enough completion budget to close the document")
	lengths := make([]int, 0, len(fakeAI.batches))
	for _, batch := range fakeAI.batches {
		require.LessOrEqual(t, len(batch), ozonAttributeSuggestionMaxBatchSize)
		lengths = append(lengths, len(batch))
	}
	sort.Ints(lengths)
	require.Equal(t, []int{1, 15, 15, 15}, lengths)

	candidates, prompts, skipped := buildOzonAttributeSuggestionCandidates(attrs, nil, nil, 1)
	require.Empty(t, skipped)
	promptByKey := map[string]ozonAttributePromptCandidate{}
	for _, prompt := range prompts {
		promptByKey[prompt.AttributeKey] = prompt
	}
	first := buildOzonAttributeBatchWorks(candidates, promptByKey, ozonAttributeSuggestionMaxBatchSize, nil)
	second := buildOzonAttributeBatchWorks(candidates, promptByKey, ozonAttributeSuggestionMaxBatchSize, nil)
	require.Equal(t, first, second, "deterministic input must produce stable batches")
	for _, work := range first {
		encoded, marshalErr := json.Marshal(work.Prompt)
		require.NoError(t, marshalErr)
		require.LessOrEqual(t, len(encoded), ozonAttributeSuggestionMaxBatchBytes+1024)
	}
	largeCandidates := make([]ozonAttributeSuggestionCandidate, 0, 8)
	largePrompts := map[string]ozonAttributePromptCandidate{}
	for index := 1; index <= 8; index++ {
		key := fmt.Sprintf("attribute_%d", index)
		largeCandidates = append(largeCandidates, ozonAttributeSuggestionCandidate{key: key})
		options := make([]string, 0, 50)
		for option := 0; option < 50; option++ {
			options = append(options, fmt.Sprintf("option-%02d-%s", option, strings.Repeat("x", 70)))
		}
		largePrompts[key] = ozonAttributePromptCandidate{AttributeKey: key, Name: "large", ValueType: "String", DictionaryOptions: options}
	}
	volumeWorks := buildOzonAttributeBatchWorks(largeCandidates, largePrompts, ozonAttributeSuggestionMaxBatchSize, nil)
	require.Greater(t, len(volumeWorks), 1, "prompt volume must split a batch before the count limit")
	for _, work := range volumeWorks {
		encoded, marshalErr := json.Marshal(work.Prompt)
		require.NoError(t, marshalErr)
		require.LessOrEqual(t, len(encoded), ozonAttributeSuggestionMaxBatchBytes+1024)
	}

	var task aitask.AITask
	require.NoError(t, fixture.svc.DB.First(&task, "id = ?", result.TaskID).Error)
	var audit map[string]any
	require.NoError(t, json.Unmarshal(task.Input, &audit))
	require.Equal(t, float64(4), audit["attributeBatchCount"])
	require.Equal(t, float64(5), audit["aiCallCount"])
	require.Equal(t, float64(46), audit["lowCount"])
}

func TestOzonAttributeOmissionsAreRepairedOnce(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	attrs := plainOzonAttributeSuggestionAttrs(5)
	fixture.attrs = attrs
	fixture.catalog.attrs["100:200"] = attrs
	fakeAI := &concurrentOzonAttributeAI{omitInitial: map[string]bool{"attribute_2": true}}
	fixture.svc.OzonAttributeAI = fakeAI

	result, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
	)
	require.NoError(t, err)
	require.Equal(t, OzonAttributeSuggestionReady, result.Status)
	require.Len(t, result.Suggestions, 5)
	require.Len(t, fakeAI.batches, 1)
	require.Equal(t, [][]string{{"attribute_2"}}, fakeAI.repairBatches)
	require.Equal(t, 2, fakeAI.callsByKey["attribute_2"])
	for _, key := range []string{"attribute_1", "attribute_3", "attribute_4", "attribute_5"} {
		require.Equal(t, 1, fakeAI.callsByKey[key])
	}
}

func TestOzonAttributeBatchTransportFailureReturnsPartialWithoutRetry(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	attrs := plainOzonAttributeSuggestionAttrs(31)
	fixture.attrs = attrs
	fixture.catalog.attrs["100:200"] = attrs
	fakeAI := &concurrentOzonAttributeAI{failKeys: map[string]bool{"attribute_1": true}}
	fixture.svc.OzonAttributeAI = fakeAI

	result, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
	)
	require.NoError(t, err)
	require.Equal(t, OzonAttributeSuggestionPartial, result.Status)
	require.Len(t, result.Suggestions, 16)
	require.Equal(t, 15, result.Summary.ValidationSkipped)
	require.Empty(t, fakeAI.repairBatches)
	for key, count := range fakeAI.callsByKey {
		require.Equal(t, 1, count, key+" must not be retried after provider transport failure")
	}
	batchFailed := 0
	for _, skipped := range result.Skipped {
		if skipped.Kind == ozonAttributeSkipBatch {
			batchFailed++
		}
	}
	require.Equal(t, 15, batchFailed)
}

func TestOzonAttributeAllBatchTransportFailuresReturnBadGatewayWithoutRetry(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	attrs := plainOzonAttributeSuggestionAttrs(16)
	fixture.attrs = attrs
	fixture.catalog.attrs["100:200"] = attrs
	fakeAI := &concurrentOzonAttributeAI{failEveryBatch: true}
	fixture.svc.OzonAttributeAI = fakeAI

	result, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
	)
	require.Error(t, err)
	require.Nil(t, result)
	var apiErr *ozonAttributeSuggestionAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.HTTPStatus())
	require.Len(t, fakeAI.batches, 2)
	require.Empty(t, fakeAI.repairBatches)
	for key, count := range fakeAI.callsByKey {
		require.Equal(t, 1, count, key)
	}
	var task aitask.AITask
	require.NoError(t, fixture.svc.DB.Where("task_type = ?", ozonAttributeSuggestionTaskType).First(&task).Error)
	require.Equal(t, aitask.StatusFailed, task.Status)
	require.Equal(t, 10, task.TokenInput, "successful fact-call tokens remain audited on aggregate failure")
}

func TestOzonAttributeFinalRevalidationRejectsProductTemplateAndAuthorizationChanges(t *testing.T) {
	t.Run("product updated during AI calls", func(t *testing.T) {
		fixture := setupOzonAttributeSuggestionFixture(t)
		attrs := plainOzonAttributeSuggestionAttrs(3)
		fixture.attrs = attrs
		fixture.catalog.attrs["100:200"] = attrs
		baseAI := &concurrentOzonAttributeAI{}
		fixture.svc.OzonAttributeAI = &callbackOzonAttributeAI{base: baseAI, callback: func() {
			if updateErr := fixture.svc.DB.Model(&Product{}).Where("id = ?", fixture.product.ID).
				UpdateColumn("updated_at", time.Now().UTC().Add(time.Minute)).Error; updateErr != nil {
				t.Errorf("update product during AI call: %v", updateErr)
			}
		}}

		result, err := fixture.svc.SuggestOzonAttributes(
			tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
		)
		require.Error(t, err)
		require.Nil(t, result)
		var apiErr *ozonAttributeSuggestionAPIError
		require.ErrorAs(t, err, &apiErr)
		require.Equal(t, http.StatusConflict, apiErr.HTTPStatus())
	})

	t.Run("template changes before response", func(t *testing.T) {
		fixture := setupOzonAttributeSuggestionFixture(t)
		attrs := plainOzonAttributeSuggestionAttrs(3)
		fixture.attrs = attrs
		fixture.catalog.attrs["100:200"] = attrs
		fixture.svc.OzonCategories = &staleTemplateOzonCatalog{fakeOzonRecommendationCatalog: fixture.catalog}
		fixture.svc.OzonAttributeAI = &concurrentOzonAttributeAI{}

		result, err := fixture.svc.SuggestOzonAttributes(
			tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
		)
		require.Error(t, err)
		require.Nil(t, result)
		var apiErr *ozonAttributeSuggestionAPIError
		require.ErrorAs(t, err, &apiErr)
		require.Equal(t, http.StatusConflict, apiErr.HTTPStatus())
	})

	t.Run("shop authorization revoked before response", func(t *testing.T) {
		fixture := setupOzonAttributeSuggestionFixture(t)
		attrs := plainOzonAttributeSuggestionAttrs(3)
		fixture.attrs = attrs
		fixture.catalog.attrs["100:200"] = attrs
		fixture.svc.OzonCategories = &revokedOzonCatalog{fakeOzonRecommendationCatalog: fixture.catalog}
		fixture.svc.OzonAttributeAI = &concurrentOzonAttributeAI{}

		result, err := fixture.svc.SuggestOzonAttributes(
			tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
		)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
		require.Nil(t, result)
	})
}

func TestOzonAttributeConfidenceIsDerivedAndForgedHighIsDowngraded(t *testing.T) {
	candidate := ozonAttributeSuggestionCandidate{
		key:  "attribute_1",
		attr: shop.OzonAttributeDTO{AttrID: "color", Name: "颜色", ValueType: "String"},
	}
	contextInfo := ozonAttributePromptContext{
		ProductTitle:      "Red steel controller",
		AllowedSourceRefs: []string{"product.title", "image.1", "category.path", "common_knowledge"},
		Facts: []ozonAttributePromptFact{{
			FactKey: "fact_1", Name: "颜色", Value: "Black", Evidence: "外壳为黑色", SourceRefs: []string{"image.1"},
		}},
	}
	knownSources := map[string]bool{"product.title": true, "image.1": true, "category.path": true, "common_knowledge": true}
	knownFacts := map[string]bool{"fact_1": true}

	forged := validateOzonAttributeCandidateOutput(candidate, []ozonAttributeAICandidate{{
		AttributeKey: "attribute_1", Values: []string{"Blue"}, Confidence: 1,
		InferenceBasis: ozonAttributeBasisDirect, Reason: "声称标题直接支持", SourceRefs: []string{"product.title"},
	}}, knownSources, knownFacts, contextInfo)
	require.NotNil(t, forged.Suggestion)
	require.Equal(t, ozonAttributeBasisStandard, forged.Suggestion.InferenceBasis)
	require.Equal(t, 0.7, forged.Suggestion.Confidence)

	visual := validateOzonAttributeCandidateOutput(candidate, []ozonAttributeAICandidate{{
		AttributeKey: "attribute_1", Values: []string{"Black"}, Confidence: 0.1,
		InferenceBasis: ozonAttributeBasisDirect, Reason: "图片事实明确", SourceRefs: []string{"image.1"}, FactRefs: []string{"fact_1"},
	}}, knownSources, knownFacts, contextInfo)
	require.NotNil(t, visual.Suggestion)
	require.Equal(t, ozonAttributeBasisDirect, visual.Suggestion.InferenceBasis)
	require.Equal(t, 0.9, visual.Suggestion.Confidence)

	fallback := validateOzonAttributeCandidateOutput(candidate, []ozonAttributeAICandidate{{
		AttributeKey: "attribute_1", Values: []string{"White"}, Confidence: 1,
		InferenceBasis: "invented_super_high", Reason: "纯常识", SourceRefs: []string{"category.path", "common_knowledge"},
	}}, knownSources, knownFacts, contextInfo)
	require.NotNil(t, fallback.Suggestion)
	require.Equal(t, ozonAttributeBasisFallback, fallback.Suggestion.InferenceBasis)
	require.Equal(t, 0.3, fallback.Suggestion.Confidence)
	require.True(t, fallback.Suggestion.RequiresReview)

	unknownSource := validateOzonAttributeCandidateOutput(candidate, []ozonAttributeAICandidate{{
		AttributeKey: "attribute_1", Values: []string{"Red"}, Confidence: 1,
		InferenceBasis: ozonAttributeBasisDirect, Reason: "未知引用", SourceRefs: []string{"private.source"},
	}}, knownSources, knownFacts, contextInfo)
	require.Nil(t, unknownSource.Suggestion)
	require.Equal(t, ozonAttributeSkipValidation, unknownSource.Kind)
}

func TestOzonAttributeDictionaryUsesCacheThenUniqueReadOnlyExactSearch(t *testing.T) {
	t.Run("cached exact semantic value", func(t *testing.T) {
		fixture := setupOzonAttributeSuggestionFixture(t)
		fixture.svc.OzonAttributeAI = &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{{Content: `{"suggestions":[{"attributeKey":"attribute_1","values":["Acme"],"confidence":0.3,"inferenceBasis":"category_fallback_guess","reason":"测试","sourceRefs":["category.path","common_knowledge"]}]}`}}}
		result, err := fixture.svc.SuggestOzonAttributes(
			tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID,
			suggestionBody(fixture, filledOzonAttributeFixtureValuesExcept("10-brand")), nil,
		)
		require.NoError(t, err)
		require.Len(t, result.Suggestions, 1)
		require.Equal(t, "10", result.Suggestions[0].Values[0].DictionaryValueID)
		require.Empty(t, fixture.catalog.dictionaryCalls)
	})

	t.Run("uncached unique exact official search", func(t *testing.T) {
		fixture := setupOzonAttributeSuggestionFixture(t)
		fixture.attrs[0].Options = nil
		fixture.catalog.attrs["100:200"] = fixture.attrs
		key := "100:200\n10-brand\nRemote Brand"
		fixture.catalog.dictionaryValues = map[string][]platformozon.DictionaryValue{
			key: {{ID: "77", Value: "Remote Brand"}},
		}
		fixture.svc.OzonAttributeAI = &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{{Content: `{"suggestions":[{"attributeKey":"attribute_1","values":["Remote Brand"],"confidence":0.9,"inferenceBasis":"category_fallback_guess","reason":"测试","sourceRefs":["category.path","common_knowledge"]}]}`}}}
		result, err := fixture.svc.SuggestOzonAttributes(
			tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID,
			suggestionBody(fixture, filledOzonAttributeFixtureValuesExcept("10-brand")), nil,
		)
		require.NoError(t, err)
		require.Len(t, result.Suggestions, 1)
		require.Equal(t, "77", result.Suggestions[0].Values[0].DictionaryValueID)
		require.Equal(t, []string{key}, fixture.catalog.dictionaryCalls)
	})

	t.Run("ambiguous official matches are rejected", func(t *testing.T) {
		fixture := setupOzonAttributeSuggestionFixture(t)
		fixture.attrs[0].Options = nil
		fixture.catalog.attrs["100:200"] = fixture.attrs
		key := "100:200\n10-brand\nAmbiguous"
		fixture.catalog.dictionaryValues = map[string][]platformozon.DictionaryValue{
			key: {{ID: "77", Value: "Ambiguous"}, {ID: "88", Value: "Ambiguous"}},
		}
		fixture.svc.OzonAttributeAI = &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{{Content: `{"suggestions":[{"attributeKey":"attribute_1","values":["Ambiguous"],"confidence":0.3,"inferenceBasis":"category_fallback_guess","reason":"测试","sourceRefs":["category.path","common_knowledge"]}]}`}}}
		result, err := fixture.svc.SuggestOzonAttributes(
			tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID,
			suggestionBody(fixture, filledOzonAttributeFixtureValuesExcept("10-brand")), nil,
		)
		require.NoError(t, err)
		require.Empty(t, result.Suggestions)
		require.Condition(t, func() bool {
			for _, skipped := range result.Skipped {
				if skipped.AttributeID == "10-brand" && skipped.Kind == ozonAttributeSkipDictionary {
					return true
				}
			}
			return false
		})
	})

	t.Run("numeric model ids are rejected before official search", func(t *testing.T) {
		fixture := setupOzonAttributeSuggestionFixture(t)
		fixture.attrs[0].Options = nil
		fixture.catalog.attrs["100:200"] = fixture.attrs
		fixture.svc.OzonAttributeAI = &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{{Content: `{"suggestions":[{"attributeKey":"attribute_1","values":["999999"],"confidence":0.3,"inferenceBasis":"category_fallback_guess","reason":"模型返回数字 ID","sourceRefs":["category.path","common_knowledge"]}]}`}}}
		result, err := fixture.svc.SuggestOzonAttributes(
			tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID,
			suggestionBody(fixture, filledOzonAttributeFixtureValuesExcept("10-brand")), nil,
		)
		require.NoError(t, err)
		require.Empty(t, result.Suggestions)
		require.Empty(t, fixture.catalog.dictionaryCalls, "numeric model output must never be sent to official dictionary search")
	})
}

func TestOzonAttributeOfficialDictionarySearchRunsAtMostTwoConcurrently(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	attrs := plainOzonAttributeSuggestionAttrs(5)
	for index := range attrs {
		attrs[index].DictionaryID = fmt.Sprintf("dict-%d", index+1)
		attrs[index].Options = nil
	}
	fixture.attrs = attrs
	fixture.catalog.attrs["100:200"] = attrs
	catalog := &concurrentDictionaryOzonCatalog{fakeOzonRecommendationCatalog: fixture.catalog, delay: 25 * time.Millisecond}
	fixture.svc.OzonCategories = catalog
	fixture.svc.OzonAttributeAI = &concurrentOzonAttributeAI{}

	result, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
	)
	require.NoError(t, err)
	require.Len(t, result.Suggestions, 5)
	require.Equal(t, 5, catalog.calls)
	require.GreaterOrEqual(t, catalog.maxActive, 2)
	require.LessOrEqual(t, catalog.maxActive, ozonAttributeSuggestionDictConcurrency)
	for _, suggestion := range result.Suggestions {
		require.NotEmpty(t, suggestion.Values[0].DictionaryValueID)
	}
}

func plainOzonAttributeSuggestionAttrs(count int) []shop.OzonAttributeDTO {
	now := time.Now().UTC()
	attrs := make([]shop.OzonAttributeDTO, 0, count)
	for index := 1; index <= count; index++ {
		attrs = append(attrs, shop.OzonAttributeDTO{
			CategoryID: "100:200", AttrID: fmt.Sprintf("attr-%03d", index),
			Name: fmt.Sprintf("普通属性 %03d", index), ValueType: "String", SyncedAt: &now,
		})
	}
	return attrs
}

func filledOzonAttributeFixtureValuesExcept(attributeID string) map[string]json.RawMessage {
	values := map[string]json.RawMessage{}
	for _, id := range []string{"10-brand", "20-quantity", "30-auto", "50-url", "60-filled"} {
		if id != attributeID {
			values[id] = json.RawMessage(strconv.Quote("manual"))
		}
	}
	return values
}

func TestOzonAttributeSuggestionsSendsFullCategoryThreeSKUsAndTwoSafeVisionImages(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	for index, attrs := range []map[string]any{
		{"模式": "手动", "电压": "12V"},
		{"模式": "远程", "电压": "220V", "外壳": "金属"},
		{"模式": "定时", "电压": "24V", "外壳": "塑料"},
		{"模式": "脉冲", "电压": "48V", "外壳": "铝"},
	} {
		require.NoError(t, fixture.svc.DB.Create(&ProductSKU{
			ProductID: fixture.product.ID, SKUCode: fmt.Sprintf("EXTRA-%d", index+1), SKUName: fmt.Sprintf("扩展款 %d", index+1),
			Attrs: jsonBytes(t, attrs),
		}).Error)
	}
	require.NoError(t, fixture.svc.DB.Create(&[]ProductImage{
		{ProductID: fixture.product.ID, ImageType: ImageTypeMain, OriginURL: "https://cdn.example.test/main.jpg?width=1200", IsBestMain: true, SortOrder: 1},
		{ProductID: fixture.product.ID, ImageType: ImageTypeDetail, OriginURL: "https://cdn.example.test/same-position-copy.jpg", SortOrder: 1},
		{ProductID: fixture.product.ID, ImageType: ImageTypeDetail, OriginURL: "https://cdn.example.test/main.jpg?width=800", SortOrder: 2},
		{ProductID: fixture.product.ID, ImageType: ImageTypeDetail, OriginURL: "https://cdn.example.test/tiny-icon.jpg", SortOrder: 3},
		{ProductID: fixture.product.ID, ImageType: ImageTypeDetail, OriginURL: "https://cdn.example.test/wide-logistics-banner.jpg", SortOrder: 4},
		{ProductID: fixture.product.ID, ImageType: ImageTypeDetail, OriginURL: "https://cdn.example.test/square-service-card.jpg", SortOrder: 5},
		{ProductID: fixture.product.ID, ImageType: ImageTypeDetail, OriginURL: "https://cdn.example.test/product-detail.jpg", SortOrder: 6},
		{ProductID: fixture.product.ID, ImageType: ImageTypeMarketing, OriginURL: "https://private.example.test/pack.jpg?X-Amz-Signature=TEST_ONLY_SIGNATURE", SortOrder: 7},
		{ProductID: fixture.product.ID, ImageType: ImageTypeMarketing, OriginURL: "https://cdn.example.test/marketing.jpg", SortOrder: 8},
	}).Error)
	fixture.svc.ozonAttributeImageProbe = func(_ context.Context, imageURL string) (ozonAttributeSuggestionImageMetadata, error) {
		switch {
		case strings.Contains(imageURL, "tiny-icon"):
			return ozonAttributeSuggestionImageMetadata{width: 84, height: 82}, nil
		case strings.Contains(imageURL, "wide-logistics-banner"):
			return ozonAttributeSuggestionImageMetadata{width: 1600, height: 300}, nil
		case strings.Contains(imageURL, "square-service-card"):
			return ozonAttributeSuggestionImageMetadata{width: 790, height: 655}, nil
		case strings.Contains(imageURL, "product-detail"):
			return ozonAttributeSuggestionImageMetadata{width: 750, height: 920}, nil
		default:
			return ozonAttributeSuggestionImageMetadata{width: 1200, height: 1200}, nil
		}
	}
	fakeAI := &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{{Content: `{"suggestions":[]}`}}}
	fixture.svc.OzonAttributeAI = fakeAI

	_, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
	)
	require.NoError(t, err)
	factRequests := ozonAttributeFactRequests(fakeAI.requests)
	fillRequests := ozonAttributeFillRequests(fakeAI.requests)
	require.Len(t, factRequests, 1)
	require.Len(t, fillRequests, 2)
	require.Len(t, factRequests[0].Messages, 2)
	userMessage := factRequests[0].Messages[1]
	require.Equal(t, []string{
		"https://cdn.example.test/main.jpg?width=1200",
		"https://cdn.example.test/product-detail.jpg",
	}, userMessage.ImageURLs)
	require.Equal(t,
		canonicalOzonAttributeSuggestionImageKey("https://img.example.test/item.jpg"),
		canonicalOzonAttributeSuggestionImageKey("https://img.example.test/item.jpg_q50.jpg"),
	)
	require.Equal(t, 3, strings.Count(userMessage.Content, `"sourceRef":"sku.`))
	require.Contains(t, userMessage.Content, `"fullPath":"Electronics / Industrial automation / Industrial controllers"`)
	require.Contains(t, userMessage.Content, `"productType":"Industrial controllers"`)
	require.NotContains(t, userMessage.Content, "https://cdn.example.test")
	require.NotContains(t, userMessage.Content, "TEST_ONLY_SIGNATURE")
	require.NotContains(t, strings.Join(userMessage.ImageURLs, "\n"), "private.example.test")
	for _, request := range fillRequests {
		require.Empty(t, request.Messages[1].ImageURLs, "images are sent only to the fact extraction call")
	}
	require.Contains(t, fillRequests[0].Messages[0].Content, aiprompt.OzonAttributeSuggestionPolicyVersion)
	require.NotContains(t, fillRequests[0].Messages[0].Content, "没有证据就省略该属性")
}

func TestOzonAttributeSuggestionsWithoutProductTextStillUsesCategoryPolicyContext(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	emptyJSON := jsonBytes(t, map[string]any{})
	require.NoError(t, fixture.svc.DB.Model(&Product{}).
		Where("id = ?", fixture.product.ID).
		Updates(map[string]any{
			"title": "", "original_title": "", "description": "", "ai_description": "", "raw_data": emptyJSON,
		}).Error)
	require.NoError(t, fixture.svc.DB.Model(&ProductSKU{}).
		Where("product_id = ?", fixture.product.ID).
		Updates(map[string]any{"attrs": emptyJSON, "raw_data": emptyJSON}).Error)
	fakeAI := &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{{Content: `{"suggestions":[]}`}}}
	fixture.svc.OzonAttributeAI = fakeAI

	result, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
	)
	require.NoError(t, err)
	require.Equal(t, OzonAttributeSuggestionNoMatch, result.Status)
	require.Empty(t, result.Suggestions)
	require.Len(t, ozonAttributeFactRequests(fakeAI.requests), 1)
	require.Len(t, ozonAttributeFillRequests(fakeAI.requests), 2)
	require.Positive(t, result.Summary.NotFound)
	prompt := ozonAttributeRequestText(fakeAI.requests)
	require.Contains(t, prompt, `"fullPath":"Electronics / Industrial automation / Industrial controllers"`)
	require.Contains(t, prompt, `"common_knowledge"`)
	require.NotContains(t, prompt, fixture.product.Title)

	var task aitask.AITask
	require.NoError(t, fixture.svc.DB.First(&task, "id = ?", result.TaskID).Error)
	require.Equal(t, aitask.StatusSuccess, task.Status)
	require.Empty(t, task.RawResponse)
	require.NotContains(t, string(task.Input), fixture.product.Title)
}

func TestOzonAttributeSuggestionsProviderFailureKeepsInputsAndStoresRedactedAudit(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	fakeAI := &fakeOzonRecommendationAI{errors: []error{errors.New("provider detail TEST_ONLY_PROVIDER_SECRET")}}
	fixture.svc.OzonAttributeAI = fakeAI
	beforeUpdatedAt := fixture.product.UpdatedAt

	result, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
	)
	require.Error(t, err)
	require.Nil(t, result)
	require.Len(t, fakeAI.requests, 2)

	var task aitask.AITask
	require.NoError(t, fixture.svc.DB.Where("task_type = ?", ozonAttributeSuggestionTaskType).First(&task).Error)
	require.Equal(t, aitask.StatusFailed, task.Status)
	require.Equal(t, "provider request failed: all_batches_failed", task.ErrorMessage)
	require.Empty(t, task.RawResponse)
	require.NotContains(t, string(task.Input), "TEST_ONLY")
	var failureAudit map[string]any
	require.NoError(t, json.Unmarshal(task.Input, &failureAudit))
	require.Equal(t, float64(1), failureAudit["attributeBatchCount"])
	require.Equal(t, float64(2), failureAudit["aiCallCount"])
	require.Equal(t, false, failureAudit["requestDeadlinePresent"])

	var log operationlog.OperationLog
	require.NoError(t, fixture.svc.DB.Where("action = ?", "ai.ozon_attribute_suggestions.failed").First(&log).Error)
	require.NotContains(t, log.Message, "TEST_ONLY_PROVIDER_SECRET")
	var persisted Product
	require.NoError(t, fixture.svc.DB.First(&persisted, "id = ?", fixture.product.ID).Error)
	require.True(t, persisted.UpdatedAt.Equal(beforeUpdatedAt))
}

func TestOzonAttributeFactExtractionFailureStopsBeforeFieldBatches(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	fakeAI := &fakeOzonRecommendationAI{factError: errors.New("fake vision provider failed")}
	fixture.svc.OzonAttributeAI = fakeAI

	result, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
	)
	require.Error(t, err)
	require.Nil(t, result)
	require.Len(t, ozonAttributeFactRequests(fakeAI.requests), 1)
	require.Empty(t, ozonAttributeFillRequests(fakeAI.requests))
	var apiErr *ozonAttributeSuggestionAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.HTTPStatus())
}

func TestOzonAttributeMalformedFactJSONIsRepairedOnceWithoutRepeatingImages(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	require.NoError(t, fixture.svc.DB.Create(&ProductImage{
		ProductID: fixture.product.ID, ImageType: ImageTypeMain,
		OriginURL: "https://cdn.example.test/fact-repair.jpg", IsBestMain: true,
	}).Error)
	attrs := plainOzonAttributeSuggestionAttrs(3)
	fixture.attrs = attrs
	fixture.catalog.attrs["100:200"] = attrs
	fakeAI := &repairingFactOzonAttributeAI{base: &concurrentOzonAttributeAI{}}
	fixture.svc.OzonAttributeAI = fakeAI

	result, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
	)
	require.NoError(t, err)
	require.Len(t, result.Suggestions, 3)
	require.Len(t, fakeAI.factRequests, 2)
	require.NotEmpty(t, fakeAI.factRequests[0].Messages[1].ImageURLs)
	require.Empty(t, fakeAI.factRequests[1].Messages[1].ImageURLs)
	require.Contains(t, fakeAI.factRequests[1].Messages[0].Content, "只修复")

	var task aitask.AITask
	require.NoError(t, fixture.svc.DB.First(&task, "id = ?", result.TaskID).Error)
	var audit map[string]any
	require.NoError(t, json.Unmarshal(task.Input, &audit))
	require.Equal(t, float64(1), audit["factRepairCount"])
	require.Equal(t, float64(1), audit["attributeBatchCount"])
	require.Equal(t, float64(3), audit["aiCallCount"])
	require.Empty(t, task.RawResponse)
}

func TestOzonAttributeSuggestionsDelegatesProviderTimeoutToAIGateway(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	fakeAI := &deadlineCapturingOzonAttributeAI{response: &aigate.ChatResponse{Content: `{"suggestions":[]}`}}
	fixture.svc.OzonAttributeAI = fakeAI

	result, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, fakeAI.hasDeadline, "service must not replace AIGateway's completion-aware timeout with a shorter deadline")
}

func TestOzonAttributeSuggestionsRejectUntrustedCurrentValuesBeforeTaskOrAI(t *testing.T) {
	tests := []struct {
		name    string
		current OzonAttributeSuggestionEditorValues
	}{
		{name: "unknown attribute", current: OzonAttributeSuggestionEditorValues{Attributes: map[string]json.RawMessage{"unknown": json.RawMessage(`"value"`)}}},
		{name: "invalid value shape", current: OzonAttributeSuggestionEditorValues{Attributes: map[string]json.RawMessage{"20-quantity": json.RawMessage(`{"value":"12"}`)}}},
		{name: "too many values for scalar", current: OzonAttributeSuggestionEditorValues{Attributes: map[string]json.RawMessage{"20-quantity": json.RawMessage(`["12","13"]`)}}},
		{name: "unknown variant", current: OzonAttributeSuggestionEditorValues{SKUVariantAttributeIDs: []string{"unknown"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := setupOzonAttributeSuggestionFixture(t)
			fakeAI := &fakeOzonRecommendationAI{}
			fixture.svc.OzonAttributeAI = fakeAI
			body := suggestionBody(fixture, nil)
			body.CurrentValues = tt.current
			_, err := fixture.svc.SuggestOzonAttributes(tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, body, nil)
			require.Error(t, err)
			require.Empty(t, fakeAI.requests)
			var taskCount int64
			require.NoError(t, fixture.svc.DB.Model(&aitask.AITask{}).Count(&taskCount).Error)
			require.Zero(t, taskCount)
		})
	}
}

func TestOzonAttributeSuggestionsRejectNonLeafOrInactiveCategoryBeforeTaskOrAI(t *testing.T) {
	tests := []struct {
		name   string
		leaf   bool
		status string
	}{
		{name: "non leaf", leaf: false, status: "active"},
		{name: "inactive", leaf: true, status: "inactive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := setupOzonAttributeSuggestionFixture(t)
			require.NoError(t, fixture.svc.DB.Model(&shop.PlatformCategory{}).
				Where("platform = ? AND category_id = ?", "ozon", "100:200").
				Updates(map[string]any{"is_leaf": tt.leaf, "status": tt.status}).Error)
			fakeAI := &fakeOzonRecommendationAI{}
			fixture.svc.OzonAttributeAI = fakeAI
			_, err := fixture.svc.SuggestOzonAttributes(
				tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
			)
			require.Error(t, err)
			require.Empty(t, fakeAI.requests)
			var taskCount int64
			require.NoError(t, fixture.svc.DB.Model(&aitask.AITask{}).Count(&taskCount).Error)
			require.Zero(t, taskCount)
		})
	}
}

func TestOzonAttributeSuggestionsRejectStaleTemplateFingerprintBeforeTaskOrAI(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	fakeAI := &fakeOzonRecommendationAI{}
	fixture.svc.OzonAttributeAI = fakeAI
	body := suggestionBody(fixture, nil)
	body.TemplateFingerprint = "stale-template"

	_, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, body, nil,
	)
	require.Error(t, err)
	var apiErr *ozonAttributeSuggestionAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusConflict, apiErr.HTTPStatus())
	require.Equal(t, OzonAttributeSuggestionContextStale, apiErr.code)
	require.Empty(t, fakeAI.requests)
	var taskCount int64
	require.NoError(t, fixture.svc.DB.Model(&aitask.AITask{}).Count(&taskCount).Error)
	require.Zero(t, taskCount)
}

func TestOzonAttributeSuggestionHTTPRejectsReadonlyTenantAndShopBeforeAI(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	fakeAI := &fakeOzonRecommendationAI{}
	fixture.svc.OzonAttributeAI = fakeAI
	handler := &Handler{Svc: fixture.svc}
	body, err := json.Marshal(suggestionBody(fixture, nil))
	require.NoError(t, err)

	call := func(principal *adminperm.Principal, tenantID int64, productID, shopID uuid.UUID) *httptest.ResponseRecorder {
		var requestBody OzonAttributeSuggestionBody
		require.NoError(t, json.Unmarshal(body, &requestBody))
		requestBody.ShopID = shopID.String()
		raw, marshalErr := json.Marshal(requestBody)
		require.NoError(t, marshalErr)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/products/"+productID.String()+"/ai/ozon-attribute-suggestions", bytes.NewReader(raw))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: productID.String()}}
		c.Set(ctxkey.TenantID, tenantID)
		c.Set("adminperm.principal", principal)
		handler.SuggestOzonAttributesHTTP(c)
		return w
	}

	readonly := &adminperm.Principal{TenantID: 1, Role: adminperm.RoleReadonly, Permissions: adminperm.PermissionsForRole(adminperm.RoleReadonly)}
	require.Equal(t, http.StatusForbidden, call(readonly, 1, fixture.product.ID, fixture.shopID).Code)
	tenantAdmin := &adminperm.Principal{TenantID: 2, Role: adminperm.RoleTenantAdmin, Permissions: adminperm.PermissionsForRole(adminperm.RoleTenantAdmin)}
	require.Equal(t, http.StatusNotFound, call(tenantAdmin, 2, fixture.product.ID, fixture.shopID).Code)
	tenantAdmin = &adminperm.Principal{TenantID: 1, Role: adminperm.RoleTenantAdmin, Permissions: adminperm.PermissionsForRole(adminperm.RoleTenantAdmin)}
	require.Equal(t, http.StatusNotFound, call(tenantAdmin, 1, fixture.product.ID, uuid.New()).Code)
	require.Empty(t, fakeAI.requests)
	var taskCount int64
	require.NoError(t, fixture.svc.DB.Model(&aitask.AITask{}).Count(&taskCount).Error)
	require.Zero(t, taskCount)
}

func TestOzonAttributeSuggestionHTTPSuccessUsesEnvelopeAndRouteIsRegistered(t *testing.T) {
	fixture := setupOzonAttributeSuggestionFixture(t)
	fixture.svc.OzonAttributeAI = &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{{
		Content: `{"suggestions":[{"attributeKey":"attribute_1","values":["Acme"],"confidence":0.9,"reason":"title","sourceRefs":["product.title"]}]}`,
	}}}
	handler := &Handler{Svc: fixture.svc}
	engine := gin.New()
	Register(engine.Group("/api/v1"), handler)
	found := false
	for _, route := range engine.Routes() {
		if route.Method == http.MethodPost && route.Path == "/api/v1/products/:id/ai/ozon-attribute-suggestions" {
			found = true
		}
	}
	require.True(t, found)

	raw, err := json.Marshal(suggestionBody(fixture, nil))
	require.NoError(t, err)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/products/"+fixture.product.ID.String()+"/ai/ozon-attribute-suggestions", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: fixture.product.ID.String()}}
	c.Set(ctxkey.TenantID, int64(1))
	c.Set("adminperm.principal", &adminperm.Principal{TenantID: 1, Role: adminperm.RoleTenantAdmin, Permissions: adminperm.PermissionsForRole(adminperm.RoleTenantAdmin)})
	handler.SuggestOzonAttributesHTTP(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var envelope struct {
		Code int                           `json:"code"`
		Data OzonAttributeSuggestionResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, response.CodeOK, envelope.Code)
	require.NotEmpty(t, envelope.Data.Context.Fingerprint)
	require.Len(t, envelope.Data.Suggestions, 1)
}

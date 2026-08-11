package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
)

type deadlineCapturingOzonAttributeAI struct {
	response    *aigate.ChatResponse
	hasDeadline bool
}

func (f *deadlineCapturingOzonAttributeAI) Chat(ctx context.Context, _ aigate.ChatRequest) (*aigate.ChatResponse, error) {
	_, f.hasDeadline = ctx.Deadline()
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
	require.Equal(t, OzonAttributeSuggestionSummary{Filled: 2, RequiresReview: 1, NotFound: 3}, result.Summary)
	require.Len(t, result.Suggestions, 2)
	require.Equal(t, "10", result.Suggestions[0].Values[0].DictionaryValueID)
	require.Equal(t, "Acme", result.Suggestions[0].Values[0].Value)
	require.NotContains(t, result.Suggestions[0].Reason, "TEST_ONLY_MODEL_SECRET")
	require.Equal(t, "30-auto", result.Suggestions[1].AttributeID)
	require.Equal(t, "low", result.Suggestions[1].ConfidenceLevel)
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

	require.Len(t, fakeAI.requests, 1)
	prompt := fakeAI.requests[0].Messages[0].Content + fakeAI.requests[0].Messages[1].Content
	require.NotContains(t, prompt, "TEST_ONLY_PRODUCT_TOKEN")
	require.NotContains(t, prompt, "TEST_ONLY_SKU_SECRET")
	require.NotContains(t, prompt, "TEST_ONLY_DESCRIPTION_SECRET")
	require.NotContains(t, prompt, "private.example.test")
	require.NotContains(t, prompt, "库存")
	require.NotContains(t, prompt, "用户原值")

	var task aitask.AITask
	require.NoError(t, fixture.svc.DB.First(&task, "id = ?", result.TaskID).Error)
	require.Equal(t, aitask.StatusSuccess, task.Status)
	require.Equal(t, "fake-attributes", task.Model)
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
	require.Len(t, fakeAI.requests, 1)
	prompt := fakeAI.requests[0].Messages[1].Content
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
		{ProductID: fixture.product.ID, ImageType: ImageTypeDetail, OriginURL: "https://cdn.example.test/main.jpg?width=800", SortOrder: 2},
		{ProductID: fixture.product.ID, ImageType: ImageTypeDetail, OriginURL: "https://cdn.example.test/detail.jpg", SortOrder: 3},
		{ProductID: fixture.product.ID, ImageType: ImageTypeMarketing, OriginURL: "https://private.example.test/pack.jpg?X-Amz-Signature=TEST_ONLY_SIGNATURE", SortOrder: 4},
		{ProductID: fixture.product.ID, ImageType: ImageTypeMarketing, OriginURL: "https://cdn.example.test/marketing.jpg", SortOrder: 5},
	}).Error)
	fakeAI := &fakeOzonRecommendationAI{responses: []*aigate.ChatResponse{{Content: `{"suggestions":[]}`}}}
	fixture.svc.OzonAttributeAI = fakeAI

	_, err := fixture.svc.SuggestOzonAttributes(
		tenantProductAdminContext(t, fixture.svc, 1), fixture.product.ID, suggestionBody(fixture, nil), nil,
	)
	require.NoError(t, err)
	require.Len(t, fakeAI.requests, 1)
	require.Len(t, fakeAI.requests[0].Messages, 2)
	userMessage := fakeAI.requests[0].Messages[1]
	require.Equal(t, []string{
		"https://cdn.example.test/main.jpg?width=1200",
		"https://cdn.example.test/detail.jpg",
	}, userMessage.ImageURLs)
	require.Equal(t, 3, strings.Count(userMessage.Content, `"sourceRef":"sku.`))
	require.Contains(t, userMessage.Content, `"fullPath":"Electronics / Industrial automation / Industrial controllers"`)
	require.Contains(t, userMessage.Content, `"productType":"Industrial controllers"`)
	require.NotContains(t, userMessage.Content, "https://cdn.example.test")
	require.NotContains(t, userMessage.Content, "TEST_ONLY_SIGNATURE")
	require.NotContains(t, strings.Join(userMessage.ImageURLs, "\n"), "private.example.test")
	require.Contains(t, fakeAI.requests[0].Messages[0].Content, aiprompt.OzonAttributeSuggestionPolicyVersion)
	require.NotContains(t, fakeAI.requests[0].Messages[0].Content, "没有证据就省略该属性")
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
	require.Len(t, fakeAI.requests, 1)
	require.Positive(t, result.Summary.NotFound)
	prompt := fakeAI.requests[0].Messages[1].Content
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
	require.Len(t, fakeAI.requests, 1)

	var task aitask.AITask
	require.NoError(t, fixture.svc.DB.Where("task_type = ?", ozonAttributeSuggestionTaskType).First(&task).Error)
	require.Equal(t, aitask.StatusFailed, task.Status)
	require.Equal(t, "provider request failed", task.ErrorMessage)
	require.Empty(t, task.RawResponse)
	require.NotContains(t, string(task.Input), "TEST_ONLY")

	var log operationlog.OperationLog
	require.NoError(t, fixture.svc.DB.Where("action = ?", "ai.ozon_attribute_suggestions.failed").First(&log).Error)
	require.NotContains(t, log.Message, "TEST_ONLY_PROVIDER_SECRET")
	var persisted Product
	require.NoError(t, fixture.svc.DB.First(&persisted, "id = ?", fixture.product.ID).Error)
	require.True(t, persisted.UpdatedAt.Equal(beforeUpdatedAt))
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

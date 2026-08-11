package product

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	OzonAttributeSuggestionReady   = "ready"
	OzonAttributeSuggestionPartial = "partial"
	OzonAttributeSuggestionNoMatch = "no_match"

	OzonAttributeSuggestionInvalid      = "OZON_ATTRIBUTE_SUGGESTION_INVALID"
	OzonAttributeSuggestionContextStale = "OZON_ATTRIBUTE_SUGGESTION_CONTEXT_STALE"
	OzonAttributeSuggestionUnavailable  = "OZON_ATTRIBUTE_SUGGESTION_UNAVAILABLE"
	OzonAttributeSuggestionAIFailed     = "OZON_ATTRIBUTE_SUGGESTION_AI_FAILED"

	ozonAttributeSuggestionTaskType        = "ozon_attribute_suggestions"
	ozonAttributeSuggestionMediumThreshold = 0.55
	ozonAttributeSuggestionHighThreshold   = 0.80
	ozonAttributeSuggestionMaxAttributes   = 120
	ozonAttributeSuggestionMaxOptions      = 100
	ozonAttributeSuggestionMaxProductAttrs = 40
	ozonAttributeSuggestionMaxSKUAttrs     = 40
	ozonAttributeSuggestionMaxSKUs         = 3
	ozonAttributeSuggestionMaxImages       = 2
)

var (
	ozonAttributeSuggestionURLPattern    = regexp.MustCompile(`(?i)(https?://|data:)[^\t\r\n <>"']+`)
	ozonAttributeSuggestionSecretPattern = regexp.MustCompile(
		`(?i)(^|[^a-z0-9])(api[_-]?(key|token)|access[_-]?(key|token)|refresh[_-]?token|client[_-]?secret|app[_-]?secret|secret[_-]?key|private[_-]?key|shop[_-]?secret|authorization|password|passwd|cookie|credential|session[_-]?id|bearer)([^a-z0-9]|$)`,
	)
	ozonAttributeSuggestionNonAlnumPattern = regexp.MustCompile(`[^a-z0-9]+`)
)

type OzonAttributeSuggestionBody struct {
	ShopID              string                              `json:"shopId"`
	CategoryID          string                              `json:"categoryId"`
	TemplateFingerprint string                              `json:"templateFingerprint"`
	CurrentValues       OzonAttributeSuggestionEditorValues `json:"currentValues"`
}

// OzonAttributeSuggestionEditorValues mirrors only the editor branches needed
// to decide whether a product-level attribute is blank. Complex and per-SKU
// values remain outside this MVP and are never sent to the model.
type OzonAttributeSuggestionEditorValues struct {
	Attributes             map[string]json.RawMessage `json:"attributes"`
	SKUVariantAttributeIDs []string                   `json:"skuVariantAttributeIds,omitempty"`
}

type OzonAttributeSuggestionContext struct {
	ProductID           uuid.UUID `json:"productId"`
	ProductUpdatedAt    time.Time `json:"productUpdatedAt"`
	ShopID              uuid.UUID `json:"shopId"`
	CategoryID          string    `json:"categoryId"`
	TemplateFingerprint string    `json:"templateFingerprint"`
	Fingerprint         string    `json:"fingerprint"`
}

type OzonAttributeSuggestion struct {
	AttributeID     string                   `json:"attributeId"`
	AttributeName   string                   `json:"attributeName"`
	Values          []OzonAttributeSelection `json:"values"`
	Confidence      float64                  `json:"confidence"`
	ConfidenceLevel string                   `json:"confidenceLevel"`
	RequiresReview  bool                     `json:"requiresReview"`
	Reason          string                   `json:"reason,omitempty"`
	SourceRefs      []string                 `json:"sourceRefs,omitempty"`
}

type OzonAttributeSuggestionSkipped struct {
	AttributeID   string `json:"attributeId"`
	AttributeName string `json:"attributeName"`
	Reason        string `json:"reason"`
}

type OzonAttributeSuggestionSummary struct {
	Filled         int `json:"filled"`
	RequiresReview int `json:"requiresReview"`
	NotFound       int `json:"notFound"`
}

type OzonAttributeSuggestionResult struct {
	Status      string                           `json:"status"`
	TaskID      *uuid.UUID                       `json:"taskId,omitempty"`
	Context     OzonAttributeSuggestionContext   `json:"context"`
	Suggestions []OzonAttributeSuggestion        `json:"suggestions"`
	Skipped     []OzonAttributeSuggestionSkipped `json:"skipped"`
	Summary     OzonAttributeSuggestionSummary   `json:"summary"`
	Warnings    []string                         `json:"warnings"`
}

type ozonAttributeSuggestionAPIError struct {
	status  int
	code    string
	message string
	err     error
}

func (e *ozonAttributeSuggestionAPIError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *ozonAttributeSuggestionAPIError) Unwrap() error       { return e.err }
func (e *ozonAttributeSuggestionAPIError) HTTPStatus() int     { return e.status }
func (e *ozonAttributeSuggestionAPIError) SafeMessage() string { return e.message }
func (e *ozonAttributeSuggestionAPIError) SafeData() any {
	return map[string]any{"errorCode": e.code}
}

func invalidOzonAttributeSuggestion(message string, err error) error {
	return &ozonAttributeSuggestionAPIError{status: http.StatusBadRequest, code: OzonAttributeSuggestionInvalid, message: message, err: err}
}

func staleOzonAttributeSuggestion(message string, err error) error {
	return &ozonAttributeSuggestionAPIError{status: http.StatusConflict, code: OzonAttributeSuggestionContextStale, message: message, err: err}
}

func unavailableOzonAttributeSuggestion(message string, err error) error {
	return &ozonAttributeSuggestionAPIError{status: http.StatusServiceUnavailable, code: OzonAttributeSuggestionUnavailable, message: message, err: err}
}

func failedOzonAttributeSuggestion(message string, err error) error {
	return &ozonAttributeSuggestionAPIError{status: http.StatusBadGateway, code: OzonAttributeSuggestionAIFailed, message: message, err: err}
}

type ozonAttributePromptCategory struct {
	FullPath    string `json:"fullPath"`
	Name        string `json:"name"`
	ProductType string `json:"productType"`
}

type ozonAttributePromptSKU struct {
	SourceRef  string            `json:"sourceRef"`
	SKUCode    string            `json:"skuCode,omitempty"`
	SKUName    string            `json:"skuName,omitempty"`
	Attributes map[string]string `json:"attributes"`
}

type ozonAttributePromptImage struct {
	SourceRef string `json:"sourceRef"`
	Role      string `json:"role"`
}

type ozonAttributePromptContext struct {
	ProductTitle       string                      `json:"productTitle,omitempty"`
	ProductDescription string                      `json:"productDescription,omitempty"`
	ProductAttributes  map[string]string           `json:"productAttributes"`
	Category           ozonAttributePromptCategory `json:"category"`
	RepresentativeSKUs []ozonAttributePromptSKU    `json:"representativeSkus"`
	Images             []ozonAttributePromptImage  `json:"images"`
	AllowedSourceRefs  []string                    `json:"allowedSourceRefs"`
}

type ozonAttributePromptCandidate struct {
	AttributeKey      string   `json:"attributeKey"`
	Name              string   `json:"name"`
	Description       string   `json:"description,omitempty"`
	ValueType         string   `json:"valueType"`
	IsCollection      bool     `json:"isCollection"`
	MaxValueCount     int64    `json:"maxValueCount,omitempty"`
	DictionaryOptions []string `json:"dictionaryOptions,omitempty"`
}

type ozonAttributeAICandidate struct {
	AttributeKey string   `json:"attributeKey"`
	Values       []string `json:"values"`
	Confidence   float64  `json:"confidence"`
	Reason       string   `json:"reason"`
	SourceRefs   []string `json:"sourceRefs,omitempty"`
	// EvidenceKeys accepts only the known v1 field while persisted prompts are
	// migrated. Values still have to belong to the v2 source-ref allowlist.
	EvidenceKeys []string `json:"evidenceKeys,omitempty"`
}

type ozonAttributeAIOutput struct {
	Suggestions []ozonAttributeAICandidate `json:"suggestions"`
}

type ozonAttributeSuggestionCandidate struct {
	key     string
	attr    shop.OzonAttributeDTO
	options []ozonAttributeDictionaryOption
}

type ozonAttributeDictionaryOption struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// SuggestOzonAttributes returns validated suggestions only. It never changes a
// product, platform configuration, readiness result, or Ozon state.
func (s *Service) SuggestOzonAttributes(
	c *gin.Context,
	productID uuid.UUID,
	body OzonAttributeSuggestionBody,
	adminID *uuid.UUID,
) (*OzonAttributeSuggestionResult, error) {
	if s == nil || s.DB == nil {
		return nil, unavailableOzonAttributeSuggestion("AI 属性建议服务暂不可用", fmt.Errorf("product: no db"))
	}
	if s.OzonCategories == nil {
		return nil, unavailableOzonAttributeSuggestion("Ozon 类目属性服务暂不可用", fmt.Errorf("ozon catalog not configured"))
	}
	if s.AITasks == nil {
		return nil, unavailableOzonAttributeSuggestion("AI 审计服务暂不可用", fmt.Errorf("ai task audit not configured"))
	}

	// Match the existing Ozon platform-config edit boundary: the product must be
	// visible in this tenant and the selected Ozon shop must be operable by the
	// current principal. All checks happen before an AI task or provider call.
	productRow, err := s.findTenantProduct(c, productID, "SKUs", "Images")
	if err != nil {
		return nil, err
	}
	shopID, err := uuid.Parse(strings.TrimSpace(body.ShopID))
	if err != nil || shopID == uuid.Nil {
		return nil, invalidOzonAttributeSuggestion("shopId 必须是有效的 Ozon 店铺 ID", err)
	}
	if err := s.requirePlatformShopOperate(c, shopID, "ozon"); err != nil {
		return nil, err
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if err := s.OzonCategories.EnsureAuthorizedOzonShop(c.Request.Context(), tenantID, shopID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, unavailableOzonAttributeSuggestion("Ozon 店铺授权状态暂无法核对", err)
	}

	categoryID := strings.TrimSpace(body.CategoryID)
	if categoryID == "" || len(categoryID) > 128 {
		return nil, invalidOzonAttributeSuggestion("categoryId 必须是当前 Ozon 叶子类目 ID", nil)
	}
	var category shop.PlatformCategory
	if err := s.DB.WithContext(c.Request.Context()).
		Where("platform = ? AND category_id = ?", "ozon", categoryID).
		First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, unavailableOzonAttributeSuggestion("当前 Ozon 类目暂无法核对", err)
	}
	if !category.IsLeaf || !strings.EqualFold(strings.TrimSpace(category.Status), "active") {
		return nil, invalidOzonAttributeSuggestion("categoryId 必须是当前有效的 Ozon 叶子类目 ID", nil)
	}
	attrs, err := s.OzonCategories.ListOzonCategoryAttributes(c.Request.Context(), categoryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, unavailableOzonAttributeSuggestion("当前 Ozon 类目属性模板暂无法读取", err)
	}
	if len(attrs) == 0 {
		return nil, staleOzonAttributeSuggestion("当前 Ozon 类目属性模板为空，请刷新模板后重试", nil)
	}
	if len(attrs) > ozonAttributeSuggestionMaxAttributes {
		return nil, unavailableOzonAttributeSuggestion("当前 Ozon 属性模板超过 AI 建议安全上限，请手动填写", nil)
	}
	for _, attr := range attrs {
		if attr.CacheStale {
			return nil, staleOzonAttributeSuggestion("当前 Ozon 类目属性模板已过期，请刷新模板后重试", nil)
		}
		if attr.CategoryID != "" && strings.TrimSpace(attr.CategoryID) != categoryID {
			return nil, staleOzonAttributeSuggestion("Ozon 类目属性模板上下文不一致，请重新加载", nil)
		}
	}
	templateFingerprint := shop.OzonCategoryAttributeSchemaHash(attrs)
	requestedFingerprint := strings.TrimSpace(body.TemplateFingerprint)
	if requestedFingerprint == "" {
		return nil, invalidOzonAttributeSuggestion("templateFingerprint 必填，请重新加载当前属性模板", nil)
	}
	if requestedFingerprint != templateFingerprint {
		return nil, staleOzonAttributeSuggestion("当前 Ozon 属性模板已变化，旧请求已拒绝，请重新加载后再试", nil)
	}

	filled, selectedVariants, err := validateOzonSuggestionCurrentValues(attrs, body.CurrentValues)
	if err != nil {
		return nil, invalidOzonAttributeSuggestion("当前未保存属性值未通过模板校验："+err.Error(), err)
	}
	contextInfo := newOzonAttributeSuggestionContext(*productRow, shopID, categoryID, templateFingerprint)
	result := &OzonAttributeSuggestionResult{
		Status:      OzonAttributeSuggestionNoMatch,
		Context:     contextInfo,
		Suggestions: []OzonAttributeSuggestion{}, Skipped: []OzonAttributeSuggestionSkipped{}, Warnings: []string{},
	}
	candidates, promptCandidates, initialSkipped := buildOzonAttributeSuggestionCandidates(attrs, filled, selectedVariants, len(productRow.SKUs))
	result.Skipped = append(result.Skipped, initialSkipped...)
	promptContext, imageURLs := buildOzonAttributeSuggestionPromptContext(*productRow, category, s.DB, c.Request.Context())

	auditInput, _ := json.Marshal(map[string]any{
		"productId": productID.String(), "shopId": shopID.String(), "categoryId": categoryID,
		"templateFingerprint": templateFingerprint, "blankCandidateCount": len(candidates), "skippedCount": len(initialSkipped),
		"representativeSkuCount": len(promptContext.RepresentativeSKUs), "imageCount": len(imageURLs),
		"sourceRefCount": len(promptContext.AllowedSourceRefs),
	})
	task := &aitask.AITask{
		TenantID: tenantID, TaskType: ozonAttributeSuggestionTaskType,
		Provider: s.providerNameFromSettings(c), PromptCode: aiprompt.CodeOzonAttributeSuggestions,
		Input: datatypes.JSON(auditInput), ProductID: &productID, CreatedBy: adminID,
	}
	if err := s.AITasks.Create(c.Request.Context(), task); err != nil {
		return nil, unavailableOzonAttributeSuggestion("无法建立 AI 属性建议审计记录", err)
	}
	result.TaskID = &task.ID

	if len(candidates) == 0 {
		result.Warnings = append(result.Warnings, "当前没有可安全填写的普通商品级空白属性")
		result.Summary.NotFound = len(result.Skipped)
		s.finishOzonAttributeSuggestionAudit(c, adminID, task.ID, productID, result, 0, 0, "")
		return result, nil
	}
	if s.Prompts == nil {
		_ = s.AITasks.MarkFailed(c.Request.Context(), task.ID, "prompt service unavailable")
		s.writeOzonAttributeSuggestionLog(c, adminID, productID, task.ID, "failed", "prompt_unavailable", OzonAttributeSuggestionSummary{})
		return nil, unavailableOzonAttributeSuggestion("AI 属性建议提示词服务暂不可用", nil)
	}
	promptRow, err := s.Prompts.GetEnabledByCode(c.Request.Context(), aiprompt.CodeOzonAttributeSuggestions)
	if err != nil {
		_ = s.AITasks.MarkFailed(c.Request.Context(), task.ID, "prompt missing or disabled")
		s.writeOzonAttributeSuggestionLog(c, adminID, productID, task.ID, "failed", "prompt_unavailable", OzonAttributeSuggestionSummary{})
		return nil, unavailableOzonAttributeSuggestion("AI 属性建议提示词未启用", err)
	}
	client := s.OzonAttributeAI
	if client == nil && s.AIGateway != nil {
		client = s.AIGateway
	}
	if client == nil {
		_ = s.AITasks.MarkFailed(c.Request.Context(), task.ID, "ai client unavailable")
		s.writeOzonAttributeSuggestionLog(c, adminID, productID, task.ID, "failed", "ai_unavailable", OzonAttributeSuggestionSummary{})
		return nil, unavailableOzonAttributeSuggestion("AI 属性建议服务未配置", nil)
	}

	contextJSON, _ := json.Marshal(promptContext)
	attributesJSON, _ := json.Marshal(promptCandidates)
	sourceRefsJSON, _ := json.Marshal(promptContext.AllowedSourceRefs)
	vars := map[string]string{
		// evidence remains as a compatibility alias for custom persisted v1
		// prompts. It carries the same bounded v2 context, never current values.
		"evidence": string(contextJSON), "context": string(contextJSON),
		"attributes": string(attributesJSON), "sourceRefs": string(sourceRefsJSON),
	}
	maxTokens := promptRow.MaxTokens
	if maxTokens < 512 {
		maxTokens = 512
	}
	if maxTokens > 4096 {
		maxTokens = 4096
	}
	storedSystemPrompt := aiprompt.ReplaceVariables(promptRow.SystemPrompt, vars)
	runtimePolicy := aiprompt.OzonAttributeSuggestionRuntimePolicy()
	req := aigate.ChatRequest{
		Model: strings.TrimSpace(promptRow.Model),
		Messages: []aigate.Message{
			{Role: "system", Content: storedSystemPrompt + "\n\n" + runtimePolicy},
			{Role: "user", Content: aiprompt.ReplaceVariables(promptRow.UserPrompt, vars), ImageURLs: imageURLs},
		},
		Temperature: promptRow.Temperature, MaxTokens: maxTokens,
		ResponseFormat: &aigate.ResponseFormat{Type: "json_object"},
	}
	// AIGateway owns the bounded provider timeout (including its completion-size
	// floor and the configured timeout_sec). Adding a shorter service deadline
	// here can cancel otherwise healthy large prompts before the gateway policy
	// has a chance to complete them. The incoming request context still carries
	// client cancellation through to the provider.
	resp, err := client.Chat(c.Request.Context(), req)
	if err != nil || resp == nil {
		_ = s.AITasks.MarkFailed(c.Request.Context(), task.ID, "provider request failed")
		s.writeOzonAttributeSuggestionLog(c, adminID, productID, task.ID, "failed", "provider_failed", OzonAttributeSuggestionSummary{})
		return nil, failedOzonAttributeSuggestion("AI 填写失败，现有输入未变更，请稍后手动重试", err)
	}
	var modelOutput ozonAttributeAIOutput
	if err := decodeStrictOzonRecommendationJSON(resp.Content, &modelOutput); err != nil || len(modelOutput.Suggestions) > ozonAttributeSuggestionMaxAttributes*2 {
		_ = s.AITasks.MarkFailedWithMeta(c.Request.Context(), task.ID, "model output rejected", nil, resp.InputTokens, resp.OutputTokens, resp.Model)
		s.writeOzonAttributeSuggestionLog(c, adminID, productID, task.ID, "failed", "invalid_model_output", OzonAttributeSuggestionSummary{})
		return nil, failedOzonAttributeSuggestion("AI 返回结果未通过格式校验，现有输入未变更", err)
	}

	applyOzonAttributeAIOutput(result, candidates, promptContext.AllowedSourceRefs, modelOutput)
	usedModel := strings.TrimSpace(resp.Model)
	if usedModel == "" {
		usedModel = strings.TrimSpace(promptRow.Model)
	}
	s.finishOzonAttributeSuggestionAudit(c, adminID, task.ID, productID, result, resp.InputTokens, resp.OutputTokens, usedModel)
	return result, nil
}

func newOzonAttributeSuggestionContext(productRow Product, shopID uuid.UUID, categoryID, templateFingerprint string) OzonAttributeSuggestionContext {
	updatedAt := productRow.UpdatedAt.UTC()
	raw := strings.Join([]string{productRow.ID.String(), updatedAt.Format(time.RFC3339Nano), shopID.String(), categoryID, templateFingerprint}, "\n")
	sum := sha256.Sum256([]byte(raw))
	return OzonAttributeSuggestionContext{
		ProductID: productRow.ID, ProductUpdatedAt: updatedAt, ShopID: shopID,
		CategoryID: categoryID, TemplateFingerprint: templateFingerprint, Fingerprint: fmt.Sprintf("%x", sum[:]),
	}
}

func validateOzonSuggestionCurrentValues(
	attrs []shop.OzonAttributeDTO,
	current OzonAttributeSuggestionEditorValues,
) (map[string]bool, map[string]bool, error) {
	byID := make(map[string]shop.OzonAttributeDTO, len(attrs))
	for _, attr := range attrs {
		id := strings.TrimSpace(attr.AttrID)
		if id == "" || byID[id].AttrID != "" {
			return nil, nil, fmt.Errorf("属性模板包含空或重复 attrId")
		}
		byID[id] = attr
	}
	selectedVariants := map[string]bool{}
	for _, rawID := range current.SKUVariantAttributeIDs {
		id := strings.TrimSpace(rawID)
		_, ok := byID[id]
		if !ok {
			return nil, nil, fmt.Errorf("SKU 变体属性 %s 已不在当前模板中", id)
		}
		if selectedVariants[id] {
			return nil, nil, fmt.Errorf("SKU 变体属性 %s 重复", id)
		}
		// The suggestion endpoint does not apply or save variant configuration.
		// A current template member is sufficient to keep that field out of the
		// product-level AI candidate set; save/preflight owns eligibility checks.
		selectedVariants[id] = true
	}
	filled := map[string]bool{}
	for rawID, rawValue := range current.Attributes {
		id := strings.TrimSpace(rawID)
		attr, ok := byID[id]
		if !ok {
			return nil, nil, fmt.Errorf("属性 %s 已不在当前模板中", id)
		}
		values, err := decodeOzonSuggestionEditorValues(rawValue)
		if err != nil {
			return nil, nil, fmt.Errorf("属性 %s：%w", attr.Name, err)
		}
		if len(values) == 0 {
			continue
		}
		if !attr.IsCollection && len(values) > 1 {
			return nil, nil, fmt.Errorf("属性 %s 不是多值属性", attr.Name)
		}
		if attr.MaxValueCount > 0 && int64(len(values)) > attr.MaxValueCount {
			return nil, nil, fmt.Errorf("属性 %s 最多允许 %d 个值", attr.Name, attr.MaxValueCount)
		}
		if len(values) > 50 {
			return nil, nil, fmt.Errorf("属性 %s 的值数量超过安全上限", attr.Name)
		}
		// Current editor values are untrusted, but only their bounded non-empty
		// shape is used as opaque "filled" state. They are never sent to the
		// model, echoed in the response, saved, or published. This deliberately
		// avoids blocking suggestions for other blanks while a user is midway
		// through editing a value. Strict type and dictionary membership checks
		// remain mandatory for every AI-produced value below.
		filled[id] = true
	}
	return filled, selectedVariants, nil
}

func decodeOzonSuggestionEditorValues(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var single string
	if json.Unmarshal(raw, &single) == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			return nil, nil
		}
		return []string{single}, nil
	}
	var multiple []string
	if json.Unmarshal(raw, &multiple) != nil {
		return nil, fmt.Errorf("值必须是字符串或字符串数组")
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(multiple))
	for _, value := range multiple {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("多值属性不能包含空值")
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out, nil
}

func buildOzonAttributeSuggestionCandidates(
	attrs []shop.OzonAttributeDTO,
	filled map[string]bool,
	selectedVariants map[string]bool,
	skuCount int,
) ([]ozonAttributeSuggestionCandidate, []ozonAttributePromptCandidate, []OzonAttributeSuggestionSkipped) {
	ordered := append([]shop.OzonAttributeDTO(nil), attrs...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].AttrID < ordered[j].AttrID })
	candidates := make([]ozonAttributeSuggestionCandidate, 0, len(ordered))
	prompt := make([]ozonAttributePromptCandidate, 0, len(ordered))
	skipped := make([]OzonAttributeSuggestionSkipped, 0)
	for _, attr := range ordered {
		if filled[attr.AttrID] {
			continue
		}
		name := strings.TrimSpace(attr.Name)
		if name == "" {
			name = attr.AttrID
		}
		reason := ozonAttributeSuggestionSkipReason(attr, selectedVariants[attr.AttrID], skuCount)
		options := []ozonAttributeDictionaryOption{}
		if reason == "" && attr.DictionaryID != "" {
			var optionsErr error
			options, optionsErr = strictOzonAttributeDictionaryOptions(attr)
			if optionsErr != nil {
				reason = "当前模板没有可核验的官方词典选项，已留空"
			}
		}
		if reason != "" {
			skipped = append(skipped, OzonAttributeSuggestionSkipped{AttributeID: attr.AttrID, AttributeName: name, Reason: reason})
			continue
		}
		key := fmt.Sprintf("attribute_%d", len(candidates)+1)
		candidate := ozonAttributeSuggestionCandidate{key: key, attr: attr, options: options}
		candidates = append(candidates, candidate)
		labels := make([]string, 0, minInt(len(options), ozonAttributeSuggestionMaxOptions))
		for _, option := range options {
			labels = append(labels, option.Value)
			if len(labels) >= ozonAttributeSuggestionMaxOptions {
				break
			}
		}
		prompt = append(prompt, ozonAttributePromptCandidate{
			AttributeKey: key, Name: name, Description: truncateRunes(strings.TrimSpace(attr.Description), 300),
			ValueType: strings.TrimSpace(attr.ValueType), IsCollection: attr.IsCollection,
			MaxValueCount: attr.MaxValueCount, DictionaryOptions: labels,
		})
	}
	return candidates, prompt, skipped
}

func strictOzonAttributeDictionaryOptions(attr shop.OzonAttributeDTO) ([]ozonAttributeDictionaryOption, error) {
	if len(attr.Options) == 0 {
		return nil, fmt.Errorf("当前模板没有可核验的官方词典选项")
	}
	var decoded []ozonAttributeDictionaryOption
	if err := json.Unmarshal(attr.Options, &decoded); err != nil || len(decoded) == 0 {
		return nil, fmt.Errorf("当前模板的官方词典选项无效")
	}
	seenIDs := make(map[string]string, len(decoded))
	out := make([]ozonAttributeDictionaryOption, 0, len(decoded))
	for _, option := range decoded {
		option.ID = strings.TrimSpace(option.ID)
		option.Value = strings.TrimSpace(option.Value)
		if option.ID == "" || option.Value == "" {
			return nil, fmt.Errorf("当前模板包含不完整的官方词典选项")
		}
		if previous, exists := seenIDs[option.ID]; exists {
			if previous != option.Value {
				return nil, fmt.Errorf("当前模板包含冲突的官方词典选项")
			}
			continue
		}
		seenIDs[option.ID] = option.Value
		out = append(out, option)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("当前模板没有可核验的官方词典选项")
	}
	return out, nil
}

func ozonAttributeSuggestionSkipReason(attr shop.OzonAttributeDTO, selectedVariant bool, skuCount int) string {
	if attr.AttributeComplexID > 0 {
		return "组合属性需按字段组人工确认，MVP 不自动填写"
	}
	if selectedVariant || (skuCount > 1 && attr.SKUVariantEligible) {
		return "多 SKU 变体属性需逐 SKU 确认，MVP 不自动填写"
	}
	if attr.IsCollection && attr.MaxValueCount <= 0 {
		return "多值属性缺少明确上限，已留空"
	}
	if attr.DictionaryID != "" {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(attr.ValueType)) {
	case "string", "text", "integer", "int", "int64", "decimal", "float", "double", "number", "boolean", "bool", "url", "uri", "image", "date", "datetime", "date_time", "timestamp":
		return ""
	case "":
		return "当前模板缺少 valueType，已留空"
	default:
		return fmt.Sprintf("当前 valueType=%s 不支持安全 AI 回填，已留空", attr.ValueType)
	}
}

func buildOzonAttributeSuggestionPromptContext(
	productRow Product,
	category shop.PlatformCategory,
	db *gorm.DB,
	ctx context.Context,
) (ozonAttributePromptContext, []string) {
	categoryName := sanitizeOzonAttributeSuggestionEvidenceValue(category.Name)
	categoryPath := sanitizeOzonAttributeSuggestionEvidenceValue(shop.CanonicalOzonCategoryPath(ctx, db, category))
	if categoryPath == "" {
		categoryPath = categoryName
	}
	prompt := ozonAttributePromptContext{
		ProductTitle:       sanitizeOzonAttributeSuggestionEvidenceValue(firstNonEmptyProduct(productRow.Title, productRow.OriginalTitle)),
		ProductDescription: sanitizeOzonAttributeSuggestionEvidenceValue(firstNonEmptyProduct(productRow.Description, productRow.AIDescription)),
		ProductAttributes:  trustedOzonAttributeTextAttributes(productRow),
		Category: ozonAttributePromptCategory{
			FullPath: categoryPath, Name: categoryName, ProductType: categoryName,
		},
		RepresentativeSKUs: selectRepresentativeOzonAttributeSKUs(productRow.SKUs),
	}
	imageURLs := selectOzonAttributeSuggestionImages(productRow.Images, productRow.SKUs)
	for index := range imageURLs {
		role := "detail_or_packaging"
		if index == 0 {
			role = "main"
		}
		prompt.Images = append(prompt.Images, ozonAttributePromptImage{
			SourceRef: fmt.Sprintf("image.%d", index+1), Role: role,
		})
	}
	if prompt.ProductTitle != "" {
		prompt.AllowedSourceRefs = append(prompt.AllowedSourceRefs, "product.title")
	}
	if prompt.ProductDescription != "" {
		prompt.AllowedSourceRefs = append(prompt.AllowedSourceRefs, "product.description")
	}
	if len(prompt.ProductAttributes) > 0 {
		prompt.AllowedSourceRefs = append(prompt.AllowedSourceRefs, "product.attributes")
	}
	if prompt.Category.FullPath != "" {
		prompt.AllowedSourceRefs = append(prompt.AllowedSourceRefs, "category.path")
	}
	if prompt.Category.Name != "" {
		prompt.AllowedSourceRefs = append(prompt.AllowedSourceRefs, "category.name", "category.product_type")
	}
	for _, sku := range prompt.RepresentativeSKUs {
		prompt.AllowedSourceRefs = append(prompt.AllowedSourceRefs, sku.SourceRef)
	}
	for _, image := range prompt.Images {
		prompt.AllowedSourceRefs = append(prompt.AllowedSourceRefs, image.SourceRef)
	}
	prompt.AllowedSourceRefs = append(prompt.AllowedSourceRefs, "common_knowledge")
	return prompt, imageURLs
}

func trustedOzonAttributeTextAttributes(productRow Product) map[string]string {
	attrsRaw, _ := rawDraftDebugFields(json.RawMessage(productRow.RawData))
	return trustedOzonAttributeStringMap(flatStringMap(attrsRaw, false), ozonAttributeSuggestionMaxProductAttrs)
}

func trustedOzonAttributeSKUAttributes(sku ProductSKU) map[string]string {
	attrs := flatStringMap(json.RawMessage(sku.Attrs), false)
	for key, value := range flatStringMap(json.RawMessage(sku.RawData), true) {
		if _, exists := attrs[key]; !exists {
			attrs[key] = value
		}
	}
	return trustedOzonAttributeStringMap(attrs, ozonAttributeSuggestionMaxSKUAttrs)
}

func trustedOzonAttributeStringMap(input map[string]string, limit int) map[string]string {
	out := map[string]string{}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		cleanKey := truncateRunes(strings.TrimSpace(key), 160)
		if cleanKey == "" || !ozonRecommendationSelectionKeyAllowed(cleanKey) || containsOzonAttributeSuggestionSecret(cleanKey) {
			continue
		}
		value := sanitizeOzonAttributeSuggestionEvidenceValue(input[key])
		if value == "" {
			continue
		}
		out[cleanKey] = value
		if len(out) >= limit {
			break
		}
	}
	return out
}

type ozonAttributePromptSKUWork struct {
	sku    ProductSKU
	prompt ozonAttributePromptSKU
}

// selectRepresentativeOzonAttributeSKUs uses deterministic farthest-point
// sampling: the earliest collected SKU is the main/default representative,
// then the SKU most different from it, then the SKU farthest from both.
func selectRepresentativeOzonAttributeSKUs(skus []ProductSKU) []ozonAttributePromptSKU {
	work := make([]ozonAttributePromptSKUWork, 0, len(skus))
	for _, sku := range skus {
		work = append(work, ozonAttributePromptSKUWork{sku: sku, prompt: ozonAttributePromptSKU{
			SKUCode:    sanitizeOzonAttributeSuggestionEvidenceValue(sku.SKUCode),
			SKUName:    sanitizeOzonAttributeSuggestionEvidenceValue(sku.SKUName),
			Attributes: trustedOzonAttributeSKUAttributes(sku),
		}})
	}
	sort.SliceStable(work, func(i, j int) bool {
		if !work[i].sku.CreatedAt.Equal(work[j].sku.CreatedAt) {
			return work[i].sku.CreatedAt.Before(work[j].sku.CreatedAt)
		}
		return work[i].sku.ID.String() < work[j].sku.ID.String()
	})
	if len(work) == 0 {
		return []ozonAttributePromptSKU{}
	}
	selected := []int{0}
	for len(selected) < minInt(ozonAttributeSuggestionMaxSKUs, len(work)) {
		bestIndex, bestDistance := -1, -1
		for index := range work {
			if containsInt(selected, index) {
				continue
			}
			distance := -1
			for _, selectedIndex := range selected {
				current := ozonAttributeSKUDistance(work[index].prompt.Attributes, work[selectedIndex].prompt.Attributes)
				if distance < 0 || current < distance {
					distance = current
				}
			}
			if distance > bestDistance {
				bestIndex, bestDistance = index, distance
			}
		}
		if bestIndex < 0 {
			break
		}
		selected = append(selected, bestIndex)
	}
	out := make([]ozonAttributePromptSKU, 0, len(selected))
	for index, selectedIndex := range selected {
		item := work[selectedIndex].prompt
		item.SourceRef = fmt.Sprintf("sku.%d", index+1)
		out = append(out, item)
	}
	return out
}

func ozonAttributeSKUDistance(left, right map[string]string) int {
	keys := map[string]bool{}
	for key := range left {
		keys[key] = true
	}
	for key := range right {
		keys[key] = true
	}
	distance := 0
	for key := range keys {
		if left[key] != right[key] {
			distance++
		}
	}
	return distance
}

func containsInt(values []int, expected int) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

type ozonAttributeImageCandidate struct {
	url       string
	imageType string
	isBest    bool
	sortOrder int
	createdAt time.Time
	id        string
}

func selectOzonAttributeSuggestionImages(
	images []ProductImage,
	allSKUs []ProductSKU,
) []string {
	mainImages := []ozonAttributeImageCandidate{}
	detailImages := []ozonAttributeImageCandidate{}
	for _, image := range images {
		if strings.EqualFold(strings.TrimSpace(image.Source), ImageSourceAI) || strings.EqualFold(strings.TrimSpace(image.ImageType), ImageTypeAIGenerated) {
			continue
		}
		imageURL := firstSafeOzonAttributeSuggestionImageURL(image.OriginURL, image.PublicURL)
		if imageURL == "" {
			continue
		}
		candidate := ozonAttributeImageCandidate{
			url: imageURL, imageType: strings.ToLower(strings.TrimSpace(image.ImageType)), isBest: image.IsBestMain,
			sortOrder: image.SortOrder, createdAt: image.CreatedAt, id: image.ID.String(),
		}
		if candidate.imageType == ImageTypeMain {
			mainImages = append(mainImages, candidate)
		} else {
			detailImages = append(detailImages, candidate)
		}
	}
	sort.SliceStable(mainImages, func(i, j int) bool {
		if mainImages[i].isBest != mainImages[j].isBest {
			return mainImages[i].isBest
		}
		return ozonAttributeImageCandidateLess(mainImages[i], mainImages[j])
	})
	sort.SliceStable(detailImages, func(i, j int) bool {
		leftPriority := ozonAttributeImageTypePriority(detailImages[i].imageType)
		rightPriority := ozonAttributeImageTypePriority(detailImages[j].imageType)
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		return ozonAttributeImageCandidateLess(detailImages[i], detailImages[j])
	})
	out := make([]string, 0, ozonAttributeSuggestionMaxImages)
	seen := map[string]bool{}
	appendURL := func(imageURL string) {
		if len(out) >= ozonAttributeSuggestionMaxImages {
			return
		}
		key := canonicalOzonAttributeSuggestionImageKey(imageURL)
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, imageURL)
	}
	mainStart := 0
	if len(mainImages) > 0 {
		appendURL(mainImages[0].url)
		mainStart = 1
	}
	for _, candidate := range detailImages {
		appendURL(candidate.url)
	}
	for _, candidate := range mainImages[mainStart:] {
		appendURL(candidate.url)
	}
	if len(out) < ozonAttributeSuggestionMaxImages {
		orderedSKUs := append([]ProductSKU(nil), allSKUs...)
		sort.SliceStable(orderedSKUs, func(i, j int) bool {
			if !orderedSKUs[i].CreatedAt.Equal(orderedSKUs[j].CreatedAt) {
				return orderedSKUs[i].CreatedAt.Before(orderedSKUs[j].CreatedAt)
			}
			return orderedSKUs[i].ID.String() < orderedSKUs[j].ID.String()
		})
		for _, sku := range orderedSKUs {
			appendURL(firstSafeOzonAttributeSuggestionImageURL(sku.ImageURL))
		}
	}
	return out
}

func ozonAttributeImageCandidateLess(left, right ozonAttributeImageCandidate) bool {
	if left.sortOrder != right.sortOrder {
		return left.sortOrder < right.sortOrder
	}
	if !left.createdAt.Equal(right.createdAt) {
		return left.createdAt.Before(right.createdAt)
	}
	return left.id < right.id
}

func ozonAttributeImageTypePriority(imageType string) int {
	switch imageType {
	case ImageTypeDetail, ImageTypeDescription:
		return 0
	case ImageTypeMarketing:
		return 1
	default:
		return 2
	}
}

func firstSafeOzonAttributeSuggestionImageURL(values ...string) string {
	for _, value := range values {
		if safe, ok := safeOzonAttributeSuggestionImageURL(value); ok {
			return safe
		}
	}
	return ""
}

func safeOzonAttributeSuggestionImageURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 2048 {
		return "", false
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.User != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return "", false
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast()) {
		return "", false
	}
	for key, values := range parsed.Query() {
		compactKey := ozonAttributeSuggestionNonAlnumPattern.ReplaceAllString(strings.ToLower(key), "")
		for _, blocked := range []string{"token", "signature", "signed", "credential", "secret", "password", "cookie", "authorization", "authkey", "accesskey", "expires", "policy", "xamz", "xoss", "ossaccesskeyid", "keypairid"} {
			if strings.Contains(compactKey, blocked) {
				return "", false
			}
		}
		for _, value := range values {
			if containsOzonAttributeSuggestionSecret(value) {
				return "", false
			}
		}
	}
	parsed.Fragment = ""
	return parsed.String(), true
}

func canonicalOzonAttributeSuggestionImageKey(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host) + parsed.EscapedPath()
}

func sanitizeOzonAttributeSuggestionEvidenceValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || containsOzonAttributeSuggestionSecret(value) {
		return ""
	}
	value = ozonAttributeSuggestionURLPattern.ReplaceAllString(value, " ")
	value = strings.Join(strings.Fields(value), " ")
	return truncateRunes(value, 400)
}

func containsOzonAttributeSuggestionSecret(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return false
	}
	if ozonAttributeSuggestionSecretPattern.MatchString(lower) {
		return true
	}
	compact := ozonAttributeSuggestionNonAlnumPattern.ReplaceAllString(lower, "")
	for _, marker := range []string{
		"apikey", "apitoken", "accesskey", "accesstoken", "refreshtoken",
		"clientsecret", "appsecret", "secretkey", "privatekey", "shopsecret",
		"authorization", "password", "passwd", "cookie", "credential", "sessionid", "bearer",
	} {
		if strings.Contains(compact, marker) {
			return true
		}
	}
	return false
}

func applyOzonAttributeAIOutput(
	result *OzonAttributeSuggestionResult,
	candidates []ozonAttributeSuggestionCandidate,
	allowedSourceRefs []string,
	output ozonAttributeAIOutput,
) {
	if result == nil {
		return
	}
	byKey := make(map[string][]ozonAttributeAICandidate, len(output.Suggestions))
	knownCandidates := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		knownCandidates[candidate.key] = true
	}
	for _, item := range output.Suggestions {
		key := strings.TrimSpace(item.AttributeKey)
		if !knownCandidates[key] {
			result.Warnings = append(result.Warnings, "AI 返回了当前模板之外的属性引用，已丢弃")
			continue
		}
		byKey[key] = append(byKey[key], item)
	}
	knownSourceRefs := make(map[string]bool, len(allowedSourceRefs))
	for _, sourceRef := range allowedSourceRefs {
		knownSourceRefs[sourceRef] = true
	}
	for _, candidate := range candidates {
		items := byKey[candidate.key]
		name := strings.TrimSpace(candidate.attr.Name)
		if name == "" {
			name = candidate.attr.AttrID
		}
		skip := func(reason string) {
			result.Skipped = append(result.Skipped, OzonAttributeSuggestionSkipped{AttributeID: candidate.attr.AttrID, AttributeName: name, Reason: reason})
		}
		if len(items) == 0 {
			skip("AI 未返回该属性的语义建议，已留空")
			continue
		}
		if len(items) > 1 {
			skip("AI 对同一属性返回冲突建议，已留空")
			continue
		}
		item := items[0]
		if item.Confidence < 0 || item.Confidence > 1 {
			skip("AI 可信度格式无效，已留空")
			continue
		}
		sourceRefs := item.SourceRefs
		if len(sourceRefs) == 0 {
			sourceRefs = item.EvidenceKeys
		}
		sourceRefs = boundedStrings(sourceRefs, 12, 80)
		if len(sourceRefs) == 0 {
			skip("AI 建议缺少来源引用，已留空")
			continue
		}
		validSourceRefs := true
		for _, sourceRef := range sourceRefs {
			if !knownSourceRefs[sourceRef] {
				validSourceRefs = false
				break
			}
		}
		if !validSourceRefs {
			skip("AI 建议引用了未知来源，已留空")
			continue
		}
		if strings.TrimSpace(item.Reason) == "" {
			skip("AI 建议缺少可审核的推断理由，已留空")
			continue
		}
		reason := truncateRunes(sanitizeOzonAttributeSuggestionEvidenceValue(item.Reason), 240)
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
			skip("AI 未返回可用值，已留空")
			continue
		}
		selections, err := validatedOzonAttributeSuggestionSelections(candidate, values)
		if err != nil {
			skip("AI 值未通过当前模板校验：" + truncateRunes(err.Error(), 220))
			continue
		}
		level := "low"
		if item.Confidence >= ozonAttributeSuggestionHighThreshold {
			level = "high"
		} else if item.Confidence >= ozonAttributeSuggestionMediumThreshold {
			level = "medium"
		}
		result.Suggestions = append(result.Suggestions, OzonAttributeSuggestion{
			AttributeID: candidate.attr.AttrID, AttributeName: name, Values: selections,
			Confidence: item.Confidence, ConfidenceLevel: level, RequiresReview: level != "high",
			Reason: reason, SourceRefs: sourceRefs,
		})
	}
	result.Warnings = boundedStrings(result.Warnings, 10, 240)
	result.Summary = OzonAttributeSuggestionSummary{Filled: len(result.Suggestions), NotFound: len(result.Skipped)}
	for _, suggestion := range result.Suggestions {
		if suggestion.RequiresReview {
			result.Summary.RequiresReview++
		}
	}
	switch {
	case len(result.Suggestions) == 0:
		result.Status = OzonAttributeSuggestionNoMatch
	case len(result.Skipped) > 0 || len(result.Warnings) > 0:
		result.Status = OzonAttributeSuggestionPartial
	default:
		result.Status = OzonAttributeSuggestionReady
	}
}

func validatedOzonAttributeSuggestionSelections(candidate ozonAttributeSuggestionCandidate, semanticValues []string) ([]OzonAttributeSelection, error) {
	selections := make([]OzonAttributeSelection, 0, len(semanticValues))
	if candidate.attr.DictionaryID != "" {
		bySemantic := map[string][]ozonAttributeDictionaryOption{}
		for _, option := range candidate.options {
			key := normalizeOzonRecommendationText(option.Value)
			if key != "" {
				bySemantic[key] = append(bySemantic[key], option)
			}
		}
		for _, semantic := range semanticValues {
			matches := bySemantic[normalizeOzonRecommendationText(semantic)]
			if len(matches) != 1 {
				return nil, fmt.Errorf("词典候选无法唯一映射到当前官方选项")
			}
			selections = append(selections, OzonAttributeSelection{Value: matches[0].Value, DictionaryValueID: matches[0].ID})
		}
	} else {
		for _, value := range semanticValues {
			selections = append(selections, OzonAttributeSelection{Value: value})
		}
	}
	if err := validateOzonAttributeSelections(ozonSuggestionProductAttribute(candidate.attr), ozonSuggestionAttributeMeta(candidate.attr), selections); err != nil {
		return nil, err
	}
	return selections, nil
}

func ozonSuggestionProductAttribute(attr shop.OzonAttributeDTO) shop.PlatformCategoryAttribute {
	return shop.PlatformCategoryAttribute{
		CategoryID: attr.CategoryID, AttrID: attr.AttrID, Name: attr.Name, Required: attr.Required,
		ValueType: attr.ValueType, Options: datatypes.JSON(append([]byte(nil), attr.Options...)),
	}
}

func ozonSuggestionAttributeMeta(attr shop.OzonAttributeDTO) ozonAttributeMeta {
	return ozonAttributeMeta{
		DictionaryID: attr.DictionaryID, SKUVariantEligible: attr.SKUVariantEligible,
		SKUVariantEligibilityKnown: attr.SKUVariantEligibilityKnown, IsCollection: attr.IsCollection,
		AttributeComplexID: attr.AttributeComplexID, MaxValueCount: attr.MaxValueCount,
		ComplexIsCollection: attr.ComplexIsCollection,
	}
}

func (s *Service) finishOzonAttributeSuggestionAudit(
	c *gin.Context,
	adminID *uuid.UUID,
	taskID uuid.UUID,
	productID uuid.UUID,
	result *OzonAttributeSuggestionResult,
	inputTokens, outputTokens int,
	model string,
) {
	output, err := json.Marshal(result)
	if err != nil {
		_ = s.AITasks.MarkFailed(c.Request.Context(), taskID, "validated output serialization failed")
		return
	}
	// Persist only the validated, bounded response. Provider raw output is
	// omitted; the optional user-facing reason has already been URL/secret
	// sanitized above.
	_ = s.AITasks.MarkSuccess(c.Request.Context(), taskID, output, nil, inputTokens, outputTokens, model)
	s.writeOzonAttributeSuggestionLog(c, adminID, productID, taskID, "success", "validated_suggestions", result.Summary)
}

func (s *Service) writeOzonAttributeSuggestionLog(
	c *gin.Context,
	adminID *uuid.UUID,
	productID, taskID uuid.UUID,
	status, reason string,
	summary OzonAttributeSuggestionSummary,
) {
	if s == nil || s.OpLog == nil {
		return
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID, Action: "ai.ozon_attribute_suggestions." + status,
		Resource: "product", ResourceID: productID.String(), Status: status,
		Message: fmt.Sprintf("taskId=%s reason=%s filled=%d review=%d skipped=%d", taskID.String(), reason, summary.Filled, summary.RequiresReview, summary.NotFound),
	})
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

package aiprompt

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const CodeProductTitleOptimize = "product_title_optimize"
const CodeProductDescriptionGenerate = "product_description_generate"
const CodeOzonAttributeSuggestions = "ozon_attribute_suggestions"
const CodeOzonAttributeFacts = "ozon_attribute_facts"
const CodeCustomerReplyGenerate = "customer_reply_generate"
const CodeCollectRuleGenerate = "collect_rule_generate"

const OzonAttributeSuggestionPolicyVersion = "ozon_attribute_suggestions_policy_v3"

// EnsureDefaults creates built-in prompts when missing.
func EnsureDefaults(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if err := ensureProductTitleOptimize(ctx, db); err != nil {
		return err
	}
	if err := ensureProductDescriptionGenerate(ctx, db); err != nil {
		return err
	}
	if err := ensureOzonAttributeFacts(ctx, db); err != nil {
		return err
	}
	if err := ensureOzonAttributeSuggestions(ctx, db); err != nil {
		return err
	}
	if err := migrateOzonAttributeSuggestionsPolicyV3(ctx, db); err != nil {
		return err
	}
	if err := ensureCustomerReplyGenerate(ctx, db); err != nil {
		return err
	}
	if err := ensureCollectRuleGenerate(ctx, db); err != nil {
		return err
	}
	if err := migrateProductTitleOptimizeMaxTokens(ctx, db); err != nil {
		return err
	}
	if err := migrateCustomerReplyGenerateOrderContext(ctx, db); err != nil {
		return err
	}
	return migrateCollectRuleGenerateQualityHints(ctx, db)
}

func migrateProductTitleOptimizeMaxTokens(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	const minTokens = 1024
	return db.WithContext(ctx).Model(&AIPrompt{}).
		Where("code = ? AND max_tokens > 0 AND max_tokens > ?", CodeProductTitleOptimize, minTokens).
		Update("max_tokens", minTokens).Error
}

func ensureProductTitleOptimize(ctx context.Context, db *gorm.DB) error {
	schema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"optimizedTitle": map[string]string{"type": "string"},
			"keywords": map[string]any{
				"type":  "array",
				"items": map[string]string{"type": "string"},
			},
			"reason": map[string]string{"type": "string"},
		},
		"required": []string{"optimizedTitle", "keywords", "reason"},
	})
	defaultSys := strings.TrimSpace(`You are an expert cross-border e-commerce copywriter.
Return ONLY valid JSON (no markdown fences) with keys: optimizedTitle (string), keywords (string array), reason (short string in the same language as the user's requested listing language).
The optimizedTitle must respect max length and platform style hints from the user message.`)
	defaultUser := strings.TrimSpace(`Optimize this product listing title.

Context:
- Current title: {{title}}
- Category: {{category}}
- Attributes / specs: {{attributes}}
- Target language: {{language}}
- Target platform: {{platform}}
- Max title length (characters): {{maxLength}}

Reply with JSON only.`)

	var count int64
	if err := db.WithContext(ctx).Model(&AIPrompt{}).Where("code = ?", CodeProductTitleOptimize).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	row := &AIPrompt{
		Code:         CodeProductTitleOptimize,
		Name:         "商品标题优化",
		Scene:        "product",
		Provider:     "",
		Model:        "",
		SystemPrompt: defaultSys,
		UserPrompt:   defaultUser,
		OutputSchema: datatypes.JSON(schema),
		Temperature:  0.4,
		MaxTokens:    1024,
		Enabled:      true,
	}
	return db.WithContext(ctx).Create(row).Error
}

func ensureProductDescriptionGenerate(ctx context.Context, db *gorm.DB) error {
	schema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]string{"type": "string"},
			"highlights": map[string]any{
				"type":  "array",
				"items": map[string]string{"type": "string"},
			},
			"specifications": map[string]any{
				"type":  "array",
				"items": map[string]string{"type": "string"},
			},
			"packageIncludes": map[string]any{
				"type":  "array",
				"items": map[string]string{"type": "string"},
			},
			"notes":  map[string]string{"type": "string"},
			"reason": map[string]string{"type": "string"},
		},
		"required": []string{"description", "highlights", "specifications", "packageIncludes", "notes", "reason"},
	})
	defaultSys := strings.TrimSpace(`You are an expert cross-border e-commerce copywriter for marketplace product detail pages.
Return ONLY valid JSON (no markdown fences) with exactly these keys: description (string), highlights (string array), specifications (string array), packageIncludes (string array), notes (string), reason (short string explaining choices, same language as description).

Rules:
- Base copy ONLY on facts present in the user message. Do not invent features, materials, certifications, or guarantees the product does not have.
- No exaggerated claims, medical claims, or policy-bypass language. Avoid hype words that platforms often restrict.
- Structure the detail page for cross-border sellers: cover Product Highlights, Specifications, Package Includes, and Notes where appropriate (you may weave these into description or use list fields).
- Default listing context in the user message uses English on TikTok Shop unless overridden; match the requested language and tone.
- Keep bullets concise; description can be several short paragraphs suitable for a PDP.`)
	defaultUser := strings.TrimSpace(`Generate a product detail page copy package.

Product context:
- Listing title (seller/current): {{title}}
- Original title (source): {{originalTitle}}
- AI-optimized title (if any): {{aiTitle}}
- Attributes / raw specs summary: {{attributes}}
- SKU lines: {{skus}}
- Target language: {{language}}
- Target platform: {{platform}}
- Tone: {{tone}}

Reply with JSON only using the schema from the system message.`)

	var count int64
	if err := db.WithContext(ctx).Model(&AIPrompt{}).Where("code = ?", CodeProductDescriptionGenerate).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	row := &AIPrompt{
		Code:         CodeProductDescriptionGenerate,
		Name:         "商品描述生成",
		Scene:        "product",
		Provider:     "",
		Model:        "",
		SystemPrompt: defaultSys,
		UserPrompt:   defaultUser,
		OutputSchema: datatypes.JSON(schema),
		Temperature:  0.45,
		MaxTokens:    2500,
		Enabled:      true,
	}
	return db.WithContext(ctx).Create(row).Error
}

func builtinOzonAttributeSuggestionsV2() (string, string, datatypes.JSON) {
	schema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"suggestions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"attributeKey": map[string]string{"type": "string"},
						"values":       map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
						"confidence":   map[string]string{"type": "number"},
						"reason":       map[string]string{"type": "string"},
						"sourceRefs":   map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
					},
					"required": []string{"attributeKey", "values", "confidence", "reason", "sourceRefs"},
				},
			},
		},
		"required": []string{"suggestions"},
	})
	defaultSys := strings.TrimSpace(`你是 Ozon 商品级类目属性建议器。必须遵守服务端附加的产品策略与安全规则，并严格输出单个 JSON 对象。`)
	defaultUser := strings.TrimSpace(`请尽量为当前普通商品级空白属性生成建议。

商品、类目、代表 SKU 与图片引用 JSON：
{{context}}

当前允许建议的空白属性 JSON：
{{attributes}}

允许引用的来源：
{{sourceRefs}}

只输出 JSON：{"suggestions":[{"attributeKey":"attribute_1","values":["语义文本"],"confidence":0.0,"reason":"简短依据","sourceRefs":["product.title"]}]}`)
	return defaultSys, defaultUser, datatypes.JSON(schema)
}

func builtinOzonAttributeSuggestions() (string, string, datatypes.JSON) {
	schema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"suggestions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"attributeKey":   map[string]string{"type": "string"},
						"values":         map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
						"confidence":     map[string]string{"type": "number"},
						"inferenceBasis": map[string]string{"type": "string"},
						"reason":         map[string]string{"type": "string"},
						"sourceRefs":     map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
						"factRefs":       map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
					},
					"required": []string{"attributeKey", "values", "confidence", "inferenceBasis", "reason", "sourceRefs"},
				},
			},
		},
		"required": []string{"suggestions"},
	})
	defaultSys := strings.TrimSpace(`你是 Ozon 普通商品级类目属性建议器。服务端会分批给出必须逐项决策的空白属性；必须遵守服务端附加的 policy v3 与安全规则，并严格输出单个 JSON 对象。`)
	defaultUser := strings.TrimSpace(`请为本批每个空白 Ozon 普通商品级属性各返回一项建议，不得漏项。

脱敏商品、完整类目路径、代表 SKU、图片事实表与允许来源 JSON：
{{context}}

本批允许建议的空白属性 JSON：
{{attributes}}

允许引用的来源：
{{sourceRefs}}

values 数组的每个元素都必须是 JSON 字符串；即使属性类型是 Integer、Decimal 或 Boolean，也必须分别写成如 "1"、"8.75"、"true" 的字符串，不能输出 JSON 数字、布尔值或 null。

只输出 JSON：{"suggestions":[{"attributeKey":"attribute_1","values":["语义文本"],"confidence":0.7,"inferenceBasis":"product_standard_inference","reason":"简短可审核依据","sourceRefs":["product.title","common_knowledge"],"factRefs":[]}]}`)
	return defaultSys, defaultUser, datatypes.JSON(schema)
}

func builtinOzonAttributeFacts() (string, string, datatypes.JSON) {
	schema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"facts": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":       map[string]string{"type": "string"},
						"value":      map[string]string{"type": "string"},
						"evidence":   map[string]string{"type": "string"},
						"sourceRefs": map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
					},
					"required": []string{"name", "value", "evidence", "sourceRefs"},
				},
			},
		},
		"required": []string{"facts"},
	})
	defaultSys := strings.TrimSpace(`你是商品图文事实提炼器。只输出单个 JSON 对象，且只能包含 facts 数组。

规则：
- 只提炼标题、描述、可信文本属性、代表 SKU 或图片中直接可核验的商品事实；不得加入类目常识、推测、营销扩写或外部资料。
- 每条必须包含 name、value、evidence、sourceRefs。文本 evidence 必须是对应输入中的简短原文片段；视觉事实用 image.1/image.2 并简述可见依据。
- sourceRefs 只能使用输入白名单中的 product.*、sku.*、image.*，不得使用 category.* 或 common_knowledge。
- 重点覆盖候选属性可能需要的材质、颜色、结构、尺寸、重量、型号、配套、用途等事实，但不存在就不要伪造。
- 对比全部代表 SKU：同一规格在 SKU 间存在不同值时，不得把其中一个值写成适用于整件商品的事实，也不要把该差异作为商品级 fact 输出；只能提炼全部代表 SKU 明确共有的事实。
- 不得输出凭证、Token、Cookie、店铺信息、图片 URL、私有链接或平台 ID。`)
	defaultUser := strings.TrimSpace(`从以下脱敏商品上下文与最多两张商品图中提炼直接可核验事实。

商品上下文 JSON：
{{context}}

候选属性的事实需求 JSON：
{{attributes}}

允许引用的来源：
{{sourceRefs}}

只输出 JSON：{"facts":[{"name":"颜色","value":"黑色","evidence":"黑色","sourceRefs":["product.title"]}]}`)
	return defaultSys, defaultUser, datatypes.JSON(schema)
}

// OzonAttributeSuggestionRuntimePolicy is appended at request time so a
// persisted or custom prompt cannot silently retain a conservative or partial
// "omit unless explicit evidence" behavior. Template/type/dictionary checks
// remain authoritative in product service after the model returns.
func OzonAttributeSuggestionRuntimePolicy() string {
	return strings.TrimSpace(`[` + OzonAttributeSuggestionPolicyVersion + `]
目标：除服务端已经排除的外部运营/物流/合规字段外，为本批每个普通商品级空白属性返回一项语义建议；用户会在保存前最终审核。

商品级一致性：建议必须适用于整个商品，而不是只适用于主 SKU。必须比较全部代表 SKU；若颜色、尺寸、材质、型号、套餐/标配内容或其他规格在 SKU 间不同，不得把其中一个变体值写成整商品事实，也不得把它注入商品级名称、简介、标签、配套或其他普通属性。尤其禁止把仅部分 SKU 包含的底座、充电头、内存卡或赠品作为整商品“配套”。应使用全部 SKU 共有的中性表述；无法形成共同事实时只能降为 category_fallback_guess 并明确提示核对。
context.skuVariations 是服务端根据代表 SKU 确定性计算的差异清单，优先级高于图片和模型提炼的 facts。清单中的任一单值都不得写入整商品名称、简介或主题标签，也不得声称“全部 SKU 一致”。

商品文案边界：不得把老人机、按键功能机或 Series 30+ 设备写成智能手机；使用“移动电话/功能机”等与证据一致的中性类型。“4G 全网通”不能推导出兼容所有俄罗斯运营商或其他地区网络，除非商品原始证据明确给出该地区及运营商兼容性。不得把类目平台所在地扩写为商品网络能力。

品牌与系统边界：TradeMind、Ozon、AI、模型或界面/服务名称都不是商品品牌证据。商品名称中的品牌、型号及拉丁商品标识，只有直接出现在 product.title、product.description、product.attributes、代表 SKU 或已验证 facts 时才能使用；不得为补齐名称而臆造品牌或把系统名称写入商品值。

输出必须是单个 JSON 对象且只能包含 suggestions 数组。每项必须包含 attributeKey、values、confidence、inferenceBasis、reason、sourceRefs；可包含 factRefs。不得输出其他字段。suggestions 必须与输入 attributes 一一对应，不得漏项、重复或添加批次外属性。

推断层级（服务端会重新判定并规范化可信度）：
- direct_product_evidence：值直接来自商品标题、描述、可信属性、代表 SKU、图片或给定 facts；引用直接 sourceRefs，并尽量引用对应 factRefs。confidence 写 0.9。
- product_standard_inference：商品直接证据结合该类标品常识推断；同时引用商品/类目来源和 common_knowledge。confidence 写 0.7。
- category_fallback_guess：无法从商品本身确认，只能依据完整类目和通用知识猜测；引用 category.* 与 common_knowledge。confidence 写 0.3。
- 不得把纯类目常识或猜测标成 direct_product_evidence。即使只能低可信猜测，也必须返回并说明待审核风险，不能因证据不足省略。

安全与格式：
- 只能使用输入中的 attributeKey；不得输出或猜测类目 ID、属性 ID、词典 ID或其他平台 ID。
- values 只返回语义文本，禁止把数字 ID 当作词典值。dictionaryOptionsTruncated=false 时，词典值必须逐字选自 dictionaryOptions；为 true 时可返回预计的官方语义文本，服务端只会接受官方只读搜索的唯一精确匹配。
- sourceRefs 只能从 allowedSourceRefs 中选择。factRefs 只能引用 context.facts 中的 factKey。
- 单值属性只返回一个值；多值属性不得超过 maxValueCount。Integer、Decimal、Boolean、URL、date/datetime 必须使用对应标准格式。
- semanticHint 是服务端基于当前官方属性/词典语义提供的消歧说明，优先级高于机器翻译后的字面猜测；必须返回 dictionaryOptions 中对应的官方语义文本，不能返回 hint 中的解释文字。
- 属性名称中的计量单位是硬约束。若依据使用其他单位，必须先准确换算后再返回，例如“28 天”用于“小时”字段时必须返回 672，禁止明知单位不一致仍复制 28 或按运营习惯改写单位。
- values 必须始终是 JSON 字符串数组；Integer、Decimal、Boolean 也只能分别返回如 ["1"]、["8.75"]、["true"]，禁止返回 JSON 数字、布尔值、对象或 null。
- 不要输出凭证、Token、Cookie、店铺信息、图片 URL、私有链接或当前用户已填值。`)
}

// OzonAttributeFactsRuntimePolicy is appended at request time so an existing
// persisted/custom fact prompt still treats the representative SKU set as a
// product-wide consistency boundary without a schema migration.
func OzonAttributeFactsRuntimePolicy() string {
	return strings.TrimSpace(`[ozon_attribute_facts_policy_v1]
事实必须适用于整个商品。比较全部代表 SKU；同一规格在 SKU 间存在不同值时，禁止把单一 SKU 的颜色、尺寸、材质、型号、套餐/标配内容或其他变体值写成整商品事实，也不要输出该差异；仅部分 SKU 包含的底座、充电头、内存卡或赠品不得作为商品级配套事实。只能输出所有代表 SKU 的共同点。单张图片只证明该图可见内容，不能覆盖与其他 SKU 冲突的规格。图片可直接证明品牌文字、颜色、外形/结构等肉眼可见事实；SIM 数量、存储、电池、系统、频段、续航等隐藏规格不得仅凭外观图片输出，除非同时有可回指的商品文本来源。
context.skuVariations 是服务端确定性差异清单；其中任何单值均禁止作为商品级 fact，图片也不能推翻该清单。

严格保持既有 JSON 协议：只输出一个对象且顶层只能有 facts；facts 每项只能有 name、value、evidence、sourceRefs 四个字段。禁止增加 scope、applicability、status、timeout、notes 或任何其他字段，禁止 Markdown 和 JSON 之外的文字。`)
}

func legacyBuiltinOzonAttributeSuggestionsV1() (string, string) {
	legacySys := strings.TrimSpace(`你是 Ozon 商品级类目属性建议器。严格输出一个 JSON 对象，且只能包含 suggestions 数组，不要 Markdown 或额外说明。

每个 suggestions 项必须包含 attributeKey、values、confidence、reason、evidenceKeys。

不可违反的规则：
- 只能引用输入中已有的 attributeKey 和 evidence key；不得创造属性、证据、类目 ID、属性 ID 或词典 ID。
- values 只返回语义文本。词典属性也只能从该属性 dictionaryOptions 中逐字选择文本，绝不能返回或猜测 ID。
- 只根据 evidence 中明确出现的商品事实建议；没有证据就省略该属性，不能靠常识补齐材质、品牌、认证、日期、链接或规格。
- confidence 必须在 0 到 1 之间。证据直接且唯一时才可高于或等于 0.8；需要人工语义判断时应低于 0.8。
- 单值属性只返回一个值；多值属性不得超过 maxValueCount。
- 不要输出图片、链接、凭证、Token、Cookie、店铺信息或任何输入中不存在的内容。`)
	legacyUser := strings.TrimSpace(`请仅为下列空白 Ozon 商品级属性生成有证据的候选值。

可信商品证据 JSON：
{{evidence}}

当前模板允许建议的空白属性 JSON：
{{attributes}}

只输出 JSON：{"suggestions":[...]}`)
	return legacySys, legacyUser
}

func migrateOzonAttributeSuggestionsPolicyV3(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	var row AIPrompt
	if err := db.WithContext(ctx).Where("code = ?", CodeOzonAttributeSuggestions).First(&row).Error; err != nil {
		return nil
	}
	legacySys, legacyUser := legacyBuiltinOzonAttributeSuggestionsV1()
	v2Sys, v2User, v2Schema := builtinOzonAttributeSuggestionsV2()
	isLegacyV1 := strings.TrimSpace(row.SystemPrompt) == legacySys && strings.TrimSpace(row.UserPrompt) == legacyUser
	isBuiltinV2 := strings.TrimSpace(row.SystemPrompt) == v2Sys &&
		strings.TrimSpace(row.UserPrompt) == v2User && sameJSONDocument(row.OutputSchema, v2Schema)
	if !isLegacyV1 && !isBuiltinV2 {
		return nil
	}
	sys, user, schema := builtinOzonAttributeSuggestions()
	row.SystemPrompt = sys
	row.UserPrompt = user
	row.OutputSchema = schema
	if row.MaxTokens < 4096 {
		row.MaxTokens = 4096
	}
	return db.WithContext(ctx).Save(&row).Error
}

func sameJSONDocument(left, right []byte) bool {
	var leftValue any
	var rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func ensureOzonAttributeFacts(ctx context.Context, db *gorm.DB) error {
	defaultSys, defaultUser, schema := builtinOzonAttributeFacts()
	var count int64
	if err := db.WithContext(ctx).Model(&AIPrompt{}).Where("code = ?", CodeOzonAttributeFacts).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	row := &AIPrompt{
		Code: CodeOzonAttributeFacts, Name: "Ozon 商品图文事实提炼", Scene: "product",
		SystemPrompt: defaultSys, UserPrompt: defaultUser, OutputSchema: schema,
		Temperature: 0.1, MaxTokens: 1400, Enabled: true,
	}
	return db.WithContext(ctx).Create(row).Error
}

func ensureOzonAttributeSuggestions(ctx context.Context, db *gorm.DB) error {
	defaultSys, defaultUser, schema := builtinOzonAttributeSuggestions()
	var count int64
	if err := db.WithContext(ctx).Model(&AIPrompt{}).Where("code = ?", CodeOzonAttributeSuggestions).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	row := &AIPrompt{
		Code: CodeOzonAttributeSuggestions, Name: "Ozon 类目属性建议", Scene: "product",
		SystemPrompt: defaultSys, UserPrompt: defaultUser, OutputSchema: schema,
		Temperature: 0.2, MaxTokens: 4096, Enabled: true,
	}
	return db.WithContext(ctx).Create(row).Error
}

func builtinCustomerReplyGenerate() (string, string, datatypes.JSON) {
	schema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"reply":     map[string]string{"type": "string"},
			"intent":    map[string]string{"type": "string"},
			"sentiment": map[string]string{"type": "string"},
			"riskLevel": map[string]string{"type": "string"},
			"notes":     map[string]string{"type": "string"},
		},
		"required": []string{"reply", "intent", "sentiment", "riskLevel", "notes"},
	})
	defaultSys := strings.TrimSpace(`You are a professional cross-border e-commerce customer support assistant.
Return ONLY valid JSON (no markdown fences) with keys: reply (string), intent (string), sentiment (string), riskLevel ("low"|"medium"|"high"), notes (short internal note for reviewers; mirrors reply language best-effort).

Non-negotiable safety:
- Be polite and professional within marketplace messaging limits.
- Use ONLY factual blocks provided as JSON strings {{orderInfo}}, {{orderItems}}, {{shipmentInfo}} plus legacy {{productInfo}} (may be blank) plus {{conversationHistory}}/{{customerMessage}}. Treat empty JSON objects / arrays / unknown / missing shipment rows as UNKNOWN — NEVER invent status, SKU colors/sizes, inventory, payouts, timelines, refunds, replacements, disputes outcomes, parcel locations, carriers, tracking numbers beyond what shipments JSON states.
- If shipmentInfo empty or lacks carrier plus tracking identifiers, NEVER claim dispatched/in-transit/delivered; explain what remains unknown politely.
- Contradictions or ambiguity among order/payment/shipment payloads → disclose uncertainty succinctly inside reply and escalate in notes toward human oversight.
- If customers mention refunds, payouts, replacements, lawsuits, regulators, harassment, counterfeit claims, wrong shipments, blacklist requests, or similar escalate risk appropriately (prefer at least medium; high for chargebacks/legal threats). Never promise automatic outcomes unless facts explicitly confirm settlement.
- Do NOT leak or guess customer emails/phones/addresses in reply.
- No automated commitments for refunds/reships/compensation timelines unless facts prove them.
- Prefer the declared Target reply language; mirror shopper wording when ambiguous.
- "reply" must stay concise for chat/email; NEVER paste raw JSON or internal jargon.`)
	defaultUser := strings.TrimSpace(`Produce a shopper-facing suggestion plus reviewer metadata.

Facts:
- Customer message focus: {{customerMessage}}
- Conversation timeline (truncated oldest→newest upstream): {{conversationHistory}}
- Legacy free-form merchandise notes (optional): {{productInfo}}
- Order snapshot JSON (possibly "{}" — never invent absent keys): {{orderInfo}}
- Line items snapshot (possibly "[]" — SKU attributes only from attrs): {{orderItems}}
- Logistics snapshots (possibly "[]" — NEVER invent shipped/in-transit without evidence): {{shipmentInfo}}
- Conversation profile JSON (language/platform/order cues; excludes email/phone): {{customerProfile}}

Operational constraints:
- Reply language preference: {{language}}
- Desired tone keyword: {{tone}}
- Selling platform label: {{platform}}
- Merchant policy excerpts (may be blank): {{shopPolicy}}

Respond with JSON envelope only.`)

	return defaultSys, defaultUser, datatypes.JSON(schema)
}

func ensureCustomerReplyGenerate(ctx context.Context, db *gorm.DB) error {
	defaultSys, defaultUser, schema := builtinCustomerReplyGenerate()

	var count int64
	if err := db.WithContext(ctx).Model(&AIPrompt{}).Where("code = ?", CodeCustomerReplyGenerate).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	row := &AIPrompt{
		Code:         CodeCustomerReplyGenerate,
		Name:         "AI 客服回复建议",
		Scene:        "customer_service",
		Provider:     "",
		Model:        "",
		SystemPrompt: defaultSys,
		UserPrompt:   defaultUser,
		OutputSchema: schema,
		Temperature:  0.35,
		MaxTokens:    1200,
		Enabled:      true,
	}
	return db.WithContext(ctx).Create(row).Error
}

func migrateCustomerReplyGenerateOrderContext(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	var row AIPrompt
	if err := db.WithContext(ctx).Where("code = ?", CodeCustomerReplyGenerate).First(&row).Error; err != nil {
		return nil
	}
	if strings.Contains(row.UserPrompt, "{{orderInfo}}") || strings.Contains(row.UserPrompt, "{{customerProfile}}") {
		return nil
	}
	if !strings.Contains(row.UserPrompt, "Product / order facts (if any; may be empty): {{productInfo}}") {
		return nil
	}
	if !strings.Contains(row.SystemPrompt, "there are none for order state in this MVP") {
		return nil
	}
	sys, usr, schema := builtinCustomerReplyGenerate()
	row.SystemPrompt = sys
	row.UserPrompt = usr
	row.OutputSchema = schema
	return db.WithContext(ctx).Save(&row).Error
}

func builtinCollectRuleGenerate() (string, string, datatypes.JSON) {
	schema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"rule":        map[string]string{"type": "object"},
			"confidence":  map[string]string{"type": "number"},
			"explanation": map[string]string{"type": "string"},
			"missingGeneratedFields": map[string]any{
				"type":  "array",
				"items": map[string]string{"type": "string"},
			},
			"warnings": map[string]any{
				"type":  "array",
				"items": map[string]string{"type": "string"},
			},
		},
		"required": []string{"rule", "confidence", "explanation", "missingGeneratedFields", "warnings"},
	})
	defaultSys := strings.TrimSpace(`你是跨境电商商品页「声明式 CSS 采集规则」专家。只输出合法 JSON，不要 markdown，不要 JSON 外的说明。

输出结构（必须包含全部 key）：
{
  "rule": { ... },
  "missingGeneratedFields": ["字段名"],
  "warnings": ["中文警告"],
  "confidence": 0.0-1.0,
  "explanation": "简短中文说明"
}

## 目标字段（用户勾选，必须尽量全部覆盖）
用户目标字段：{{targetFields}}
- 勾选 title / price / mainImages / descriptionImages / attributes 时，rule 中必须尽量包含对应 key。
- 禁止只生成 title 就结束；禁止只生成 title + fallbacks 当作成功。
- 某字段在 pageDigest 中无稳定候选时：在 missingGeneratedFields 列出，并在 warnings 说明原因，不要瞎编 selector。
- SKU / 库存若无明确候选（confidence>=0.5），不要生成 skus；不要编造 stock 字段。

## rule 允许的 key
title, price, currency, mainImages, descriptionImages, attributes, skus, fallbacks

## 禁止过宽 selector（除非无更好候选且 confidence<=0.4）
禁止优先使用：h1, img, div, span, a, [class*="title"], [class*="price"]
- 标题：优先 pageDigest 中高置信候选（如 .sku-name, .p-name, .itemInfo-wrap .sku-name, [property='og:title']），禁止全局 h1。
- 主图：禁止全局 img；必须限定在商品图廊/缩略图区域。
- 价格：生成 price 字段，不要把价格文本写入 currency；currency 仅放 ISO 代码（CNY/USD）。

## 字段模板方向（按 pageDigest 适配，不要硬编码只适配某一站点）
title: { "attr":"text", "selectors":["..."] }
price: { "attr":"text", "selectors":["..."] }
mainImages: {
  "attr":"src", "multiple":true, "limit":8,
  "selectors":["#spec-list img",".spec-list img","[property='og:image']"],
  "attrs":["src","data-src","data-lazy-img","data-origin","data-original"],
  "filters": { "minWidth":300, "minHeight":300, "excludeKeywords":["icon","logo","sprite","play","arrow","kefu","service","loading"], "dedupeByImageKey":true }
}
descriptionImages: {
  "attr":"src", "multiple":true, "limit":30,
  "selectors":[".detail-content img","#J-detail-content img"],
  "attrs":["src","data-src","data-lazy-img","data-original"],
  "filters": { "minWidth":300, "minHeight":300, "excludeKeywords":["icon","logo","sprite","loading"], "dedupeByImageKey":true }
}
attributes: { "mode":"pairs", "rowSelector":"...", "keySelector":"dt, .name", "valueSelector":"dd, .value" }
fallbacks: { "meta":true, "jsonLd":true, "openGraph":true }

## 安全与质量
- 禁止 script / eval / function / javascript:
- selectors 必须来自 pageDigest candidates 或基于候选组合的 CSS
- 若规则整体可信度低，降低 confidence 并写 warnings
- 生成后会被自动测试：若标题像「登录/购物车/最小单价计算器」或主图会匹配全站 icon，必须降低 confidence 并警告`)
	defaultUser := strings.TrimSpace(`为域名 {{domain}} 生成 custom collect_rule。

URL: {{url}}
目标字段：{{targetFields}}

页面结构摘要（截断，无完整 HTML）：
{{pageDigest}}

要求：
1. 覆盖用户勾选的全部目标字段（无法覆盖的写入 missingGeneratedFields + warnings）
2. 不要只输出 title
3. 不要使用过宽 selector
4. 主图必须带 filters；详情图考虑懒加载 attrs
5. 只输出 JSON`)
	return defaultSys, defaultUser, datatypes.JSON(schema)
}

func migrateCollectRuleGenerateQualityHints(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	var row AIPrompt
	if err := db.WithContext(ctx).Where("code = ?", CodeCollectRuleGenerate).First(&row).Error; err != nil {
		return nil
	}
	if strings.Contains(row.SystemPrompt, "missingGeneratedFields") {
		return nil
	}
	sys, usr, schema := builtinCollectRuleGenerate()
	row.SystemPrompt = sys
	row.UserPrompt = usr
	row.OutputSchema = schema
	if row.MaxTokens < 4096 {
		row.MaxTokens = 4096
	}
	return db.WithContext(ctx).Save(&row).Error
}

func ensureCollectRuleGenerate(ctx context.Context, db *gorm.DB) error {
	defaultSys, defaultUser, schema := builtinCollectRuleGenerate()

	var count int64
	if err := db.WithContext(ctx).Model(&AIPrompt{}).Where("code = ?", CodeCollectRuleGenerate).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	row := &AIPrompt{
		Code:         CodeCollectRuleGenerate,
		Name:         "AI 生成自定义采集规则",
		Scene:        "collect",
		SystemPrompt: defaultSys,
		UserPrompt:   defaultUser,
		OutputSchema: schema,
		Temperature:  0.2,
		MaxTokens:    4096,
		Enabled:      true,
	}
	return db.WithContext(ctx).Create(row).Error
}

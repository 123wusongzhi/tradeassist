package aiprompt

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const CodeProductTitleOptimize = "product_title_optimize"
const CodeProductDescriptionGenerate = "product_description_generate"
const CodeOzonAttributeSuggestions = "ozon_attribute_suggestions"
const CodeCustomerReplyGenerate = "customer_reply_generate"
const CodeCollectRuleGenerate = "collect_rule_generate"

const OzonAttributeSuggestionPolicyVersion = "ozon_attribute_suggestions_policy_v2"

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
	if err := ensureOzonAttributeSuggestions(ctx, db); err != nil {
		return err
	}
	if err := migrateOzonAttributeSuggestionsPolicyV2(ctx, db); err != nil {
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

func builtinOzonAttributeSuggestions() (string, string, datatypes.JSON) {
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

// OzonAttributeSuggestionRuntimePolicy is appended at request time so a
// persisted pre-v2 prompt cannot silently retain the former conservative
// "omit unless explicit evidence" behavior. Template/type/dictionary checks
// remain authoritative in product service after the model returns.
func OzonAttributeSuggestionRuntimePolicy() string {
	return strings.TrimSpace(`[` + OzonAttributeSuggestionPolicyVersion + `]
目标：积极使用推断能力，尽量填写当前允许处理的普通商品级空白属性；用户会在保存前最终审核。

输出必须是单个 JSON 对象且只能包含 suggestions 数组。每项必须且只能包含 attributeKey、values、confidence、reason、sourceRefs。

推断政策：
- 允许综合商品标题、描述、可信商品文本属性、完整 Ozon 类目路径/名称/商品类型、最多 3 个代表 SKU、最多 2 张商品图片以及通用商品知识主动推断。
- 不要因为缺少直接文字证据就大面积省略。品牌、产地、尺寸重量、保修、HS 编码、认证/合规、日期和 URL 等也可合理推断；不确定时仍返回，并用较低 confidence、明确 reason 和 sourceRefs 标记。
- 高、中、低可信建议都要返回。confidence 必须在 0 到 1 之间；直接明确可用高可信，需要语义判断用中可信，主要依赖视觉或通用知识用低可信。
- 只有无法形成任何语义候选、无法满足 valueType/格式/数量约束，或词典属性无法从 dictionaryOptions 中选择唯一语义文本时才省略。

安全与格式：
- 只能引用输入中的 attributeKey；不得输出或猜测类目 ID、属性 ID、词典 ID、平台 ID。
- values 只返回语义文本。词典属性只能从该属性 dictionaryOptions 中选择文本，绝不能返回数字 ID。
- sourceRefs 只能从 allowedSourceRefs 中选择；基于通用知识时使用 common_knowledge，图片按 image.1 / image.2 引用。
- 单值属性只返回一个值；多值属性不得超过 maxValueCount。Integer、Decimal、Boolean、URL、date/datetime 必须使用对应标准格式。
- 不要输出凭证、Token、Cookie、店铺信息、图片 URL 或任何私有链接。`)
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

func migrateOzonAttributeSuggestionsPolicyV2(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	var row AIPrompt
	if err := db.WithContext(ctx).Where("code = ?", CodeOzonAttributeSuggestions).First(&row).Error; err != nil {
		return nil
	}
	legacySys, legacyUser := legacyBuiltinOzonAttributeSuggestionsV1()
	if strings.TrimSpace(row.SystemPrompt) != legacySys || strings.TrimSpace(row.UserPrompt) != legacyUser {
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

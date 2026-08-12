package product

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

var (
	ozonAttributeLatinNameTokenPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*(?:[._/-][A-Za-z0-9]+)*`)
	ozonAttributeDayDurationPattern    = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:天|日|days?)`)
)

const (
	ozonAttributeBasisDirect   = "direct_product_evidence"
	ozonAttributeBasisStandard = "product_standard_inference"
	ozonAttributeBasisFallback = "category_fallback_guess"

	ozonAttributeSkipExternal    = "external"
	ozonAttributeSkipUnsupported = "unsupported"
	ozonAttributeSkipDictionary  = "dictionary_unmatched"
	ozonAttributeSkipValidation  = "validation_failed"
	ozonAttributeSkipOmitted     = "model_omitted"
	ozonAttributeSkipBatch       = "batch_failed"
)

var externalOzonAttributeIDs = map[string]string{
	// Seller/batch identifiers and marking state are listing- or shipment-specific.
	"9024":  "卖家内部代码不属于商品本体信息，已留空",
	"4381":  "批次或部件代码需要外部业务确认，已留空",
	"23489": "序列号可见性需要按实际包装确认，已留空",
	"23536": "标记码状态需要按实际合规与发货流程确认，已留空",
	// Factory/sale packaging and logistics measurements are not bare-product facts.
	"11650": "原厂包装数量需要按实际销售包装确认，已留空",
	"20361": "包装销售数量需要按实际 SKU 包装确认，已留空",
	"4082":  "含包装尺寸需要按实际物流包装测量，已留空",
	"4497":  "含包装重量需要按实际物流包装测量，已留空",
	"4386":  "外部包装信息需要按实际销售包装确认，已留空",
	"23368": "包装类型需要按实际销售包装确认，已留空",
	"4300":  "服装包装类型需要按实际销售包装确认，已留空",
	"23249": "统一计量单位数量需要按销售与法规配置确认，已留空",
	// Regulatory declarations require authoritative documents, not product guessing.
	"9782":  "危险等级需要依据安全或物流文件确认，已留空",
	"11729": "危险等级需要依据安全或物流文件确认，已留空",
	"22232": "HS 编码需要依据海关归类确认，已留空",
	// External assets cannot be fabricated from product facts.
	"11254": "富内容需要独立制作并上传，已留空",
	"8789":  "PDF 文件名称需要对应实际上传文件，已留空",
	"8790":  "PDF 文件链接需要对应实际上传文件，已留空",
	"21837": "视频标题需要对应实际上传视频，已留空",
	"21841": "视频链接需要对应实际上传视频，已留空",
	"21845": "视频背景链接需要对应实际上传视频，已留空",
	"22273": "视频商品关联需要对应实际视频与 SKU，已留空",
	// Merchandising groups are seller decisions rather than product facts.
	"22390": "相似商品分组属于店铺运营配置，已留空",
}

var intrinsicOzonAttributeIDs = map[string]bool{
	"4383":  true, // net product weight
	"4382":  true, // dimensions without packaging
	"4384":  true, // product kit/contents
	"12209": true, // charger/product contents despite the word packaging
	"4389":  true, // country of origin
	"10400": true, // warranty
	"9048":  true, // card merge model name, product-derived
	"12141": true, // name-template model name
	"4180":  true, // product name
	"4191":  true, // product introduction
}

var ozonAttributeVariantPhysicalIDs = map[string]bool{
	"4382": true, // unpackaged dimensions
	"4383": true, // net product weight
	"6877": true, // compartment volume
	"8414": true, // height
	"8415": true, // length
	"8416": true, // width
}

var ozonAttributeVariantAliasGroups = [][]string{
	{"黑色", "black", "черный", "чёрный"},
	{"白色", "white", "белый"},
	{"红色", "red", "красный"},
	{"蓝色", "blue", "синий"},
	{"绿色", "green", "зеленый", "зелёный"},
	{"灰色", "grey", "gray", "серый"},
	{"黄色", "yellow", "желтый", "жёлтый"},
	{"粉色", "pink", "розовый"},
	{"棕色", "brown", "коричневый"},
	{"紫色", "purple", "фиолетовый"},
	{"橙色", "orange", "оранжевый"},
	{"大号", "large", "большой"},
	{"中号", "medium", "средний"},
	{"小号", "small", "маленький"},
	{"加大", "extra large", "увеличенный"},
	{"柚木", "teak", "тик"},
	{"原木", "natural wood", "натуральное дерево"},
	{"猫抓皮", "scratch resistant leather", "synthetic leather", "экокожа", "антивандальная кожа"},
	{"泰迪绒", "羊羔绒", "teddy fleece", "sherpa", "овчин", "букл"},
	{"底座", "charging stand", "charging dock", "зарядная подставка", "док-станция"},
	{"充电头", "charger", "power adapter", "зарядное устройство", "адаптер питания"},
	{"内存卡", "memory card", "microsd card", "карта памяти"},
}

func classifyOzonAttributeSKUVariationSemantics(attribute string, values []string) []string {
	text := strings.ToLower(strings.Join(append([]string{attribute}, values...), " "))
	out := make([]string, 0, 4)
	if containsAnyOzonAttributePolicy(text, "颜色", "色号", "color", "colour", "цвет") {
		out = append(out, "color")
	}
	if containsAnyOzonAttributePolicy(text, "尺寸", "尺码", "大小", "规格", "大号", "中号", "小号", "加大", "size", "dimension", "размер") {
		out = append(out, "size")
	}
	if containsAnyOzonAttributePolicy(
		text,
		"材质", "材料", "面料", "木板", "猫抓皮", "泰迪绒", "羊羔绒", "皮革", "绒面",
		"material", "fabric", "leather", "fleece", "teddy", "sherpa",
		"материал", "ткан", "кож", "овчин", "букле",
	) {
		out = append(out, "material")
	}
	if containsAnyOzonAttributePolicy(text, "型号", "款式", "model", "модель") {
		out = append(out, "model")
	}
	if containsAnyOzonAttributePolicy(
		text,
		"套餐", "套装", "标配", "配套", "底座", "充电头", "内存卡",
		"bundle", "package option", "standard package", "charging stand", "charger", "memory card",
		"комплект", "вариант комплекта", "зарядная подставка", "зарядное устройство", "карта памяти",
	) {
		out = append(out, "bundle")
	}
	if len(out) == 0 {
		out = append(out, "other")
	}
	return out
}

func excludeOzonAttributeSKUVariationConflicts(
	candidates []ozonAttributeSuggestionCandidate,
	prompts []ozonAttributePromptCandidate,
	variations []ozonAttributePromptSKUVariation,
) ([]ozonAttributeSuggestionCandidate, []ozonAttributePromptCandidate, []OzonAttributeSuggestionSkipped) {
	semantics := map[string]bool{}
	for _, variation := range variations {
		for _, semantic := range variation.Semantics {
			semantics[semantic] = true
		}
	}
	if len(semantics) == 0 {
		return candidates, prompts, nil
	}
	promptByKey := make(map[string]ozonAttributePromptCandidate, len(prompts))
	for _, prompt := range prompts {
		promptByKey[prompt.AttributeKey] = prompt
	}
	keptCandidates := make([]ozonAttributeSuggestionCandidate, 0, len(candidates))
	keptPrompts := make([]ozonAttributePromptCandidate, 0, len(prompts))
	skipped := make([]OzonAttributeSuggestionSkipped, 0)
	for _, candidate := range candidates {
		reason := ozonAttributeSKUVariationConflictReason(candidate.attr, semantics)
		if reason != "" {
			skipped = append(skipped, OzonAttributeSuggestionSkipped{
				AttributeID: candidate.attr.AttrID, AttributeName: candidate.attr.Name,
				Kind: ozonAttributeSkipUnsupported, Reason: reason,
			})
			continue
		}
		keptCandidates = append(keptCandidates, candidate)
		keptPrompts = append(keptPrompts, promptByKey[candidate.key])
	}
	return keptCandidates, keptPrompts, skipped
}

func ozonAttributeSKUVariationConflictReason(attr shop.OzonAttributeDTO, semantics map[string]bool) string {
	id := strings.TrimSpace(attr.AttrID)
	nameText := strings.ToLower(strings.Join(strings.Fields(attr.Name), " "))
	text := strings.ToLower(strings.Join(strings.Fields(attr.Name+" "+attr.Description), " "))
	if semantics["size"] && containsAnyOzonAttributePolicy(nameText, "尺寸", "尺码", "size", "размер") {
		return "代表 SKU 存在尺寸/尺码差异，该字段需要逐 SKU 确认，已留空"
	}
	sizeOrPhysical := ozonAttributeVariantPhysicalIDs[id] || containsAnyOzonAttributePolicy(
		text, "无包装尺寸", "商品重量", "净重", "长度", "宽度", "高度", "容积", "容量", "unpackaged dimensions", "net weight", "length", "width", "height", "volume", "размер без упаковки", "вес товара", "длина", "ширина", "высота", "объем", "объём",
	)
	if semantics["size"] && sizeOrPhysical {
		return "代表 SKU 存在尺寸规格差异，该物理数值无法可靠作为整商品普通属性，已按逐 SKU 变体留空"
	}
	if semantics["color"] && (id == "10097" || containsAnyOzonAttributePolicy(text, "颜色名称", "商品颜色", "product color", "цвет товара")) {
		return "代表 SKU 存在颜色差异，该字段需要逐 SKU 值，已留空"
	}
	if semantics["material"] && containsAnyOzonAttributePolicy(text, "材质", "材料", "material", "материал") {
		return "代表 SKU 存在材质差异，该字段需要逐 SKU 值，已留空"
	}
	if semantics["model"] && (id == "9048" || id == "12141" || containsAnyOzonAttributePolicy(text, "型号", "model", "модель")) {
		return "代表 SKU 存在型号差异，该字段需要逐 SKU 值，已留空"
	}
	if semantics["bundle"] && (id == "4384" || id == "12209" || containsAnyOzonAttributePolicy(
		text, "配套", "包装内产品配件", "套装内容", "included accessories", "in the box", "комплектация", "комплект поставки",
	)) {
		return "代表 SKU 存在套餐/配件差异，该配套字段需要逐 SKU 确认，已留空"
	}
	return ""
}

func isOzonAttributeProductCopyField(attr shop.OzonAttributeDTO) bool {
	id := strings.TrimSpace(attr.AttrID)
	if id == "4180" || id == "4191" || id == "23171" {
		return true
	}
	text := strings.ToLower(strings.Join(strings.Fields(attr.Name), " "))
	return containsAnyOzonAttributePolicy(
		text, "商品名称", "商品简介", "主题标签", "product name", "product introduction", "hashtags", "наименование товара", "аннотация товара", "хэштег",
	)
}

func isOzonAttributeProductNameField(attr shop.OzonAttributeDTO) bool {
	if strings.TrimSpace(attr.AttrID) == "4180" {
		return true
	}
	text := strings.ToLower(strings.Join(strings.Fields(attr.Name), " "))
	return containsAnyOzonAttributePolicy(text, "商品名称", "product name", "наименование товара") || text == "名称" || text == "name"
}

func ozonAttributePromptSemanticHint(attr shop.OzonAttributeDTO, options []ozonAttributeDictionaryOption) string {
	if strings.TrimSpace(attr.AttrID) != "12126" {
		return ""
	}
	labels := map[string]string{}
	for _, option := range options {
		labels[strings.TrimSpace(option.ID)] = strings.TrimSpace(option.Value)
	}
	monoblock := labels["970883112"]
	slider := labels["970883114"]
	clamshell := labels["971042229"]
	if monoblock == "" || slider == "" || clamshell == "" {
		return ""
	}
	return "这是移动电话机身形态（不是房产类型）。当前官方中文词典存在机器翻译歧义：" +
		"“" + monoblock + "”表示直板一体机，“" + slider + "”表示滑盖机，“" + clamshell +
		"”表示翻盖/折叠式手机。必须按商品结构选择，并逐字返回对应官方词典文本。"
}

func ozonAttributeKnownSemanticConflict(
	attr shop.OzonAttributeDTO,
	values []string,
	reason string,
	promptContext ozonAttributePromptContext,
) string {
	if isOzonAttributeProductCopyField(attr) {
		evidenceText := normalizeOzonRecommendationText(ozonAttributePromptProductEvidence(promptContext))
		valueText := normalizeOzonRecommendationText(strings.Join(values, " "))
		if containsAnyOzonAttributePolicy(evidenceText, "老人机", "老年机", "功能机", "featurephone", "series30", "кнопочный") &&
			containsAnyOzonAttributePolicy(valueText, "智能手机", "smartphone", "смартфон") {
			return "商品证据表明这是老人/按键功能机，文案却写成智能手机；必须改用中性的移动电话或功能机表述"
		}
		regionalCarrierClaim := containsAnyOzonAttributePolicy(
			valueText,
			"所有俄罗斯运营商", "全俄罗斯运营商", "allrussianoperator", "everyrussianoperator",
			"всехроссийскихоператор", "всемироссийскимиоператор", "любымиоператорамивроссии",
		)
		regionalCarrierEvidence := containsAnyOzonAttributePolicy(
			evidenceText,
			"俄罗斯运营商", "russianoperator", "российскихоператор", "операторамивроссии",
		)
		if regionalCarrierClaim && !regionalCarrierEvidence {
			return "商品证据没有俄罗斯运营商兼容性声明；“全网通”不能扩写成兼容所有俄罗斯运营商，必须删除该跨地区网络承诺"
		}
		if containsAnyOzonAttributePolicy(
			valueText,
			"单sim双待", "单卡双待", "singlesimdualstandby", "однаsimдвойноерезервирование",
		) {
			return "商品文案把单个物理 SIM 写成双卡/双待能力，语义自相矛盾；必须删除该表述或依据全部商品证据确认真实 SIM 配置"
		}
	}

	if strings.TrimSpace(attr.AttrID) == "12126" {
		evidence := strings.ToLower(ozonAttributePromptProductEvidence(promptContext))
		if containsAnyOzonAttributePolicy(evidence, "翻盖", "flip", "clamshell", "折叠式", "расклад") {
			valueText := normalizeOzonRecommendationText(strings.Join(values, " "))
			if containsAnyOzonAttributePolicy(valueText, "一体式", "monoblock", "моноблок", "滑块", "slider", "слайдер") {
				return "商品证据明确为翻盖/折叠式手机，当前值却是直板或滑盖机身；必须改选 semanticHint 中对应翻盖结构的官方语义文本"
			}
		}
	}

	unitText := strings.ToLower(strings.Join(strings.Fields(attr.Name+" "+attr.Description), " "))
	if !containsAnyOzonAttributePolicy(unitText, "小时", "hours", " hour", "，h", ",h", "час") || len(values) != 1 {
		return ""
	}
	hours, err := strconv.ParseFloat(strings.TrimSpace(values[0]), 64)
	if err != nil {
		return ""
	}
	for _, match := range ozonAttributeDayDurationPattern.FindAllStringSubmatch(reason, -1) {
		if len(match) < 2 {
			continue
		}
		days, parseErr := strconv.ParseFloat(match[1], 64)
		if parseErr == nil && math.Abs(hours-days) < 0.0001 {
			return "推断理由使用了天数，但属性单位是小时；必须先乘以 24 后返回，不能原样复制天数"
		}
	}
	return ""
}

func ozonAttributePromptProductEvidence(promptContext ozonAttributePromptContext) string {
	parts := []string{promptContext.ProductTitle, promptContext.ProductDescription}
	for key, value := range promptContext.ProductAttributes {
		parts = append(parts, key, value)
	}
	for _, sku := range promptContext.RepresentativeSKUs {
		parts = append(parts, sku.SKUCode, sku.SKUName)
		for key, value := range sku.Attributes {
			parts = append(parts, key, value)
		}
	}
	for _, fact := range promptContext.Facts {
		parts = append(parts, fact.Name, fact.Value, fact.Evidence)
	}
	return strings.Join(parts, " ")
}

func ozonAttributeUngroundedLatinNameToken(values []string, promptContext ozonAttributePromptContext) string {
	evidenceParts := []string{
		promptContext.ProductTitle,
		promptContext.ProductDescription,
		promptContext.Category.FullPath,
		promptContext.Category.Name,
		promptContext.Category.ProductType,
	}
	for key, value := range promptContext.ProductAttributes {
		evidenceParts = append(evidenceParts, key, value)
	}
	for _, sku := range promptContext.RepresentativeSKUs {
		evidenceParts = append(evidenceParts, sku.SKUCode, sku.SKUName)
		for key, value := range sku.Attributes {
			evidenceParts = append(evidenceParts, key, value)
		}
	}
	for _, fact := range promptContext.Facts {
		evidenceParts = append(evidenceParts, fact.Name, fact.Value, fact.Evidence)
	}
	evidenceTokens := map[string]bool{}
	for _, token := range ozonAttributeLatinNameTokenPattern.FindAllString(strings.Join(evidenceParts, " "), -1) {
		evidenceTokens[strings.ToLower(token)] = true
	}
	allowedUnits := map[string]bool{
		"cm": true, "mm": true, "m": true, "g": true, "kg": true,
		"ml": true, "l": true, "pcs": true,
	}
	for _, token := range ozonAttributeLatinNameTokenPattern.FindAllString(strings.Join(values, " "), -1) {
		normalized := strings.ToLower(token)
		if allowedUnits[normalized] || evidenceTokens[normalized] {
			continue
		}
		return token
	}
	return ""
}

func ozonAttributeSingleSKUVariantMention(text string, variations []ozonAttributePromptSKUVariation) string {
	normalizedText := normalizeOzonRecommendationText(text)
	if normalizedText == "" {
		return ""
	}
	for _, variation := range variations {
		values := make([]string, 0, len(variation.Values))
		for _, value := range variation.Values {
			if normalized := normalizeOzonRecommendationText(value); normalized != "" {
				values = append(values, normalized)
			}
		}
		if len(values) < 2 {
			continue
		}
		for _, value := range values {
			if strings.Contains(normalizedText, value) {
				return variation.Attribute
			}
		}
		for _, aliases := range ozonAttributeVariantAliasGroups {
			matchedValues := 0
			for _, value := range values {
				if containsNormalizedOzonAttributeAlias(value, aliases) {
					matchedValues++
				}
			}
			if matchedValues > 0 && matchedValues < len(values) && containsNormalizedOzonAttributeAlias(normalizedText, aliases) {
				return variation.Attribute
			}
		}
	}
	return ""
}

func containsNormalizedOzonAttributeAlias(normalized string, aliases []string) bool {
	for _, alias := range aliases {
		if marker := normalizeOzonRecommendationText(alias); marker != "" && strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func externalOzonAttributeReason(attr shop.OzonAttributeDTO) string {
	id := strings.TrimSpace(attr.AttrID)
	if intrinsicOzonAttributeIDs[id] {
		return ""
	}
	if reason := externalOzonAttributeIDs[id]; reason != "" {
		return reason
	}
	// IDs are authoritative. These narrow semantic fallbacks cover new aliases
	// without treating every occurrence of words such as "包装" or "视频" as
	// external; ambiguous attributes remain fillable by default.
	text := strings.ToLower(strings.Join(strings.Fields(attr.Name+" "+attr.Description), " "))
	switch {
	case containsAnyOzonAttributePolicy(text, "卖家代码", "seller code", "код продавца"):
		return "卖家内部代码不属于商品本体信息，已留空"
	case containsAnyOzonAttributePolicy(text, "是否有序列号", "serial number visible", "виден серийный номер"):
		return "序列号可见性需要按实际包装确认，已留空"
	case containsAnyOzonAttributePolicy(text, "需要标记代码", "marking code", "код маркировки", "киз"):
		return "标记码状态需要按实际合规与发货流程确认，已留空"
	case containsAnyOzonAttributePolicy(text, "包装尺寸", "含包装重量", "shipping weight", "package dimensions", "вес с упаковкой", "габариты упаковки"):
		return "包装物流数据需要按实际包装测量，已留空"
	case containsAnyOzonAttributePolicy(text, "产品危险等级", "烟火产品危险等级", "hazard class", "класс опасности"):
		return "危险等级需要依据安全或物流文件确认，已留空"
	case containsAnyOzonAttributePolicy(text, "hs编码", "hs code", "тн вэд"):
		return "HS 编码需要依据海关归类确认，已留空"
	case containsAnyOzonAttributePolicy(text, "臭氧。视频：链接", "视频背景：链接", "pdf 文件", "document pdf", "документ pdf", "json富内容"):
		return "外部媒体内容需要对应实际上传资产，已留空"
	case containsAnyOzonAttributePolicy(text, "组合成类似的产品", "similar product group", "объединить в похожие"):
		return "相似商品分组属于店铺运营配置，已留空"
	default:
		return ""
	}
}

func containsAnyOzonAttributePolicy(value string, markers ...string) bool {
	for _, marker := range markers {
		if strings.Contains(value, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

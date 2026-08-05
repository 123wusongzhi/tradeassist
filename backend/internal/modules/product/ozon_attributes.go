package product

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
)

const OzonPlatformAttributesVersion = 3

// OzonSKUVariantValidationError identifies failures owned by the per-SKU
// variant mapping rather than the shared product-level attribute form. Callers
// use this distinction to show the user the correct remediation path.
type OzonSKUVariantValidationError struct {
	err error
}

func (e *OzonSKUVariantValidationError) Error() string { return e.err.Error() }
func (e *OzonSKUVariantValidationError) Unwrap() error { return e.err }

func newOzonSKUVariantValidationError(format string, args ...any) error {
	return &OzonSKUVariantValidationError{err: fmt.Errorf(format, args...)}
}

// IsOzonSKUVariantValidationError reports whether validation failed in the
// per-SKU variant mapping instead of the shared product attributes.
func IsOzonSKUVariantValidationError(err error) bool {
	var target *OzonSKUVariantValidationError
	return errors.As(err, &target)
}

type OzonAttributeSelection struct {
	Value             string `json:"value"`
	DictionaryValueID string `json:"dictionaryValueId,omitempty"`
}

type OzonComplexAttributeGroup struct {
	ComplexID  int64                               `json:"complexId"`
	Attributes map[string][]OzonAttributeSelection `json:"attributes"`
}

// OzonPlatformAttributePayload preserves multiple values, repeated complex
// combinations and explicit per-SKU variant selections. Version 3 keeps the
// v2 product-level fields intact and adds a product+shop-scoped SKU mapping.
// DecodeOzonPlatformAttributes continues to read legacy and v2 configurations.
type OzonPlatformAttributePayload struct {
	Version                int                                            `json:"version"`
	Attributes             map[string][]OzonAttributeSelection            `json:"attributes"`
	ComplexGroups          []OzonComplexAttributeGroup                    `json:"complexGroups"`
	SKUVariantAttributeIDs []string                                       `json:"skuVariantAttributeIds"`
	SKUAttributeOverrides  map[string]map[string][]OzonAttributeSelection `json:"skuAttributeOverrides"`
}

// OzonEffectiveAttributePayload is the exact attribute set for one Ozon import
// item after applying the saved SKU overrides to the product-level values.
type OzonEffectiveAttributePayload struct {
	Version                int                                 `json:"version"`
	Attributes             map[string][]OzonAttributeSelection `json:"attributes"`
	ComplexGroups          []OzonComplexAttributeGroup         `json:"complexGroups"`
	SKUVariantAttributeIDs []string                            `json:"skuVariantAttributeIds"`
}

func DecodeOzonPlatformAttributes(raw []byte) (OzonPlatformAttributePayload, error) {
	payload := OzonPlatformAttributePayload{
		Version: OzonPlatformAttributesVersion, Attributes: map[string][]OzonAttributeSelection{}, ComplexGroups: []OzonComplexAttributeGroup{},
		SKUVariantAttributeIDs: []string{}, SKUAttributeOverrides: map[string]map[string][]OzonAttributeSelection{},
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("{}")) {
		return payload, nil
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return payload, fmt.Errorf("platformAttributes must be valid JSON: %w", err)
	}
	_, hasVersion := probe["version"]
	_, hasAttributes := probe["attributes"]
	_, hasComplexGroups := probe["complexGroups"]
	_, hasSKUVariantAttributeIDs := probe["skuVariantAttributeIds"]
	_, hasSKUAttributeOverrides := probe["skuAttributeOverrides"]
	if hasVersion || hasAttributes || hasComplexGroups || hasSKUVariantAttributeIDs || hasSKUAttributeOverrides {
		if err := json.Unmarshal(trimmed, &payload); err != nil {
			return payload, fmt.Errorf("Ozon platformAttributes is invalid: %w", err)
		}
		if payload.Version == 0 {
			payload.Version = OzonPlatformAttributesVersion
		}
		if payload.Version != 2 && payload.Version != OzonPlatformAttributesVersion {
			return payload, fmt.Errorf("unsupported Ozon platformAttributes version: %d", payload.Version)
		}
		if payload.Attributes == nil {
			payload.Attributes = map[string][]OzonAttributeSelection{}
		}
		if payload.ComplexGroups == nil {
			payload.ComplexGroups = []OzonComplexAttributeGroup{}
		}
		if payload.SKUVariantAttributeIDs == nil {
			payload.SKUVariantAttributeIDs = []string{}
		}
		if payload.SKUAttributeOverrides == nil {
			payload.SKUAttributeOverrides = map[string]map[string][]OzonAttributeSelection{}
		}
		return normalizeOzonPlatformAttributePayload(payload), nil
	}

	// Legacy product configs stored one value per top-level attribute ID.
	for attrID, rawValue := range probe {
		var legacy struct {
			Value             any `json:"value"`
			DictionaryValueID any `json:"dictionaryValueId"`
		}
		if err := json.Unmarshal(rawValue, &legacy); err != nil {
			return payload, fmt.Errorf("Ozon attribute %s must use {value,dictionaryValueId}: %w", attrID, err)
		}
		value := strings.TrimSpace(fmt.Sprint(legacy.Value))
		if legacy.Value == nil || value == "<nil>" {
			value = ""
		}
		dictionaryID := strings.TrimSpace(fmt.Sprint(legacy.DictionaryValueID))
		if legacy.DictionaryValueID == nil || dictionaryID == "<nil>" {
			dictionaryID = ""
		}
		if value != "" || dictionaryID != "" {
			payload.Attributes[strings.TrimSpace(attrID)] = []OzonAttributeSelection{{Value: value, DictionaryValueID: dictionaryID}}
		}
	}
	return normalizeOzonPlatformAttributePayload(payload), nil
}

func normalizeOzonPlatformAttributePayload(payload OzonPlatformAttributePayload) OzonPlatformAttributePayload {
	payload.Version = OzonPlatformAttributesVersion
	normalizedAttributes := make(map[string][]OzonAttributeSelection, len(payload.Attributes))
	for rawID, values := range payload.Attributes {
		id := strings.TrimSpace(rawID)
		normalized := normalizeOzonSelections(values)
		if id != "" && len(normalized) > 0 {
			normalizedAttributes[id] = normalized
		}
	}
	payload.Attributes = normalizedAttributes
	groups := make([]OzonComplexAttributeGroup, 0, len(payload.ComplexGroups))
	for _, group := range payload.ComplexGroups {
		normalizedGroupAttributes := make(map[string][]OzonAttributeSelection, len(group.Attributes))
		for rawID, values := range group.Attributes {
			id := strings.TrimSpace(rawID)
			normalized := normalizeOzonSelections(values)
			if id != "" && len(normalized) > 0 {
				normalizedGroupAttributes[id] = normalized
			}
		}
		group.Attributes = normalizedGroupAttributes
		groups = append(groups, group)
	}
	payload.ComplexGroups = groups
	if payload.ComplexGroups == nil {
		payload.ComplexGroups = []OzonComplexAttributeGroup{}
	}
	variantIDs := make([]string, 0, len(payload.SKUVariantAttributeIDs))
	variantSeen := map[string]struct{}{}
	for _, rawID := range payload.SKUVariantAttributeIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, exists := variantSeen[id]; exists {
			continue
		}
		variantSeen[id] = struct{}{}
		variantIDs = append(variantIDs, id)
	}
	normalizedOverrides := make(map[string]map[string][]OzonAttributeSelection, len(payload.SKUAttributeOverrides))
	for rawSKUID, rawAttributes := range payload.SKUAttributeOverrides {
		skuID := strings.TrimSpace(rawSKUID)
		if skuID == "" {
			continue
		}
		if parsed, err := uuid.Parse(skuID); err == nil && parsed != uuid.Nil {
			skuID = parsed.String()
		}
		attributes := make(map[string][]OzonAttributeSelection, len(rawAttributes))
		for rawAttrID, values := range rawAttributes {
			attrID := strings.TrimSpace(rawAttrID)
			normalized := normalizeOzonSelections(values)
			if attrID == "" || len(normalized) == 0 {
				continue
			}
			attributes[attrID] = normalized
			if _, exists := variantSeen[attrID]; !exists && len(variantIDs) == 0 {
				variantSeen[attrID] = struct{}{}
			}
		}
		if len(attributes) > 0 {
			normalizedOverrides[skuID] = attributes
		}
	}
	// Early v3 clients could omit skuVariantAttributeIds. Infer the dimensions
	// from saved overrides only when no explicit selection list was supplied.
	if len(variantIDs) == 0 && len(variantSeen) > 0 {
		for attrID := range variantSeen {
			variantIDs = append(variantIDs, attrID)
		}
	}
	sort.Strings(variantIDs)
	payload.SKUVariantAttributeIDs = variantIDs
	payload.SKUAttributeOverrides = normalizedOverrides
	if payload.SKUAttributeOverrides == nil {
		payload.SKUAttributeOverrides = map[string]map[string][]OzonAttributeSelection{}
	}
	return payload
}

func normalizeOzonSelections(values []OzonAttributeSelection) []OzonAttributeSelection {
	out := make([]OzonAttributeSelection, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value.Value = strings.TrimSpace(value.Value)
		value.DictionaryValueID = strings.TrimSpace(value.DictionaryValueID)
		if value.Value == "" && value.DictionaryValueID == "" {
			continue
		}
		key := value.DictionaryValueID + "\x00" + value.Value
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func CanonicalOzonPlatformAttributes(schema []shop.PlatformCategoryAttribute, raw datatypes.JSON, requireComplete bool) (datatypes.JSON, error) {
	return CanonicalOzonPlatformAttributesForSKUs(schema, raw, nil, requireComplete)
}

// CanonicalOzonPlatformAttributesForSKUs validates both the category schema
// and the current product SKU scope before storing the canonical v3 payload.
func CanonicalOzonPlatformAttributesForSKUs(schema []shop.PlatformCategoryAttribute, raw datatypes.JSON, skuIDs []uuid.UUID, requireComplete bool) (datatypes.JSON, error) {
	payload, err := DecodeOzonPlatformAttributes(raw)
	if err != nil {
		return nil, err
	}
	if err := ValidateOzonPlatformAttributePayloadForSKUs(schema, payload, skuIDs, requireComplete); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(encoded), nil
}

func ValidateOzonPlatformAttributePayload(schema []shop.PlatformCategoryAttribute, payload OzonPlatformAttributePayload, requireComplete bool) error {
	return ValidateOzonPlatformAttributePayloadForSKUs(schema, payload, nil, requireComplete)
}

// ValidateOzonPlatformAttributePayloadForSKUs treats product-level attributes
// as the common base and requires explicit, unique variant tuples for every
// SKU in a multi-SKU product. Complex attributes cannot be used as SKU
// dimensions until Ozon's repeated group semantics can be represented safely.
func ValidateOzonPlatformAttributePayloadForSKUs(schema []shop.PlatformCategoryAttribute, payload OzonPlatformAttributePayload, skuIDs []uuid.UUID, requireComplete bool) error {
	byID := make(map[string]shop.PlatformCategoryAttribute, len(schema))
	metaByID := make(map[string]ozonAttributeMeta, len(schema))
	complexSchema := map[int64][]shop.PlatformCategoryAttribute{}
	for _, attr := range schema {
		byID[attr.AttrID] = attr
		meta := parseOzonAttributeMeta(attr.Raw)
		metaByID[attr.AttrID] = meta
		if meta.AttributeComplexID > 0 {
			complexSchema[meta.AttributeComplexID] = append(complexSchema[meta.AttributeComplexID], attr)
		}
	}
	for attrID, values := range payload.Attributes {
		attr, ok := byID[attrID]
		if !ok {
			return fmt.Errorf("Ozon 属性模板中不存在属性 %s", attrID)
		}
		meta := metaByID[attrID]
		if meta.AttributeComplexID > 0 && len(values) > 0 {
			return fmt.Errorf("Ozon 组合属性 %s 必须填写在可重复字段组中", attr.Name)
		}
		if err := validateOzonAttributeSelections(attr, meta, values); err != nil {
			return err
		}
	}

	variantSet := make(map[string]struct{}, len(payload.SKUVariantAttributeIDs))
	for _, attrID := range payload.SKUVariantAttributeIDs {
		attr, ok := byID[attrID]
		if !ok {
			return newOzonSKUVariantValidationError("Ozon 属性模板中不存在 SKU 变体属性 %s", attrID)
		}
		if metaByID[attrID].AttributeComplexID > 0 {
			return newOzonSKUVariantValidationError("Ozon 组合属性 %s 暂不支持作为 SKU 变体；请改用普通变体属性或拆分商品", attr.Name)
		}
		variantSet[attrID] = struct{}{}
	}
	knownSKUs := make(map[string]struct{}, len(skuIDs))
	// nil means the legacy product-level caller did not provide SKU scope.
	// A non-nil empty slice is intentionally different: the product is known to
	// have zero SKUs, so any persisted override is stale and must be rejected.
	validateSKUScope := skuIDs != nil
	for _, skuID := range skuIDs {
		if skuID != uuid.Nil {
			knownSKUs[skuID.String()] = struct{}{}
		}
	}
	for skuID, override := range payload.SKUAttributeOverrides {
		if validateSKUScope {
			if _, ok := knownSKUs[skuID]; !ok {
				return newOzonSKUVariantValidationError("Ozon SKU 变体属性引用了当前商品不存在的 SKU：%s", skuID)
			}
		}
		for attrID, values := range override {
			attr, ok := byID[attrID]
			if !ok {
				return newOzonSKUVariantValidationError("Ozon 属性模板中不存在 SKU 变体属性 %s", attrID)
			}
			if metaByID[attrID].AttributeComplexID > 0 {
				return newOzonSKUVariantValidationError("Ozon 组合属性 %s 暂不支持作为 SKU 变体；请改用普通变体属性或拆分商品", attr.Name)
			}
			if _, selected := variantSet[attrID]; !selected {
				return newOzonSKUVariantValidationError("Ozon SKU %s 保存了未选作变体维度的属性：%s", skuID, attr.Name)
			}
			if err := validateOzonAttributeSelections(attr, metaByID[attrID], values); err != nil {
				return newOzonSKUVariantValidationError("Ozon SKU %s：%w", skuID, err)
			}
		}
	}

	groupCount := map[int64]int{}
	for index, group := range payload.ComplexGroups {
		defs, exists := complexSchema[group.ComplexID]
		if group.ComplexID <= 0 || !exists {
			return fmt.Errorf("Ozon 组合属性第 %d 组引用了未知的 complexId", index+1)
		}
		groupCount[group.ComplexID]++
		for attrID, values := range group.Attributes {
			attr, ok := byID[attrID]
			if !ok {
				return fmt.Errorf("Ozon 属性模板中不存在组合属性 %s", attrID)
			}
			meta := metaByID[attrID]
			if meta.AttributeComplexID != group.ComplexID {
				return fmt.Errorf("Ozon 组合属性 %s 不属于 complexId %d", attr.Name, group.ComplexID)
			}
			if err := validateOzonAttributeSelections(attr, meta, values); err != nil {
				return err
			}
		}
		if requireComplete {
			for _, attr := range defs {
				if !attr.Required {
					continue
				}
				if len(group.Attributes[attr.AttrID]) == 0 {
					return fmt.Errorf("Ozon 组合属性第 %d 组缺少必填项：%s", index+1, attr.Name)
				}
			}
		}
	}
	for complexID, count := range groupCount {
		if count <= 1 {
			continue
		}
		repeatable := false
		for _, attr := range complexSchema[complexID] {
			if metaByID[attr.AttrID].ComplexIsCollection {
				repeatable = true
				break
			}
		}
		if !repeatable {
			return fmt.Errorf("Ozon 组合属性 complexId %d 不允许重复字段组", complexID)
		}
	}
	if !requireComplete {
		return nil
	}
	if len(skuIDs) > 1 && len(variantSet) == 0 {
		return newOzonSKUVariantValidationError("Ozon 多 SKU 商品必须选择至少一个普通类目属性作为 SKU 变体维度")
	}
	seenTuples := map[string]string{}
	for _, skuID := range skuIDs {
		key := skuID.String()
		override := payload.SKUAttributeOverrides[key]
		for _, attrID := range payload.SKUVariantAttributeIDs {
			if len(override[attrID]) == 0 {
				return newOzonSKUVariantValidationError("Ozon SKU %s 缺少变体属性：%s", key, byID[attrID].Name)
			}
		}
		if len(payload.SKUVariantAttributeIDs) > 0 {
			tuple := ozonSKUVariantTuple(payload.SKUVariantAttributeIDs, override)
			if previous, exists := seenTuples[tuple]; exists {
				return newOzonSKUVariantValidationError("Ozon SKU %s 与 SKU %s 的变体属性组合重复", key, previous)
			}
			seenTuples[tuple] = key
		}
	}
	for _, attr := range schema {
		if !attr.Required {
			continue
		}
		meta := metaByID[attr.AttrID]
		if meta.AttributeComplexID > 0 {
			found := false
			for _, group := range payload.ComplexGroups {
				if group.ComplexID == meta.AttributeComplexID && len(group.Attributes[attr.AttrID]) > 0 {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Ozon 必填组合属性未填写：%s", attr.Name)
			}
			continue
		}
		if len(skuIDs) == 0 {
			if len(payload.Attributes[attr.AttrID]) == 0 {
				return fmt.Errorf("Ozon 必填属性未填写：%s", attr.Name)
			}
			continue
		}
		for _, skuID := range skuIDs {
			if len(effectiveOzonAttributeSelections(payload, skuID.String(), attr.AttrID)) == 0 {
				if _, isVariant := variantSet[attr.AttrID]; isVariant {
					return newOzonSKUVariantValidationError("Ozon SKU %s 缺少必填属性：%s", skuID, attr.Name)
				}
				return fmt.Errorf("Ozon SKU %s 缺少必填属性：%s", skuID, attr.Name)
			}
		}
	}
	return nil
}

// ResolveOzonEffectiveSKUAttributes returns the immutable per-item attribute
// payload used by preview, task snapshot construction and the Ozon adapter.
func ResolveOzonEffectiveSKUAttributes(payload OzonPlatformAttributePayload, skuID string) OzonEffectiveAttributePayload {
	attributes := make(map[string][]OzonAttributeSelection, len(payload.Attributes))
	for attrID, values := range payload.Attributes {
		attributes[attrID] = append([]OzonAttributeSelection(nil), values...)
	}
	for attrID, values := range payload.SKUAttributeOverrides[strings.TrimSpace(skuID)] {
		attributes[attrID] = append([]OzonAttributeSelection(nil), values...)
	}
	groups := make([]OzonComplexAttributeGroup, 0, len(payload.ComplexGroups))
	for _, group := range payload.ComplexGroups {
		copied := OzonComplexAttributeGroup{ComplexID: group.ComplexID, Attributes: map[string][]OzonAttributeSelection{}}
		for attrID, values := range group.Attributes {
			copied.Attributes[attrID] = append([]OzonAttributeSelection(nil), values...)
		}
		groups = append(groups, copied)
	}
	return OzonEffectiveAttributePayload{
		Version: OzonPlatformAttributesVersion, Attributes: attributes, ComplexGroups: groups,
		SKUVariantAttributeIDs: append([]string(nil), payload.SKUVariantAttributeIDs...),
	}
}

func effectiveOzonAttributeSelections(payload OzonPlatformAttributePayload, skuID, attrID string) []OzonAttributeSelection {
	if override := payload.SKUAttributeOverrides[skuID][attrID]; len(override) > 0 {
		return override
	}
	return payload.Attributes[attrID]
}

func ozonSKUVariantTuple(attributeIDs []string, override map[string][]OzonAttributeSelection) string {
	parts := make([]string, 0, len(attributeIDs))
	for _, attrID := range attributeIDs {
		values := make([]string, 0, len(override[attrID]))
		for _, selection := range override[attrID] {
			values = append(values, strings.TrimSpace(selection.DictionaryValueID)+"\x00"+strings.TrimSpace(selection.Value))
		}
		sort.Strings(values)
		parts = append(parts, attrID+"="+strings.Join(values, "\x01"))
	}
	return strings.Join(parts, "\x02")
}

type ozonAttributeMeta struct {
	DictionaryID        string
	IsCollection        bool
	AttributeComplexID  int64
	MaxValueCount       int64
	ComplexIsCollection bool
}

func parseOzonAttributeMeta(raw datatypes.JSON) ozonAttributeMeta {
	var values map[string]any
	_ = json.Unmarshal(raw, &values)
	return ozonAttributeMeta{
		DictionaryID:        dictionaryIDFromProductAttributeRaw(values["dictionary_id"]),
		IsCollection:        boolFromOzonAttributeRaw(values["is_collection"]),
		AttributeComplexID:  int64FromOzonAttributeRaw(values["attribute_complex_id"]),
		MaxValueCount:       int64FromOzonAttributeRaw(values["max_value_count"]),
		ComplexIsCollection: boolFromOzonAttributeRaw(values["complex_is_collection"]),
	}
}

func validateOzonAttributeSelections(attr shop.PlatformCategoryAttribute, meta ozonAttributeMeta, values []OzonAttributeSelection) error {
	if !meta.IsCollection && len(values) > 1 {
		return fmt.Errorf("Ozon 属性 %s 不是多值属性，不能填写多个值", attr.Name)
	}
	if meta.MaxValueCount > 0 && int64(len(values)) > meta.MaxValueCount {
		return fmt.Errorf("Ozon 属性 %s 最多允许 %d 个值", attr.Name, meta.MaxValueCount)
	}
	if len(values) > 50 {
		return fmt.Errorf("Ozon 属性 %s 的值数量超过安全上限", attr.Name)
	}
	for _, value := range values {
		if value.Value == "" {
			return fmt.Errorf("Ozon 属性 %s 包含空值", attr.Name)
		}
		if meta.DictionaryID != "" {
			if value.DictionaryValueID == "" {
				return fmt.Errorf("Ozon 词典属性缺少 dictionaryValueId：%s", attr.Name)
			}
			if !cachedOzonDictionaryValueMatches(attr.Options, value.DictionaryValueID, value.Value) {
				return fmt.Errorf("Ozon 词典值与属性不匹配：%s", attr.Name)
			}
		} else if value.DictionaryValueID != "" {
			return fmt.Errorf("Ozon 非词典属性不能包含 dictionaryValueId：%s", attr.Name)
		}
	}
	return nil
}

func dictionaryIDFromProductAttributeRaw(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if value == nil || text == "" || text == "0" || text == "<nil>" {
		return ""
	}
	return strings.TrimSuffix(text, ".0")
}

func boolFromOzonAttributeRaw(value any) bool {
	result, _ := value.(bool)
	return result
}

func int64FromOzonAttributeRaw(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case json.Number:
		result, _ := typed.Int64()
		return result
	default:
		return 0
	}
}

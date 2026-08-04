package product

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
)

const OzonPlatformAttributesVersion = 2

type OzonAttributeSelection struct {
	Value             string `json:"value"`
	DictionaryValueID string `json:"dictionaryValueId,omitempty"`
}

type OzonComplexAttributeGroup struct {
	ComplexID  int64                               `json:"complexId"`
	Attributes map[string][]OzonAttributeSelection `json:"attributes"`
}

// OzonPlatformAttributePayload preserves multiple values and repeated complex
// combinations. Version 2 replaces the legacy attrId->{value,id} shape while
// DecodeOzonPlatformAttributes continues to read old product configurations.
type OzonPlatformAttributePayload struct {
	Version       int                                 `json:"version"`
	Attributes    map[string][]OzonAttributeSelection `json:"attributes"`
	ComplexGroups []OzonComplexAttributeGroup         `json:"complexGroups"`
}

func DecodeOzonPlatformAttributes(raw []byte) (OzonPlatformAttributePayload, error) {
	payload := OzonPlatformAttributePayload{
		Version: OzonPlatformAttributesVersion, Attributes: map[string][]OzonAttributeSelection{}, ComplexGroups: []OzonComplexAttributeGroup{},
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
	if hasVersion || hasAttributes || hasComplexGroups {
		if err := json.Unmarshal(trimmed, &payload); err != nil {
			return payload, fmt.Errorf("Ozon platformAttributes v2 is invalid: %w", err)
		}
		if payload.Version == 0 {
			payload.Version = OzonPlatformAttributesVersion
		}
		if payload.Version != OzonPlatformAttributesVersion {
			return payload, fmt.Errorf("unsupported Ozon platformAttributes version: %d", payload.Version)
		}
		if payload.Attributes == nil {
			payload.Attributes = map[string][]OzonAttributeSelection{}
		}
		if payload.ComplexGroups == nil {
			payload.ComplexGroups = []OzonComplexAttributeGroup{}
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
	payload, err := DecodeOzonPlatformAttributes(raw)
	if err != nil {
		return nil, err
	}
	if err := ValidateOzonPlatformAttributePayload(schema, payload, requireComplete); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(encoded), nil
}

func ValidateOzonPlatformAttributePayload(schema []shop.PlatformCategoryAttribute, payload OzonPlatformAttributePayload, requireComplete bool) error {
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
		if len(payload.Attributes[attr.AttrID]) == 0 {
			return fmt.Errorf("Ozon 必填属性未填写：%s", attr.Name)
		}
	}
	return nil
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

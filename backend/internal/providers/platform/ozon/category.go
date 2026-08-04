package ozon

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const (
	pathCategoryAttributes = "/v1/description-category/attribute"
	pathAttributeValues    = "/v1/description-category/attribute/values"
	pathAttributeSearch    = "/v1/description-category/attribute/values/search"

	maxMappedAttributes = 40
)

type ozonAttribute struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	Type                string `json:"type"`
	DictionaryID        int64  `json:"dictionary_id"`
	IsRequired          bool   `json:"is_required"`
	IsCollection        bool   `json:"is_collection"`
	AttributeComplexID  int64  `json:"attribute_complex_id"`
	MaxValueCount       int64  `json:"max_value_count"`
	ComplexIsCollection bool   `json:"complex_is_collection"`
	CategoryDependent   bool   `json:"category_dependent"`
}

type ozonAttributeValue struct {
	ComplexID int64           `json:"complex_id"`
	ID        int64           `json:"id"`
	Values    []ozonAttrValue `json:"values"`
}

type ozonAttrValue struct {
	DictionaryValueID int64  `json:"dictionary_value_id,omitempty"`
	Value             string `json:"value"`
}

func (c *ozonClient) getCategoryAttributes(ctx context.Context, categoryID, typeID int64) ([]ozonAttribute, error) {
	var resp struct {
		Result []ozonAttribute `json:"result"`
	}
	body := map[string]any{
		"description_category_id": categoryID,
		"type_id":                 typeID,
		"language":                "DEFAULT",
	}
	if err := c.postJSON(ctx, pathCategoryAttributes, body, &resp); err != nil {
		return nil, err
	}
	return resp.Result, nil
}

type dictionaryValue struct {
	ID    int64  `json:"id"`
	Value string `json:"value"`
}

func (c *ozonClient) searchAttributeValues(ctx context.Context, categoryID, typeID, attrID int64, value string) ([]dictionaryValue, error) {
	v := strings.TrimSpace(value)
	if len([]rune(v)) < 2 {
		return nil, nil
	}
	var resp struct {
		Result []dictionaryValue `json:"result"`
	}
	body := map[string]any{
		"attribute_id":            attrID,
		"description_category_id": categoryID,
		"type_id":                 typeID,
		"limit":                   100,
		"value":                   v,
	}
	if err := c.postJSON(ctx, pathAttributeSearch, body, &resp); err != nil {
		return nil, err
	}
	return resp.Result, nil
}

// searchAttributeValue deliberately accepts exact normalized text only. Ozon's
// search endpoint can return fuzzy candidates; silently selecting its first row
// can assign a valid dictionary ID to the wrong seller value.
func (c *ozonClient) searchAttributeValue(ctx context.Context, categoryID, typeID, attrID int64, value string) (*dictionaryValue, error) {
	rows, err := c.searchAttributeValues(ctx, categoryID, typeID, attrID, value)
	if err != nil {
		return nil, err
	}
	target := normalizeText(value)
	for i := range rows {
		dv := rows[i]
		if normalizeText(dv.Value) == target {
			return &dv, nil
		}
	}
	return nil, nil
}

func (c *ozonClient) validateDictionaryValue(
	ctx context.Context,
	categoryID, typeID, attrID, dictionaryValueID int64,
	value string,
) (*dictionaryValue, error) {
	if dictionaryValueID <= 0 || strings.TrimSpace(value) == "" {
		return nil, nil
	}
	if len([]rune(strings.TrimSpace(value))) >= 2 {
		rows, err := c.searchAttributeValues(ctx, categoryID, typeID, attrID, value)
		if err != nil {
			return nil, err
		}
		target := normalizeText(value)
		for i := range rows {
			if rows[i].ID == dictionaryValueID && normalizeText(rows[i].Value) == target {
				return &rows[i], nil
			}
		}
		return nil, nil
	}

	// Ozon requires at least two characters for the search endpoint. For short
	// values, use the cursor endpoint and stop as soon as the requested ID is
	// found or the monotonically increasing cursor has passed it.
	lastID := int64(0)
	for page := 0; page < 500; page++ {
		var resp struct {
			Result []dictionaryValue `json:"result"`
		}
		body := map[string]any{
			"description_category_id": categoryID,
			"type_id":                 typeID,
			"attribute_id":            attrID,
			"language":                "DEFAULT",
			"limit":                   100,
			"last_value_id":           lastID,
		}
		if err := c.postJSON(ctx, pathAttributeValues, body, &resp); err != nil {
			return nil, err
		}
		if len(resp.Result) == 0 {
			return nil, nil
		}
		nextID := lastID
		for i := range resp.Result {
			row := resp.Result[i]
			if row.ID == dictionaryValueID && normalizeText(row.Value) == normalizeText(value) {
				return &row, nil
			}
			if row.ID > nextID {
				nextID = row.ID
			}
		}
		if len(resp.Result) < 100 || nextID <= lastID || nextID > dictionaryValueID {
			return nil, nil
		}
		lastID = nextID
	}
	return nil, fmt.Errorf("ozon dictionary validation exceeded pagination limit")
}

type explicitOzonAttribute struct {
	Value              string
	DictionaryValueID  int64
	StrictDictionaryID bool
}

// categoryAttributeMatch is one auto-filled Ozon attribute.
type categoryAttributeMatch struct {
	attr        ozonAttribute
	dictValueID int64
	value       string
}

// buildCategoryAttributes resolves required + matched category attributes from the
// local product attributes, publish-config defaults and dictionary lookup.
// It returns the payload attributes and a list of unresolved required attribute names.
func (c *ozonClient) buildCategoryAttributes(
	ctx context.Context,
	categoryID, typeID int64,
	localAttrs map[string]string,
	merged ozonPublishMerged,
) ([]ozonAttributeValue, []string, []ozonAttribute, error) {
	return c.buildCategoryAttributesForPublish(ctx, categoryID, typeID, localAttrs, merged, nil, true)
}

func (c *ozonClient) buildCategoryAttributesForPublish(
	ctx context.Context,
	categoryID, typeID int64,
	localAttrs map[string]string,
	merged ozonPublishMerged,
	explicit map[string][]explicitOzonAttribute,
	allowAutoFill bool,
	preloaded ...[]ozonAttribute,
) ([]ozonAttributeValue, []string, []ozonAttribute, error) {
	if categoryID <= 0 || typeID <= 0 {
		return nil, nil, nil, fmt.Errorf("ozon publish requires description_category_id and type_id")
	}
	var attrs []ozonAttribute
	if len(preloaded) > 0 {
		attrs = append([]ozonAttribute(nil), preloaded[0]...)
	} else {
		var err error
		attrs, err = c.getCategoryAttributes(ctx, categoryID, typeID)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	if len(attrs) == 0 {
		return nil, nil, nil, nil
	}

	// 1) required attributes first, then optional matched by name.
	sort.SliceStable(attrs, func(i, j int) bool {
		if attrs[i].IsRequired != attrs[j].IsRequired {
			return attrs[i].IsRequired
		}
		return attrs[i].ID < attrs[j].ID
	})

	out := make([]ozonAttributeValue, 0, len(attrs))
	missing := make([]string, 0, 4)
	missingDefs := make([]ozonAttribute, 0, 4)
	seen := map[int64]bool{}
	usedExplicit := map[string]bool{}
	added := 0

	for _, a := range attrs {
		if seen[a.ID] || a.ID <= 0 {
			continue
		}
		selected, explicitKey, hasExplicit := explicitAttributeFor(a, explicit)
		if added >= maxMappedAttributes && !a.IsRequired && !hasExplicit {
			continue
		}
		value := ""
		dictID := int64(0)
		ok := false
		if hasExplicit {
			usedExplicit[explicitKey] = true
			resolved, resolvedOK, resolveErr := c.resolveExplicitOzonSelections(ctx, categoryID, typeID, a, selected)
			if resolveErr != nil {
				return nil, nil, nil, resolveErr
			}
			if resolvedOK {
				seen[a.ID] = true
				added++
				out = append(out, resolved)
				continue
			}
		} else if allowAutoFill {
			var resolveErr error
			value, dictID, ok, resolveErr = resolveAttributeValue(ctx, c, categoryID, typeID, a, localAttrs, merged)
			if resolveErr != nil {
				return nil, nil, nil, resolveErr
			}
		}
		if !ok {
			if a.IsRequired {
				missing = append(missing, attributeDisplayName(a))
				missingDefs = append(missingDefs, a)
			}
			continue
		}
		seen[a.ID] = true
		added++
		vals := []ozonAttrValue{{Value: value}}
		if dictID != 0 {
			vals[0].DictionaryValueID = dictID
		}
		out = append(out, ozonAttributeValue{
			ComplexID: a.AttributeComplexID,
			ID:        a.ID,
			Values:    vals,
		})
	}
	for key := range explicit {
		if !usedExplicit[key] {
			return nil, nil, nil, fmt.Errorf("ozon category template does not contain explicit attribute %s", key)
		}
	}
	return out, missing, missingDefs, nil
}

func explicitAttributeFor(a ozonAttribute, explicit map[string][]explicitOzonAttribute) ([]explicitOzonAttribute, string, bool) {
	if len(explicit) == 0 {
		return nil, "", false
	}
	id := strconv.FormatInt(a.ID, 10)
	if value, ok := explicit[id]; ok {
		return value, id, true
	}
	target := normalizeText(a.Name)
	for key, value := range explicit {
		if normalizeText(key) == target {
			return value, key, true
		}
	}
	return nil, "", false
}

func (c *ozonClient) resolveExplicitOzonSelections(
	ctx context.Context,
	categoryID, typeID int64,
	attr ozonAttribute,
	selections []explicitOzonAttribute,
) (ozonAttributeValue, bool, error) {
	if len(selections) == 0 {
		return ozonAttributeValue{}, false, nil
	}
	if !attr.IsCollection && len(selections) > 1 {
		return ozonAttributeValue{}, false, fmt.Errorf("ozon attribute %s does not allow multiple values", attributeDisplayName(attr))
	}
	if attr.MaxValueCount > 0 && int64(len(selections)) > attr.MaxValueCount {
		return ozonAttributeValue{}, false, fmt.Errorf("ozon attribute %s allows at most %d values", attributeDisplayName(attr), attr.MaxValueCount)
	}
	if len(selections) > 50 {
		return ozonAttributeValue{}, false, fmt.Errorf("ozon attribute %s contains too many values", attributeDisplayName(attr))
	}
	values := make([]ozonAttrValue, 0, len(selections))
	for _, selected := range selections {
		value := strings.TrimSpace(selected.Value)
		if value == "" {
			return ozonAttributeValue{}, false, fmt.Errorf("ozon attribute %s contains an empty selected value", attributeDisplayName(attr))
		}
		if attr.DictionaryID != 0 {
			if selected.StrictDictionaryID && selected.DictionaryValueID <= 0 {
				return ozonAttributeValue{}, false, fmt.Errorf("ozon dictionary attribute %s requires dictionaryValueId", attributeDisplayName(attr))
			}
			if selected.DictionaryValueID > 0 {
				matched, err := c.validateDictionaryValue(ctx, categoryID, typeID, attr.ID, selected.DictionaryValueID, value)
				if err != nil {
					return ozonAttributeValue{}, false, err
				}
				if matched == nil {
					return ozonAttributeValue{}, false, fmt.Errorf("ozon dictionary value does not belong to attribute %s", attributeDisplayName(attr))
				}
				values = append(values, ozonAttrValue{DictionaryValueID: matched.ID, Value: matched.Value})
				continue
			}
			matched, err := c.searchAttributeValue(ctx, categoryID, typeID, attr.ID, value)
			if err != nil {
				return ozonAttributeValue{}, false, err
			}
			if matched == nil {
				return ozonAttributeValue{}, false, fmt.Errorf("ozon dictionary value is not an exact match for attribute %s", attributeDisplayName(attr))
			}
			values = append(values, ozonAttrValue{DictionaryValueID: matched.ID, Value: matched.Value})
			continue
		}
		if selected.DictionaryValueID > 0 {
			return ozonAttributeValue{}, false, fmt.Errorf("ozon non-dictionary attribute %s cannot use dictionaryValueId", attributeDisplayName(attr))
		}
		values = append(values, ozonAttrValue{Value: value})
	}
	if len(values) == 0 {
		return ozonAttributeValue{}, false, nil
	}
	return ozonAttributeValue{ComplexID: attr.AttributeComplexID, ID: attr.ID, Values: values}, true, nil
}

func (c *ozonClient) buildExplicitOzonComplexGroups(
	ctx context.Context,
	categoryID, typeID int64,
	schema []ozonAttribute,
	payload explicitOzonAttributesPayload,
) ([]ozonComplexAttributeGroup, []string, error) {
	if payload.Legacy {
		return nil, nil, nil
	}
	defsByComplex := map[int64][]ozonAttribute{}
	byID := map[string]ozonAttribute{}
	for _, attr := range schema {
		if attr.AttributeComplexID <= 0 {
			continue
		}
		defsByComplex[attr.AttributeComplexID] = append(defsByComplex[attr.AttributeComplexID], attr)
		byID[strconv.FormatInt(attr.ID, 10)] = attr
	}
	counts := map[int64]int{}
	out := make([]ozonComplexAttributeGroup, 0, len(payload.ComplexGroups))
	for index, group := range payload.ComplexGroups {
		defs, exists := defsByComplex[group.ComplexID]
		if !exists {
			return nil, nil, fmt.Errorf("ozon complex attribute group %d references unknown complexId %d", index+1, group.ComplexID)
		}
		counts[group.ComplexID]++
		for key := range group.Attributes {
			attr, exists := byID[strings.TrimSpace(key)]
			if !exists || attr.AttributeComplexID != group.ComplexID {
				return nil, nil, fmt.Errorf("ozon complex group %d contains unknown attribute %s", index+1, key)
			}
		}
		sort.SliceStable(defs, func(i, j int) bool { return defs[i].ID < defs[j].ID })
		values := make([]ozonAttributeValue, 0, len(group.Attributes))
		for _, attr := range defs {
			key := strconv.FormatInt(attr.ID, 10)
			selections, hasValue := group.Attributes[key]
			if !hasValue {
				if attr.IsRequired {
					return nil, nil, fmt.Errorf("ozon complex attribute group %d is missing required attribute %s", index+1, attributeDisplayName(attr))
				}
				continue
			}
			resolved, ok, err := c.resolveExplicitOzonSelections(ctx, categoryID, typeID, attr, selections)
			if err != nil {
				return nil, nil, err
			}
			if !ok {
				if attr.IsRequired {
					return nil, nil, fmt.Errorf("ozon complex attribute group %d is missing required attribute %s", index+1, attributeDisplayName(attr))
				}
				continue
			}
			values = append(values, resolved)
		}
		if len(values) > 0 {
			out = append(out, ozonComplexAttributeGroup{Attributes: values})
		}
	}
	for complexID, count := range counts {
		if count <= 1 {
			continue
		}
		repeatable := false
		for _, attr := range defsByComplex[complexID] {
			if attr.ComplexIsCollection {
				repeatable = true
				break
			}
		}
		if !repeatable {
			return nil, nil, fmt.Errorf("ozon complexId %d does not allow repeated groups", complexID)
		}
	}
	missing := make([]string, 0)
	for complexID, defs := range defsByComplex {
		if counts[complexID] > 0 {
			continue
		}
		for _, attr := range defs {
			if attr.IsRequired {
				missing = append(missing, attributeDisplayName(attr))
			}
		}
	}
	return out, missing, nil
}

func attributeDisplayName(a ozonAttribute) string {
	if name := strings.TrimSpace(a.Name); name != "" {
		return name
	}
	return strconv.FormatInt(a.ID, 10)
}

// applySuggestedAttributes maps LLM-suggested values for missing required
// attributes. Dictionary attributes only keep values that matched a dictionary
// entry; unmatched ones stay missing instead of failing the whole import.
func (c *ozonClient) applySuggestedAttributes(
	ctx context.Context,
	categoryID, typeID int64,
	defs []ozonAttribute,
	suggestions map[string]string,
) ([]ozonAttributeValue, []string) {
	out := make([]ozonAttributeValue, 0, len(defs))
	missing := make([]string, 0, len(defs))
	for _, a := range defs {
		v := strings.TrimSpace(suggestions[a.Name])
		if v == "" {
			missing = append(missing, a.Name)
			continue
		}
		if a.DictionaryID != 0 {
			dv, err := c.searchAttributeValue(ctx, categoryID, typeID, a.ID, v)
			if err != nil || dv == nil {
				missing = append(missing, a.Name)
				continue
			}
			out = append(out, ozonAttributeValue{ComplexID: a.AttributeComplexID, ID: a.ID, Values: []ozonAttrValue{{DictionaryValueID: dv.ID, Value: dv.Value}}})
			continue
		}
		out = append(out, ozonAttributeValue{ComplexID: a.AttributeComplexID, ID: a.ID, Values: []ozonAttrValue{{Value: v}}})
	}
	return out, missing
}

func resolveAttributeValue(
	ctx context.Context,
	client *ozonClient,
	categoryID, typeID int64,
	a ozonAttribute,
	localAttrs map[string]string,
	merged ozonPublishMerged,
) (value string, dictID int64, ok bool, err error) {
	if v := strings.TrimSpace(localAttrs[strconv.FormatInt(a.ID, 10)]); v != "" {
		if a.DictionaryID != 0 {
			dv, lookupErr := client.searchAttributeValue(ctx, categoryID, typeID, a.ID, v)
			if lookupErr != nil {
				return "", 0, false, lookupErr
			}
			if dv == nil {
				return "", 0, false, nil
			}
			return dv.Value, dv.ID, true, nil
		}
		return v, 0, true, nil
	}
	// 1) local attribute by alias / normalized name.
	if v := matchLocalAttribute(a, localAttrs); v != "" {
		if a.DictionaryID != 0 {
			dv, lookupErr := client.searchAttributeValue(ctx, categoryID, typeID, a.ID, v)
			if lookupErr != nil {
				return "", 0, false, lookupErr
			}
			if dv != nil {
				return dv.Value, dv.ID, true, nil
			}
			return "", 0, false, nil
		}
		return v, 0, true, nil
	}
	// 2) publish-config defaults for common attributes.
	if v := defaultForAttribute(a, merged); v != "" {
		if a.DictionaryID != 0 {
			dv, lookupErr := client.searchAttributeValue(ctx, categoryID, typeID, a.ID, v)
			if lookupErr != nil {
				return "", 0, false, lookupErr
			}
			if dv != nil {
				return dv.Value, dv.ID, true, nil
			}
			return "", 0, false, nil
		}
		return v, 0, true, nil
	}
	// 3) user-configured extra defaults (attr_id -> value).
	if v := merged.AttributeDefaults[strconv.FormatInt(a.ID, 10)]; v != "" {
		if a.DictionaryID != 0 {
			dv, lookupErr := client.searchAttributeValue(ctx, categoryID, typeID, a.ID, v)
			if lookupErr != nil {
				return "", 0, false, lookupErr
			}
			if dv != nil {
				return dv.Value, dv.ID, true, nil
			}
			return "", 0, false, nil
		}
		return v, 0, true, nil
	}
	return "", 0, false, nil
}

func matchLocalAttribute(a ozonAttribute, local map[string]string) string {
	if len(local) == 0 {
		return ""
	}
	aliases := attributeAliases(strconv.FormatInt(a.ID, 10))
	aliases = append(aliases, a.Name)
	for _, alias := range aliases {
		key := normalizeText(alias)
		if key == "" {
			continue
		}
		if v, ok := local[key]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func defaultForAttribute(a ozonAttribute, merged ozonPublishMerged) string {
	switch a.ID {
	case 85: // Бренд
		return merged.DefaultBrand
	case 4389: // Страна-изготовитель
		return merged.DefaultCountry
	case 23487: // Производитель
		return merged.DefaultManufacturer
	case 8229: // Тип
		return merged.DefaultType
	default:
		return ""
	}
}

// attributeAliases maps well-known Ozon attribute IDs to common source names.
func attributeAliases(attrID string) []string {
	switch attrID {
	case "9048":
		return []string{"model", "model name", "название модели", "型号", "模型", "name"}
	case "85":
		return []string{"brand", "бренд", "品牌", "商标"}
	case "4389":
		return []string{"country", "country of origin", "origin country", "страна", "страна-изготовитель", "原产国", "产地", "国家", "制造国"}
	case "23487":
		return []string{"manufacturer", "производитель", "制造商", "生产商", "厂家"}
	case "8229":
		return []string{"type", "тип", "类型", "种类", "品类"}
	case "8205":
		return []string{"shelf life", "shelf life in days", "срок годности", "保质期"}
	case "8050":
		return []string{"composition", "состав", "材质", "成分", "材料"}
	case "4191":
		return []string{"description", "аннотация", "описание", "描述", "商品描述", "商品详情", "详情"}
	case "10096":
		return []string{"color", "цвет", "颜色", "色彩"}
	case "4180":
		return []string{"size", "размер", "尺寸", "尺码", "规格"}
	default:
		return nil
	}
}

// normalizeText normalizes names/values for fuzzy matching (case + punctuation insensitive).
func normalizeText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// localAttributeMap converts the provider-neutral draft attributes into a normalized map.
func localAttributeMap(draft platformp.PlatformProductDraft) map[string]string {
	out := map[string]string{}
	var raw any = draft.Attributes
	if m, ok := raw.(map[string]any); ok {
		if list, ok := m["attributes"].([]any); ok {
			for _, row := range list {
				rm, ok := row.(map[string]any)
				if !ok {
					continue
				}
				name := fmt.Sprint(rm["name"])
				value := fmt.Sprint(rm["value"])
				if name == "" || value == "" {
					continue
				}
				out[normalizeText(name)] = strings.TrimSpace(value)
			}
		}
		for k, v := range m {
			if k == "attributes" {
				continue
			}
			if v == nil {
				continue
			}
			out[normalizeText(k)] = strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return out
}

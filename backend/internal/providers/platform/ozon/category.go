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
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	DictionaryID int64  `json:"dictionary_id"`
	IsRequired   bool   `json:"is_required"`
	IsCollection bool   `json:"is_collection"`
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

func (c *ozonClient) searchAttributeValue(ctx context.Context, categoryID, typeID, attrID int64, value string) (*dictionaryValue, error) {
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
	target := normalizeText(v)
	var best *dictionaryValue
	for i := range resp.Result {
		dv := resp.Result[i]
		if normalizeText(dv.Value) == target {
			return &dv, nil
		}
		if best == nil {
			best = &dv
		}
	}
	return best, nil
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
	if categoryID <= 0 || typeID <= 0 {
		return nil, nil, nil, fmt.Errorf("ozon publish requires description_category_id and type_id")
	}
	attrs, err := c.getCategoryAttributes(ctx, categoryID, typeID)
	if err != nil {
		return nil, nil, nil, err
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
	added := 0

	for _, a := range attrs {
		if seen[a.ID] || a.ID <= 0 {
			continue
		}
		if added >= maxMappedAttributes {
			break
		}
		value, dictID, ok := resolveAttributeValue(ctx, c, categoryID, typeID, a, localAttrs, merged)
		if !ok {
			if a.IsRequired {
				missing = append(missing, a.Name)
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
			ComplexID: 0,
			ID:        a.ID,
			Values:    vals,
		})
	}
	return out, missing, missingDefs, nil
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
			out = append(out, ozonAttributeValue{ID: a.ID, Values: []ozonAttrValue{{DictionaryValueID: dv.ID, Value: dv.Value}}})
			continue
		}
		out = append(out, ozonAttributeValue{ID: a.ID, Values: []ozonAttrValue{{Value: v}}})
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
) (value string, dictID int64, ok bool) {
	// 1) local attribute by alias / normalized name.
	if v := matchLocalAttribute(a, localAttrs); v != "" {
		if a.DictionaryID != 0 {
			if dv, err := client.searchAttributeValue(ctx, categoryID, typeID, a.ID, v); err == nil && dv != nil {
				return dv.Value, dv.ID, true
			}
		}
		return v, 0, true
	}
	// 2) publish-config defaults for common attributes.
	if v := defaultForAttribute(a, merged); v != "" {
		if a.DictionaryID != 0 {
			if dv, err := client.searchAttributeValue(ctx, categoryID, typeID, a.ID, v); err == nil && dv != nil {
				return dv.Value, dv.ID, true
			}
		}
		return v, 0, true
	}
	// 3) user-configured extra defaults (attr_id -> value).
	if v := merged.AttributeDefaults[strconv.FormatInt(a.ID, 10)]; v != "" {
		if a.DictionaryID != 0 {
			if dv, err := client.searchAttributeValue(ctx, categoryID, typeID, a.ID, v); err == nil && dv != nil {
				return dv.Value, dv.ID, true
			}
		}
		return v, 0, true
	}
	return "", 0, false
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

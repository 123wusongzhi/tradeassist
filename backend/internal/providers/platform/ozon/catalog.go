package ozon

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const (
	pathCategoryTree     = "/v1/description-category/tree"
	pathDictionaryValues = "/v1/description-category/attribute/values"
)

// Client is the exported, credential-scoped Ozon Seller API client used by
// TradeMind services (shop category sync, product mapping). Publish flows use
// the internal client through PublishProduct; this wrapper exposes read-only
// catalog helpers only.
type Client struct {
	inner *ozonClient
}

// NewClient builds a catalog client from a decrypted shop auth request.
// AppKey is the Ozon Client-ID, AccessToken is the Api-Key.
func NewClient(req platformp.TestConnectionRequest) (*Client, error) {
	cfg, err := ResolveRuntime(req)
	if err != nil {
		return nil, err
	}
	return &Client{inner: newClient(cfg)}, nil
}

// CatalogNode is one flattened row of the Ozon description category tree.
// Leaf nodes carry TypeID; non-leaf nodes carry DescriptionCategoryID.
type CatalogNode struct {
	DescriptionCategoryID string `json:"descriptionCategoryId,omitempty"`
	TypeID                string `json:"typeId,omitempty"`
	Name                  string `json:"name"`
	ParentID              string `json:"parentId,omitempty"`
	Level                 int    `json:"level"`
	IsLeaf                bool   `json:"isLeaf"`
	Disabled              bool   `json:"disabled"`
}

type treeResponse struct {
	Result []treeCategory `json:"result"`
}

type treeCategory struct {
	DescriptionCategoryID int64          `json:"description_category_id"`
	CategoryName          string         `json:"category_name"`
	TypeID                int64          `json:"type_id"`
	TypeName              string         `json:"type_name"`
	Disabled              bool           `json:"disabled"`
	Children              []treeCategory `json:"children"`
}

const (
	ozonCategoryTreeMaxDepth = 16
	ozonCategoryTreeMaxNodes = 20000
)

// FetchCategoryTree downloads and flattens Ozon's recursive category tree.
// Intermediate nodes have description_category_id/category_name; only terminal
// nodes have type_id/type_name. Limits protect the sync worker from malformed
// or unexpectedly huge payloads; hitting either limit returns an error so the
// caller never treats a partial tree as authoritative.
func (c *Client) FetchCategoryTree(ctx context.Context) ([]CatalogNode, error) {
	if c == nil || c.inner == nil {
		return nil, fmt.Errorf("ozon client not configured")
	}
	var resp treeResponse
	if err := c.inner.postJSON(ctx, pathCategoryTree, map[string]any{"language": "DEFAULT"}, &resp); err != nil {
		return nil, err
	}
	out := make([]CatalogNode, 0, 4096)
	var flatten func(nodes []treeCategory, inheritedDescriptionID, parentID string, level int, inheritedDisabled bool, path map[string]struct{}) error
	flatten = func(nodes []treeCategory, inheritedDescriptionID, parentID string, level int, inheritedDisabled bool, path map[string]struct{}) error {
		if level > ozonCategoryTreeMaxDepth {
			return fmt.Errorf("ozon category tree exceeded maximum depth %d", ozonCategoryTreeMaxDepth)
		}
		for _, node := range nodes {
			if len(out) >= ozonCategoryTreeMaxNodes {
				return fmt.Errorf("ozon category tree exceeded maximum nodes %d", ozonCategoryTreeMaxNodes)
			}
			descriptionID := inheritedDescriptionID
			if node.DescriptionCategoryID > 0 {
				descriptionID = strconv.FormatInt(node.DescriptionCategoryID, 10)
			}
			disabled := inheritedDisabled || node.Disabled
			if node.TypeID > 0 {
				if descriptionID == "" {
					continue // malformed terminal node cannot be used for attribute requests
				}
				typeID := strconv.FormatInt(node.TypeID, 10)
				key := "type:" + descriptionID + ":" + typeID
				if _, seen := path[key]; seen {
					return fmt.Errorf("ozon category tree contains a cycle at %s", key)
				}
				out = append(out, CatalogNode{DescriptionCategoryID: descriptionID, TypeID: typeID, Name: node.TypeName, ParentID: parentID, Level: level, IsLeaf: true, Disabled: disabled})
				continue
			}
			if descriptionID == "" {
				continue
			}
			key := "category:" + descriptionID
			if _, seen := path[key]; seen {
				return fmt.Errorf("ozon category tree contains a cycle at %s", key)
			}
			out = append(out, CatalogNode{DescriptionCategoryID: descriptionID, Name: node.CategoryName, ParentID: parentID, Level: level, Disabled: disabled})
			nextPath := make(map[string]struct{}, len(path)+1)
			for k := range path {
				nextPath[k] = struct{}{}
			}
			nextPath[key] = struct{}{}
			if err := flatten(node.Children, descriptionID, descriptionID, level+1, disabled, nextPath); err != nil {
				return err
			}
		}
		return nil
	}
	if err := flatten(resp.Result, "", "", 1, false, map[string]struct{}{}); err != nil {
		return nil, err
	}
	return out, nil
}

// CategoryAttr mirrors one Ozon category attribute (normalized, export-safe).
type CategoryAttr struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	ValueType           string `json:"valueType,omitempty"`
	DictionaryID        string `json:"dictionaryId,omitempty"`
	Required            bool   `json:"required"`
	IsCollection        bool   `json:"isCollection,omitempty"`
	AttributeComplexID  int64  `json:"attributeComplexId,omitempty"`
	MaxValueCount       int64  `json:"maxValueCount,omitempty"`
	ComplexIsCollection bool   `json:"complexIsCollection,omitempty"`
	CategoryDependent   bool   `json:"categoryDependent,omitempty"`
}

// CategorySchemaHash is the canonical template fingerprint shared by the
// provider and persisted product configuration. It excludes cache timestamps,
// database IDs and prefetched dictionary options.
func CategorySchemaHash(attrs []CategoryAttr) string {
	type field struct {
		ID, Name, ValueType, DictionaryID                              string
		Required, IsCollection, ComplexIsCollection, CategoryDependent bool
		AttributeComplexID, MaxValueCount                              int64
	}
	rows := make([]field, 0, len(attrs))
	for _, attr := range attrs {
		rows = append(rows, field{
			ID: attr.ID, Name: attr.Name, ValueType: attr.ValueType, DictionaryID: attr.DictionaryID,
			Required: attr.Required, IsCollection: attr.IsCollection, ComplexIsCollection: attr.ComplexIsCollection,
			CategoryDependent: attr.CategoryDependent, AttributeComplexID: attr.AttributeComplexID, MaxValueCount: attr.MaxValueCount,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	raw, _ := json.Marshal(rows)
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum[:])
}

// FetchCategoryAttributes returns attributes for one leaf (type) under a
// description category.
func (c *Client) FetchCategoryAttributes(ctx context.Context, descriptionCategoryID, typeID string) ([]CategoryAttr, error) {
	if c == nil || c.inner == nil {
		return nil, fmt.Errorf("ozon client not configured")
	}
	catID, err := strconv.ParseInt(descriptionCategoryID, 10, 64)
	if err != nil || catID <= 0 {
		return nil, fmt.Errorf("ozon: invalid description_category_id %q", descriptionCategoryID)
	}
	tid, err := strconv.ParseInt(typeID, 10, 64)
	if err != nil || tid <= 0 {
		return nil, fmt.Errorf("ozon: invalid type_id %q", typeID)
	}
	attrs, err := c.inner.getCategoryAttributes(ctx, catID, tid)
	if err != nil {
		return nil, err
	}
	out := categoryAttrsForHash(attrs)
	return out, nil
}

func categoryAttrsForHash(attrs []ozonAttribute) []CategoryAttr {
	out := make([]CategoryAttr, 0, len(attrs))
	for _, attr := range attrs {
		dictionaryID := ""
		if attr.DictionaryID > 0 {
			dictionaryID = strconv.FormatInt(attr.DictionaryID, 10)
		}
		out = append(out, CategoryAttr{
			ID: strconv.FormatInt(attr.ID, 10), Name: attr.Name, ValueType: attr.Type, DictionaryID: dictionaryID,
			Required: attr.IsRequired, IsCollection: attr.IsCollection, AttributeComplexID: attr.AttributeComplexID,
			MaxValueCount: attr.MaxValueCount, ComplexIsCollection: attr.ComplexIsCollection, CategoryDependent: attr.CategoryDependent,
		})
	}
	return out
}

// DictionaryValue is one allowed value of a dictionary attribute.
type DictionaryValue struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// FetchDictionaryValues returns dictionary values for an attribute (paginated
// with last_value_id cursor until all pages are read).
func (c *Client) FetchDictionaryValues(ctx context.Context, descriptionCategoryID, typeID, attrID string) ([]DictionaryValue, error) {
	return c.fetchDictionaryValues(ctx, descriptionCategoryID, typeID, attrID, 0)
}

// FetchDictionaryValuesLimited returns the first bounded set of values for UI
// warm-up. Call SearchDictionaryValues for values outside that cache.
func (c *Client) FetchDictionaryValuesLimited(ctx context.Context, descriptionCategoryID, typeID, attrID string, limit int) ([]DictionaryValue, error) {
	if limit < 1 || limit > 1000 {
		limit = 200
	}
	return c.fetchDictionaryValues(ctx, descriptionCategoryID, typeID, attrID, limit)
}

func (c *Client) fetchDictionaryValues(ctx context.Context, descriptionCategoryID, typeID, attrID string, maxRows int) ([]DictionaryValue, error) {
	if c == nil || c.inner == nil {
		return nil, fmt.Errorf("ozon client not configured")
	}
	catID, err := strconv.ParseInt(descriptionCategoryID, 10, 64)
	if err != nil || catID <= 0 {
		return nil, fmt.Errorf("ozon: invalid description_category_id %q", descriptionCategoryID)
	}
	tid, err := strconv.ParseInt(typeID, 10, 64)
	if err != nil || tid <= 0 {
		return nil, fmt.Errorf("ozon: invalid type_id %q", typeID)
	}
	aid, err := strconv.ParseInt(attrID, 10, 64)
	if err != nil || aid <= 0 {
		return nil, fmt.Errorf("ozon: invalid attribute_id %q", attrID)
	}
	var out []DictionaryValue
	lastID := int64(0)
	for page := 0; page < 1000; page++ {
		previousLastID := lastID
		var resp struct {
			Result []dictionaryValue `json:"result"`
		}
		body := map[string]any{
			"description_category_id": catID,
			"type_id":                 tid,
			"attribute_id":            aid,
			"language":                "DEFAULT",
			"limit":                   100,
			"last_value_id":           lastID,
		}
		if err := c.inner.postJSON(ctx, pathDictionaryValues, body, &resp); err != nil {
			return nil, err
		}
		if len(resp.Result) == 0 {
			return out, nil
		}
		for _, dv := range resp.Result {
			if dv.ID <= 0 {
				continue
			}
			out = append(out, DictionaryValue{ID: strconv.FormatInt(dv.ID, 10), Value: dv.Value})
			if maxRows > 0 && len(out) >= maxRows {
				return out, nil
			}
			if dv.ID > lastID {
				lastID = dv.ID
			}
		}
		if len(resp.Result) < 100 {
			return out, nil
		}
		if lastID <= previousLastID {
			return nil, fmt.Errorf("ozon dictionary pagination did not advance")
		}
	}
	return nil, fmt.Errorf("ozon dictionary pagination exceeded safety limit")
}

// SearchDictionaryValues returns user-reviewable Ozon candidates. It never
// auto-selects a fuzzy result; the caller must explicitly choose one.
func (c *Client) SearchDictionaryValues(ctx context.Context, descriptionCategoryID, typeID, attrID, query string) ([]DictionaryValue, error) {
	if c == nil || c.inner == nil {
		return nil, fmt.Errorf("ozon client not configured")
	}
	catID, err := positiveOzonCatalogID("description_category_id", descriptionCategoryID)
	if err != nil {
		return nil, err
	}
	tid, err := positiveOzonCatalogID("type_id", typeID)
	if err != nil {
		return nil, err
	}
	aid, err := positiveOzonCatalogID("attribute_id", attrID)
	if err != nil {
		return nil, err
	}
	if len([]rune(strings.TrimSpace(query))) < 2 {
		return nil, fmt.Errorf("ozon dictionary search requires at least two characters")
	}
	rows, err := c.inner.searchAttributeValues(ctx, catID, tid, aid, query)
	if err != nil {
		return nil, err
	}
	out := make([]DictionaryValue, 0, len(rows))
	for _, row := range rows {
		if row.ID > 0 {
			out = append(out, DictionaryValue{ID: strconv.FormatInt(row.ID, 10), Value: row.Value})
		}
	}
	return out, nil
}

// ValidateDictionaryValue performs a read-only, authoritative membership check
// against Ozon for one selected dictionary value. It verifies both the value ID
// and its display text for the exact category/type/attribute tuple.
func (c *Client) ValidateDictionaryValue(ctx context.Context, descriptionCategoryID, typeID, attrID, valueID, value string) (bool, error) {
	if c == nil || c.inner == nil {
		return false, fmt.Errorf("ozon client not configured")
	}
	catID, err := positiveOzonCatalogID("description_category_id", descriptionCategoryID)
	if err != nil {
		return false, err
	}
	tid, err := positiveOzonCatalogID("type_id", typeID)
	if err != nil {
		return false, err
	}
	aid, err := positiveOzonCatalogID("attribute_id", attrID)
	if err != nil {
		return false, err
	}
	vid, err := positiveOzonCatalogID("dictionary_value_id", valueID)
	if err != nil {
		return false, err
	}
	matched, err := c.inner.validateDictionaryValue(ctx, catID, tid, aid, vid, value)
	return matched != nil, err
}

func positiveOzonCatalogID(field, raw string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("ozon: invalid %s %q", field, raw)
	}
	return value, nil
}

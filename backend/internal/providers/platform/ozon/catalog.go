package ozon

import (
	"context"
	"fmt"
	"strconv"

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
}

type treeResponse struct {
	Result []treeCategory `json:"result"`
}

type treeCategory struct {
	DescriptionCategoryID int64         `json:"description_category_id"`
	CategoryName          string        `json:"category_name"`
	Disabled              bool          `json:"disabled"`
	Children              []treeChild   `json:"children"`
}

type treeChild struct {
	TypeID   int64  `json:"type_id"`
	TypeName string `json:"type_name"`
	Disabled bool   `json:"disabled"`
	Children string `json:"children"`
}

// FetchCategoryTree downloads the full 3-level Ozon category tree and flattens
// it into CatalogNode rows (level 1..3; leaves have type_id set).
func (c *Client) FetchCategoryTree(ctx context.Context) ([]CatalogNode, error) {
	if c == nil || c.inner == nil {
		return nil, fmt.Errorf("ozon client not configured")
	}
	var resp treeResponse
	if err := c.inner.postJSON(ctx, pathCategoryTree, map[string]any{"language": "DEFAULT"}, &resp); err != nil {
		return nil, err
	}
	out := make([]CatalogNode, 0, 4096)
	for _, cat := range resp.Result {
		if cat.Disabled || cat.DescriptionCategoryID <= 0 {
			continue
		}
		catID := strconv.FormatInt(cat.DescriptionCategoryID, 10)
		out = append(out, CatalogNode{
			DescriptionCategoryID: catID,
			Name:                  cat.CategoryName,
			Level:                 1,
		})
		for _, child := range cat.Children {
			if child.Disabled || child.TypeID <= 0 {
				continue
			}
			out = append(out, CatalogNode{
				DescriptionCategoryID: catID,
				TypeID:                strconv.FormatInt(child.TypeID, 10),
				Name:                  child.TypeName,
				ParentID:              catID,
				Level:                 2,
				IsLeaf:                true,
			})
		}
	}
	return out, nil
}

// CategoryAttr mirrors one Ozon category attribute (normalized, export-safe).
type CategoryAttr struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ValueType    string `json:"valueType,omitempty"`
	DictionaryID string `json:"dictionaryId,omitempty"`
	Required     bool   `json:"required"`
	IsCollection bool   `json:"isCollection,omitempty"`
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
	out := make([]CategoryAttr, 0, len(attrs))
	for _, a := range attrs {
		out = append(out, CategoryAttr{
			ID:           strconv.FormatInt(a.ID, 10),
			Name:         a.Name,
			ValueType:    a.Type,
			DictionaryID: strconv.FormatInt(a.DictionaryID, 10),
			Required:     a.IsRequired,
			IsCollection: a.IsCollection,
		})
	}
	return out, nil
}

// DictionaryValue is one allowed value of a dictionary attribute.
type DictionaryValue struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// FetchDictionaryValues returns dictionary values for an attribute (paginated
// with last_value_id cursor until all pages are read).
func (c *Client) FetchDictionaryValues(ctx context.Context, descriptionCategoryID, typeID, attrID string) ([]DictionaryValue, error) {
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
	for {
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
			break
		}
		for _, dv := range resp.Result {
			if dv.ID <= 0 {
				continue
			}
			out = append(out, DictionaryValue{ID: strconv.FormatInt(dv.ID, 10), Value: dv.Value})
			if dv.ID > lastID {
				lastID = dv.ID
			}
		}
		if len(resp.Result) < 100 {
			break
		}
	}
	return out, nil
}

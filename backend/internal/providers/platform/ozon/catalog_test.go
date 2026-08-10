package ozon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func TestCategorySchemaHashIgnoresLocalizedNames(t *testing.T) {
	chinese := []CategoryAttr{{ID: "85", Name: "品牌", ValueType: "String", DictionaryID: "28732849", Required: true, SKUVariantEligibilityKnown: true}}
	russian := []CategoryAttr{{ID: "85", Name: "Бренд", ValueType: "String", DictionaryID: "28732849", Required: true, SKUVariantEligibilityKnown: true}}
	if CategorySchemaHash(chinese) != CategorySchemaHash(russian) {
		t.Fatal("localized display names must not change the category schema hash")
	}
	russian[0].Required = false
	if CategorySchemaHash(chinese) == CategorySchemaHash(russian) {
		t.Fatal("validation semantics must still change the category schema hash")
	}
	russian[0].Required = true
	russian[0].SKUVariantEligible = true
	if CategorySchemaHash(chinese) == CategorySchemaHash(russian) {
		t.Fatal("is_aspect variant eligibility must change the category schema hash")
	}
}

func TestFetchCategoryAttributesPreservesTwoCategoryVariantContracts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathCategoryAttributes {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch body["type_id"] {
		case float64(200):
			_, _ = w.Write([]byte(`{"result":[
				{"id":85,"name":"Brand","description":"Manufacturer brand","type":"String","is_aspect":false,"is_required":true},
				{"id":10096,"name":"Color","description":"Storage body color","type":"String","is_aspect":true,"is_required":true,"dictionary_id":7},
				{"id":20001,"name":"Capacity","type":"Integer","is_aspect":true,"is_required":true}
			]}`))
		case float64(300):
			_, _ = w.Write([]byte(`{"result":[
				{"id":85,"name":"Brand","type":"String","is_aspect":false,"is_required":true},
				{"id":30001,"name":"Frame color","type":"String","is_aspect":true,"is_required":true,"dictionary_id":9},
				{"id":30002,"name":"Polarized","type":"Boolean","is_aspect":false,"is_required":false},
				{"id":30003,"name":"Product page","type":"URL","is_aspect":false,"is_required":false}
			]}`))
		default:
			t.Fatalf("unexpected type_id: %#v", body["type_id"])
		}
	}))
	t.Cleanup(srv.Close)

	client, err := NewClient(platformp.TestConnectionRequest{AppKey: "client", AccessToken: "key", Extra: map[string]string{"api_base_url": srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := client.FetchCategoryAttributes(context.Background(), "100", "200")
	if err != nil {
		t.Fatal(err)
	}
	glasses, err := client.FetchCategoryAttributes(context.Background(), "100", "300")
	if err != nil {
		t.Fatal(err)
	}
	storageEligible := 0
	for _, attr := range storage {
		if attr.SKUVariantEligibilityKnown && attr.SKUVariantEligible {
			storageEligible++
		}
		if attr.ID == "85" && (attr.SKUVariantEligible || !attr.SKUVariantEligibilityKnown || attr.Description == "") {
			t.Fatalf("brand contract lost: %#v", attr)
		}
	}
	glassesEligible := 0
	seenBoolean, seenURL := false, false
	for _, attr := range glasses {
		if attr.SKUVariantEligibilityKnown && attr.SKUVariantEligible {
			glassesEligible++
		}
		seenBoolean = seenBoolean || attr.ValueType == "Boolean"
		seenURL = seenURL || attr.ValueType == "URL"
	}
	if storageEligible != 2 || glassesEligible != 1 || !seenBoolean || !seenURL {
		t.Fatalf("distinct category contracts were flattened: storage=%#v glasses=%#v", storage, glasses)
	}
}

func TestFetchCategoryTreeFlattensRecursiveOfficialShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathCategoryTree {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["language"] != ozonCatalogLanguage {
			t.Fatalf("language = %v, want %s", body["language"], ozonCatalogLanguage)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[{"description_category_id":100,"category_name":"Root","children":[{"description_category_id":110,"category_name":"Middle","disabled":true,"children":[{"type_id":200,"type_name":"Leaf","children":[]}]}]}]}`))
	}))
	t.Cleanup(srv.Close)

	client, err := NewClient(platformp.TestConnectionRequest{AppKey: "client", AccessToken: "key", Extra: map[string]string{"api_base_url": srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := client.FetchCategoryTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 3 {
		t.Fatalf("node count = %d, want 3: %#v", len(nodes), nodes)
	}
	if got := nodes[1]; got.DescriptionCategoryID != "110" || got.ParentID != "100" || got.Level != 2 || got.IsLeaf {
		t.Fatalf("middle node = %#v", got)
	}
	if got := nodes[2]; got.DescriptionCategoryID != "110" || got.TypeID != "200" || got.ParentID != "110" || got.Level != 3 || !got.IsLeaf || !got.Disabled {
		t.Fatalf("leaf node = %#v", got)
	}
}

func TestFetchWarehousesUsesCredentialScopedReadEndpoint(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != pathWarehouseList {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			if _, exists := body["cursor"]; exists {
				t.Fatalf("first request must not send cursor: %#v", body)
			}
			_, _ = w.Write([]byte(`{"warehouses":[{"warehouse_id":42,"name":"Main FBS","is_rfbs":false},{"warehouse_id":7,"name":"Backup rFBS","is_rfbs":true},{"warehouse_id":0,"name":"ignored"}],"has_next":true,"cursor":"next-page"}`))
			return
		}
		if body["cursor"] != "next-page" {
			t.Fatalf("cursor = %#v", body["cursor"])
		}
		_, _ = w.Write([]byte(`{"warehouses":[{"warehouse_id":42,"name":"duplicate"},{"warehouse_id":99,"name":"Overflow FBS"}],"has_next":false,"cursor":""}`))
	}))
	t.Cleanup(srv.Close)

	client, err := NewClient(platformp.TestConnectionRequest{AppKey: "client", AccessToken: "key", Extra: map[string]string{"api_base_url": srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := client.FetchWarehouses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].ID != "7" || !rows[0].IsRFBS || rows[1].ID != "42" || rows[2].ID != "99" {
		t.Fatalf("warehouses = %#v", rows)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

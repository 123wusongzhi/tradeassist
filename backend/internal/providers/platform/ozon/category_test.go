package ozon

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func TestBuildCategoryAttributes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":[
			{"id":85,"name":"Бренд","type":"string","dictionary_id":28732849,"is_required":true},
			{"id":23487,"name":"Производитель","type":"string","dictionary_id":0,"is_required":true},
			{"id":9048,"name":"Название модели","type":"string","dictionary_id":0,"is_required":false}
		]}`))
	})
	mux.HandleFunc(pathAttributeSearch, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["attribute_id"] == float64(85) {
			_, _ = w.Write([]byte(`{"result":[{"id":5055881,"value":"Sunshine"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	client := newTestClient(t, mux)

	merged := ozonPublishMerged{DefaultBrand: "", DefaultManufacturer: ""}
	attrs, missing, missingDefs, err := client.buildCategoryAttributes(
		context.Background(),
		200001240,
		93488,
		map[string]string{
			normalizeText("品牌"):            "Sunshine",
			normalizeText("Производитель"): "X Ltd",
		},
		merged,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("expected no missing required attributes, got %v", missing)
	}
	if len(missingDefs) != 0 {
		t.Fatalf("expected no missing required attribute definitions, got %v", missingDefs)
	}
	if len(attrs) != 2 {
		t.Fatalf("expected 2 mapped attributes, got %d: %+v", len(attrs), attrs)
	}
	byID := map[int64]ozonAttributeValue{}
	for _, a := range attrs {
		byID[a.ID] = a
	}
	brand := byID[85]
	if len(brand.Values) != 1 || brand.Values[0].DictionaryValueID != 5055881 || brand.Values[0].Value != "Sunshine" {
		t.Fatalf("unexpected brand mapping: %+v", brand)
	}
	manu := byID[23487]
	if len(manu.Values) != 1 || manu.Values[0].Value != "X Ltd" || manu.Values[0].DictionaryValueID != 0 {
		t.Fatalf("unexpected manufacturer mapping: %+v", manu)
	}
}

func TestBuildCategoryAttributesMissingRequired(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":[
			{"id":8205,"name":"Срок годности","type":"integer","dictionary_id":0,"is_required":true}
		]}`))
	})
	client := newTestClient(t, mux)
	_, missing, missingDefs, err := client.buildCategoryAttributes(context.Background(), 1, 2, map[string]string{}, ozonPublishMerged{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(missing) != 1 || missing[0] != "Срок годности" {
		t.Fatalf("expected missing required attribute, got %v", missing)
	}
	if len(missingDefs) != 1 || missingDefs[0].Name != "Срок годности" {
		t.Fatalf("expected missing required attribute definition, got %v", missingDefs)
	}
}

func TestLocalAttributeMap(t *testing.T) {
	draft := platformp.PlatformProductDraft{
		Attributes: map[string]any{
			"attributes": []any{
				map[string]any{"name": "品牌", "value": "Apple"},
				map[string]any{"name": "color", "value": "Black"},
			},
		},
	}
	m := localAttributeMap(draft)
	if m[normalizeText("品牌")] != "Apple" || m["color"] != "Black" {
		t.Fatalf("unexpected local attribute map: %+v", m)
	}
}

func TestNormalizeText(t *testing.T) {
	if normalizeText("  Страна-изготовитель  ") != "странаизготовитель" {
		t.Fatalf("unexpected normalization: %q", normalizeText("  Страна-изготовитель  "))
	}
	if normalizeText("Country of Origin") != "countryoforigin" {
		t.Fatalf("unexpected normalization: %q", normalizeText("Country of Origin"))
	}
}

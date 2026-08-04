package ozon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func TestPublishProductRejectsDictionaryValueFromWrongAttribute(t *testing.T) {
	importCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":85,"name":"Brand","type":"string","dictionary_id":10,"is_required":true}]}`))
	})
	mux.HandleFunc(pathAttributeSearch, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":999,"value":"Sunshine"}]}`))
	})
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, _ *http.Request) {
		importCalls++
		_, _ = w.Write([]byte(`{"result":{"task_id":1}}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	_, err := ozonProvider{}.PublishProduct(context.Background(), explicitAttributePublishRequest(server.URL, map[string]any{
		"85": map[string]any{"value": "Sunshine", "dictionaryValueId": "123"},
	}))
	if err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("expected dictionary ownership error, got %v", err)
	}
	if importCalls != 0 {
		t.Fatalf("product import must not be called after dictionary validation failure; calls=%d", importCalls)
	}
}

func TestPublishProductUsesValidatedExplicitAttributeAndComplexID(t *testing.T) {
	var importBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":85,"name":"Brand","type":"string","dictionary_id":10,"is_required":true,"attribute_complex_id":7}]}`))
	})
	mux.HandleFunc(pathAttributeSearch, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":123,"value":"Sunshine"}]}`))
	})
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&importBody)
		_, _ = w.Write([]byte(`{"result":{"task_id":1}}`))
	})
	mux.HandleFunc(pathImportInfo, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"items":[{"offer_id":"SKU-1","product_id":101,"status":"imported","errors":[]}]}}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	_, err := ozonProvider{}.PublishProduct(context.Background(), explicitAttributePublishRequest(server.URL, map[string]any{
		"85": map[string]any{"value": "Sunshine", "dictionaryValueId": "123"},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	items := importBody["items"].([]any)
	complexAttributes := items[0].(map[string]any)["complex_attributes"].([]any)
	if len(complexAttributes) != 1 {
		t.Fatalf("expected one complex attribute group, got %#v", complexAttributes)
	}
	attributes := complexAttributes[0].(map[string]any)["attributes"].([]any)
	attribute := attributes[0].(map[string]any)
	if attribute["complex_id"] != float64(7) {
		t.Fatalf("expected live attribute_complex_id 7, got %v", attribute["complex_id"])
	}
	values := attribute["values"].([]any)
	if values[0].(map[string]any)["dictionary_value_id"] != float64(123) {
		t.Fatalf("expected validated dictionary value ID 123, got %v", values[0])
	}
}

func TestPartitionOzonImportAttributesSeparatesAndGroupsComplexAttributes(t *testing.T) {
	ordinary, complex := partitionOzonImportAttributes([]ozonAttributeValue{
		{ID: 10, Values: []ozonAttrValue{{Value: "ordinary"}}},
		{ComplexID: 7, ID: 11, Values: []ozonAttrValue{{Value: "first"}}},
		{ComplexID: 9, ID: 12, Values: []ozonAttrValue{{Value: "other"}}},
		{ComplexID: 7, ID: 13, Values: []ozonAttrValue{{Value: "second"}}},
	})
	if len(ordinary) != 1 || ordinary[0].ID != 10 {
		t.Fatalf("ordinary attributes = %#v", ordinary)
	}
	if len(complex) != 2 || len(complex[0].Attributes) != 2 || len(complex[1].Attributes) != 1 {
		t.Fatalf("complex groups = %#v", complex)
	}
	if complex[0].Attributes[0].ComplexID != 7 || complex[0].Attributes[1].ID != 13 || complex[1].Attributes[0].ComplexID != 9 {
		t.Fatalf("complex grouping = %#v", complex)
	}
}

func TestPublishProductChecksRequiredAttributesWhenAutoFillDisabled(t *testing.T) {
	importCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":23487,"name":"Manufacturer","type":"string","is_required":true}]}`))
	})
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, _ *http.Request) {
		importCalls++
		_, _ = w.Write([]byte(`{"result":{"task_id":1}}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	_, err := ozonProvider{}.PublishProduct(context.Background(), explicitAttributePublishRequest(server.URL, nil))
	if err == nil || !strings.Contains(err.Error(), "missing required category attributes") {
		t.Fatalf("expected missing required attribute error, got %v", err)
	}
	if importCalls != 0 {
		t.Fatalf("product import must not be called when a required attribute is missing; calls=%d", importCalls)
	}
}

func TestPublishProductDoesNotRetryUncertainImportMutation(t *testing.T) {
	importCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, _ *http.Request) {
		importCalls++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"uncertain acceptance"}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	_, err := ozonProvider{}.PublishProduct(context.Background(), explicitAttributePublishRequest(server.URL, nil))
	if err == nil {
		t.Fatal("expected uncertain import failure")
	}
	if importCalls != 1 {
		t.Fatalf("mutation calls = %d, want exactly one", importCalls)
	}
}

func TestPublishProductBlocksSchemaDriftBeforeImport(t *testing.T) {
	importCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":85,"name":"Brand","type":"string","is_required":true}]}`))
	})
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, _ *http.Request) {
		importCalls++
		_, _ = w.Write([]byte(`{"result":{"task_id":1}}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	req := explicitAttributePublishRequest(server.URL, map[string]any{"85": map[string]any{"value": "Acme"}})
	req.Options["ozon_schema_hash"] = "stale-schema"

	_, err := ozonProvider{}.PublishProduct(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "schema changed") {
		t.Fatalf("expected schema drift block, got %v", err)
	}
	if importCalls != 0 {
		t.Fatalf("product import must not be called after schema drift; calls=%d", importCalls)
	}
}

func TestPublishProductPreservesV2MultiValuesAndRepeatedComplexGroups(t *testing.T) {
	var importBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[
			{"id":10,"name":"Colors","type":"string","is_required":true,"is_collection":true,"max_value_count":3},
			{"id":20,"name":"Volume","type":"string","is_required":true,"attribute_complex_id":7,"complex_is_collection":true},
			{"id":21,"name":"Unit","type":"string","is_required":true,"attribute_complex_id":7,"complex_is_collection":true}
		]}`))
	})
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&importBody)
		_, _ = w.Write([]byte(`{"result":{"task_id":1}}`))
	})
	mux.HandleFunc(pathImportInfo, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"items":[{"offer_id":"SKU-1","product_id":101,"status":"imported","errors":[]}]}}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	req := explicitAttributePublishRequest(server.URL, nil)
	req.Options["platform_attributes"] = map[string]any{
		"version":    2,
		"attributes": map[string]any{"10": []any{map[string]any{"value": "Red"}, map[string]any{"value": "Blue"}}},
		"complexGroups": []any{
			map[string]any{"complexId": 7, "attributes": map[string]any{"20": []any{map[string]any{"value": "100"}}, "21": []any{map[string]any{"value": "ml"}}}},
			map[string]any{"complexId": 7, "attributes": map[string]any{"20": []any{map[string]any{"value": "200"}}, "21": []any{map[string]any{"value": "ml"}}}},
		},
	}
	if _, err := (ozonProvider{}).PublishProduct(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	items := importBody["items"].([]any)
	item := items[0].(map[string]any)
	ordinary := item["attributes"].([]any)
	if len(ordinary) != 1 || len(ordinary[0].(map[string]any)["values"].([]any)) != 2 {
		t.Fatalf("ordinary multi values were flattened: %#v", ordinary)
	}
	complex := item["complex_attributes"].([]any)
	if len(complex) != 2 {
		t.Fatalf("repeated complex groups were merged: %#v", complex)
	}
}

func TestPublishProductBlocksRepeatedNonCollectionComplexGroupBeforeImport(t *testing.T) {
	importCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":20,"name":"Volume","type":"string","is_required":true,"attribute_complex_id":7,"complex_is_collection":false}]}`))
	})
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, _ *http.Request) {
		importCalls++
		_, _ = w.Write([]byte(`{"result":{"task_id":1}}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	req := explicitAttributePublishRequest(server.URL, nil)
	req.Options["platform_attributes"] = map[string]any{
		"version":    2,
		"attributes": map[string]any{},
		"complexGroups": []any{
			map[string]any{"complexId": 7, "attributes": map[string]any{"20": []any{map[string]any{"value": "100"}}}},
			map[string]any{"complexId": 7, "attributes": map[string]any{"20": []any{map[string]any{"value": "200"}}}},
		},
	}
	_, err := (ozonProvider{}).PublishProduct(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "does not allow repeated groups") {
		t.Fatalf("expected repeated-group block, got %v", err)
	}
	if importCalls != 0 {
		t.Fatalf("product/import must not be called, calls=%d", importCalls)
	}
}

func TestPublishProductRejectsEmptyValueInsideMultiSelection(t *testing.T) {
	importCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":10,"name":"Colors","type":"string","is_required":true,"is_collection":true,"max_value_count":3}]}`))
	})
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, _ *http.Request) {
		importCalls++
		_, _ = w.Write([]byte(`{"result":{"task_id":1}}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	req := explicitAttributePublishRequest(server.URL, nil)
	req.Options["platform_attributes"] = map[string]any{
		"version": 2,
		"attributes": map[string]any{
			"10": []any{map[string]any{"value": "Red"}, map[string]any{"value": " "}},
		},
	}
	_, err := (ozonProvider{}).PublishProduct(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "empty selected value") {
		t.Fatalf("expected explicit empty-value block, got %v", err)
	}
	if importCalls != 0 {
		t.Fatalf("product/import must not be called, calls=%d", importCalls)
	}
}

func TestParseExplicitOzonAttributesRejectsNonIntegerVersion(t *testing.T) {
	_, err := parseExplicitOzonAttributes(map[string]any{
		"platform_attributes": map[string]any{"version": "2", "attributes": map[string]any{}},
	})
	if err == nil || !strings.Contains(err.Error(), "version must be an integer") {
		t.Fatalf("expected invalid version error, got %v", err)
	}
}

func explicitAttributePublishRequest(baseURL string, attributes map[string]any) platformp.PublishProductRequest {
	options := map[string]any{}
	if attributes != nil {
		options["platform_attributes"] = attributes
	}
	return platformp.PublishProductRequest{
		ShopID: uuid.New(),
		Auth: platformp.TestConnectionRequest{
			AppKey:      "client-id",
			AccessToken: "api-key",
			Extra:       map[string]string{"api_base_url": baseURL},
		},
		PublishConfig: map[string]any{
			"description_category_id": "100",
			"type_id":                 "200",
			"default_weight":          "100",
			"default_width":           "100",
			"default_height":          "100",
			"default_depth":           "100",
			"currency_code":           "RUB",
			"auto_fill_attributes":    "false",
		},
		Options: options,
		Product: platformp.PlatformProductDraft{
			ProductID: uuid.New(),
			Title:     "Test product",
			Images:    []platformp.PlatformProductImage{{URL: "https://example.com/main.jpg", Type: "main"}},
			SKUs:      []platformp.PlatformProductSKU{{LocalSKUID: uuid.New(), SKUCode: "SKU-1", Price: 10, Stock: 1, ImageURL: "https://example.com/main.jpg"}},
		},
	}
}

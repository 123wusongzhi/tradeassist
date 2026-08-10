package ozon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func TestPublishProductHappyPath(t *testing.T) {
	var gotImportBody map[string]any
	var gotStockBody struct {
		Stocks []struct {
			OfferID     string `json:"offer_id"`
			ProductID   int64  `json:"product_id"`
			Stock       int64  `json:"stock"`
			WarehouseID int64  `json:"warehouse_id"`
		} `json:"stocks"`
	}
	var importCalls, stockCalls int
	redSKUID := uuid.New()
	blueSKUID := uuid.New()

	mux := http.NewServeMux()
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":[
			{"id":10096,"name":"Color","type":"String","dictionary_id":0,"is_required":true,"is_aspect":true,"is_collection":false,"attribute_complex_id":0,"max_value_count":1,"complex_is_collection":false},
			{"id":85,"name":"Brand","type":"String","dictionary_id":0,"is_required":false,"is_aspect":false,"is_collection":false,"attribute_complex_id":0,"max_value_count":1,"complex_is_collection":false}
		]}`))
	})
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, r *http.Request) {
		importCalls++
		_ = json.NewDecoder(r.Body).Decode(&gotImportBody)
		_, _ = w.Write([]byte(`{"result":{"task_id":172549793}}`))
	})
	mux.HandleFunc(pathImportInfo, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"items":[
			{"offer_id":"SKU-001","product_id":137285792,"status":"imported","errors":[]},
			{"offer_id":"SKU-002","product_id":137285793,"status":"imported","errors":[]}
		],"total":2}}`))
	})
	mux.HandleFunc(pathStocks, func(w http.ResponseWriter, r *http.Request) {
		stockCalls++
		_ = json.NewDecoder(r.Body).Decode(&gotStockBody)
		_, _ = w.Write([]byte(`{"result":[
			{"offer_id":"SKU-001","product_id":137285792,"updated":true,"errors":[]},
			{"offer_id":"SKU-002","product_id":137285793,"updated":true,"errors":[]}
		]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prov := ozonProvider{}
	res, err := prov.PublishProduct(context.Background(), platformp.PublishProductRequest{
		ShopID:   uuid.New(),
		Platform: "ozon",
		Auth: platformp.TestConnectionRequest{
			AppKey:      "123456",
			AccessToken: "secret",
			Extra:       map[string]string{"api_base_url": srv.URL},
		},
		PublishConfig: map[string]any{
			"description_category_id": "200001240",
			"type_id":                 "93488",
			"default_weight":          "100",
			"default_width":           "100",
			"default_height":          "100",
			"default_depth":           "100",
			"warehouse_id":            "22142605386000",
			"currency_code":           "RUB",
			"vat":                     "0.1",
			"auto_fill_attributes":    "true",
		},
		Product: platformp.PlatformProductDraft{
			ProductID:   uuid.New(),
			Title:       "Test Product",
			Description: "Test description",
			Currency:    "CNY",
			Attributes:  map[string]any{"Brand": "LocalBrand"},
			Images: []platformp.PlatformProductImage{
				{URL: "https://example.com/main.jpg", Type: "main"},
				{URL: "https://example.com/detail.jpg", Type: "detail"},
			},
			SKUs: []platformp.PlatformProductSKU{
				{
					LocalSKUID: redSKUID, SKUCode: "SKU-001", SKUName: "Red", Price: 129.9, Stock: 10,
					ImageURL: "https://example.com/red.jpg",
					PlatformAttributes: map[string]any{
						"version": 3, "attributes": map[string]any{"10096": []map[string]any{{"value": "Red"}}}, "complexGroups": []any{}, "skuVariantAttributeIds": []string{"10096"},
					},
					Images: []platformp.PlatformProductImage{
						{URL: "https://example.com/red.jpg", Type: "main"},
						{URL: "https://example.com/shared.jpg", Type: "detail"},
						{URL: "https://example.com/shared.jpg", Type: "detail"},
					},
				},
				{
					LocalSKUID: blueSKUID, SKUCode: "SKU-002", SKUName: "Blue", Price: 99, Stock: 5,
					ImageURL: "https://example.com/blue.jpg",
					PlatformAttributes: map[string]any{
						"version": 3, "attributes": map[string]any{"10096": []map[string]any{{"value": "Blue"}}}, "complexGroups": []any{}, "skuVariantAttributeIds": []string{"10096"},
					},
					Images: []platformp.PlatformProductImage{
						{URL: "https://example.com/blue.jpg", Type: "main"},
						{URL: "https://example.com/blue-detail.jpg", Type: "detail"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if importCalls != 1 || stockCalls != 1 {
		t.Fatalf("expected 1 import + 1 stock call, got %d/%d", importCalls, stockCalls)
	}
	if res.ExternalProductID != "137285792" {
		t.Fatalf("unexpected external product id: %s", res.ExternalProductID)
	}
	if res.Status != platformp.PublishStatusImported {
		t.Fatalf("unexpected status: %s", res.Status)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %+v", res.Warnings)
	}
	if res.RawSummary["platformStatus"] != importStatusImported || res.RawSummary["verificationStatus"] != platformp.PublishStatusImported {
		t.Fatalf("unexpected verification summary: %+v", res.RawSummary)
	}
	if res.RawSummary["sellableVerified"] != false || res.RawSummary["needsAction"] != false {
		t.Fatalf("imported result must not claim sellability: %+v", res.RawSummary)
	}
	if len(res.SKUMappings) != 2 {
		t.Fatalf("expected 2 sku mappings, got %d", len(res.SKUMappings))
	}
	if res.SKUMappings[0].ExternalSKUID != "137285792" || res.SKUMappings[1].ExternalSKUID != "137285793" {
		t.Fatalf("unexpected mappings: %+v", res.SKUMappings)
	}

	items := gotImportBody["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 import items, got %d", len(items))
	}
	first := items[0].(map[string]any)
	if first["offer_id"] != "SKU-001" || first["name"] != "Test Product / Red" {
		t.Fatalf("unexpected first item: %+v", first)
	}
	if first["price"] != "129.9" || first["currency_code"] != "RUB" {
		t.Fatalf("unexpected price/currency: %+v", first)
	}
	imgs := first["images"].([]any)
	if len(imgs) != 2 || imgs[0] != "https://example.com/red.jpg" || imgs[1] != "https://example.com/shared.jpg" || first["primary_image"] != "https://example.com/red.jpg" {
		t.Fatalf("unexpected images: %+v", first)
	}
	second := items[1].(map[string]any)
	if second["name"] != "Test Product / Blue" {
		t.Fatalf("unexpected variant name: %+v", second)
	}
	secondImages := second["images"].([]any)
	if len(secondImages) != 2 || secondImages[0] != "https://example.com/blue.jpg" || secondImages[1] != "https://example.com/blue-detail.jpg" || second["primary_image"] != "https://example.com/blue.jpg" {
		t.Fatalf("unexpected second SKU images: %+v", second)
	}
	firstAttributes := first["attributes"].([]any)
	secondAttributes := second["attributes"].([]any)
	if len(firstAttributes) != 1 || len(secondAttributes) != 1 || firstAttributes[0].(map[string]any)["values"].([]any)[0].(map[string]any)["value"] != "Red" || secondAttributes[0].(map[string]any)["values"].([]any)[0].(map[string]any)["value"] != "Blue" {
		t.Fatalf("SKU variant attributes were not submitted independently: first=%+v second=%+v", firstAttributes, secondAttributes)
	}

	stocks := gotStockBody.Stocks
	if len(stocks) != 2 {
		t.Fatalf("expected 2 stock rows, got %d", len(stocks))
	}
	stock0 := stocks[0]
	if stock0.WarehouseID != 22142605386000 || stock0.Stock != 10 {
		t.Fatalf("unexpected stock row: %+v", stock0)
	}
}

func TestPublishProductImportedWithErrorNeedsAction(t *testing.T) {
	mux := http.NewServeMux()
	handleEmptyCategoryAttributes(mux)
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"task_id":11}}`))
	})
	mux.HandleFunc(pathImportInfo, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"items":[{
			"offer_id":"SKU-STATUS","product_id":201,"status":"imported",
			"errors":[{"code":"invalid_dimensions","field":"depth","level":"error","message":"invalid package dimensions"}]
		}]}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res, err := (ozonProvider{}).PublishProduct(context.Background(), statusTestPublishRequest(srv.URL, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != platformp.PublishStatusNeedsAction {
		t.Fatalf("status = %q, want %q", res.Status, platformp.PublishStatusNeedsAction)
	}
	if res.ExternalProductID != "201" {
		t.Fatalf("external product id = %q", res.ExternalProductID)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("warnings = %+v", res.Warnings)
	}
	warning := res.Warnings[0]
	if warning.Stage != "import" || warning.Severity != "error" || warning.Code != "invalid_dimensions" || warning.Field != "depth" || warning.OfferID != "SKU-STATUS" || warning.Message != "invalid package dimensions" {
		t.Fatalf("unexpected warning: %+v", warning)
	}
	if res.RawSummary["platformStatus"] != importStatusImported || res.RawSummary["verificationStatus"] != platformp.PublishStatusNeedsAction || res.RawSummary["needsAction"] != true || res.RawSummary["sellableVerified"] != false {
		t.Fatalf("unexpected verification summary: %+v", res.RawSummary)
	}
}

func TestPublishProductMissingImportResultNeedsAction(t *testing.T) {
	mux := http.NewServeMux()
	handleEmptyCategoryAttributes(mux)
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"task_id":13}}`))
	})
	mux.HandleFunc(pathImportInfo, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"items":[{"offer_id":"UNEXPECTED-OFFER","product_id":203,"status":"imported","errors":[]}]}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res, err := (ozonProvider{}).PublishProduct(context.Background(), statusTestPublishRequest(srv.URL, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != platformp.PublishStatusNeedsAction {
		t.Fatalf("status = %q, want %q", res.Status, platformp.PublishStatusNeedsAction)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("warnings = %+v", res.Warnings)
	}
	warning := res.Warnings[0]
	if warning.Stage != "import" || warning.Code != "import_result_missing" || warning.OfferID != "SKU-STATUS" {
		t.Fatalf("unexpected warning: %+v", warning)
	}
}

func TestPublishProductStockFailureNeedsAction(t *testing.T) {
	var stockCalls int
	mux := http.NewServeMux()
	handleEmptyCategoryAttributes(mux)
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"task_id":12}}`))
	})
	mux.HandleFunc(pathImportInfo, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"items":[{"offer_id":"SKU-STATUS","product_id":202,"status":"imported","errors":[]}]}}`))
	})
	mux.HandleFunc(pathStocks, func(w http.ResponseWriter, _ *http.Request) {
		stockCalls++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"warehouse unavailable"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res, err := (ozonProvider{}).PublishProduct(context.Background(), statusTestPublishRequest(srv.URL, "123"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stockCalls != 1 {
		t.Fatalf("stock calls = %d, want 1", stockCalls)
	}
	if res.Status != platformp.PublishStatusNeedsAction {
		t.Fatalf("status = %q, want %q", res.Status, platformp.PublishStatusNeedsAction)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("warnings = %+v", res.Warnings)
	}
	warning := res.Warnings[0]
	if warning.Stage != "stock" || warning.Severity != "error" || warning.Code != "stock_sync_request_failed" {
		t.Fatalf("unexpected warning: %+v", warning)
	}
	if res.RawSummary["stockSyncStatus"] != "failed" || res.RawSummary["needsAction"] != true || res.RawSummary["sellableVerified"] != false {
		t.Fatalf("unexpected stock verification summary: %+v", res.RawSummary)
	}
}

func TestPublishProductExplicitZeroStockSkipsStockMutation(t *testing.T) {
	var stockCalls int
	mux := http.NewServeMux()
	handleEmptyCategoryAttributes(mux)
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"task_id":14}}`))
	})
	mux.HandleFunc(pathImportInfo, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"items":[{"offer_id":"SKU-STATUS","product_id":204,"status":"imported","errors":[]}]}}`))
	})
	mux.HandleFunc(pathStocks, func(w http.ResponseWriter, _ *http.Request) {
		stockCalls++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Product is not created"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	req := statusTestPublishRequest(srv.URL, "123")
	req.Product.SKUs[0].Stock = 0
	res, err := (ozonProvider{}).PublishProduct(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stockCalls != 0 {
		t.Fatalf("zero stock must not call Ozon stocks API, calls=%d", stockCalls)
	}
	if res.Status != platformp.PublishStatusImported || len(res.Warnings) != 0 {
		t.Fatalf("zero stock import must remain imported without warnings: status=%s warnings=%+v", res.Status, res.Warnings)
	}
	if res.RawSummary["stockSyncStatus"] != "zero_stock_not_required" || res.RawSummary["needsAction"] != false || res.RawSummary["sellableVerified"] != false {
		t.Fatalf("unexpected zero-stock summary: %+v", res.RawSummary)
	}
	for _, warning := range res.Warnings {
		if strings.Contains(strings.ToLower(warning.Message), "product is not created") {
			t.Fatalf("skipped stock mutation produced a synthetic platform warning: %+v", warning)
		}
	}
}

func TestAllImportedSKUsHaveExplicitZeroStock(t *testing.T) {
	draft := func(stocks ...int) platformp.PlatformProductDraft {
		skus := make([]platformp.PlatformProductSKU, 0, len(stocks))
		for i, stock := range stocks {
			skus = append(skus, platformp.PlatformProductSKU{SKUCode: fmt.Sprintf("SKU-%d", i+1), Stock: stock})
		}
		return platformp.PlatformProductDraft{SKUs: skus}
	}
	tests := []struct {
		name     string
		draft    platformp.PlatformProductDraft
		imported []importedRow
		want     bool
	}{
		{name: "one explicit zero", draft: draft(0), imported: []importedRow{{OfferID: "SKU-1", ProductID: 1}}, want: true},
		{name: "all imported are explicit zero", draft: draft(0, 0), imported: []importedRow{{OfferID: "SKU-1", ProductID: 1}, {OfferID: "SKU-2", ProductID: 2}}, want: true},
		{name: "positive stock is not zero", draft: draft(1), imported: []importedRow{{OfferID: "SKU-1", ProductID: 1}}},
		{name: "negative stock is not zero", draft: draft(-1), imported: []importedRow{{OfferID: "SKU-1", ProductID: 1}}},
		{name: "unknown imported offer is not zero", draft: draft(0), imported: []importedRow{{OfferID: "UNKNOWN", ProductID: 1}}},
		{name: "missing platform product id is not trusted", draft: draft(0), imported: []importedRow{{OfferID: "SKU-1"}}},
		{name: "empty imported result is not zero", draft: draft(0)},
		{name: "duplicate imported offer is not trusted", draft: draft(0), imported: []importedRow{{OfferID: "SKU-1", ProductID: 1}, {OfferID: "SKU-1", ProductID: 1}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allImportedSKUsHaveExplicitZeroStock(tt.draft, tt.imported); got != tt.want {
				t.Fatalf("allImportedSKUsHaveExplicitZeroStock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPublishProductBlocksLegacyMultiSKUBeforeOzonWrite(t *testing.T) {
	var importCalls int
	mux := http.NewServeMux()
	handleEmptyCategoryAttributes(mux)
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, r *http.Request) {
		importCalls++
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prov := ozonProvider{}
	_, err := prov.PublishProduct(context.Background(), platformp.PublishProductRequest{
		ShopID:   uuid.New(),
		Platform: "ozon",
		Auth: platformp.TestConnectionRequest{
			AppKey:      "1",
			AccessToken: "2",
			Extra:       map[string]string{"api_base_url": srv.URL},
		},
		PublishConfig: map[string]any{
			"description_category_id": "200001240",
			"type_id":                 "93488",
			"default_weight":          "100",
			"default_width":           "100",
			"default_height":          "100",
			"default_depth":           "100",
			"currency_code":           "RUB",
			"vat":                     "0",
			"auto_fill_attributes":    "false",
		},
		Product: platformp.PlatformProductDraft{
			ProductID: uuid.New(),
			Title:     "Legacy multi-SKU task",
			SKUs: []platformp.PlatformProductSKU{
				{LocalSKUID: uuid.New(), SKUCode: "OLD-1", Price: 10, ImageURL: "https://example.com/old-1.jpg"},
				{LocalSKUID: uuid.New(), SKUCode: "OLD-2", Price: 11, ImageURL: "https://example.com/old-2.jpg"},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "no per-SKU variant mapping") {
		t.Fatalf("expected legacy multi-SKU snapshot to be blocked, got %v", err)
	}
	if importCalls != 0 {
		t.Fatalf("expected no Ozon product import write, got %d", importCalls)
	}
}

func TestPublishProductBlocksNonAspectVariantBeforeOzonWrite(t *testing.T) {
	tests := []struct {
		name       string
		aspectJSON string
		want       string
	}{
		{name: "brand explicitly not aspect", aspectJSON: `,"is_aspect":false`, want: "not eligible"},
		{name: "missing aspect evidence", aspectJSON: ``, want: "no live is_aspect eligibility evidence"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var importCalls int
			mux := http.NewServeMux()
			mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(w, `{"result":[{"id":85,"name":"Brand","type":"String","is_required":true%s}]}`, tt.aspectJSON)
			})
			mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, _ *http.Request) {
				importCalls++
				_, _ = w.Write([]byte(`{"result":{"task_id":1}}`))
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			req := statusTestPublishRequest(srv.URL, "")
			req.Product.SKUs = []platformp.PlatformProductSKU{
				{LocalSKUID: uuid.New(), SKUCode: "BRAND-A", Price: 10, Stock: 1, ImageURL: "https://example.com/a.jpg", PlatformAttributes: map[string]any{"version": 3, "attributes": map[string]any{"85": []map[string]any{{"value": "A"}}}, "complexGroups": []any{}, "skuVariantAttributeIds": []string{"85"}}},
				{LocalSKUID: uuid.New(), SKUCode: "BRAND-B", Price: 11, Stock: 1, ImageURL: "https://example.com/b.jpg", PlatformAttributes: map[string]any{"version": 3, "attributes": map[string]any{"85": []map[string]any{{"value": "B"}}}, "complexGroups": []any{}, "skuVariantAttributeIds": []string{"85"}}},
			}
			_, err := (ozonProvider{}).PublishProduct(context.Background(), req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v want %q", err, tt.want)
			}
			if importCalls != 0 {
				t.Fatalf("product/import must not be called, calls=%d", importCalls)
			}
		})
	}
}

func TestPublishProductBlocksInvalidValueTypeBeforeOzonWrite(t *testing.T) {
	var importCalls int
	mux := http.NewServeMux()
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":20001,"name":"Capacity","type":"Integer","is_aspect":false,"is_required":true}]}`))
	})
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, _ *http.Request) {
		importCalls++
		_, _ = w.Write([]byte(`{"result":{"task_id":1}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	req := statusTestPublishRequest(srv.URL, "")
	req.Product.SKUs[0].PlatformAttributes = map[string]any{
		"version": 3, "attributes": map[string]any{"20001": []map[string]any{{"value": "twelve"}}}, "complexGroups": []any{}, "skuVariantAttributeIds": []string{},
	}
	_, err := (ozonProvider{}).PublishProduct(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "signed 64-bit integer") {
		t.Fatalf("error=%v", err)
	}
	if importCalls != 0 {
		t.Fatalf("product/import must not be called, calls=%d", importCalls)
	}
}

func TestPublishProductImportFailure(t *testing.T) {
	mux := http.NewServeMux()
	handleEmptyCategoryAttributes(mux)
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"task_id":1}}`))
	})
	mux.HandleFunc(pathImportInfo, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"items":[
			{"offer_id":"SKU-001","product_id":0,"status":"failed","errors":[{"code":"bad_value","field":"attributes","level":"error","message":"bad attribute value","description":"bad attribute value"}]}
		],"total":1}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prov := ozonProvider{}
	_, err := prov.PublishProduct(context.Background(), platformp.PublishProductRequest{
		ShopID:   uuid.New(),
		Platform: "ozon",
		Auth: platformp.TestConnectionRequest{
			AppKey:      "1",
			AccessToken: "2",
			Extra:       map[string]string{"api_base_url": srv.URL},
		},
		PublishConfig: map[string]any{
			"description_category_id": "200001240",
			"type_id":                 "93488",
			"default_weight":          "100",
			"default_width":           "100",
			"default_height":          "100",
			"default_depth":           "100",
			"currency_code":           "RUB",
			"vat":                     "0",
			"auto_fill_attributes":    "false",
		},
		Product: platformp.PlatformProductDraft{
			ProductID: uuid.New(),
			Title:     "T",
			Images:    []platformp.PlatformProductImage{{URL: "https://example.com/a.jpg", Type: "main"}},
			SKUs:      []platformp.PlatformProductSKU{{LocalSKUID: uuid.New(), SKUCode: "SKU-001", Price: 10, Stock: 1, ImageURL: "https://example.com/a.jpg"}},
		},
	})
	if err == nil {
		t.Fatal("expected import failure")
	}
	if !strings.Contains(err.Error(), "bad attribute value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPublishProductResolvesContractCurrency(t *testing.T) {
	var sellerCalls int
	mux := http.NewServeMux()
	handleEmptyCategoryAttributes(mux)
	mux.HandleFunc(pathSellerInfo, func(w http.ResponseWriter, r *http.Request) {
		sellerCalls++
		_, _ = w.Write([]byte(`{"company":{"name":"X","country":"CHN","currency":"CNY"}}`))
	})
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		items := body["items"].([]any)
		first := items[0].(map[string]any)
		if first["currency_code"] != "CNY" {
			t.Fatalf("expected CNY from seller contract, got %v", first["currency_code"])
		}
		if first["vat"] != "0" {
			t.Fatalf("expected default vat 0, got %v", first["vat"])
		}
		_, _ = w.Write([]byte(`{"result":{"task_id":5}}`))
	})
	mux.HandleFunc(pathImportInfo, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"items":[{"offer_id":"S1","product_id":101,"status":"imported","errors":[]}],"total":1}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prov := ozonProvider{}
	_, err := prov.PublishProduct(context.Background(), platformp.PublishProductRequest{
		ShopID:   uuid.New(),
		Platform: "ozon",
		Auth: platformp.TestConnectionRequest{
			AppKey:      "1",
			AccessToken: "2",
			Extra:       map[string]string{"api_base_url": srv.URL},
		},
		PublishConfig: map[string]any{
			"description_category_id": "200001240",
			"type_id":                 "93488",
			"default_weight":          "100",
			"default_width":           "100",
			"default_height":          "100",
			"default_depth":           "100",
			"auto_fill_attributes":    "false",
		},
		Product: platformp.PlatformProductDraft{
			ProductID: uuid.New(),
			Title:     "T",
			Images:    []platformp.PlatformProductImage{{URL: "https://example.com/a.jpg", Type: "main"}},
			SKUs:      []platformp.PlatformProductSKU{{LocalSKUID: uuid.New(), SKUCode: "S1", Price: 10, Stock: 1, ImageURL: "https://example.com/a.jpg"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sellerCalls != 1 {
		t.Fatalf("expected 1 seller info call, got %d", sellerCalls)
	}
}

func TestPublishProductSkippedIsFailure(t *testing.T) {
	mux := http.NewServeMux()
	handleEmptyCategoryAttributes(mux)
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"task_id":9}}`))
	})
	mux.HandleFunc(pathImportInfo, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"items":[{"offer_id":"S1","product_id":0,"status":"skipped","errors":[]}],"total":1}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prov := ozonProvider{}
	_, err := prov.PublishProduct(context.Background(), platformp.PublishProductRequest{
		ShopID:   uuid.New(),
		Platform: "ozon",
		Auth: platformp.TestConnectionRequest{
			AppKey:      "1",
			AccessToken: "2",
			Extra:       map[string]string{"api_base_url": srv.URL},
		},
		PublishConfig: map[string]any{
			"description_category_id": "200001240",
			"type_id":                 "93488",
			"default_weight":          "100",
			"default_width":           "100",
			"default_height":          "100",
			"default_depth":           "100",
			"currency_code":           "RUB",
			"vat":                     "0",
			"auto_fill_attributes":    "false",
		},
		Product: platformp.PlatformProductDraft{
			ProductID: uuid.New(),
			Title:     "T",
			Images:    []platformp.PlatformProductImage{{URL: "https://example.com/a.jpg", Type: "main"}},
			SKUs:      []platformp.PlatformProductSKU{{LocalSKUID: uuid.New(), SKUCode: "S1", Price: 10, Stock: 1, ImageURL: "https://example.com/a.jpg"}},
		},
	})
	if err == nil {
		t.Fatal("expected skipped import to fail")
	}
	if !strings.Contains(err.Error(), "skipped") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPublishProductValidation(t *testing.T) {
	prov := ozonProvider{}
	if _, err := prov.PublishProduct(context.Background(), platformp.PublishProductRequest{}); err == nil {
		t.Fatal("expected shop id error")
	}
	_, err := prov.PublishProduct(context.Background(), platformp.PublishProductRequest{
		ShopID:   uuid.New(),
		Platform: "ozon",
		Auth: platformp.TestConnectionRequest{
			AppKey:      "1",
			AccessToken: "2",
		},
		PublishConfig: map[string]any{
			"description_category_id": "0",
			"type_id":                 "0",
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "description_category_id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveOzonSKUImagePlanRejectsMissingOrDisplacedOriginal(t *testing.T) {
	missing := platformp.PlatformProductSKU{LocalSKUID: uuid.New(), SKUCode: "BLUE-L", SKUName: "蓝色 / L"}
	if _, err := resolveOzonSKUImagePlan(missing); err == nil || !strings.Contains(err.Error(), "蓝色 / L") {
		t.Fatalf("missing image error = %v", err)
	}
	displaced := platformp.PlatformProductSKU{
		LocalSKUID: uuid.New(),
		SKUCode:    "RED",
		ImageURL:   "https://example.com/red.jpg",
		Images: []platformp.PlatformProductImage{
			{URL: "https://example.com/shared.jpg", Type: "main"},
			{URL: "https://example.com/red.jpg", Type: "detail"},
		},
	}
	if _, err := resolveOzonSKUImagePlan(displaced); err == nil || !strings.Contains(err.Error(), "must be the first image") {
		t.Fatalf("displaced original error = %v", err)
	}
}

func TestResolveOzonSKUImagePlanAllowsExplicitFallbackForMissingOriginal(t *testing.T) {
	plan, err := resolveOzonSKUImagePlan(platformp.PlatformProductSKU{
		LocalSKUID: uuid.New(),
		SKUCode:    "NO-ORIGINAL",
		Images: []platformp.PlatformProductImage{
			{URL: "https://example.com/fallback.jpg", Type: "main"},
			{URL: "https://example.com/detail.jpg", Type: "detail"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Primary != "https://example.com/fallback.jpg" || len(plan.Images) != 2 {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestValidateOzonPublishMergedRequiresPositivePackageDimensions(t *testing.T) {
	valid := ozonPublishMerged{DescriptionCategoryID: 1, TypeID: 2, WeightG: 1, WidthMM: 1, HeightMM: 1, DepthMM: 1}
	if err := validateOzonPublishMerged(valid); err != nil {
		t.Fatalf("valid dimensions rejected: %v", err)
	}
	valid.HeightMM = 0
	err := validateOzonPublishMerged(valid)
	if err == nil || !strings.Contains(err.Error(), "default_height") {
		t.Fatalf("zero height error = %v", err)
	}
}

func TestFormatOzonPrice(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{1000, "1000"},
		{129.9, "129.9"},
		{129.99, "129.99"},
		{0, "0"},
		{-5, "0"},
	}
	for _, tt := range tests {
		if got := formatOzonPrice(tt.in); got != tt.want {
			t.Fatalf("formatOzonPrice(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func handleEmptyCategoryAttributes(mux *http.ServeMux) {
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
}

func statusTestPublishRequest(baseURL, warehouseID string) platformp.PublishProductRequest {
	publishConfig := map[string]any{
		"description_category_id": "200001240",
		"type_id":                 "93488",
		"default_weight":          "100",
		"default_width":           "100",
		"default_height":          "100",
		"default_depth":           "100",
		"currency_code":           "RUB",
		"vat":                     "0",
		"auto_fill_attributes":    "false",
	}
	if warehouseID != "" {
		publishConfig["warehouse_id"] = warehouseID
	}
	return platformp.PublishProductRequest{
		ShopID:   uuid.New(),
		Platform: PlatformID,
		Auth: platformp.TestConnectionRequest{
			AppKey:      "client-id",
			AccessToken: "test-api-key",
			Extra:       map[string]string{"api_base_url": baseURL},
		},
		PublishConfig: publishConfig,
		Product: platformp.PlatformProductDraft{
			ProductID: uuid.New(),
			Title:     "Status test product",
			Images:    []platformp.PlatformProductImage{{URL: "https://example.com/status.jpg", Type: "main"}},
			SKUs:      []platformp.PlatformProductSKU{{LocalSKUID: uuid.New(), SKUCode: "SKU-STATUS", Price: 10, Stock: 1, ImageURL: "https://example.com/status.jpg"}},
		},
	}
}

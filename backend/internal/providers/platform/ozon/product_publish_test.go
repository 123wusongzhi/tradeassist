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

	mux := http.NewServeMux()
	handleEmptyCategoryAttributes(mux)
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
			"auto_fill_attributes":    "false",
		},
		Product: platformp.PlatformProductDraft{
			ProductID:   uuid.New(),
			Title:       "Test Product",
			Description: "Test description",
			Currency:    "CNY",
			Images: []platformp.PlatformProductImage{
				{URL: "https://example.com/main.jpg", Type: "main"},
				{URL: "https://example.com/detail.jpg", Type: "detail"},
			},
			SKUs: []platformp.PlatformProductSKU{
				{LocalSKUID: uuid.New(), SKUCode: "SKU-001", SKUName: "Red", Price: 129.9, Stock: 10},
				{LocalSKUID: uuid.New(), SKUCode: "SKU-002", SKUName: "Blue", Price: 99, Stock: 5},
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
	if res.Status != "published" {
		t.Fatalf("unexpected status: %s", res.Status)
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
	if len(imgs) != 2 || first["primary_image"] != "https://example.com/main.jpg" {
		t.Fatalf("unexpected images: %+v", first)
	}
	second := items[1].(map[string]any)
	if second["name"] != "Test Product / Blue" {
		t.Fatalf("unexpected variant name: %+v", second)
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
			SKUs:      []platformp.PlatformProductSKU{{LocalSKUID: uuid.New(), SKUCode: "SKU-001", Price: 10, Stock: 1}},
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
			SKUs:      []platformp.PlatformProductSKU{{LocalSKUID: uuid.New(), SKUCode: "S1", Price: 10, Stock: 1}},
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
			SKUs:      []platformp.PlatformProductSKU{{LocalSKUID: uuid.New(), SKUCode: "S1", Price: 10, Stock: 1}},
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

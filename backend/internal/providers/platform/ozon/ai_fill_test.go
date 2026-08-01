package ozon

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func TestFillMissingAttributesWithAI(t *testing.T) {
	chat := func(ctx context.Context, req aigate.ChatRequest) (*aigate.ChatResponse, error) {
		if !strings.Contains(req.Messages[0].Content, "Срок годности") {
			t.Fatalf("prompt missing attribute name")
		}
		return &aigate.ChatResponse{Content: `{"attributes":{"Срок годности":"365 дней"}}`}, nil
	}
	got, err := fillMissingAttributesWithAI(context.Background(), chat,
		platformp.PlatformProductDraft{Title: "Test", Description: "desc"},
		[]ozonAttribute{{ID: 8205, Name: "Срок годности", IsRequired: true}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["Срок годности"] != "365 дней" {
		t.Fatalf("unexpected suggestion: %+v", got)
	}
}

func TestFillMissingAttributesWithAIParseFailure(t *testing.T) {
	chat := func(ctx context.Context, req aigate.ChatRequest) (*aigate.ChatResponse, error) {
		return &aigate.ChatResponse{Content: "не json"}, nil
	}
	if _, err := fillMissingAttributesWithAI(context.Background(), chat,
		platformp.PlatformProductDraft{Title: "Test"},
		[]ozonAttribute{{ID: 85, Name: "Бренд", IsRequired: true}},
	); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestFillMissingAttributesWithAIChatError(t *testing.T) {
	chat := func(ctx context.Context, req aigate.ChatRequest) (*aigate.ChatResponse, error) {
		return nil, context.DeadlineExceeded
	}
	if _, err := fillMissingAttributesWithAI(context.Background(), chat,
		platformp.PlatformProductDraft{Title: "Test"},
		[]ozonAttribute{{ID: 85, Name: "Бренд", IsRequired: true}},
	); err == nil {
		t.Fatalf("expected chat error")
	}
}

func TestApplySuggestedAttributes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(pathAttributeSearch, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":5055881,"value":"Apple"}]}`))
	})
	client := newTestClient(t, mux)

	defs := []ozonAttribute{
		{ID: 85, Name: "Бренд", DictionaryID: 28732849, IsRequired: true},
		{ID: 23487, Name: "Производитель", IsRequired: true},
	}
	out, missing := client.applySuggestedAttributes(
		context.Background(), 200001240, 93488, defs,
		map[string]string{"Бренд": "Apple", "Производитель": "X Ltd"},
	)
	if len(missing) != 0 {
		t.Fatalf("unexpected missing: %v", missing)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(out))
	}
	if out[0].Values[0].DictionaryValueID != 5055881 {
		t.Fatalf("expected dictionary match: %+v", out[0])
	}
	if out[1].Values[0].Value != "X Ltd" {
		t.Fatalf("expected raw string value: %+v", out[1])
	}
}

func TestApplySuggestedAttributesDictionaryMissStaysMissing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(pathAttributeSearch, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	client := newTestClient(t, mux)
	out, missing := client.applySuggestedAttributes(
		context.Background(), 200001240, 93488,
		[]ozonAttribute{{ID: 85, Name: "Бренд", DictionaryID: 28732849, IsRequired: true}},
		map[string]string{"Бренд": "Apple"},
	)
	if len(out) != 0 || len(missing) != 1 {
		t.Fatalf("expected dictionary miss to stay missing, out=%v missing=%v", out, missing)
	}
}

func TestPublishProductAIFillApplied(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(pathCategoryAttributes, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":[
			{"id":23487,"name":"Производитель","type":"string","dictionary_id":0,"is_required":true}
		]}`))
	})
	mux.HandleFunc(pathProductImport, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		items := body["items"].([]any)
		attrs := items[0].(map[string]any)["attributes"].([]any)
		if len(attrs) != 1 {
			t.Fatalf("expected AI-filled attribute, got %d", len(attrs))
		}
		_, _ = w.Write([]byte(`{"result":{"task_id":42}}`))
	})
	mux.HandleFunc(pathImportInfo, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"items":[{"offer_id":"SKU-1","product_id":100,"status":"imported","errors":[]}]}}`))
	})
	client := newTestClient(t, mux)
	prov := ozonProvider{}

	restore := bindChatForTest(func(ctx context.Context, req aigate.ChatRequest) (*aigate.ChatResponse, error) {
		return &aigate.ChatResponse{Content: `{"attributes":{"Производитель":"X Ltd"}}`}, nil
	})
	defer restore()

	res, err := prov.PublishProduct(context.Background(), platformp.PublishProductRequest{
		ShopID:   uuid.New(),
		Platform: "ozon",
		Auth: platformp.TestConnectionRequest{
			AppKey:      "1",
			AccessToken: "k",
			Extra:       map[string]string{"api_base_url": client.cfg.BaseURL},
		},
		PublishConfig: map[string]any{
			"description_category_id": "200001240",
			"type_id":                 "93488",
			"currency_code":           "RUB",
			"auto_fill_attributes":    "true",
			"ai_auto_fill":            "true",
		},
		Product: platformp.PlatformProductDraft{
			ProductID:   uuid.New(),
			Title:       "Test",
			Description: "desc",
			Images:      []platformp.PlatformProductImage{{URL: "https://example.com/a.jpg", Type: "main"}},
			SKUs:        []platformp.PlatformProductSKU{{LocalSKUID: uuid.New(), SKUCode: "SKU-1", Price: 10, Stock: 1}},
		},
	})
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if res.RawSummary["aiFillUsed"] != true {
		t.Fatalf("expected aiFillUsed in summary, got %+v", res.RawSummary)
	}
}

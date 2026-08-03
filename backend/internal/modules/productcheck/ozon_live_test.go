package productcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestValidateOzonDictionarySelectionsLiveRejectsWrongOwnership(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/description-category/attribute/values/search" {
			t.Fatalf("unexpected Ozon path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"result":[{"id":999,"value":"Acme"}]}`))
	}))
	t.Cleanup(server.Close)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&shop.Shop{}, &shop.ShopAuthToken{}, &shop.PlatformCategoryAttribute{}); err != nil {
		t.Fatal(err)
	}
	encrypter, err := encrypt.NewService("ozon-live-test-key")
	if err != nil {
		t.Fatal(err)
	}
	shopRow := shop.Shop{TenantID: 7, Platform: "ozon", ShopName: "Ozon", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	if err := db.Create(&shopRow).Error; err != nil {
		t.Fatal(err)
	}
	apiKey, _ := encrypter.Encrypt([]byte("api-key"))
	authConfig, _ := json.Marshal(map[string]string{"api_base_url": server.URL})
	if err := db.Create(&shop.ShopAuthToken{ShopID: shopRow.ID, Platform: "ozon", AuthType: "api_key", AppKey: "client-id", AccessTokenEnc: apiKey, AuthConfig: datatypes.JSON(authConfig)}).Error; err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"dictionary_id": "10"})
	if err := db.Create(&shop.PlatformCategoryAttribute{Platform: "ozon", CategoryID: "100:200", AttrID: "85", Name: "Brand", Required: true, Raw: datatypes.JSON(raw)}).Error; err != nil {
		t.Fatal(err)
	}
	config := product.ProductPlatformPublishConfig{
		CategoryID:         "100:200",
		PlatformAttributes: datatypes.JSON([]byte(`{"85":{"value":"Acme","dictionaryValueId":"123"}}`)),
	}
	svc := &Service{DB: db, Shops: &shop.Service{
		DB:        db,
		Encrypter: encrypter,
		TrustedProviderRuntimeOverrides: map[string]map[string]string{
			"ozon": {"api_base_url": server.URL, "timeout_sec": "5"},
		},
	}}
	checks, err := svc.validateOzonDictionarySelectionsLive(context.Background(), 7, shopRow.ID, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 || checks[0].Code != "OZON_DICTIONARY_VALUE_CHANGED" {
		t.Fatalf("expected ownership check failure, got %+v", checks)
	}
}

func TestOzonDictionaryIDTreatsZeroAsNonDictionary(t *testing.T) {
	for _, raw := range [][]byte{[]byte(`{"dictionary_id":0}`), []byte(`{"dictionary_id":"0"}`), nil} {
		if got := ozonDictionaryID(raw); got != "" {
			t.Fatalf("expected non-dictionary for %s, got %q", raw, got)
		}
	}
	if got := ozonDictionaryID([]byte(`{"dictionary_id":"123"}`)); got != "123" {
		t.Fatalf("expected dictionary ID 123, got %q", got)
	}
}

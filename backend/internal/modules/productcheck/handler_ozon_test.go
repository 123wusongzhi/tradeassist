package productcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ozonReadinessHTTPFixture struct {
	db        *gorm.DB
	service   *Service
	productID uuid.UUID
	shopA     uuid.UUID
	shopB     uuid.UUID
}

type readinessEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	TraceID string          `json:"traceId"`
}

func newOzonReadinessHTTPFixture(t *testing.T, apiURL string) ozonReadinessHTTPFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:ozon_readiness_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&product.Product{},
		&product.ProductImage{},
		&product.ProductSKU{},
		&product.ProductPlatformPublishConfig{},
		&shop.Shop{},
		&shop.ShopAuthToken{},
		&shop.PlatformCategory{},
		&shop.PlatformCategoryAttribute{},
		&settings.Setting{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE product_publications (product_id text, shop_id text, deleted_at datetime)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE ai_image_task_items (source_image_id text, score_json text, status text, updated_at datetime)").Error; err != nil {
		t.Fatal(err)
	}
	productRow := product.Product{TenantID: 1, Source: "test", Title: "Ozon live readiness", Description: "Long enough product description for readiness", Currency: "CNY", Status: product.StatusReady}
	if err := db.Create(&productRow).Error; err != nil {
		t.Fatal(err)
	}
	price, stock := 99.0, 10
	if err := db.Create(&product.ProductSKU{ProductID: productRow.ID, SKUName: "Default", Price: &price, Stock: &stock}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&product.ProductImage{ProductID: productRow.ID, ImageType: product.ImageTypeMain, PublicURL: "https://example.test/ozon.jpg"}).Error; err != nil {
		t.Fatal(err)
	}
	shops := []shop.Shop{
		{TenantID: 1, Platform: "ozon", ShopName: "Ozon A", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized},
		{TenantID: 1, Platform: "ozon", ShopName: "Ozon B", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized},
	}
	if err := db.Create(&shops).Error; err != nil {
		t.Fatal(err)
	}
	encrypter, err := encrypt.NewService("ozon-readiness-http-test-key")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range shops {
		apiKey, encErr := encrypter.Encrypt([]byte("api-key-private"))
		if encErr != nil {
			t.Fatal(encErr)
		}
		if err := db.Create(&shop.ShopAuthToken{ShopID: row.ID, Platform: "ozon", AuthType: "api_key", AppKey: "client-id", AccessTokenEnc: apiKey}).Error; err != nil {
			t.Fatal(err)
		}
	}
	category := shop.PlatformCategory{Platform: "ozon", CategoryID: "100:200", Name: "桌子", Level: 2, IsLeaf: true, Status: "active"}
	if err := db.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	attribute := shop.PlatformCategoryAttribute{Platform: "ozon", CategoryID: category.CategoryID, AttrID: "85", Name: "品牌", Required: true, ValueType: "string", Raw: datatypes.JSON([]byte(`{"dictionary_id":0}`))}
	if err := db.Create(&attribute).Error; err != nil {
		t.Fatal(err)
	}
	config := product.ProductPlatformPublishConfig{
		ProductID:          productRow.ID,
		Platform:           "ozon",
		ShopID:             &shops[0].ID,
		CategoryID:         category.CategoryID,
		CategoryPath:       category.Name,
		PlatformAttributes: datatypes.JSON([]byte(`{"85":{"value":"Acme"}}`)),
		SchemaHash:         shop.OzonCategorySchemaHash([]shop.PlatformCategoryAttribute{attribute}),
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatal(err)
	}
	shopService := &shop.Service{
		DB:        db,
		Encrypter: encrypter,
		TrustedProviderRuntimeOverrides: map[string]map[string]string{
			"ozon": {"api_base_url": apiURL, "timeout_sec": "2"},
		},
	}
	return ozonReadinessHTTPFixture{
		db:        db,
		service:   &Service{DB: db, Settings: &settings.Service{DB: db}, Shops: shopService},
		productID: productRow.ID,
		shopA:     shops[0].ID,
		shopB:     shops[1].ID,
	}
}

func runOzonReadinessRequest(t *testing.T, fixture ozonReadinessHTTPFixture, tenantID int64, principal *adminperm.Principal, shopID uuid.UUID) (int, readinessEnvelope) {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, tenantID)
		c.Set(ctxkey.TraceID, "trace-ozon-readiness")
		c.Set("adminperm.principal", principal)
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: fixture.service})
	body := bytes.NewBufferString(fmt.Sprintf(`{"platform":"ozon","shopId":%q}`, shopID.String()))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/"+fixture.productID.String()+"/readiness/validate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var envelope readinessEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode status=%d body=%s: %v", w.Code, w.Body.String(), err)
	}
	return w.Code, envelope
}

func principalForReadiness(tenantID int64, role string, grants ...adminperm.StoreGrant) *adminperm.Principal {
	return &adminperm.Principal{TenantID: tenantID, Role: role, Permissions: adminperm.PermissionsForRole(role), StoreGrants: grants}
}

func TestValidateOzonReadinessHTTPAuthorizationMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var attributeCalls atomic.Int32
	var sellerCalls atomic.Int32
	var importCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/description-category/attribute":
			attributeCalls.Add(1)
			_, _ = w.Write([]byte(`{"result":[{"id":85,"name":"Brand","type":"string","dictionary_id":0,"is_required":true}]}`))
		case "/v1/seller/info":
			sellerCalls.Add(1)
			_, _ = w.Write([]byte(`{"company":{"currency":"RUB"}}`))
		case "/v3/product/import":
			importCalls.Add(1)
			w.WriteHeader(http.StatusTeapot)
		default:
			t.Fatalf("unexpected Ozon path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	fixture := newOzonReadinessHTTPFixture(t, server.URL)
	tests := []struct {
		name       string
		tenantID   int64
		principal  *adminperm.Principal
		shopID     uuid.UUID
		wantStatus int
		wantCode   int
		wantText   string
	}{
		{name: "global admin cross tenant is explicit forbidden", tenantID: 1, principal: principalForReadiness(0, adminperm.RoleAdmin), shopID: fixture.shopA, wantStatus: http.StatusForbidden, wantCode: response.CodePermissionDenied, wantText: "全局管理员仅可跨租户查看"},
		{name: "tenant admin same tenant allowed", tenantID: 1, principal: principalForReadiness(1, adminperm.RoleTenantAdmin), shopID: fixture.shopA, wantStatus: http.StatusOK, wantCode: response.CodeOK},
		{name: "operator operate grant allowed", tenantID: 1, principal: principalForReadiness(1, adminperm.RoleOperator, adminperm.StoreGrant{StoreID: fixture.shopA, PermissionScope: "operate"}), shopID: fixture.shopA, wantStatus: http.StatusOK, wantCode: response.CodeOK},
		{name: "operator view grant forbidden", tenantID: 1, principal: principalForReadiness(1, adminperm.RoleOperator, adminperm.StoreGrant{StoreID: fixture.shopA, PermissionScope: "view"}), shopID: fixture.shopA, wantStatus: http.StatusForbidden, wantCode: response.CodePermissionDenied, wantText: "仅有查看权限"},
		{name: "operator without visibility gets not found", tenantID: 1, principal: principalForReadiness(1, adminperm.RoleOperator), shopID: fixture.shopA, wantStatus: http.StatusNotFound, wantCode: response.CodeNotFound, wantText: "资源不存在"},
		{name: "cross tenant operator gets not found", tenantID: 2, principal: principalForReadiness(2, adminperm.RoleOperator), shopID: fixture.shopA, wantStatus: http.StatusNotFound, wantCode: response.CodeNotFound, wantText: "资源不存在"},
		{name: "cross tenant tenant-admin gets not found", tenantID: 2, principal: principalForReadiness(2, adminperm.RoleTenantAdmin), shopID: fixture.shopA, wantStatus: http.StatusNotFound, wantCode: response.CodeNotFound, wantText: "资源不存在"},
		{name: "operator cannot select ungranted shop", tenantID: 1, principal: principalForReadiness(1, adminperm.RoleOperator, adminperm.StoreGrant{StoreID: fixture.shopA, PermissionScope: "operate"}), shopID: fixture.shopB, wantStatus: http.StatusNotFound, wantCode: response.CodeNotFound, wantText: "资源不存在"},
		{name: "tenant admin sees unconfigured second store independently", tenantID: 1, principal: principalForReadiness(1, adminperm.RoleTenantAdmin), shopID: fixture.shopB, wantStatus: http.StatusOK, wantCode: response.CodeOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, envelope := runOzonReadinessRequest(t, fixture, tt.tenantID, tt.principal, tt.shopID)
			if status != tt.wantStatus || envelope.Code != tt.wantCode || envelope.TraceID != "trace-ozon-readiness" {
				t.Fatalf("status=%d envelope=%+v", status, envelope)
			}
			if tt.wantText != "" && !strings.Contains(envelope.Message, tt.wantText) {
				t.Fatalf("message=%q want substring %q", envelope.Message, tt.wantText)
			}
			if status == http.StatusOK {
				var result CheckProductReadinessResult
				if err := json.Unmarshal(envelope.Data, &result); err != nil || result.ProductID != fixture.productID || len(result.Checks) == 0 {
					t.Fatalf("expected structured readiness result, got %s err=%v", envelope.Data, err)
				}
			}
		})
	}
	if got := attributeCalls.Load(); got != 2 {
		t.Fatalf("Ozon read-only attribute calls=%d, want 2 allowed requests only", got)
	}
	if got := sellerCalls.Load(); got != 2 {
		t.Fatalf("Ozon seller calls=%d, want only the two configured-store requests", got)
	}
	if got := importCalls.Load(); got != 0 {
		t.Fatalf("real Ozon import must never be called, got %d", got)
	}
}

func TestValidateOzonReadinessHTTPMapsOzonFailuresWithoutLeakingSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		upstream   int
		wantStatus int
		wantCode   int
		errorCode  string
		wantText   string
	}{
		{name: "invalid credential", upstream: http.StatusForbidden, wantStatus: http.StatusBadGateway, wantCode: response.CodeBadGateway, errorCode: "OZON_CREDENTIAL_INVALID", wantText: "API Key 已停用"},
		{name: "temporary outage", upstream: http.StatusInternalServerError, wantStatus: http.StatusServiceUnavailable, wantCode: response.CodeServiceUnavailable, errorCode: "OZON_UPSTREAM_UNAVAILABLE", wantText: "暂时不可用"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var importCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v3/product/import" {
					importCalls.Add(1)
				}
				w.WriteHeader(tt.upstream)
				_, _ = w.Write([]byte(`{"message":"api-key-private secret-token-should-not-leak"}`))
			}))
			t.Cleanup(server.Close)
			fixture := newOzonReadinessHTTPFixture(t, server.URL)
			status, envelope := runOzonReadinessRequest(t, fixture, 1, principalForReadiness(1, adminperm.RoleTenantAdmin), fixture.shopA)
			if status != tt.wantStatus || envelope.Code != tt.wantCode || envelope.TraceID != "trace-ozon-readiness" || !strings.Contains(envelope.Message, tt.wantText) {
				t.Fatalf("status=%d envelope=%+v", status, envelope)
			}
			var data map[string]any
			if err := json.Unmarshal(envelope.Data, &data); err != nil || data["errorCode"] != tt.errorCode {
				t.Fatalf("data=%s err=%v", envelope.Data, err)
			}
			body := string(envelope.Data) + envelope.Message
			if strings.Contains(body, "api-key-private") || strings.Contains(body, "secret-token") || strings.Contains(strings.ToLower(body), "internal error") {
				t.Fatalf("unsafe upstream detail leaked: %s", body)
			}
			if importCalls.Load() != 0 {
				t.Fatalf("real Ozon import must never be called")
			}
		})
	}
}

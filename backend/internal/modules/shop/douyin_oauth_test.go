package shop

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/gorm"
)

func TestDouyinStatePayloadRoundTrip(t *testing.T) {
	raw, err := encodeDouyinStatePayload(douyinOAuthStatePayload{
		Platform: "douyin_shop",
		AdminID:  "admin-1",
		ShopID:   "shop-1",
		TenantID: 1,
		Created:  1,
	})
	if err != nil {
		t.Fatalf("encodeDouyinStatePayload() error = %v", err)
	}
	got, err := decodeDouyinStatePayload(raw)
	if err != nil {
		t.Fatalf("decodeDouyinStatePayload() error = %v", err)
	}
	if got.Platform != "douyin_shop" || got.AdminID != "admin-1" || got.ShopID != "shop-1" || got.TenantID != 1 {
		t.Fatalf("unexpected state payload: %+v", got)
	}
}

func TestDouyinOAuthTenantIDAllowsLegacyZero(t *testing.T) {
	if !validDouyinOAuthTenantID(0) {
		t.Fatal("legacy default tenant must be accepted in OAuth state")
	}
	if validDouyinOAuthTenantID(-1) {
		t.Fatal("negative tenant ID must not be accepted in OAuth state")
	}
	if !validDouyinOAuthTenantID(1) {
		t.Fatal("positive tenant ID must be accepted in OAuth state")
	}
}

func TestOAuthTenantContextAttachesTrustedTenantToStandardContext(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/shops/test", nil)
	c.Set(ctxkey.TenantID, int64(0))
	ctx, tenantID, err := oauthTenantContext(c)
	if err != nil {
		t.Fatalf("oauthTenantContext() error = %v", err)
	}
	if tenantID != 0 {
		t.Fatalf("tenantID = %d, want 0", tenantID)
	}
	if tc := security.FromContext(ctx); tc == nil || tc.TenantID != tenantID {
		t.Fatalf("standard context lost trusted tenant: %+v", tc)
	}
}

func TestDouyinStatePayloadRejectsWrongPlatform(t *testing.T) {
	if _, err := decodeDouyinStatePayload(`{"platform":"tiktok"}`); err == nil {
		t.Fatalf("expected platform mismatch error")
	}
}

func TestDouyinStatePayloadRequiresExplicitTenant(t *testing.T) {
	if _, err := decodeDouyinStatePayload(`{"platform":"douyin_shop","created":1}`); err == nil {
		t.Fatal("state without captured tenant must be rejected")
	}

	got, err := decodeDouyinStatePayload(`{"platform":"douyin_shop","tenantId":0,"created":1}`)
	if err != nil {
		t.Fatalf("explicit tenant zero must remain valid: %v", err)
	}
	if got.TenantID != 0 {
		t.Fatalf("tenantID = %d, want exact legacy tenant 0", got.TenantID)
	}
}

func TestUpdateAuthRejectsCrossTenantShopWithoutTokenWrite(t *testing.T) {
	db := newDouyinShopTestDB(t)
	shop := Shop{TenantID: 2, Platform: "tiktok", ShopName: "tenant-b", Status: StatusActive, AuthStatus: AuthUnauthorized}
	if err := db.Create(&shop).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("PUT", "/api/v1/shops/"+shop.ID.String()+"/auth", nil)
	c.Set(ctxkey.TenantID, int64(1))
	_, err := (&Service{DB: db}).UpdateAuth(c, shop.ID, UpdateAuthBody{}, nil)
	if err == nil {
		t.Fatal("expected cross-tenant update to fail")
	}
	var count int64
	if err := db.Model(&ShopAuthToken{}).Where("shop_id = ?", shop.ID).Count(&count).Error; err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if count != 0 {
		t.Fatalf("cross-tenant request wrote %d token rows", count)
	}
}

func TestDouyinFriendlyMessages(t *testing.T) {
	for _, code := range []string{
		DouyinAppConfigIncomplete,
		DouyinOAuthStateInvalid,
		DouyinTokenExchangeFailed,
		DouyinShopInfoFailed,
		DouyinAuthExpired,
	} {
		if douyinFriendlyMessage(code) == "" || douyinFriendlyMessage(code) == code {
			t.Fatalf("missing friendly message for %s", code)
		}
	}
}

func TestPersistDouyinShopInfoUpdatesShopAndToken(t *testing.T) {
	db := newDouyinShopTestDB(t)
	svc := &Service{DB: db}
	shopID := uuid.New()
	if err := db.Create(&Shop{
		Platform:   "douyin_shop",
		ShopName:   "Old Shop",
		Status:     StatusActive,
		AuthStatus: AuthNeedCheck,
		Currency:   "CNY",
	}).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	var shopRow Shop
	if err := db.First(&shopRow).Error; err != nil {
		t.Fatalf("load shop: %v", err)
	}
	shopID = shopRow.ID
	if err := db.Create(&ShopAuthToken{
		ShopID:   shopID,
		Platform: "douyin_shop",
		AuthType: "oauth2",
	}).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	exp := time.Date(2026, 6, 6, 1, 2, 3, 0, time.UTC)
	if err := svc.persistDouyinShopInfo(context.Background(), 0, shopID, &platformdouyin.ShopInfo{
		PlatformShopID:   "shop-1",
		ShopName:         "Demo Shop",
		ShopStatus:       "normal",
		AuthorityID:      "auth-1",
		ShopBizType:      "local",
		AuthorizedScopes: []any{"shop", "product"},
		ExpiresAt:        &exp,
		Raw:              map[string]any{"shop_id": "shop-1"},
	}); err != nil {
		t.Fatalf("persistDouyinShopInfo() error = %v", err)
	}
	if err := db.First(&shopRow, "id = ?", shopID).Error; err != nil {
		t.Fatalf("reload shop: %v", err)
	}
	if shopRow.ShopName != "Demo Shop" || shopRow.ExternalShopID != "shop-1" || shopRow.AuthStatus != AuthAuthorized {
		t.Fatalf("unexpected shop row: %+v", shopRow)
	}
	var tok ShopAuthToken
	if err := db.First(&tok, "shop_id = ?", shopID).Error; err != nil {
		t.Fatalf("reload token: %v", err)
	}
	if tok.ExpiresAt == nil || !tok.ExpiresAt.Equal(exp) {
		t.Fatalf("unexpected expiry: %v", tok.ExpiresAt)
	}
	if strings.Contains(strings.ToLower(string(tok.RawData)), "token") || strings.Contains(strings.ToLower(string(tok.RawData)), "secret") {
		t.Fatalf("raw data leaked sensitive text: %s", string(tok.RawData))
	}
	var scopes []any
	if err := json.Unmarshal(tok.Scopes, &scopes); err != nil || len(scopes) != 2 {
		t.Fatalf("unexpected scopes: %s err=%v", string(tok.Scopes), err)
	}
}

func TestMarkDouyinShopInfoFailedIsSafe(t *testing.T) {
	db := newDouyinShopTestDB(t)
	svc := &Service{DB: db}
	shop := Shop{Platform: "douyin_shop", ShopName: "Demo", Status: StatusActive, AuthStatus: AuthAuthorized}
	if err := db.Create(&shop).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	if err := db.Create(&ShopAuthToken{ShopID: shop.ID, Platform: "douyin_shop", AuthType: "oauth2"}).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	svc.markDouyinShopInfoFailed(context.Background(), 0, shop.ID, DouyinShopInfoFailed, "access_token=secret", AuthNeedCheck)
	var row Shop
	if err := db.First(&row, "id = ?", shop.ID).Error; err != nil {
		t.Fatalf("reload shop: %v", err)
	}
	if row.AuthStatus != AuthNeedCheck {
		t.Fatalf("expected need_check, got %s", row.AuthStatus)
	}
	var tok ShopAuthToken
	if err := db.First(&tok, "shop_id = ?", shop.ID).Error; err != nil {
		t.Fatalf("reload token: %v", err)
	}
	raw := strings.ToLower(string(tok.RawData))
	if strings.Contains(raw, "access_token") || strings.Contains(raw, "secret") {
		t.Fatalf("raw data leaked sensitive text: %s", string(tok.RawData))
	}
}

func TestPersistDouyinShopInfoRejectsCrossTenantWithoutSideEffects(t *testing.T) {
	db := newDouyinShopTestDB(t)
	svc := &Service{DB: db}
	shop := Shop{TenantID: 2, Platform: "douyin_shop", ShopName: "tenant-b", Status: StatusActive, AuthStatus: AuthAuthorized, Currency: "USD"}
	if err := db.Create(&shop).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	if err := db.Create(&ShopAuthToken{ShopID: shop.ID, Platform: "douyin_shop", AuthType: "oauth2", RawData: []byte(`{"original":true}`)}).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	err := svc.persistDouyinShopInfo(context.Background(), 1, shop.ID, &platformdouyin.ShopInfo{PlatformShopID: "cross-tenant", ShopName: "changed"})
	if err == nil {
		t.Fatal("expected cross-tenant shop-info update to fail")
	}
	var got Shop
	if err := db.First(&got, "id = ?", shop.ID).Error; err != nil {
		t.Fatalf("reload shop: %v", err)
	}
	if got.ShopName != shop.ShopName || got.ExternalShopID != "" || got.Currency != "USD" || got.AuthStatus != AuthAuthorized {
		t.Fatalf("cross-tenant update mutated shop: %+v", got)
	}
	var tok ShopAuthToken
	if err := db.First(&tok, "shop_id = ?", shop.ID).Error; err != nil {
		t.Fatalf("reload token: %v", err)
	}
	if string(tok.RawData) != `{"original":true}` {
		t.Fatalf("cross-tenant update mutated token: %s", string(tok.RawData))
	}
}

func TestMarkDouyinShopInfoFailedRejectsCrossTenantWithoutSideEffects(t *testing.T) {
	db := newDouyinShopTestDB(t)
	svc := &Service{DB: db}
	shop := Shop{TenantID: 2, Platform: "douyin_shop", ShopName: "tenant-b", Status: StatusActive, AuthStatus: AuthAuthorized}
	if err := db.Create(&shop).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	if err := db.Create(&ShopAuthToken{ShopID: shop.ID, Platform: "douyin_shop", AuthType: "oauth2", RawData: []byte(`{"original":true}`)}).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	svc.markDouyinShopInfoFailed(context.Background(), 1, shop.ID, DouyinShopInfoFailed, "failed", AuthExpired)
	var got Shop
	if err := db.First(&got, "id = ?", shop.ID).Error; err != nil {
		t.Fatalf("reload shop: %v", err)
	}
	if got.AuthStatus != AuthAuthorized {
		t.Fatalf("cross-tenant failure marker changed auth status: %s", got.AuthStatus)
	}
	var tok ShopAuthToken
	if err := db.First(&tok, "shop_id = ?", shop.ID).Error; err != nil {
		t.Fatalf("reload token: %v", err)
	}
	if string(tok.RawData) != `{"original":true}` {
		t.Fatalf("cross-tenant failure marker mutated token: %s", string(tok.RawData))
	}
}

func TestDouyinClientLoggerWritesHTTPAndBackgroundAuditRows(t *testing.T) {
	db := newDouyinShopTestDB(t)
	svc := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}
	adminID := uuid.New()
	shopID := uuid.New()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/shops/douyin/test", nil)
	c.Set(ctxkey.TenantID, int64(41))
	c.Set(ctxkey.TraceID, "http-trace")
	svc.douyinClientLogger(c, 41, shopID, &adminID).LogDouyinRequest(context.Background(), platformdouyin.SafeRequestLog{
		Method: "shop.get", RequestID: "request-http", TraceID: "trace-http", ElapsedMs: 12, Success: true,
	})
	svc.douyinClientLogger(nil, 42, shopID, &adminID).LogDouyinRequest(context.Background(), platformdouyin.SafeRequestLog{
		Method: "shop.get", RequestID: "request-background", TraceID: "trace-background", ElapsedMs: 8, Success: false, ErrorCode: "denied",
	})

	var logs []operationlog.OperationLog
	if err := db.Order("created_at ASC").Find(&logs).Error; err != nil {
		t.Fatalf("load operation logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("operation log count = %d, want 2", len(logs))
	}
	if logs[0].TenantID != 41 || logs[0].Method != "POST" || logs[0].Path != "/api/v1/shops/douyin/test" || logs[0].RequestID != "http-trace" {
		t.Fatalf("unexpected HTTP audit row: %+v", logs[0])
	}
	if logs[1].TenantID != 42 || logs[1].Method != "INTERNAL" || logs[1].Path != "/internal/worker" || logs[1].Platform != "douyin_shop" || logs[1].Action != "douyin.client.failed" || logs[1].Status != "failed" || logs[1].AdminUserID == nil || *logs[1].AdminUserID != adminID || logs[1].ShopID == nil || *logs[1].ShopID != shopID {
		t.Fatalf("unexpected background audit row: %+v", logs[1])
	}
	for _, log := range logs {
		if strings.Contains(strings.ToLower(log.Message), "token") || strings.Contains(strings.ToLower(log.Message), "secret") {
			t.Fatalf("audit row leaked sensitive data: %s", log.Message)
		}
	}
}

func newDouyinShopTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Shop{}, &ShopAuthToken{}, &operationlog.OperationLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

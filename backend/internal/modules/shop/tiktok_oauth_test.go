package shop

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestPlatformOAuthStateBindsTenantAndShop(t *testing.T) {
	shopID := uuid.New()
	raw, err := encodePlatformOAuthState("tiktok", 1, shopID)
	if err != nil {
		t.Fatalf("encode state: %v", err)
	}
	if err := decodePlatformOAuthState(raw, "tiktok", 1, shopID); err != nil {
		t.Fatalf("valid state rejected: %v", err)
	}
	if err := decodePlatformOAuthState(raw, "tiktok", 2, shopID); err == nil {
		t.Fatal("cross-tenant state was accepted")
	}
	if err := decodePlatformOAuthState(raw, "tiktok", 1, uuid.New()); err == nil {
		t.Fatal("cross-shop state was accepted")
	}
	if err := decodePlatformOAuthState(raw, "shopee", 1, shopID); err == nil {
		t.Fatal("cross-platform state was accepted")
	}
}

func TestOAuthTenantContextRequiresGinTenantAndAllowsLegacyZero(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	if _, _, err := oauthTenantContext(c); err == nil {
		t.Fatal("missing tenant context was accepted")
	}
	c.Set(ctxkey.TenantID, int64(0))
	_, tenantID, err := oauthTenantContext(c)
	if err != nil || tenantID != 0 {
		t.Fatalf("legacy tenant zero rejected: id=%d err=%v", tenantID, err)
	}
}

func TestSetAuthStatusCtxRejectsCrossTenantWithoutSideEffect(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Shop{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	shop := Shop{TenantID: 2, Platform: "tiktok", ShopName: "tenant-b", Status: StatusActive, AuthStatus: AuthAuthorized}
	if err := db.Create(&shop).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	if err := (&Service{DB: db}).setAuthStatusCtx(t.Context(), 1, shop.ID, AuthError); err == nil {
		t.Fatal("cross-tenant status update was accepted")
	}
	var got Shop
	if err := db.First(&got, "id = ?", shop.ID).Error; err != nil {
		t.Fatalf("reload shop: %v", err)
	}
	if got.AuthStatus != AuthAuthorized {
		t.Fatalf("cross-tenant request changed status to %q", got.AuthStatus)
	}
}

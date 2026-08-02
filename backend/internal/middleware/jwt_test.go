package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func TestSecureSessionBearerRejectsUnboundAccessTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		JWTSecret: "middleware-secure-session-test-secret",
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeSecure,
			AccessTokenTTLMinutes: 15,
		},
	}
	db, err := gorm.Open(sqlite.Open("file:middleware_secure_session?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sessions := &auth.SessionService{Cfg: cfg, DB: db}
	userID := uuid.New()

	cases := []struct {
		name      string
		tokenType string
		sessionID string
	}{
		{name: "missing session", tokenType: "access"},
		{name: "nil session", tokenType: "access", sessionID: uuid.Nil.String()},
		{name: "malformed session", tokenType: "access", sessionID: "not-a-uuid"},
		{name: "missing token type", sessionID: uuid.NewString()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := signMiddlewareAccessToken(t, cfg, auth.AccessClaims{
				TokenType: tc.tokenType,
				SessionID: tc.sessionID,
				TenantID:  17,
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   userID.String(),
					ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
				},
			})
			res := requestThroughBearer(cfg, db, sessions, token)
			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusUnauthorized, res.Body.String())
			}
		})
	}
}

func TestLegacyBearerAllowsSessionlessAccessToken(t *testing.T) {
	cfg := &config.Config{
		JWTSecret: "middleware-legacy-session-test-secret",
		JWTExpHrs: 1,
		Auth:      config.AuthConfig{SessionMode: config.AuthSessionModeLegacy},
	}
	token, _, err := auth.LegacyMintToken(cfg, uuid.New(), "legacy")
	if err != nil {
		t.Fatal(err)
	}
	res := requestThroughBearer(cfg, nil, nil, token)
	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusNoContent, res.Body.String())
	}
}

func TestBearerAuthPropagatesTrustedTenantToRequestContext(t *testing.T) {
	cfg := &config.Config{
		JWTSecret: "middleware-request-context-test-secret",
		JWTExpHrs: 1,
		Auth:      config.AuthConfig{SessionMode: config.AuthSessionModeLegacy},
	}
	const tenantID int64 = 17
	token := signMiddlewareAccessToken(t, cfg, auth.AccessClaims{
		TenantID: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
		},
	})

	router := gin.New()
	router.Use(BearerAuth(cfg))
	router.GET("/protected", func(c *gin.Context) {
		tc := security.FromContext(c.Request.Context())
		if tc == nil || tc.TenantID != tenantID || tc.AuthSource != security.AuthSourceAccessToken {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusNoContent, res.Body.String())
	}
}

func TestSecureBearerFailsClosedWithoutSessionService(t *testing.T) {
	cfg := &config.Config{
		JWTSecret: "middleware-misconfigured-session-test-secret",
		Auth:      config.AuthConfig{SessionMode: config.AuthSessionModeSecure},
	}
	token := signMiddlewareAccessToken(t, cfg, auth.AccessClaims{
		TokenType: "access",
		SessionID: uuid.NewString(),
		TenantID:  17,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
		},
	})
	res := requestThroughBearer(cfg, nil, nil, token)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusInternalServerError, res.Body.String())
	}
}

func TestProductionBearerAllowsOnlyValidatedGlobalSystemAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		AppEnv:    config.EnvProduction,
		JWTSecret: "middleware-production-system-admin-test-secret",
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeSecure,
			AccessTokenTTLMinutes: 15,
		},
	}

	tests := []struct {
		name       string
		tenantID   int64
		role       string
		expectCode int
	}{
		{name: "global admin", tenantID: 0, role: "admin", expectCode: http.StatusNoContent},
		{name: "global operator denied", tenantID: 0, role: "operator", expectCode: http.StatusForbidden},
		{name: "tenant admin unchanged", tenantID: 17, role: "admin", expectCode: http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, sessions, userID, sessionID := newMiddlewareSession(t, cfg, tt.tenantID, tt.role)
			token := signMiddlewareAccessToken(t, cfg, auth.AccessClaims{
				TokenType:    "access",
				SessionID:    sessionID.String(),
				TenantID:     tt.tenantID,
				TokenVersion: 1,
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   userID.String(),
					ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
				},
			})
			res := requestThroughBearer(cfg, db, sessions, token)
			if res.Code != tt.expectCode {
				t.Fatalf("status = %d, want %d; body=%s", res.Code, tt.expectCode, res.Body.String())
			}
		})
	}
}

func TestProductionBearerRejectsMissingOrLegacyZeroTenant(t *testing.T) {
	cfg := &config.Config{
		AppEnv:    config.EnvProduction,
		JWTSecret: "middleware-production-zero-rejection-test-secret",
		Auth:      config.AuthConfig{SessionMode: config.AuthSessionModeSecure},
	}
	missingSession := signMiddlewareAccessToken(t, cfg, auth.AccessClaims{
		TokenType: "access",
		TenantID:  0,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
		},
	})
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if res := requestThroughBearer(cfg, db, &auth.SessionService{Cfg: cfg, DB: db}, missingSession); res.Code != http.StatusUnauthorized {
		t.Fatalf("missing secure session status = %d, want %d", res.Code, http.StatusUnauthorized)
	}

	legacyCfg := &config.Config{
		AppEnv:    config.EnvProduction,
		JWTSecret: "middleware-production-legacy-zero-rejection-test-secret",
		Auth:      config.AuthConfig{SessionMode: config.AuthSessionModeLegacy},
	}
	legacy := signMiddlewareAccessToken(t, legacyCfg, auth.AccessClaims{
		TenantID: 0,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
		},
	})
	if res := requestThroughBearer(legacyCfg, nil, nil, legacy); res.Code != http.StatusForbidden {
		t.Fatalf("legacy zero status = %d, want %d; body=%s", res.Code, http.StatusForbidden, res.Body.String())
	}
}

func newMiddlewareSession(t testing.TB, cfg *config.Config, tenantID int64, role string) (*gorm.DB, *auth.SessionService, uuid.UUID, uuid.UUID) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&auth.AuthSession{}, &admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	userID := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: userID},
		TenantID:     tenantID,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "test-password-hash",
		Role:         role,
		Status:       "active",
		TokenVersion: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	sessionID := uuid.New()
	if err := db.Create(&auth.AuthSession{ID: sessionID, TenantID: tenantID, UserID: userID, Status: auth.SessionStatusActive}).Error; err != nil {
		t.Fatal(err)
	}
	return db, &auth.SessionService{Cfg: cfg, DB: db}, userID, sessionID
}

func signMiddlewareAccessToken(t *testing.T, cfg *config.Config, claims auth.AccessClaims) string {
	t.Helper()
	keys, err := auth.BuildKeySet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = keys.ActiveID
	signed, err := token.SignedString(keys.ActiveSecret)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func requestThroughBearer(cfg *config.Config, db *gorm.DB, sessions *auth.SessionService, token string) *httptest.ResponseRecorder {
	router := gin.New()
	router.Use(BearerAuthWithDB(cfg, db, sessions))
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

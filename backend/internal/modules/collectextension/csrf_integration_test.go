package collectextension

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	securitypkg "github.com/trademind-ai/trademind/backend/internal/pkg/security"
)

const collectExtensionTestOrigin = "chrome-extension://abcdefghijklmnopabcdefghijklmnop"

func newCSRFProtectedBrowserExtensionRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(securitypkg.CSRFProtection(&config.Config{
		AppEnv:         config.EnvProduction,
		AdminPublicURL: "https://admin.example.com",
		APIPublicURL:   "https://api.example.com",
		Auth:           config.AuthConfig{SessionMode: config.AuthSessionModeSecure},
	}))
	RegisterPublic(router.Group("/api/v1"), handler)
	return router
}

func serveBrowserExtensionRequest(
	t *testing.T,
	router http.Handler,
	method, path, body, token string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Origin", collectExtensionTestOrigin)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestBrowserExtensionPairingExchangePassesCSRFAndRemainsOneTime(t *testing.T) {
	db := openBrowserExtensionServiceTestDB(t)
	service := &Service{DB: db}
	pairing, err := service.CreatePairing(t.Context(), 42, uuid.New())
	require.NoError(t, err)

	body, err := json.Marshal(ExchangePairingBody{Code: pairing.Code, DeviceName: "CSRF regression"})
	require.NoError(t, err)
	router := newCSRFProtectedBrowserExtensionRouter(&Handler{Svc: service})

	response := serveBrowserExtensionRequest(
		t,
		router,
		http.MethodPost,
		"/api/v1/collect/browser-extension/pairings/exchange",
		string(body),
		"",
	)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	var envelope struct {
		Code int                   `json:"code"`
		Data ExchangePairingResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	require.Zero(t, envelope.Code)
	require.NotEmpty(t, envelope.Data.DeviceToken)

	var storedDevice BrowserExtensionDevice
	require.NoError(t, db.First(&storedDevice, "id = ?", envelope.Data.Device.ID).Error)
	require.NotEqual(t, envelope.Data.DeviceToken, storedDevice.TokenHash)
	require.Equal(t, authutil.HashToken(envelope.Data.DeviceToken, ""), storedDevice.TokenHash)

	replayed := serveBrowserExtensionRequest(
		t,
		router,
		http.MethodPost,
		"/api/v1/collect/browser-extension/pairings/exchange",
		string(body),
		"",
	)
	require.Equal(t, http.StatusBadRequest, replayed.Code, replayed.Body.String())
}

func TestBrowserExtensionPairingExchangeRejectsExpiredCode(t *testing.T) {
	db := openBrowserExtensionServiceTestDB(t)
	service := &Service{DB: db}
	code := "ABCDEFGHJK"
	expired := &BrowserExtensionPairing{
		TenantID:    42,
		AdminUserID: uuid.New(),
		CodeHash:    authutil.HashToken(code, ""),
		ExpiresAt:   time.Now().UTC().Add(-time.Minute),
	}
	require.NoError(t, db.Create(expired).Error)

	body, err := json.Marshal(ExchangePairingBody{Code: code, DeviceName: "Expired code"})
	require.NoError(t, err)
	router := newCSRFProtectedBrowserExtensionRouter(&Handler{Svc: service})
	response := serveBrowserExtensionRequest(
		t,
		router,
		http.MethodPost,
		"/api/v1/collect/browser-extension/pairings/exchange",
		string(body),
		"",
	)

	require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	var deviceCount int64
	require.NoError(t, db.Model(&BrowserExtensionDevice{}).Count(&deviceCount).Error)
	require.Zero(t, deviceCount)
}

func TestBrowserExtensionDeviceWriteRoutesStillRequireDeviceToken(t *testing.T) {
	db := openBrowserExtensionServiceTestDB(t)
	router := newCSRFProtectedBrowserExtensionRouter(&Handler{Svc: &Service{DB: db}})
	taskID := "11111111-1111-1111-1111-111111111111"

	tests := []struct {
		name  string
		path  string
		body  string
		token string
	}{
		{
			name: "create task without token",
			path: "/api/v1/collect/browser-extension/tasks",
			body: `{"source":"taobao_tmall","url":"https://detail.tmall.com/item.htm?id=1"}`,
		},
		{
			name:  "submit result with invalid token",
			path:  "/api/v1/collect/browser-extension/tasks/" + taskID + "/result",
			body:  `{"product":{}}`,
			token: "tmx_" + strings.Repeat("x", 40),
		},
		{
			name:  "submit failure with invalid token",
			path:  "/api/v1/collect/browser-extension/tasks/" + taskID + "/failure",
			body:  `{"errorCode":"TEST","message":"test"}`,
			token: "tmx_" + strings.Repeat("x", 40),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := serveBrowserExtensionRequest(
				t,
				router,
				http.MethodPost,
				tt.path,
				tt.body,
				tt.token,
			)
			require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
		})
	}
}

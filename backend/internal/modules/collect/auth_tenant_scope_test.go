package collect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func TestAuthCheckFallbackURLIsTenantScoped(t *testing.T) {
	svc, db := newCollectTenantScopeService(t)
	for _, task := range []CollectTask{
		{TenantID: 11, Source: "pinduoduo", SourceURL: "https://a.example/pdd", Status: StatusFailed},
		{TenantID: 22, Source: "pinduoduo", SourceURL: "https://b.example/pdd", Status: StatusFailed},
		{TenantID: 0, Source: "pinduoduo", SourceURL: "https://zero.example/pdd", Status: StatusFailed},
		{TenantID: 11, Source: "taobao_tmall", SourceURL: "https://a.example/tb", Status: StatusFailed},
		{TenantID: 22, Source: "taobao_tmall", SourceURL: "https://b.example/tb", Status: StatusFailed},
		{TenantID: 0, Source: "taobao_tmall", SourceURL: "https://zero.example/tb", Status: StatusFailed},
	} {
		require.NoError(t, db.Create(&task).Error)
	}

	for _, tc := range []struct {
		name   string
		tenant int64
		pdd    string
		tb     string
	}{
		{name: "tenant A", tenant: 11, pdd: "https://a.example/pdd", tb: "https://a.example/tb"},
		{name: "tenant B", tenant: 22, pdd: "https://b.example/pdd", tb: "https://b.example/tb"},
		{name: "system tenant", tenant: 0, pdd: "https://zero.example/pdd", tb: "https://zero.example/tb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pdd, _ := svc.ResolvePinduoduoAuthCheckInputs(context.Background(), tc.tenant, "")
			tb, _ := svc.ResolveTaobaoTmallAuthCheckInputs(context.Background(), tc.tenant, "")
			require.Equal(t, tc.pdd, pdd)
			require.Equal(t, tc.tb, tb)
		})
	}

	pdd, _ := svc.ResolvePinduoduoAuthCheckInputs(context.Background(), -1, "")
	tb, _ := svc.ResolveTaobaoTmallAuthCheckInputs(context.Background(), -1, "")
	require.Empty(t, pdd)
	require.Empty(t, tb)
}

func TestPinduoduoAuthStatusDoesNotSendOtherTenantFallbackURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, db := newCollectTenantScopeService(t)
	require.NoError(t, db.Create(&CollectTask{TenantID: 22, Source: "pinduoduo", SourceURL: "https://b.example/secret", Status: StatusFailed}).Error)

	var collectorBody map[string]string
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/providers/pinduoduo/check-login", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&collectorBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{"provider":"pinduoduo","checkedUrl":"` + collectorBody["url"] + `"}}`))
	}))
	defer collector.Close()
	svc.Client = NewCollectorClient(collector.URL, 0)

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(ctxkey.TenantID, int64(11)) })
	router.GET("/auth-status", (&Handler{Svc: svc}).GetPinduoduoAuthStatus)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth-status", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, collectorBody["url"])
	require.NotContains(t, rec.Body.String(), "b.example/secret")
}

func TestPinduoduoAuthStatusUsesCurrentTenantFallbackURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, db := newCollectTenantScopeService(t)
	require.NoError(t, db.Create(&CollectTask{TenantID: 11, Source: "pinduoduo", SourceURL: "https://a.example/owned", Status: StatusFailed}).Error)
	require.NoError(t, db.Create(&CollectTask{TenantID: 22, Source: "pinduoduo", SourceURL: "https://b.example/secret", Status: StatusFailed}).Error)

	var collectorBody map[string]string
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&collectorBody))
		_, _ = w.Write([]byte(`{"ok":true,"data":{"provider":"pinduoduo","checkedUrl":"https://a.example/owned"}}`))
	}))
	defer collector.Close()
	svc.Client = NewCollectorClient(collector.URL, 0)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(ctxkey.TenantID, int64(11)) })
	router.GET("/auth-status", (&Handler{Svc: svc}).GetPinduoduoAuthStatus)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth-status", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "https://a.example/owned", collectorBody["url"])
	require.NotContains(t, rec.Body.Bytes(), []byte("b.example/secret"))
}

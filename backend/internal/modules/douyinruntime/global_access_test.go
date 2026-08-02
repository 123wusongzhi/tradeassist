package douyinruntime_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/douyinpreflight"
	"github.com/trademind-ai/trademind/backend/internal/modules/douyinruntime"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/storagepublic"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGlobalRuntimeDiagnosticRoutesRequireSystemAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:douyin_global_runtime_access?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &settings.Setting{}))

	users := map[string]admin.AdminUser{
		"global": {TenantID: 0, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive},
		"tenant": {TenantID: 88, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleTenantAdmin, Status: admin.StatusActive},
	}
	for name, user := range users {
		require.NoErrorf(t, db.Create(&user).Error, "create %s", name)
		users[name] = user
	}

	settingSvc := &settings.Service{DB: db}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if user, ok := users[c.GetHeader("X-Test-Actor")]; ok {
			c.Set(ctxkey.AdminID, user.ID.String())
		}
		c.Next()
	})
	api := router.Group("/api/v1")
	preflightSvc := &douyinpreflight.Service{DB: db, Settings: settingSvc}
	douyinruntime.Register(api, &douyinruntime.Handler{Svc: &douyinruntime.Service{DB: db, Settings: settingSvc, Preflight: preflightSvc}})
	douyinpreflight.Register(api, &douyinpreflight.Handler{Svc: preflightSvc})
	storagepublic.Register(api, &storagepublic.Handler{DB: db, Svc: &storagepublic.Service{Settings: settingSvc}})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/platform/douyin/runtime-status"},
		{http.MethodPost, "/api/v1/platform/douyin/runtime-status/pause"},
		{http.MethodPost, "/api/v1/platform/douyin/runtime-status/resume"},
		{http.MethodPost, "/api/v1/platform/douyin/runtime-status/emergency-disable"},
		{http.MethodGet, "/api/v1/platform/douyin/health"},
		{http.MethodGet, "/api/v1/platform/douyin/metrics-summary"},
		{http.MethodGet, "/api/v1/platform/douyin/release-gate"},
		{http.MethodPost, "/api/v1/platform/douyin/run-health-check"},
		{http.MethodPost, "/api/v1/platform/douyin/production-preflight"},
		{http.MethodGet, "/api/v1/platform/douyin/production-preflight/latest"},
		{http.MethodPost, "/api/v1/storage/test-public-access"},
		{http.MethodPost, "/api/v1/settings/storage/public-check"},
		{http.MethodGet, "/api/v1/settings/storage/public-check/latest"},
	}
	for _, route := range routes {
		req := httptest.NewRequest(route.method, route.path, nil)
		req.Header.Set("X-Test-Actor", "tenant")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equalf(t, http.StatusForbidden, rec.Code, "tenant admin must not access %s %s", route.method, route.path)
	}

	for _, route := range routes {
		req := httptest.NewRequest(route.method, route.path, nil)
		req.Header.Set("X-Test-Actor", "global")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.NotEqualf(t, http.StatusForbidden, rec.Code, "global admin must reach %s %s", route.method, route.path)
	}
}

func TestGlobalRuntimeRouteGuardsFailClosedWithoutDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		method   string
		path     string
		message  string
		register func(*gin.RouterGroup)
	}{
		{
			name:    "runtime nil service",
			method:  http.MethodGet,
			path:    "/api/v1/platform/douyin/runtime-status",
			message: "douyin runtime routes unavailable",
			register: func(g *gin.RouterGroup) {
				douyinruntime.Register(g, &douyinruntime.Handler{})
			},
		},
		{
			name:    "runtime nil database",
			method:  http.MethodGet,
			path:    "/api/v1/platform/douyin/runtime-status",
			message: "douyin runtime routes unavailable",
			register: func(g *gin.RouterGroup) {
				douyinruntime.Register(g, &douyinruntime.Handler{Svc: &douyinruntime.Service{}})
			},
		},
		{
			name:    "preflight nil service",
			method:  http.MethodGet,
			path:    "/api/v1/platform/douyin/production-preflight/latest",
			message: "douyin preflight routes unavailable",
			register: func(g *gin.RouterGroup) {
				douyinpreflight.Register(g, &douyinpreflight.Handler{})
			},
		},
		{
			name:    "preflight nil database",
			method:  http.MethodGet,
			path:    "/api/v1/platform/douyin/production-preflight/latest",
			message: "douyin preflight routes unavailable",
			register: func(g *gin.RouterGroup) {
				douyinpreflight.Register(g, &douyinpreflight.Handler{Svc: &douyinpreflight.Service{}})
			},
		},
		{
			name:    "storage nil database",
			method:  http.MethodGet,
			path:    "/api/v1/settings/storage/public-check/latest",
			message: "storage public routes unavailable",
			register: func(g *gin.RouterGroup) {
				storagepublic.Register(g, &storagepublic.Handler{})
			},
		},
		{
			name:    "storage write nil database",
			method:  http.MethodPost,
			path:    "/api/v1/storage/test-public-access",
			message: "storage public routes unavailable",
			register: func(g *gin.RouterGroup) {
				storagepublic.Register(g, &storagepublic.Handler{})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			tc.register(router.Group("/api/v1"))

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))

			require.Equal(t, http.StatusInternalServerError, rec.Code)
			var envelope response.Envelope
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
			require.Equal(t, response.CodeInternalError, envelope.Code)
			require.Equal(t, tc.message, envelope.Message)
		})
	}
}

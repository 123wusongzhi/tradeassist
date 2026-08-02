package shop

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestPlatformSettingsRequireGlobalConfigAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:shop_platform_settings_permissions?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}))
	users := map[string]admin.AdminUser{
		"global":       {TenantID: 0, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive},
		"tenant_admin": {TenantID: 42, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleTenantAdmin, Status: admin.StatusActive},
		"legacy_admin": {TenantID: 42, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive},
	}
	for name, user := range users {
		require.NoErrorf(t, db.Create(&user).Error, "create %s", name)
		users[name] = user
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		if user, ok := users[c.GetHeader("X-Test-Actor")]; ok {
			c.Set(ctxkey.AdminID, user.ID.String())
			c.Set(ctxkey.TenantID, user.TenantID)
		}
	})
	h := &Handler{Svc: &Service{DB: db}}
	router.GET("/platform/settings/:platform", h.GetPlatformAppSettings)
	router.PUT("/platform/settings/:platform", h.PutPlatformAppSettings)
	router.POST("/platform/settings/:platform/test-connection", h.TestPlatformAppSettings)
	router.GET("/platform/publish-settings/:platform", h.GetPlatformPublishSettings)
	router.PUT("/platform/publish-settings/:platform", h.PutPlatformPublishSettings)

	routes := []struct{ method, path string }{
		{http.MethodGet, "/platform/settings/tiktok"},
		{http.MethodPut, "/platform/settings/tiktok"},
		{http.MethodPost, "/platform/settings/tiktok/test-connection"},
		{http.MethodGet, "/platform/publish-settings/tiktok"},
		{http.MethodPut, "/platform/publish-settings/tiktok"},
	}
	for _, actor := range []string{"tenant_admin", "legacy_admin"} {
		for _, route := range routes {
			req := httptest.NewRequest(route.method, route.path, nil)
			req.Header.Set("X-Test-Actor", actor)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			require.Equalf(t, http.StatusForbidden, res.Code, "%s must not access %s %s", actor, route.method, route.path)
		}
	}
	for _, route := range routes {
		req := httptest.NewRequest(route.method, route.path, nil)
		req.Header.Set("X-Test-Actor", "global")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.NotEqualf(t, http.StatusForbidden, res.Code, "global admin must reach %s %s", route.method, route.path)
	}
}

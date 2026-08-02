package idor_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/adminuser"
	"github.com/trademind-ai/trademind/backend/internal/modules/configstatus"
	"github.com/trademind-ai/trademind/backend/internal/modules/observabilitymod"
	"github.com/trademind-ai/trademind/backend/internal/modules/securitymod"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGlobalResourceRoutesRejectNonSystemAdministrators(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:global_resource_routes?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}))

	users := map[string]admin.AdminUser{
		"global": {TenantID: 0, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive},
		// A legacy admin label on a non-system tenant must receive the same denial.
		"tenant": {TenantID: 42, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive},
	}
	for name, user := range users {
		require.NoErrorf(t, db.Create(&user).Error, "create %s", name)
		users[name] = user
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		if user, ok := users[c.GetHeader("X-Test-Actor")]; ok {
			c.Set(ctxkey.AdminID, user.ID.String())
		}
		c.Next()
	})
	api := r.Group("/api/v1")
	configstatus.Register(api, &configstatus.Handler{Svc: &configstatus.Service{DB: db}})
	observabilitymod.Register(api, &observabilitymod.Handler{DB: db})
	securitymod.RegisterRoutes(api, &securitymod.Handler{Svc: &securitymod.Service{DB: db}, DB: db})
	adminuser.Register(api, &adminuser.Handler{Svc: &adminuser.Service{DB: db}})
	settingsHandler := &settings.Handler{Svc: &settings.Service{DB: db}, DB: db}
	api.GET("/settings", settingsHandler.List)

	for _, path := range []string{
		"/api/v1/settings",
		"/api/v1/settings/config-status",
		"/api/v1/observability/overview",
		"/api/v1/security/overview",
		"/api/v1/admin/users",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Test-Actor", "tenant")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equalf(t, http.StatusForbidden, rec.Code, "non-system admin must not access %s", path)
	}

	// A system administrator clears the access guard; handler-specific setup may
	// still reject the request, but it must not be denied as a tenant boundary.
	for _, path := range []string{
		"/api/v1/settings",
		"/api/v1/settings/config-status",
		"/api/v1/observability/overview",
		"/api/v1/security/overview",
		"/api/v1/admin/users",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Test-Actor", "global")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.NotEqualf(t, http.StatusForbidden, rec.Code, "system admin must clear guard for %s", path)
	}
}

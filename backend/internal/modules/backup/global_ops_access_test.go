package backup_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/backup"
	"github.com/trademind-ai/trademind/backend/internal/modules/disasterrecovery"
	"github.com/trademind-ai/trademind/backend/internal/modules/release"
	"github.com/trademind-ai/trademind/backend/internal/modules/restore"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGlobalOpsRoutesRequireSystemAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:global_ops_access?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &backup.Job{}, &restore.Job{}, &release.Run{}, &disasterrecovery.Drill{}))

	users := map[string]admin.AdminUser{
		"global":       {TenantID: 0, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive},
		"tenant_admin": {TenantID: 101, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleTenantAdmin, Status: admin.StatusActive},
		"operator":     {TenantID: 101, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleOperator, Status: admin.StatusActive},
		"readonly":     {TenantID: 101, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleReadonly, Status: admin.StatusActive},
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
	backup.Register(api, &backup.Handler{Svc: &backup.Service{DB: db}})
	restore.Register(api, &restore.Handler{Svc: &restore.Service{DB: db}})
	release.Register(api, &release.Handler{Svc: &release.Service{DB: db}})
	disasterrecovery.Register(api, &disasterrecovery.Handler{Svc: &disasterrecovery.Service{DB: db}})

	allRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/ops/backups"},
		{http.MethodPost, "/api/v1/ops/backups"},
		{http.MethodGet, "/api/v1/ops/backups/backup123"},
		{http.MethodPost, "/api/v1/ops/backups/backup123/verify"},
		{http.MethodPost, "/api/v1/ops/backups/backup123/hold"},
		{http.MethodDelete, "/api/v1/ops/backups/backup123"},
		{http.MethodGet, "/api/v1/ops/restores"},
		{http.MethodPost, "/api/v1/ops/restores"},
		{http.MethodGet, "/api/v1/ops/restores/restore123"},
		{http.MethodPost, "/api/v1/ops/restores/restore123/verify"},
		{http.MethodGet, "/api/v1/ops/releases"},
		{http.MethodPost, "/api/v1/ops/releases"},
		{http.MethodGet, "/api/v1/ops/releases/release123"},
		{http.MethodPost, "/api/v1/ops/releases/release123/execute"},
		{http.MethodPost, "/api/v1/ops/releases/release123/rollback"},
		{http.MethodGet, "/api/v1/ops/dr/status"},
		{http.MethodPost, "/api/v1/ops/dr/drills"},
	}
	for _, role := range []string{"tenant_admin", "operator", "readonly"} {
		for _, route := range allRoutes {
			req := httptest.NewRequest(route.method, route.path, nil)
			req.Header.Set("X-Test-Actor", role)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			require.Equalf(t, http.StatusForbidden, rec.Code, "%s must not access %s %s", role, route.method, route.path)
		}
	}

	for _, path := range []string{
		"/api/v1/ops/backups",
		"/api/v1/ops/restores",
		"/api/v1/ops/releases",
		"/api/v1/ops/dr/status",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Test-Actor", "global")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equalf(t, http.StatusOK, rec.Code, "global admin must reach %s", path)
	}
}

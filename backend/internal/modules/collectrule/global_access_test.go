package collectrule

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
	"gorm.io/gorm/logger"
)

func TestCollectRuleRoutesRequireGlobalSettingsAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:collectrule_global_access?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &CollectRule{}))

	users := map[string]admin.AdminUser{
		"global":       {TenantID: 0, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive},
		"legacy_admin": {TenantID: 101, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive},
		"tenant_admin": {TenantID: 101, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleTenantAdmin, Status: admin.StatusActive},
	}
	for name, user := range users {
		require.NoErrorf(t, db.Create(&user).Error, "create %s", name)
		users[name] = user
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		if user, ok := users[c.GetHeader("X-Test-Actor")]; ok {
			c.Set(ctxkey.AdminID, user.ID.String())
		}
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})

	for _, actor := range []string{"legacy_admin", "tenant_admin"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/collect/rules", nil)
		req.Header.Set("X-Test-Actor", actor)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equalf(t, http.StatusForbidden, rec.Code, "%s must not access instance collect rules", actor)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/collect/rules", nil)
	req.Header.Set("X-Test-Actor", "global")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "system admin must reach instance collect rules")
}

func TestCollectRuleRoutesFailClosedWithoutServiceDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for name, handler := range map[string]*Handler{
		"service":  {Svc: nil},
		"database": {Svc: &Service{}},
	} {
		t.Run(name, func(t *testing.T) {
			router := gin.New()
			Register(router.Group("/api/v1"), handler)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/collect/rules", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusInternalServerError, rec.Code)
		})
	}
}

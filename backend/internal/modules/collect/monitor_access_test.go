package collect

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

func TestMonitorRequiresGlobalAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:collect_monitor_access?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &CollectTask{}, &CollectBatch{}))

	users := map[string]admin.AdminUser{
		"global":   {TenantID: 0, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive},
		"legacy":   {TenantID: 91, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive},
		"operator": {TenantID: 91, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleOperator, Status: admin.StatusActive},
		"readonly": {TenantID: 91, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleReadonly, Status: admin.StatusActive},
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

	for _, actor := range []string{"legacy", "operator", "readonly"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/collect/monitor", nil)
		req.Header.Set("X-Test-Actor", actor)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equalf(t, http.StatusForbidden, rec.Code, "%s must not access instance monitor", actor)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/collect/monitor", nil)
	req.Header.Set("X-Test-Actor", "global")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "system admin must clear the monitor access gate")
}

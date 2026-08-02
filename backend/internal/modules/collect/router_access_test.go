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

func TestCollectBusinessReadRoutesRequireProductView(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:collect_router_access?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &CollectTask{}, &CollectBatch{}, &CollectTaskEvent{}))

	users := map[string]admin.AdminUser{
		"reviewer": {TenantID: 11, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleReviewer, Status: admin.StatusActive},
		"readonly": {TenantID: 11, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleReadonly, Status: admin.StatusActive},
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
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})

	for _, path := range []string{
		"/api/v1/collect/providers",
		"/api/v1/collect/engines/status",
		"/api/v1/collect/tasks",
		"/api/v1/collect/tasks/not-a-uuid/events",
		"/api/v1/collect/tasks/not-a-uuid",
		"/api/v1/collect/batches",
		"/api/v1/collect/batches/not-a-uuid/tasks",
		"/api/v1/collect/batches/not-a-uuid",
		"/api/v1/collector/providers/1688/auth-status",
		"/api/v1/collector/providers/pinduoduo/auth-status",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Test-Actor", "reviewer")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equalf(t, http.StatusForbidden, rec.Code, "%s must require product.view", path)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/collect/providers", nil)
	req.Header.Set("X-Test-Actor", "readonly")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "product.view role must read providers")
}

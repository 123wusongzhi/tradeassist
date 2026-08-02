package operationdashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type dashboardShopRecord struct {
	ID       uuid.UUID `gorm:"type:char(36);primaryKey"`
	TenantID int64     `gorm:"not null;index"`
}

func (dashboardShopRecord) TableName() string { return "shops" }

func dashboardTenantFixture(t *testing.T) (*gorm.DB, *Service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:operationdashboard_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&admin.AdminUser{}, &product.Product{}, &aitask.AITask{}, &aioperationbatch.AIOperationBatch{},
		&imagetask.ImageTask{}, &collect.CollectTask{}, &taskcenter.TaskAlert{}, &dashboardShopRecord{}, &productpublish.ProductPublishTask{},
	))

	for _, tenantID := range []int64{1, 2} {
		p := product.Product{TenantID: tenantID, Source: "test", Title: "tenant-" + string(rune('0'+tenantID)), Status: product.StatusDraft, Currency: "USD"}
		require.NoError(t, db.Create(&p).Error)
		store := dashboardShopRecord{ID: uuid.New(), TenantID: tenantID}
		require.NoError(t, db.Create(&store).Error)
		require.NoError(t, db.Create(&aitask.AITask{TenantID: tenantID, ProductID: &p.ID, TaskType: "title_optimize", Status: aitask.StatusFailed, ErrorMessage: "tenant task"}).Error)
		require.NoError(t, db.Create(&aioperationbatch.AIOperationBatch{TenantID: tenantID, BatchNo: "batch-" + string(rune('0'+tenantID)), OperationType: aioperationbatch.OperationTitleOptimize, Status: aioperationbatch.StatusRunning}).Error)
		require.NoError(t, db.Create(&imagetask.ImageTask{TenantID: tenantID, ProductID: &p.ID, TaskType: imagetask.TaskTypeGenerateScene, Provider: "test", Status: imagetask.StatusFailed, ErrorMessage: "tenant image"}).Error)
		require.NoError(t, db.Create(&collect.CollectTask{TenantID: tenantID, Source: "test", SourceURL: "https://example.test/" + string(rune('0'+tenantID)), Status: collect.StatusFailed, ErrorMessage: "tenant collect"}).Error)
		require.NoError(t, db.Create(&productpublish.ProductPublishTask{TenantID: tenantID, ProductID: p.ID, ShopID: store.ID, TargetStoreID: store.ID, Platform: "test", TaskType: "publish", Mode: "manual", Status: productpublish.TaskFailed, ErrorMessage: "tenant publish"}).Error)
		require.NoError(t, db.Create(&taskcenter.TaskAlert{ID: uuid.New(), TenantID: tenantID, TaskType: "test", SourceID: uuid.NewString(), FailureCategory: "test", Severity: "critical", Title: "tenant alert", Status: taskcenter.TaskAlertStatusOpen, FirstSeenAt: time.Now().UTC(), LastSeenAt: time.Now().UTC()}).Error)
	}
	return db, &Service{DB: db}
}

func TestDashboardAggregatesAreTenantScopedAndFailClosed(t *testing.T) {
	_, svc := dashboardTenantFixture(t)
	ctx := context.Background()

	for _, tenantID := range []int64{1, 2} {
		out, err := svc.GetProductOperationDashboard(ctx, Query{Scope: Scope{TenantID: tenantID}}, Scope{TenantID: tenantID})
		require.NoError(t, err)
		require.Equal(t, int64(1), out.Summary.TotalProducts)
		require.Equal(t, int64(1), out.Summary.AiTaskFailedCount)
		require.Equal(t, int64(1), out.Summary.AiBatchRunningCount)
		require.Equal(t, int64(1), out.Summary.ImageTaskFailed)
		require.Equal(t, int64(1), out.Summary.CollectFailedCount)
		require.Equal(t, int64(1), out.Summary.PublishFailedTasks)
		require.Equal(t, int64(1), out.Summary.OpenAlertCount)
		require.Len(t, out.Recent.AiTasks, 1)
		require.Len(t, out.Recent.AiBatches, 1)
		require.Len(t, out.Recent.ImageTasks, 1)
		require.Len(t, out.Recent.Alerts, 1)
		require.Len(t, out.Recent.PublishTasks, 1)
	}

	closed, err := svc.GetProductOperationDashboard(ctx, Query{Scope: Scope{}}, Scope{})
	require.NoError(t, err)
	require.Zero(t, closed.Summary.TotalProducts)
	require.Zero(t, closed.Summary.AiTaskFailedCount)
	require.Zero(t, closed.Summary.AiBatchRunningCount)
	require.Zero(t, closed.Summary.ImageTaskFailed)
	require.Zero(t, closed.Summary.CollectFailedCount)
	require.Zero(t, closed.Summary.PublishFailedTasks)
	require.Zero(t, closed.Summary.OpenAlertCount)
	require.Empty(t, closed.Recent.AiTasks)
	require.Empty(t, closed.Recent.AiBatches)
	require.Empty(t, closed.Recent.ImageTasks)
	require.Empty(t, closed.Recent.Alerts)
	require.Empty(t, closed.Recent.PublishTasks)

	global, err := svc.GetProductOperationDashboard(ctx, Query{Scope: Scope{IsAdmin: true}}, Scope{IsAdmin: true})
	require.NoError(t, err)
	require.Equal(t, int64(2), global.Summary.TotalProducts)
	require.Equal(t, int64(2), global.Summary.AiTaskFailedCount)
	require.Equal(t, int64(2), global.Summary.AiBatchRunningCount)
	require.Equal(t, int64(2), global.Summary.ImageTaskFailed)
	require.Equal(t, int64(2), global.Summary.CollectFailedCount)
	require.Equal(t, int64(2), global.Summary.PublishFailedTasks)
	require.Equal(t, int64(2), global.Summary.OpenAlertCount)
}

func TestDashboardScopeAndPermissionHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, svc := dashboardTenantFixture(t)
	readonly := admin.AdminUser{TenantID: 1, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleReadonly, Status: admin.StatusActive}
	require.NoError(t, db.Create(&readonly).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		if c.Request.Header.Get("X-Admin-ID") != "" {
			c.Set(ctxkey.AdminID, c.Request.Header.Get("X-Admin-ID"))
		}
	})
	Register(router.Group("/api/v1"), &Handler{Svc: svc})

	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/overview", nil))
	require.Equal(t, http.StatusForbidden, missing.Code)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/overview", nil)
	request.Header.Set("X-Admin-ID", readonly.ID.String())
	allowed := httptest.NewRecorder()
	router.ServeHTTP(allowed, request)
	require.Equal(t, http.StatusOK, allowed.Code, allowed.Body.String())
	require.NotContains(t, allowed.Body.String(), "config_risk")
}

func TestScopeFromContextIsFailClosedAndOnlySystemAdminIsGlobal(t *testing.T) {
	db, _ := dashboardTenantFixture(t)
	systemAdmin := admin.AdminUser{TenantID: 0, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	tenantAdmin := admin.AdminUser{TenantID: 1, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleTenantAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&systemAdmin).Error)
	require.NoError(t, db.Create(&tenantAdmin).Error)

	newContext := func(adminID string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		if adminID != "" {
			c.Set(ctxkey.AdminID, adminID)
		}
		return c
	}

	missing := scopeFromContext(newContext(""), db)
	require.False(t, missing.IsAdmin)
	require.Zero(t, missing.TenantID)

	global := scopeFromContext(newContext(systemAdmin.ID.String()), db)
	require.True(t, global.IsAdmin)

	tenant := scopeFromContext(newContext(tenantAdmin.ID.String()), db)
	require.False(t, tenant.IsAdmin)
	require.Equal(t, int64(1), tenant.TenantID)
}

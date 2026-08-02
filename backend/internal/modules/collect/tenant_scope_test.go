package collect

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func newCollectTenantScopeService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:collect_tenant_scope_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&CollectBatch{}, &CollectTask{}, &CollectTaskEvent{}); err != nil {
		t.Fatal(err)
	}
	return &Service{DB: db, QueueEnabled: true}, db
}

func collectTenantContext(t *testing.T, tenantID int64) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func TestCollectTenantScopeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, db := newCollectTenantScopeService(t)
	batchA := CollectBatch{TenantID: 11, Source: "1688", Status: BatchStatusFailed, TotalCount: 1, FailedCount: 1}
	batchB := CollectBatch{TenantID: 22, Source: "1688", Status: BatchStatusFailed, TotalCount: 1, FailedCount: 1}
	batch0 := CollectBatch{TenantID: 0, Source: "1688", Status: BatchStatusSuccess}
	if err := db.Create(&batchA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&batchB).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&batch0).Error; err != nil {
		t.Fatal(err)
	}
	taskA := CollectTask{TenantID: 11, BatchID: &batchA.ID, Source: "1688", SourceURL: "https://a.example", Status: StatusFailed}
	taskB := CollectTask{TenantID: 22, BatchID: &batchB.ID, Source: "1688", SourceURL: "https://b.example", Status: StatusFailed}
	if err := db.Create(&taskA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&taskB).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&CollectTaskEvent{TaskID: taskB.ID, BatchID: &batchB.ID, EventType: EventTaskFailed}).Error; err != nil {
		t.Fatal(err)
	}

	if list, err := svc.List(collectTenantContext(t, 11), ListQuery{}); err != nil || list.Total != 1 || list.Items[0].SourceURL != taskA.SourceURL {
		t.Fatalf("tenant A task list = %+v, %v", list, err)
	}
	if list, err := svc.ListBatches(collectTenantContext(t, 11), BatchListQuery{}); err != nil || list.Total != 1 || list.Items[0].Source != batchA.Source {
		t.Fatalf("tenant A batch list = %+v, %v", list, err)
	}
	if _, err := svc.GetDTO(collectTenantContext(t, 11), taskB.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant task get = %v", err)
	}
	if _, err := svc.GetBatchDTO(collectTenantContext(t, 11), batchB.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant batch get = %v", err)
	}
	if _, err := svc.ListTaskEvents(collectTenantContext(t, 11), taskB.ID, TaskEventsListQuery{}); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant events = %v", err)
	}
	if _, err := svc.RetryAsync(collectTenantContext(t, 11), taskB.ID, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant task retry = %v", err)
	}
	if _, err := svc.RetryFailedBatchTasks(collectTenantContext(t, 11), batchB.ID, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant batch retry = %v", err)
	}
	var unchanged CollectTask
	if err := db.First(&unchanged, "id = ?", taskB.ID).Error; err != nil || unchanged.Status != StatusFailed {
		t.Fatalf("cross-tenant retry changed task: %+v, %v", unchanged, err)
	}
	if list, err := svc.List(collectTenantContext(t, 0), ListQuery{}); err != nil || list.Total != 0 {
		t.Fatalf("tenant zero task list = %+v, %v", list, err)
	}
	if list, err := svc.ListBatches(collectTenantContext(t, 0), BatchListQuery{}); err != nil || list.Total != 1 || list.Items[0].Source != batch0.Source {
		t.Fatalf("tenant zero batch list = %+v, %v", list, err)
	}
	if _, err := svc.List(collectTenantContext(t, -1), ListQuery{}); err == nil {
		t.Fatal("negative tenant must be rejected")
	}
}

func TestCollectTenantScopeHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, db := newCollectTenantScopeService(t)
	owned := CollectTask{TenantID: 11, Source: "1688", SourceURL: "https://owned.example", Status: StatusFailed}
	other := CollectTask{TenantID: 22, Source: "1688", SourceURL: "https://other.example", Status: StatusFailed}
	if err := db.Create(&owned).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	h := &Handler{Svc: svc}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(ctxkey.TenantID, int64(11)) })
	r.GET("/collect/tasks", h.List)
	r.GET("/collect/tasks/:id", h.Get)
	r.GET("/collect/tasks/:id/events", h.ListTaskEvents)
	r.POST("/collect/tasks/:id/retry", h.Retry)

	list := httptest.NewRecorder()
	r.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/collect/tasks", nil))
	if list.Code != http.StatusOK || string(list.Body.Bytes()) == "" || bytes.Contains(list.Body.Bytes(), []byte(other.ID.String())) {
		t.Fatalf("tenant HTTP list leaked: status=%d body=%s", list.Code, list.Body.String())
	}
	for _, path := range []string{
		"/collect/tasks/" + other.ID.String(),
		"/collect/tasks/" + other.ID.String() + "/events",
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("cross-tenant HTTP %s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
	retry := httptest.NewRecorder()
	r.ServeHTTP(retry, httptest.NewRequest(http.MethodPost, "/collect/tasks/"+other.ID.String()+"/retry", nil))
	if retry.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant HTTP retry status=%d body=%s", retry.Code, retry.Body.String())
	}
}

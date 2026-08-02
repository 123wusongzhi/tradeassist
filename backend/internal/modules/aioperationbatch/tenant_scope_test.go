package aioperationbatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func newTenantScopeService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:aioperationbatch_tenant_scope?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AIOperationBatch{}, &product.Product{}, &aitask.AITask{}); err != nil {
		t.Fatal(err)
	}
	return &Service{DB: db}, db
}

func tenantContext(t *testing.T, tenantID int64) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func TestBatchTenantScopeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, db := newTenantScopeService(t)
	owned := AIOperationBatch{TenantID: 11, BatchNo: "AI-11", OperationType: OperationTitleOptimize, Status: StatusSuccess}
	other := AIOperationBatch{TenantID: 22, BatchNo: "AI-22", OperationType: OperationTitleOptimize, Status: StatusSuccess}
	legacy := AIOperationBatch{TenantID: 0, BatchNo: "AI-0", OperationType: OperationTitleOptimize, Status: StatusSuccess}
	if err := db.Create(&[]AIOperationBatch{owned, other, legacy}).Error; err != nil {
		t.Fatal(err)
	}

	items, total, err := svc.ListBatches(tenantContext(t, 11), ListBatchesQuery{Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(items) != 1 || items[0].TenantID != 11 {
		t.Fatalf("tenant list leaked batches: items=%+v total=%d err=%v", items, total, err)
	}
	if _, err := svc.GetByID(tenantContext(t, 11), other.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("other tenant batch must be hidden, got %v", err)
	}
	if _, err := svc.GetByID(tenantContext(t, 11), legacy.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("legacy tenant-zero batch must be hidden, got %v", err)
	}
	if _, _, err := svc.ListBatches(tenantContext(t, 0), ListBatchesQuery{}); err == nil {
		t.Fatal("legacy tenant-zero request context must be rejected")
	}
}

func TestApplyBatchResultsCannotCrossTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, db := newTenantScopeService(t)
	p := product.Product{TenantID: 11, Source: "test", Status: product.StatusDraft, Title: "original"}
	batch := AIOperationBatch{TenantID: 11, BatchNo: "AI-apply", OperationType: OperationTitleOptimize, Status: StatusSuccess}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(map[string]string{"optimizedTitle": "tenant-one-title"})
	if err := db.Create(&aitask.AITask{TaskType: "title_optimize", Status: "success", ProductID: &p.ID, BatchID: &batch.ID, Output: datatypes.JSON(out)}).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ApplyBatchResults(tenantContext(t, 22), batch.ID, ApplyBatchResultsBody{Target: applyTargetAIField}, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant apply must be hidden, got %v", err)
	}
	var fresh product.Product
	if err := db.First(&fresh, "id = ?", p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.AITitle != "" {
		t.Fatalf("cross-tenant apply changed product: %q", fresh.AITitle)
	}
	if applied, err := svc.ApplyBatchResults(tenantContext(t, 11), batch.ID, ApplyBatchResultsBody{Target: applyTargetAIField}, nil); err != nil || applied != 1 {
		t.Fatalf("owned apply = %d, %v", applied, err)
	}
	if err := db.First(&fresh, "id = ?", p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.AITitle != "tenant-one-title" {
		t.Fatalf("owned apply did not update product: %q", fresh.AITitle)
	}
}

func TestBatchTenantScopeHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, db := newTenantScopeService(t)
	owned := AIOperationBatch{TenantID: 11, BatchNo: "AI-http-11", OperationType: OperationTitleOptimize, Status: StatusSuccess}
	other := AIOperationBatch{TenantID: 22, BatchNo: "AI-http-22", OperationType: OperationTitleOptimize, Status: StatusSuccess}
	if err := db.Create(&[]AIOperationBatch{owned, other}).Error; err != nil {
		t.Fatal(err)
	}
	h := &Handler{Svc: svc}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, int64(11))
	})
	r.GET("/ai/batches", h.List)
	r.GET("/ai/batches/:id", h.Get)

	list := httptest.NewRecorder()
	r.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/ai/batches", nil))
	if list.Code != http.StatusOK || bytes.Contains(list.Body.Bytes(), []byte(other.BatchNo)) || !bytes.Contains(list.Body.Bytes(), []byte(owned.BatchNo)) {
		t.Fatalf("tenant HTTP list leaked data: status=%d body=%s", list.Code, list.Body.String())
	}
	get := httptest.NewRecorder()
	r.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/ai/batches/"+other.ID.String(), nil))
	if get.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant HTTP get status=%d body=%s", get.Code, get.Body.String())
	}
}

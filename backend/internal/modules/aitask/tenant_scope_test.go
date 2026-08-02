package aitask_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

type productTenantRecord struct {
	ID       uuid.UUID `gorm:"type:char(36);primaryKey"`
	TenantID int64     `gorm:"not null;default:0;index"`
}

func (productTenantRecord) TableName() string { return "products" }

func newAITaskTenantFixture(t *testing.T) (*gorm.DB, productTenantRecord, productTenantRecord, aitask.AITask, aitask.AITask) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:aitask_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&productTenantRecord{}, &aitask.AITask{}); err != nil {
		t.Fatal(err)
	}
	first := productTenantRecord{ID: uuid.New(), TenantID: 11}
	second := productTenantRecord{ID: uuid.New(), TenantID: 22}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	one := aitask.AITask{TenantID: 11, TaskType: "title", Status: aitask.StatusSuccess, ProductID: &first.ID}
	two := aitask.AITask{TenantID: 22, TaskType: "title", Status: aitask.StatusSuccess, ProductID: &second.ID}
	if err := db.Create(&one).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&two).Error; err != nil {
		t.Fatal(err)
	}
	return db, first, second, one, two
}

func TestHTTPListAndGetAreTenantScoped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _, _, owned, other := newAITaskTenantFixture(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, int64(11))
		c.Set("adminperm.principal", &adminperm.Principal{
			TenantID: 11,
			Role:     adminperm.RoleTenantAdmin,
		})
	})
	aitask.Register(r.Group("/api/v1"), &aitask.Handler{Svc: &aitask.Service{DB: db}})

	list := httptest.NewRecorder()
	r.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/ai/tasks", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), owned.ID.String()) || strings.Contains(list.Body.String(), other.ID.String()) {
		t.Fatalf("tenant list leaked task: status=%d body=%s", list.Code, list.Body.String())
	}
	get := httptest.NewRecorder()
	r.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/ai/tasks/"+other.ID.String(), nil))
	if get.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant get status=%d body=%s", get.Code, get.Body.String())
	}
}

func TestCreateDerivesTenantFromProductAndRejectsAmbiguousOwnership(t *testing.T) {
	db, first, second, _, _ := newAITaskTenantFixture(t)
	svc := &aitask.Service{DB: db}
	created := &aitask.AITask{TaskType: "description", Status: aitask.StatusRunning, ProductID: &first.ID}
	if err := svc.Create(t.Context(), created); err != nil {
		t.Fatal(err)
	}
	if created.TenantID != first.TenantID {
		t.Fatalf("created task tenant=%d, want %d", created.TenantID, first.TenantID)
	}
	if err := svc.Create(t.Context(), &aitask.AITask{TaskType: "description", Status: aitask.StatusRunning}); err == nil {
		t.Fatal("unowned task creation must fail closed")
	}
	// Product ownership and an explicitly conflicting tenant context cannot be mixed.
	if err := svc.Create(t.Context(), &aitask.AITask{TenantID: second.TenantID, TaskType: "description", Status: aitask.StatusRunning, ProductID: &first.ID}); err == nil {
		t.Fatal("conflicting task ownership must fail")
	}
}

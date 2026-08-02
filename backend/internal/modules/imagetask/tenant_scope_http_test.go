package imagetask

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

type imageTenantFixture struct {
	db       *gorm.DB
	productA product.Product
	productB product.Product
	taskA    ImageTask
	taskB    ImageTask
	itemB    ImageTaskItem
	imageB   product.ProductImage
	adminA   admin.AdminUser
	readonly admin.AdminUser
}

func newImageTenantFixture(t *testing.T) imageTenantFixture {
	t.Helper()
	dsn := fmt.Sprintf("file:imagetask_tenant_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &product.Product{}, &product.ProductImage{}, &ImageTask{}, &ImageTaskItem{}); err != nil {
		t.Fatal(err)
	}
	fx := imageTenantFixture{db: db}
	fx.productA = product.Product{TenantID: 11, Source: "test", Status: product.StatusDraft, Title: "tenant-a"}
	fx.productB = product.Product{TenantID: 22, Source: "test", Status: product.StatusDraft, Title: "tenant-b"}
	for _, row := range []*product.Product{&fx.productA, &fx.productB} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	fx.taskA = ImageTask{TenantID: 11, TaskType: TaskTypeGenerateScene, Provider: "test", Status: StatusSuccess, ProductID: &fx.productA.ID, ResultURL: "https://example.test/a.png"}
	fx.taskB = ImageTask{TenantID: 22, TaskType: TaskTypeGenerateScene, Provider: "test", Status: StatusSuccess, ProductID: &fx.productB.ID, ResultURL: "https://example.test/b.png"}
	for _, row := range []*ImageTask{&fx.taskA, &fx.taskB} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	fx.itemB = ImageTaskItem{TenantID: 22, TaskID: fx.taskB.ID, ProductID: &fx.productB.ID, OutputImageURL: fx.taskB.ResultURL, Status: ItemStatusSuccess}
	if err := db.Create(&fx.itemB).Error; err != nil {
		t.Fatal(err)
	}
	fx.imageB = product.ProductImage{ProductID: fx.productB.ID, ImageType: product.ImageTypeMain, PublicURL: fx.taskB.ResultURL}
	if err := db.Create(&fx.imageB).Error; err != nil {
		t.Fatal(err)
	}
	fx.adminA = admin.AdminUser{TenantID: 11, Username: uuid.NewString(), Email: "admin-a@example.test", PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 1}
	fx.readonly = admin.AdminUser{TenantID: 11, Username: uuid.NewString(), Email: "readonly-a@example.test", PasswordHash: "test", Role: adminperm.RoleReadonly, Status: admin.StatusActive, TokenVersion: 1}
	for _, row := range []*admin.AdminUser{&fx.adminA, &fx.readonly} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return fx
}

func imageTenantRouter(fx imageTenantFixture, user admin.AdminUser) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, user.ID.String())
		c.Set(ctxkey.TenantID, user.TenantID)
		c.Next()
	})
	Register(g, &Handler{Svc: &Service{DB: fx.db}})
	return r
}

func performImageTenantRequest(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	r.ServeHTTP(rec, req)
	return rec
}

func TestImageTaskHTTPResourcesAreTenantScoped(t *testing.T) {
	fx := newImageTenantFixture(t)
	r := imageTenantRouter(fx, fx.adminA)

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/image/tasks/" + fx.taskB.ID.String(), ""},
		{http.MethodGet, "/api/v1/image/tasks/" + fx.taskB.ID.String() + "/items", ""},
		{http.MethodPost, "/api/v1/image/tasks/" + fx.taskB.ID.String() + "/apply", `{"productId":"` + fx.productB.ID.String() + `"}`},
		{http.MethodDelete, "/api/v1/image/tasks/" + fx.taskB.ID.String() + "/items/" + fx.itemB.ID.String(), ""},
		{http.MethodPost, "/api/v1/ai/image/task-items/" + fx.itemB.ID.String() + "/save-to-product", `{"productId":"` + fx.productB.ID.String() + `"}`},
		{http.MethodPost, "/api/v1/ai/image/task-items/" + fx.itemB.ID.String() + "/set-as-main", `{"productId":"` + fx.productB.ID.String() + `"}`},
		{http.MethodPost, "/api/v1/ai/image/score", `{"sourceImageId":"` + fx.imageB.ID.String() + `"}`},
		{http.MethodPost, "/api/v1/products/" + fx.productB.ID.String() + "/images/select-best-main", `{}`},
	} {
		rec := performImageTenantRequest(r, tc.method, tc.path, tc.body)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s status=%d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}

	list := performImageTenantRequest(r, http.MethodGet, "/api/v1/image/tasks?page=1&pageSize=20", "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	if !strings.Contains(list.Body.String(), fx.taskA.ID.String()) || strings.Contains(list.Body.String(), fx.taskB.ID.String()) {
		t.Fatalf("tenant list leaked task: %s", list.Body.String())
	}
}

func TestImageScoreWriteRequiresProductPermission(t *testing.T) {
	fx := newImageTenantFixture(t)
	r := imageTenantRouter(fx, fx.readonly)
	rec := performImageTenantRequest(r, http.MethodPost, "/api/v1/ai/image/score", `{"sourceImageId":"`+fx.imageB.ID.String()+`"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("readonly score status=%d body=%s", rec.Code, rec.Body.String())
	}
}

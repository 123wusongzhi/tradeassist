package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiopsworkbench"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/webhook"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func TestBusinessReadRoutesRejectReviewerWithoutDomainViewPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:business_read_permissions?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, int64(11))
		c.Set("adminperm.principal", &adminperm.Principal{TenantID: 11, Role: adminperm.RoleReviewer})
		c.Request = c.Request.WithContext(security.WithTenantContext(c.Request.Context(), &security.TenantContext{TenantID: 11}))
	})
	g := r.Group("/api/v1")
	product.Register(g, &product.Handler{Svc: &product.Service{DB: db}})
	order.Register(g, &order.Handler{Svc: &order.Service{DB: db}})
	inventory.Register(g, &inventory.Handler{Svc: &inventory.Service{DB: db}})
	imagetask.Register(g, &imagetask.Handler{Svc: &imagetask.Service{DB: db}})
	aioperationbatch.Register(g, &aioperationbatch.Handler{Svc: &aioperationbatch.Service{DB: db}})
	aitask.Register(g, &aitask.Handler{Svc: &aitask.Service{DB: db}})
	webhook.Register(g, &webhook.Handler{Svc: &webhook.Service{DB: db}})
	aiopsworkbench.Register(g, &aiopsworkbench.Handler{Svc: &aiopsworkbench.Service{DB: db}})

	paths := []string{
		"/api/v1/product-skus/search",
		"/api/v1/orders",
		"/api/v1/inventory",
		"/api/v1/image/tasks",
		"/api/v1/ai/batches",
		"/api/v1/ai/tasks",
		"/api/v1/webhook-events",
		"/api/v1/ai/operation-workbench/summary",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			if w.Code != http.StatusForbidden {
				t.Fatalf("GET %s status=%d body=%s", path, w.Code, w.Body.String())
			}
		})
	}
}

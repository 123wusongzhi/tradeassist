package aiproductimage

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func TestWithTenantContextPropagatesTrustedTenantToDetachedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(ctxkey.TenantID, int64(42))

	ctx, tenantID, err := withTenantContext(c)
	if err != nil {
		t.Fatalf("with tenant context: %v", err)
	}
	if tenantID != 42 {
		t.Fatalf("tenant id = %d, want 42", tenantID)
	}
	c.Request = c.Request.WithContext(ctx)
	if got, err := tenantIDFromContext(detachedGinContext(c).Request.Context()); err != nil || got != 42 {
		t.Fatalf("detached request tenant = %d, err = %v; want 42, nil", got, err)
	}
}

func TestWithTenantContextRejectsMissingTrustedTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	if _, _, err := withTenantContext(c); err == nil {
		t.Fatal("expected missing trusted tenant to be rejected")
	}
}

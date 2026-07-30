package adminperm

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func TestTenantIDFromGin_allowsLegacyZero(t *testing.T) {
	t.Parallel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ctxkey.TenantID, int64(0))
	tid, err := TenantIDFromGin(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tid != 0 {
		t.Fatalf("got tid=%d", tid)
	}
}

func TestTenantIDFromGin_missingContext(t *testing.T) {
	t.Parallel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, err := TenantIDFromGin(c)
	if err == nil {
		t.Fatal("expected missing tenant error")
	}
}

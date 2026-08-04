package productpublish

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func TestApplyResolvedOzonPublishOptionsPreservesFallbackCurrency(t *testing.T) {
	options := map[string]any{"currency_code": "RUB"}
	applyResolvedOzonPublishOptions(options, product.OzonResolvedListingDTO{})
	if got := options["currency_code"]; got != "RUB" {
		t.Fatalf("fallback currency overwritten: %v", got)
	}
	applyResolvedOzonPublishOptions(options, product.OzonResolvedListingDTO{Currency: product.OzonResolvedString{Value: "CNY"}})
	if got := options["currency_code"]; got != "CNY" {
		t.Fatalf("resolved currency not applied: %v", got)
	}
}

func TestValidatePublishIdempotencyKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		platform  string
		key       string
		want      string
		wantError bool
	}{
		{name: "ozon requires key", platform: "ozon", wantError: true},
		{name: "ozon rejects whitespace key", platform: "ozon", key: "  ", wantError: true},
		{name: "ozon trims valid key", platform: "ozon", key: "  submit-1  ", want: "submit-1"},
		{name: "other platform remains backward compatible", platform: "shopee", want: ""},
		{name: "oversized key is rejected", platform: "ozon", key: strings.Repeat("x", maxPublishIdempotencyKeyLength+1), wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := validatePublishIdempotencyKey(tt.platform, tt.key)
			if tt.wantError {
				if err == nil {
					t.Fatalf("validatePublishIdempotencyKey() error = nil")
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("validatePublishIdempotencyKey() = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestRequestIdempotencyOwnerUsesTraceAndUniqueFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTrace, _ := gin.CreateTestContext(httptest.NewRecorder())
	withTrace.Set(ctxkey.TraceID, "request-123")
	if got := requestIdempotencyOwner(withTrace, "publish"); got != "request-123" {
		t.Fatalf("requestIdempotencyOwner() = %q, want request trace", got)
	}

	first, _ := gin.CreateTestContext(httptest.NewRecorder())
	second, _ := gin.CreateTestContext(httptest.NewRecorder())
	firstOwner := requestIdempotencyOwner(first, "publish")
	secondOwner := requestIdempotencyOwner(second, "publish")
	if firstOwner == "" || secondOwner == "" || firstOwner == secondOwner {
		t.Fatalf("fallback owners must be non-empty and unique: %q, %q", firstOwner, secondOwner)
	}
}

package metrics

import (
	"testing"
)

func TestForbiddenLabelKeys(t *testing.T) {
	v := NewLabelValidator(10)
	for _, k := range []string{"request_id", "user_id", "task_id", "tenant_id"} {
		if err := v.ValidateKey(k); err == nil {
			t.Fatalf("expected forbidden label %s", k)
		}
	}
}

func TestCardinalityDoesNotGrowWithIDs(t *testing.T) {
	v := NewLabelValidator(256)
	reg := NewRegistry("test")
	cat, err := RegisterCatalog(reg)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		_ = v.Validate("method", "GET") // allowed key
		if err := v.Validate("request_id", "x"); err == nil {
			t.Fatal("request_id should be forbidden")
		}
	}
	if cat.HTTPRequestsTotal != nil {
		cat.HTTPRequestsTotal.WithLabelValues("GET", "/api/v1/health", "2xx", "success").Inc()
	}
	if reg.Validator().SeriesCount() > 300 {
		t.Fatalf("too many series: %d", reg.Validator().SeriesCount())
	}
}

func TestNormalizeResult(t *testing.T) {
	if NormalizeResult("provider_timeout") != "timeout" {
		t.Fatal("expected timeout")
	}
	if NormalizeResult("environment_blocked") != "environment_blocked" {
		t.Fatal("expected environment_blocked")
	}
}

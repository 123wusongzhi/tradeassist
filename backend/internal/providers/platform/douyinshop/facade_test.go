package douyinshop_test

import (
	"testing"

	douyinshop "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

// TestFacadeCompile verifies that DouyinProvider interface is properly implemented.
// This is a compile-time assertion test.
func TestFacadeCompile(t *testing.T) {
	// nil client → nil facade
	f := douyinshop.NewFacade(nil)
	if f != nil {
		t.Error("expected nil facade for nil client")
	}

	// Non-nil client → non-nil facade
	c := &douyinshop.Client{}
	f2 := douyinshop.NewFacade(c)
	if f2 == nil {
		t.Error("expected non-nil facade for non-nil client")
	}

	// Verify BrandStatus returns unsupported
	bs := f2.BrandStatus()
	if bs.Supported {
		t.Error("brand list should be unsupported in P3")
	}
	if bs.Reason == "" {
		t.Error("expected non-empty brand status reason")
	}

	// CustomerCapability should be disabled
	cap := f2.CustomerCapability()
	if cap == nil {
		t.Fatal("expected non-nil CustomerCapability")
	}
	if cap.IsEnabled() {
		t.Error("customer capability should be disabled by default")
	}
}

// TestBrandListUnsupportedError verifies the brand list error shape.
func TestBrandListUnsupportedError(t *testing.T) {
	err := douyinshop.BrandListUnsupportedError()
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if err.Code != douyinshop.CodeDouyinContractMismatch {
		t.Errorf("expected CodeDouyinContractMismatch, got %s", err.Code)
	}
	if err.Retryable {
		t.Error("brand list error should not be retryable")
	}
}

package security_test

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
)

func TestPIIMasking(t *testing.T) {
	if security.MaskPhone("13812345678") != "138****5678" {
		t.Fatal("phone mask")
	}
	if security.MaskEmail("user@example.com") == "" {
		t.Fatal("email mask")
	}
}

func TestSafeRedirect(t *testing.T) {
	if _, err := security.SafeRedirect("/dashboard", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := security.SafeRedirect("javascript:alert(1)", nil); err == nil {
		t.Fatal("expected block javascript redirect")
	}
}

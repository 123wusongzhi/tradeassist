package douyinshop_test

import (
	"testing"

	douyinshop "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

func TestTokenVersionKey(t *testing.T) {
	key := douyinshop.TokenVersionKey("shop-123", 5)
	expected := "douyin-token-refresh:shop-123:5"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestTokenVersionConflictError(t *testing.T) {
	err := douyinshop.TokenVersionConflictError("shop-abc", 3, 5)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if err.Code != douyinshop.CodeDouyinTokenVersionConflict {
		t.Errorf("expected code %s, got %s", douyinshop.CodeDouyinTokenVersionConflict, err.Code)
	}
	if err.Retryable {
		t.Error("token version conflict should not be retryable")
	}
}

func TestDefaultTokenLocker(t *testing.T) {
	locker := douyinshop.DefaultTokenLocker()
	if locker == nil {
		t.Fatal("expected non-nil token locker")
	}
}

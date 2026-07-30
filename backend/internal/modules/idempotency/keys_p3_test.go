package idempotency_test

import (
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
)

func TestDouyinProductDraftCreate(t *testing.T) {
	key := idempotency.DouyinProductDraftCreate("shop-1", "draft-1", "v2")
	if key == "" {
		t.Error("expected non-empty key")
	}
	if !strings.HasPrefix(key, "douyin-product-draft-create:") {
		t.Errorf("expected key prefix, got: %s", key)
	}
	// Same inputs → same key
	key2 := idempotency.DouyinProductDraftCreate("shop-1", "draft-1", "v2")
	if key != key2 {
		t.Errorf("expected deterministic key: %s != %s", key, key2)
	}
	// Different version → different key
	key3 := idempotency.DouyinProductDraftCreate("shop-1", "draft-1", "v3")
	if key == key3 {
		t.Error("different version should produce different key")
	}
}

func TestDouyinImageUpload(t *testing.T) {
	key := idempotency.DouyinImageUpload("shop-1", "images/foo.jpg", "abc123hash")
	if key == "" {
		t.Error("expected non-empty key")
	}
	if !strings.HasPrefix(key, "douyin-image-upload:") {
		t.Errorf("expected key prefix, got: %s", key)
	}
}

func TestAIProductApply(t *testing.T) {
	key := idempotency.AIProductApply("prod-1", "title", "task-1", "snaphash")
	if key == "" {
		t.Error("expected non-empty key")
	}
	if !strings.HasPrefix(key, "ai-product-apply:") {
		t.Errorf("expected key prefix, got: %s", key)
	}
	// Stable under whitespace trimming
	key2 := idempotency.AIProductApply(" prod-1 ", " title ", " task-1 ", " snaphash ")
	if key != key2 {
		t.Errorf("keys not equal after trim: %q vs %q", key, key2)
	}
}

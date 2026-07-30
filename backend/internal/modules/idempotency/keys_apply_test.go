package idempotency_test

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
)

func TestAIApplyKeyFormats(t *testing.T) {
	text := idempotency.AITextApply("b1", "i1", "p1", "v1")
	if text != "ai-text-apply:b1:i1:p1:v1" {
		t.Fatalf("AITextApply: %s", text)
	}
	if got := idempotency.LegacyAITextApply("b1", "i1", "v1"); got != "ai-text-apply:b1:i1:v1" {
		t.Fatalf("LegacyAITextApply: %s", got)
	}
	if got := idempotency.AITextUndo("app1", "v2"); got != "ai-text-undo:app1:v2" {
		t.Fatalf("AITextUndo: %s", got)
	}
	img := idempotency.AIImageApply("b1", "i1", "p1", "v1", "main")
	if img != "ai-image-apply:b1:i1:p1:v1:main" {
		t.Fatalf("AIImageApply: %s", img)
	}
	if got := idempotency.LegacyAIImageApply("b1", "i1", "v1"); got != "ai-image-apply:b1:i1:v1" {
		t.Fatalf("LegacyAIImageApply: %s", got)
	}
	if got := idempotency.AIImageUndo("app1", "v2"); got != "ai-image-undo:app1:v2" {
		t.Fatalf("AIImageUndo: %s", got)
	}
}

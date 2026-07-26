package taskcenter

import (
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
)

func TestTaskMergeCursorRoundTrip(t *testing.T) {
	filterFP, shopHash := taskCursorScope(ListFailureParams{TenantID: 7, ShopID: "shop-a"})
	payload := TaskMergeCursorPayload{
		TenantID:          7,
		ShopScopeHash:     shopHash,
		FilterFingerprint: filterFP,
		Sources: []TaskSourceCursor{
			{SourceID: TaskTypeCollect, SortTime: time.Now().UTC().Format(time.RFC3339Nano), ID: "a"},
			{SourceID: TaskTypeImage, Exhausted: true},
		},
	}
	raw, err := encodeTaskMergeCursor(payload)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := decodeTaskMergeCursor(raw, 7, shopHash, filterFP)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Sources) != 2 {
		t.Fatalf("sources=%d", len(got.Sources))
	}
}

func TestTaskMergeCursorRejectsTamper(t *testing.T) {
	filterFP, shopHash := taskCursorScope(ListFailureParams{TenantID: 1})
	raw, err := encodeTaskMergeCursor(TaskMergeCursorPayload{
		TenantID:          1,
		ShopScopeHash:     shopHash,
		FilterFingerprint: filterFP,
		Sources:           []TaskSourceCursor{{SourceID: TaskTypeCollect}},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := decodeTaskMergeCursor(raw+"x", 1, shopHash, filterFP); err == nil {
		t.Fatal("expected tamper rejection")
	}
}

func TestTaskMergeCursorRejectsCrossTenant(t *testing.T) {
	filterFP, shopHash := taskCursorScope(ListFailureParams{TenantID: 1})
	raw, err := encodeTaskMergeCursor(TaskMergeCursorPayload{
		TenantID:          1,
		ShopScopeHash:     shopHash,
		FilterFingerprint: filterFP,
		Sources:           []TaskSourceCursor{{SourceID: TaskTypeCollect}},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := decodeTaskMergeCursor(raw, 2, shopHash, filterFP); err == nil {
		t.Fatal("expected tenant mismatch")
	}
}

func TestTaskMergeCursorRejectsFilterMismatch(t *testing.T) {
	filterFP, shopHash := taskCursorScope(ListFailureParams{TenantID: 1, Status: "failed"})
	raw, err := encodeTaskMergeCursor(TaskMergeCursorPayload{
		TenantID:          1,
		ShopScopeHash:     shopHash,
		FilterFingerprint: filterFP,
		Sources:           []TaskSourceCursor{{SourceID: TaskTypeCollect}},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	otherFP, _ := taskCursorScope(ListFailureParams{TenantID: 1, Status: "retrying"})
	if _, err := decodeTaskMergeCursor(raw, 1, shopHash, otherFP); err == nil {
		t.Fatal("expected filter mismatch")
	}
}

func TestTaskMergeCursorRejectsUnknownSource(t *testing.T) {
	filterFP, shopHash := taskCursorScope(ListFailureParams{TenantID: 1})
	_, err := decodeTaskMergeCursor("", 1, shopHash, filterFP)
	if err != nil {
		t.Fatalf("empty cursor: %v", err)
	}
	raw, err := encodeTaskMergeCursor(TaskMergeCursorPayload{
		TenantID:          1,
		ShopScopeHash:     shopHash,
		FilterFingerprint: filterFP,
		Sources:           []TaskSourceCursor{{SourceID: "unknown_source"}},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := decodeTaskMergeCursor(raw, 1, shopHash, filterFP); err == nil {
		t.Fatal("expected unknown source rejection")
	}
}

func TestCompareMergeItemsDeterministic(t *testing.T) {
	now := time.Now().UTC()
	a := &mergeHeapItem{dto: UnifiedTaskDTO{SortKey: now, ID: "b"}, priority: 2}
	b := &mergeHeapItem{dto: UnifiedTaskDTO{SortKey: now, ID: "a"}, priority: 1}
	if compareMergeItems(a, b) >= 0 {
		t.Fatal("expected lower priority source to win tie")
	}
}

func TestValidateTaskMergeSourcesDuplicate(t *testing.T) {
	err := validateTaskMergeSources([]TaskSourceCursor{
		{SourceID: TaskTypeCollect},
		{SourceID: TaskTypeCollect},
	})
	if err == nil {
		t.Fatal("expected duplicate rejection")
	}
}

func TestEncodeSignedJSONMaxLength(t *testing.T) {
	big := make([]TaskSourceCursor, 20)
	for i := range big {
		big[i] = TaskSourceCursor{SourceID: TaskTypeCollect, ID: "x"}
	}
	_, err := encodeTaskMergeCursor(TaskMergeCursorPayload{
		TenantID: 1,
		Sources:  big,
	})
	if err == nil {
		t.Fatal("expected source count guard")
	}
	_ = pagination.MaxCursorLen
}

func TestApplyTaskSourceKeysetPreservesSortTime(t *testing.T) {
	payload := applyTaskSourceKeyset(TaskSourceCursor{
		SourceID: TaskTypeCollect,
		SortTime: "2026-07-14T04:27:33.818772Z",
		ID:       "81e91910-d553-4b52-9614-4f45ce191690",
	})
	if payload.SortValue != "2026-07-14T04:27:33.818772Z" {
		t.Fatalf("sort value=%q", payload.SortValue)
	}
	if payload.TieID != "81e91910-d553-4b52-9614-4f45ce191690" {
		t.Fatalf("tie=%q", payload.TieID)
	}
}

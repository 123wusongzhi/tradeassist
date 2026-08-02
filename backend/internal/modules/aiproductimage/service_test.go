package aiproductimage

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeImageOperationTypes(t *testing.T) {
	ops, err := normalizeOperationTypes([]string{"quality_check", "white_background", "quality_check"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(ops))
	}
	_, err = normalizeOperationTypes([]string{"invalid_op"})
	if err == nil {
		t.Fatal("expected error for invalid op")
	}
}

func TestBuildIdempotencyKeyStable(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	ops := []string{"quality_check", "white_background"}
	k1 := buildIdempotencyKey(nil, []uuid.UUID{id2, id1}, []uuid.UUID{id1}, ops, ImageGenerationOptions{Language: "en"})
	k2 := buildIdempotencyKey(nil, []uuid.UUID{id1, id2}, []uuid.UUID{id1}, ops, ImageGenerationOptions{Language: "en"})
	if k1 != k2 {
		t.Fatalf("idempotency key should be order-independent")
	}
}

func TestOperationToTaskType(t *testing.T) {
	if operationToTaskType(OpQualityCheck) == "" {
		t.Fatal("quality_check should map to task type")
	}
	if operationToTaskType(OpWhiteBackground) == "" {
		t.Fatal("white_background should map to task type")
	}
}

func TestCheckImageQualityWarnings(t *testing.T) {
	w := checkImageQualityWarnings("", false)
	if len(w) == 0 {
		t.Fatal("expected warnings for inaccessible empty url")
	}
}

func TestResolveGenerationTaskTypeWhiteBackground(t *testing.T) {
	if got := resolveGenerationTaskType("dashscope_image", OpWhiteBackground); got != "replace_background" {
		t.Fatalf("dashscope white bg should use replace_background, got %q", got)
	}
	if got := resolveGenerationTaskType("removebg", OpWhiteBackground); got != "remove_background" {
		t.Fatalf("removebg white bg should use remove_background, got %q", got)
	}
	if got := resolveGenerationTaskType("dashscope_image", OpQualityCheck); got != "score_image" {
		t.Fatalf("quality check unchanged, got %q", got)
	}
}

func TestClaimItemForGenerationDoesNotReviveCancelledItem(t *testing.T) {
	db := openImageApplyTestDB(t)
	item := AIProductImageItem{
		BatchID:       uuid.New(),
		ProductID:     uuid.New(),
		OperationType: OpQualityCheck,
		Status:        ItemPending,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}

	// A worker can read the pending item just before a cancel request wins.
	var observed AIProductImageItem
	if err := db.First(&observed, "id = ?", item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if observed.Status != ItemPending {
		t.Fatalf("worker observed %q, want pending", observed.Status)
	}
	res := db.Model(&AIProductImageItem{}).Where("id = ? AND status = ?", item.ID, ItemPending).Update("status", ItemCancelled)
	if res.Error != nil || res.RowsAffected != 1 {
		t.Fatalf("cancel pending item: rows=%d err=%v", res.RowsAffected, res.Error)
	}

	if (&Service{DB: db}).claimItemForGeneration(context.Background(), item.ID) {
		t.Fatal("worker claim must lose after cancellation")
	}
	var got AIProductImageItem
	if err := db.First(&got, "id = ?", item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != ItemCancelled {
		t.Fatalf("status = %q, want cancelled", got.Status)
	}
}

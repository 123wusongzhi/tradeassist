package pagination

import "testing"

func TestNormalizePageBounds(t *testing.T) {
	p, err := NormalizePage(1, 100000)
	if err != nil {
		t.Fatalf("NormalizePage unexpected error: %v", err)
	}
	if p.Limit != MaxLimit || !p.Truncated {
		t.Fatalf("expected max limit truncation, got %+v", p)
	}
	if _, err := NormalizePage(1000, 200); err == nil {
		t.Fatal("expected deep offset to be rejected")
	}
}

func TestCursorScope(t *testing.T) {
	cur, err := EncodeCursor(CursorPayload{
		TenantID:  7,
		ShopID:    "shop-a",
		SortField: "created_at",
		SortValue: "2026-07-13T00:00:00Z",
		TieID:     "row-a",
	})
	if err != nil {
		t.Fatalf("EncodeCursor: %v", err)
	}
	if _, err := DecodeCursor(cur, 8, "shop-a"); err == nil {
		t.Fatal("expected tenant mismatch")
	}
	if _, err := DecodeCursor(cur, 7, "shop-b"); err == nil {
		t.Fatal("expected shop mismatch")
	}
	p, err := DecodeCursor(cur, 7, "shop-a")
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if p.TieID != "row-a" {
		t.Fatalf("unexpected tie id %q", p.TieID)
	}
}

func TestCursorTamperRejected(t *testing.T) {
	cur, err := EncodeCursor(CursorPayload{
		TenantID:  7,
		ShopID:    "shop-a",
		SortField: "created_at",
		SortValue: "2026-07-13T00:00:00Z",
		TieID:     "row-a",
	})
	if err != nil {
		t.Fatalf("EncodeCursor: %v", err)
	}
	tampered := cur[:len(cur)-1] + "A"
	if tampered == cur {
		tampered = cur[:len(cur)-1] + "B"
	}
	if _, err := DecodeCursor(tampered, 7, "shop-a"); err == nil {
		t.Fatal("expected tampered cursor to be rejected")
	}
}

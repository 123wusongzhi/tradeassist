package product

import "testing"

func TestSourceCategoryFromRawNormalizesUnicodeLetters(t *testing.T) {
	key, name := SourceCategoryFromRaw([]byte(`{"categoryName":"Мебель / 桌子 🪑 42"}`))
	if key != "мебель桌子42" {
		t.Fatalf("unexpected source category key: %q", key)
	}
	if name != "Мебель / 桌子 🪑 42" {
		t.Fatalf("unexpected source category name: %q", name)
	}
}

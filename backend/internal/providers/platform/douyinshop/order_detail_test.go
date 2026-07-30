package douyinshop_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	douyinshop "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

func TestParseOrderDetailRaw_Fixture(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	fixturePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "testdata", "douyin", "order_detail.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("fixture not found at %s: %v", fixturePath, err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("fixture JSON unmarshal: %v", err)
	}
	// Simulate what parseOrderDetailRaw does (calling exported helper indirectly)
	detail, parseErr := douyinshop.ParseOrderDetailRawForTest(raw)
	if parseErr != nil {
		t.Fatalf("parseOrderDetailRaw: %v", parseErr)
	}
	if detail == nil {
		t.Fatal("expected non-nil detail")
	}
}

func TestParseOrderDetailRaw_NilInput(t *testing.T) {
	_, err := douyinshop.ParseOrderDetailRawForTest(nil)
	if err == nil {
		t.Error("expected error for nil input")
	}
}

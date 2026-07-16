package p7diag

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultOffDoesNotCreateWriter(t *testing.T) {
	resetForTest(t)
	t.Setenv("P7_DIAGNOSTICS_ENABLED", "")

	ObserveStage(RouteWebhookIngestion, "total", OutcomeSuccess, time.Now())
	SnapshotRuntime()

	if Enabled() {
		t.Fatal("diagnostics should default off")
	}
	if WriterCreated() {
		t.Fatal("writer should not be created when diagnostics are disabled")
	}
}

func TestFixedStageAndNoHighCardinalityLabels(t *testing.T) {
	dir := t.TempDir()
	resetForTest(t)
	t.Setenv("P7_DIAGNOSTICS_ENABLED", "true")
	t.Setenv("P7_DIAGNOSTIC_DIR", dir)
	t.Setenv("P7_DIAGNOSTIC_RUN_ID", "p7v2-diag-test")
	t.Setenv("P7_DIAGNOSTIC_ROLE", RoleBaseline)

	start := time.Now().Add(-time.Millisecond)
	ObserveStage(RouteWebhookIngestion, "event_insert", OutcomeSuccess, start)
	ObserveStage(RouteWebhookIngestion, "not_allowed", OutcomeSuccess, start)
	Path(RouteAuthInvalidLogin, "wrong_password")
	Shutdown(context.Background())

	events := readEvents(t, filepath.Join(dir, "p7v2-diag-test.jsonl"))
	stageEvents := 0
	for _, ev := range events {
		if ev["type"] == "stage_duration" || ev["type"] == "path_type" {
			stageEvents++
		}
	}
	if stageEvents != 2 {
		t.Fatalf("expected two accepted route/path events, got %d from %d total events", stageEvents, len(events))
	}
	for _, ev := range events {
		for _, forbidden := range []string{"requestId", "traceId", "userId", "email", "username", "shopId", "orderId", "eventId", "databaseName", "pid", "url", "error"} {
			if _, ok := ev[forbidden]; ok {
				t.Fatalf("event contains high-cardinality key %q: %#v", forbidden, ev)
			}
		}
		if ev["diagnostic_role"] != RoleBaseline {
			t.Fatalf("unexpected diagnostic role: %#v", ev["diagnostic_role"])
		}
	}
}

func TestBufferDropIsNonBlocking(t *testing.T) {
	dir := t.TempDir()
	resetForTest(t)
	t.Setenv("P7_DIAGNOSTICS_ENABLED", "true")
	t.Setenv("P7_DIAGNOSTIC_DIR", dir)
	t.Setenv("P7_DIAGNOSTIC_RUN_ID", "p7v2-diag-drop")
	t.Setenv("P7_DIAGNOSTIC_BUFFER", "1")

	start := time.Now().Add(-time.Millisecond)
	for i := 0; i < 10000; i++ {
		ObserveStage(RouteAuthInvalidLogin, "total", OutcomeExpectedRejection, start)
	}
	if DroppedDiagnosticEventCount() == 0 {
		t.Fatal("expected dropped diagnostics when buffer is saturated")
	}
	Shutdown(context.Background())
}

func TestDBSnapshotDeltasAreNonNegative(t *testing.T) {
	resetForTest(t)
	t.Setenv("P7_DIAGNOSTICS_ENABLED", "true")
	t.Setenv("P7_DIAGNOSTIC_DIR", t.TempDir())
	t.Setenv("P7_DIAGNOSTIC_RUN_ID", "p7v2-diag-db")

	SnapshotDB(&sql.DB{})
	Shutdown(context.Background())

	if DroppedDiagnosticEventCount() != 0 {
		t.Fatal("db snapshot should not require dropping events in empty test")
	}
}

func TestRuntimeSamplerStops(t *testing.T) {
	resetForTest(t)
	t.Setenv("P7_DIAGNOSTICS_ENABLED", "true")
	t.Setenv("P7_DIAGNOSTIC_DIR", t.TempDir())
	t.Setenv("P7_DIAGNOSTIC_RUN_ID", "p7v2-diag-runtime")
	t.Setenv("P7_DIAGNOSTIC_RUNTIME_SNAPSHOT_INTERVAL_MS", "1")

	ObserveStage(RouteWebhookIngestion, "total", OutcomeSuccess, time.Now().Add(-time.Millisecond))
	Shutdown(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if samplerStop != nil || samplerDone != nil || writer != nil {
		t.Fatal("diagnostic sampler/writer was not released")
	}
}

func resetForTest(t *testing.T) {
	t.Helper()
	Shutdown(context.Background())
	startedAt = time.Now().UTC()
	seq.Store(0)
	drops.Store(0)
	writerOpened.Store(false)
	setPreviousDBStats(0, 0)
}

func readEvents(t *testing.T, path string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatal(err)
		}
		out = append(out, ev)
	}
	return out
}

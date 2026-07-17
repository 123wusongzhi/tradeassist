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

func TestFormalPerformanceModeDisablesDiagnosticWriter(t *testing.T) {
	dir := t.TempDir()
	resetForTest(t)
	t.Setenv("APP_ENV", "performance")
	t.Setenv("formal", "true")
	t.Setenv("diagnosticOnly", "false")
	t.Setenv("P7_DIAGNOSTICS_ENABLED", "true")
	t.Setenv("P7_DIAGNOSTIC_DIR", dir)
	t.Setenv("P7_DIAGNOSTIC_RUN_ID", "p7v2-formal")

	ObserveStage(RouteAuthInvalidLogin, "total", OutcomeExpectedRejection, time.Now().Add(-time.Millisecond))
	SnapshotRuntime()
	Shutdown(context.Background())

	if Enabled() {
		t.Fatal("formal performance mode must disable diagnostics")
	}
	if WriterCreated() {
		t.Fatal("writer should not start in formal performance mode")
	}
	if _, err := os.Stat(filepath.Join(dir, "p7v2-formal.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("formal diagnostics file state = %v, want not exists", err)
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

func TestSQLFingerprintAndPasswordVerifyEvents(t *testing.T) {
	dir := t.TempDir()
	resetForTest(t)
	t.Setenv("P7_DIAGNOSTICS_ENABLED", "true")
	t.Setenv("P7_DIAGNOSTIC_DIR", dir)
	t.Setenv("P7_DIAGNOSTIC_RUN_ID", "p7v2-diag-sql")
	t.Setenv("P7_DIAGNOSTIC_ROLE", RoleCurrent)

	ObserveSQL(RouteAuthInvalidLogin, "auth", "auth.operation_log_insert", "insert", "operation_logs", OutcomeSuccess, true, SQLTiming{
		ConnectionAcquireMs: 0.5,
		QueryExecutionMs:    3.2,
		RowsAffected:        1,
		TransactionState:    "open",
	})
	ObservePasswordVerify(PathKnownWrongPassword, PasswordAlgoBcrypt, 10, 1, time.Now().Add(-20*time.Millisecond))
	Path(RouteWebhookIngestion, "normal_insert")
	Shutdown(context.Background())

	events := readEvents(t, filepath.Join(dir, "p7v2-diag-sql.jsonl"))
	var sawSQL, sawPwd, sawPath bool
	for _, ev := range events {
		blob, _ := json.Marshal(ev)
		s := string(blob)
		for _, leak := range []string{"password=", "sk-", "@example", "SELECT * FROM", "userId", "eventId"} {
			if strings.Contains(s, leak) {
				t.Fatalf("diagnostic event leaked sensitive/high-cardinality content %q: %s", leak, s)
			}
		}
		if ev["type"] == "sql_fingerprint" {
			sawSQL = true
			sqlObj, _ := ev["sql"].(map[string]any)
			if sqlObj["queryFingerprint"] == nil || sqlObj["operation"] == nil {
				t.Fatalf("sql fingerprint missing fields: %#v", ev)
			}
		}
		if ev["type"] == "password_verify" {
			sawPwd = true
		}
		if ev["type"] == "path_type" && ev["pathType"] == "normal_insert" {
			sawPath = true
		}
	}
	if !sawSQL || !sawPwd || !sawPath {
		t.Fatalf("missing expected events sql=%v pwd=%v path=%v from %d events", sawSQL, sawPwd, sawPath, len(events))
	}
}

func TestPGSnapshotSchemaWhenDBNil(t *testing.T) {
	resetForTest(t)
	t.Setenv("P7_DIAGNOSTICS_ENABLED", "true")
	t.Setenv("P7_DIAGNOSTIC_DIR", t.TempDir())
	t.Setenv("P7_DIAGNOSTIC_RUN_ID", "p7v2-diag-pg")
	BindSamplingDB(nil)
	SnapshotPG()
	Shutdown(context.Background())
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

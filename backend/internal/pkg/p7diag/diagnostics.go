package p7diag

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

const (
	MetricRequestStageDuration       = "p7_diag_request_stage_duration_ms"
	MetricDBOperationDuration        = "p7_diag_db_operation_duration_ms"
	MetricDBPoolWait                 = "p7_diag_db_pool_wait_ms"
	MetricTransactionDuration        = "p7_diag_transaction_duration_ms"
	MetricTransactionCommitDuration  = "p7_diag_transaction_commit_duration_ms"
	MetricAuditWriteDuration         = "p7_diag_audit_write_duration_ms"
	MetricRuntimeSnapshot            = "p7_diag_runtime_snapshot"
	MetricRequestCount               = "p7_diag_request_count"
	RouteWebhookIngestion            = "webhook_ingestion"
	RouteAuthInvalidLogin            = "auth_invalid_login"
	OutcomeSuccess                   = "success"
	OutcomeExpectedRejection         = "expected_rejection"
	OutcomeError                     = "error"
	RoleBaseline                     = "baseline"
	RoleCurrent                      = "current"
	DefaultDiagnosticDir             = "artifacts/p7-v2-diagnostics"
	defaultBufferSize                = 4096
	defaultRuntimeSnapshotIntervalMS = 1000
)

var (
	startedAt = time.Now().UTC()
	seq       atomic.Uint64
	drops     atomic.Uint64

	mu           sync.Mutex
	writer       *asyncWriter
	writerOpened atomic.Bool
	samplerStop  chan struct{}
	samplerDone  chan struct{}
)

// Event is the low-cardinality JSONL diagnostic record.
type Event struct {
	DiagnosticMode     string         `json:"diagnosticMode"`
	DiagnosticRunID    string         `json:"diagnosticRunId,omitempty"`
	DiagnosticSequence uint64         `json:"diagnosticSequence"`
	DiagnosticRole     string         `json:"diagnostic_role"`
	OffsetMs           int64          `json:"offsetMs"`
	Metric             string         `json:"metric"`
	Type               string         `json:"type"`
	Route              string         `json:"route,omitempty"`
	Stage              string         `json:"stage,omitempty"`
	Operation          string         `json:"operation,omitempty"`
	Outcome            string         `json:"outcome,omitempty"`
	PathType           string         `json:"pathType,omitempty"`
	DurationMs         float64        `json:"durationMs,omitempty"`
	Value              float64        `json:"value,omitempty"`
	Runtime            *RuntimeFields `json:"runtime,omitempty"`
	DB                 *DBFields      `json:"db,omitempty"`
}

type RuntimeFields struct {
	TimestampOffsetMs int64  `json:"timestampOffsetMs"`
	Goroutines        int    `json:"goroutines"`
	HeapAllocBytes    uint64 `json:"heapAllocBytes"`
	HeapObjects       uint64 `json:"heapObjects"`
	GCCycles          uint32 `json:"gcCycles"`
	GCPauseTotalNs    uint64 `json:"gcPauseTotalNs"`
	LastGCPauseNs     uint64 `json:"lastGcPauseNs"`
	GOMAXPROCS        int    `json:"GOMAXPROCS"`
	GOGC              string `json:"GOGC"`
	GOMEMLIMIT        string `json:"GOMEMLIMIT"`
	GoVersion         string `json:"GoVersion"`
}

type DBFields struct {
	OpenConnections    int     `json:"dbOpenConnections"`
	InUseConnections   int     `json:"dbInUseConnections"`
	IdleConnections    int     `json:"dbIdleConnections"`
	WaitCount          int64   `json:"dbWaitCount"`
	WaitDurationMs     float64 `json:"dbWaitDurationMs"`
	MaxOpenConnections int     `json:"dbMaxOpenConnections"`
	MaxIdleConnections int64   `json:"dbMaxIdleConnections"`
	WaitCountDelta     int64   `json:"waitCountDelta"`
	WaitDurationDelta  float64 `json:"waitDurationDeltaMs"`
}

type asyncWriter struct {
	file *os.File
	ch   chan []byte
	done chan struct{}
}

// Enabled returns true only for the explicit P7 diagnostics switch.
func Enabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("P7_DIAGNOSTICS_ENABLED")), "true")
}

// WriterCreated reports whether enabling diagnostics opened a local writer.
func WriterCreated() bool {
	return writerOpened.Load()
}

// DroppedDiagnosticEventCount returns the number of non-blocking writer drops.
func DroppedDiagnosticEventCount() uint64 {
	return drops.Load()
}

// ObserveStage records one fixed route/stage duration. It is a no-op when disabled.
func ObserveStage(route, stage, outcome string, started time.Time) {
	if !Enabled() || started.IsZero() || !validRoute(route) || !validStage(route, stage) || !validOutcome(outcome) {
		return
	}
	metric := MetricRequestStageDuration
	if stage == "transaction_commit" {
		metric = MetricTransactionCommitDuration
	}
	if stage == "transaction_begin" {
		metric = MetricTransactionDuration
	}
	emit(Event{
		Metric:     metric,
		Type:       "stage_duration",
		Route:      route,
		Stage:      stage,
		Outcome:    outcome,
		DurationMs: durationMs(time.Since(started)),
	})
}

// ObserveDBOperation records a fixed DB operation duration.
func ObserveDBOperation(route, operation, outcome string, started time.Time) {
	if !Enabled() || started.IsZero() || !validRoute(route) || !validOperation(operation) || !validOutcome(outcome) {
		return
	}
	emit(Event{
		Metric:     MetricDBOperationDuration,
		Type:       "db_operation_duration",
		Route:      route,
		Operation:  operation,
		Outcome:    outcome,
		DurationMs: durationMs(time.Since(started)),
	})
}

// ObserveAuditWrite records a fixed audit/operation-log write duration.
func ObserveAuditWrite(route, operation, outcome string, started time.Time) {
	if !Enabled() || started.IsZero() || !validRoute(route) || !validOperation(operation) || !validOutcome(outcome) {
		return
	}
	emit(Event{
		Metric:     MetricAuditWriteDuration,
		Type:       "audit_write_duration",
		Route:      route,
		Operation:  operation,
		Outcome:    outcome,
		DurationMs: durationMs(time.Since(started)),
	})
}

// Count records a fixed diagnostic counter as a local event.
func Count(route, operation string, n int) {
	if !Enabled() || n <= 0 || !validRoute(route) || !validOperation(operation) {
		return
	}
	emit(Event{
		Metric:    MetricRequestCount,
		Type:      "counter",
		Route:     route,
		Operation: operation,
		Outcome:   OutcomeSuccess,
		Value:     float64(n),
	})
}

// Path records the fixed invalid-login path type without exposing account data.
func Path(route, pathType string) {
	if !Enabled() || route != RouteAuthInvalidLogin || !validPathType(pathType) {
		return
	}
	emit(Event{
		Metric:   MetricRequestCount,
		Type:     "path_type",
		Route:    route,
		Outcome:  OutcomeExpectedRejection,
		PathType: pathType,
		Value:    1,
	})
}

// SnapshotRuntime records a fixed-field runtime snapshot.
func SnapshotRuntime() {
	if !Enabled() {
		return
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	lastPause := uint64(0)
	if ms.NumGC > 0 {
		lastPause = ms.PauseNs[(ms.NumGC+255)%256]
	}
	emit(Event{
		Metric:  MetricRuntimeSnapshot,
		Type:    "runtime_snapshot",
		Outcome: OutcomeSuccess,
		Runtime: &RuntimeFields{
			TimestampOffsetMs: offsetMs(),
			Goroutines:        runtime.NumGoroutine(),
			HeapAllocBytes:    ms.HeapAlloc,
			HeapObjects:       ms.HeapObjects,
			GCCycles:          ms.NumGC,
			GCPauseTotalNs:    ms.PauseTotalNs,
			LastGCPauseNs:     lastPause,
			GOMAXPROCS:        runtime.GOMAXPROCS(0),
			GOGC:              strings.TrimSpace(os.Getenv("GOGC")),
			GOMEMLIMIT:        strings.TrimSpace(os.Getenv("GOMEMLIMIT")),
			GoVersion:         runtime.Version(),
		},
	})
}

// SnapshotDB records safe sql.DB pool counters and nonnegative wait deltas.
func SnapshotDB(db *sql.DB) {
	if !Enabled() || db == nil {
		return
	}
	st := db.Stats()
	prevWaitCount, prevWaitDuration := previousDBStats()
	waitCountDelta := st.WaitCount - prevWaitCount
	if waitCountDelta < 0 {
		waitCountDelta = 0
	}
	waitDurationDelta := st.WaitDuration - prevWaitDuration
	if waitDurationDelta < 0 {
		waitDurationDelta = 0
	}
	setPreviousDBStats(st.WaitCount, st.WaitDuration)
	emit(Event{
		Metric:  MetricDBPoolWait,
		Type:    "db_pool_snapshot",
		Outcome: OutcomeSuccess,
		DB: &DBFields{
			OpenConnections:    st.OpenConnections,
			InUseConnections:   st.InUse,
			IdleConnections:    st.Idle,
			WaitCount:          st.WaitCount,
			WaitDurationMs:     durationMs(st.WaitDuration),
			MaxOpenConnections: st.MaxOpenConnections,
			MaxIdleConnections: st.MaxIdleClosed,
			WaitCountDelta:     waitCountDelta,
			WaitDurationDelta:  durationMs(waitDurationDelta),
		},
	})
}

// SnapshotGormDB extracts sql.DB stats from GORM without exposing connection strings.
func SnapshotGormDB(db *gorm.DB) {
	if !Enabled() || db == nil {
		return
	}
	raw, err := db.DB()
	if err != nil {
		return
	}
	SnapshotDB(raw)
}

var dbStats struct {
	mu           sync.Mutex
	waitCount    int64
	waitDuration time.Duration
}

func previousDBStats() (int64, time.Duration) {
	dbStats.mu.Lock()
	defer dbStats.mu.Unlock()
	return dbStats.waitCount, dbStats.waitDuration
}

func setPreviousDBStats(waitCount int64, waitDuration time.Duration) {
	dbStats.mu.Lock()
	dbStats.waitCount = waitCount
	dbStats.waitDuration = waitDuration
	dbStats.mu.Unlock()
}

func emit(ev Event) {
	w := ensureWriter()
	if w == nil {
		return
	}
	ev.DiagnosticMode = "p7_low_cardinality"
	ev.DiagnosticRunID = strings.TrimSpace(os.Getenv("P7_DIAGNOSTIC_RUN_ID"))
	ev.DiagnosticRole = normalizeRole(os.Getenv("P7_DIAGNOSTIC_ROLE"))
	ev.DiagnosticSequence = seq.Add(1)
	ev.OffsetMs = offsetMs()
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	b = append(b, '\n')
	select {
	case w.ch <- b:
	default:
		drops.Add(1)
	}
}

func ensureWriter() *asyncWriter {
	if !Enabled() {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if writer != nil {
		return writer
	}
	dir := strings.TrimSpace(os.Getenv("P7_DIAGNOSTIC_DIR"))
	if dir == "" {
		dir = DefaultDiagnosticDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	runID := strings.TrimSpace(os.Getenv("P7_DIAGNOSTIC_RUN_ID"))
	if runID == "" {
		runID = "p7v2-diag-local"
	}
	f, err := os.OpenFile(filepath.Join(dir, safeFileName(runID)+".jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil
	}
	size := defaultBufferSize
	if raw := strings.TrimSpace(os.Getenv("P7_DIAGNOSTIC_BUFFER")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			size = n
		}
	}
	writer = &asyncWriter{file: f, ch: make(chan []byte, size), done: make(chan struct{})}
	writerOpened.Store(true)
	go writer.run()
	startRuntimeSamplerLocked()
	return writer
}

func (w *asyncWriter) run() {
	defer close(w.done)
	for b := range w.ch {
		_, _ = w.file.Write(b)
	}
	_ = w.file.Sync()
	_ = w.file.Close()
}

func startRuntimeSamplerLocked() {
	if samplerStop != nil {
		return
	}
	interval := time.Duration(defaultRuntimeSnapshotIntervalMS) * time.Millisecond
	if raw := strings.TrimSpace(os.Getenv("P7_DIAGNOSTIC_RUNTIME_SNAPSHOT_INTERVAL_MS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			interval = time.Duration(n) * time.Millisecond
		}
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	samplerStop = stop
	samplerDone = done
	go func() {
		defer close(done)
		t := time.NewTicker(interval)
		defer t.Stop()
		SnapshotRuntime()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				SnapshotRuntime()
			}
		}
	}()
}

// Shutdown flushes diagnostics. It is safe to call when disabled.
func Shutdown(ctx context.Context) {
	mu.Lock()
	stop := samplerStop
	done := samplerDone
	samplerStop = nil
	samplerDone = nil
	mu.Unlock()
	if stop != nil {
		close(stop)
		if done != nil {
			select {
			case <-done:
			case <-ctx.Done():
			}
		}
	}
	mu.Lock()
	w := writer
	writer = nil
	mu.Unlock()
	if w != nil {
		close(w.ch)
		select {
		case <-w.done:
		case <-ctx.Done():
		}
	}
}

func durationMs(d time.Duration) float64 {
	if d < 0 {
		return 0
	}
	return float64(d.Nanoseconds()) / float64(time.Millisecond)
}

func offsetMs() int64 {
	return time.Since(startedAt).Milliseconds()
}

func safeFileName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "p7v2-diag-local"
	}
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "p7v2-diag-local"
	}
	return b.String()
}

func normalizeRole(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case RoleBaseline:
		return RoleBaseline
	case RoleCurrent:
		return RoleCurrent
	default:
		return "unknown"
	}
}

func validRoute(v string) bool {
	return v == RouteWebhookIngestion || v == RouteAuthInvalidLogin
}

func validOutcome(v string) bool {
	return v == OutcomeSuccess || v == OutcomeExpectedRejection || v == OutcomeError
}

func validPathType(v string) bool {
	switch v {
	case "account_missing", "wrong_password", "locked_account", "rate_limited":
		return true
	default:
		return false
	}
}

func validStage(route, stage string) bool {
	if stage == "" {
		return false
	}
	if route == RouteWebhookIngestion {
		switch stage {
		case "request_read", "signature_verify", "json_decode", "shop_provider_resolve", "event_insert", "duplicate_event_reload", "idempotency_check", "transaction_begin", "business_upsert", "inventory_update", "task_enqueue", "operation_log", "transaction_commit", "response_encode", "total":
			return true
		}
	}
	if route == RouteAuthInvalidLogin {
		switch stage {
		case "request_read", "json_decode", "input_normalize", "account_lookup", "password_verify", "invalid_decision", "failed_attempt_read", "failed_attempt_write", "lockout_evaluate", "rate_limit_check", "security_audit", "operation_log", "transaction_begin", "transaction_commit", "response_encode", "total":
			return true
		}
	}
	return false
}

func validOperation(v string) bool {
	switch v {
	case "observedDbEntryLatency", "event_insert", "duplicate_event_reload", "idempotency_check", "account_lookup", "password_verify", "failed_attempt_read", "failed_attempt_write", "security_audit", "operation_log", "normalInsertCount", "duplicateConflictCount", "duplicateReloadCount", "businessUpsertCount", "taskEnqueueCount", "operationLogCount", "accountLookupCount", "passwordVerifyCount", "failedAttemptReadCount", "failedAttemptWriteCount", "securityAuditWriteCount", "operationLogWriteCount":
		return true
	default:
		return false
	}
}

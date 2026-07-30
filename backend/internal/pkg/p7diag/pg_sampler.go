package p7diag

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	MetricPGWaitSnapshot = "p7_diag_pg_wait_snapshot"
	defaultPGSampleMS    = 1000
)

// PGFields are low-cardinality PostgreSQL wait/lock samples (no SQL text / PIDs / user data).
type PGFields struct {
	ActiveConnectionCount  int    `json:"activeConnectionCount"`
	IdleConnectionCount    int    `json:"idleConnectionCount"`
	IdleInTransactionCount int    `json:"idleInTransactionCount"`
	WaitingConnectionCount int    `json:"waitingConnectionCount"`
	BlockedConnectionCount int    `json:"blockedConnectionCount"`
	LongTransactionCount   int    `json:"longTransactionCount"`
	WaitEventTypeTop       string `json:"waitEventTypeTop,omitempty"`
	WaitEventTop           string `json:"waitEventTop,omitempty"`
	LockWaitCount          int    `json:"lockWaitCount"`
	PgStatStatementsAvail  bool   `json:"pgStatStatementsAvailable"`
	StatementsSampled      bool   `json:"statementsSampled"`
}

var (
	pgMu                sync.Mutex
	pgDB                *sql.DB
	pgStatStmtKnown     bool
	pgStatStmtAvail     bool
	pgSamplerStop       chan struct{}
	pgSamplerDone       chan struct{}
	longTxnThresholdSec = 5.0
)

// BindSamplingDB attaches a *sql.DB for low-frequency PG wait/lock sampling.
// Safe to call when diagnostics are disabled (no-op until enabled writer starts).
func BindSamplingDB(db *sql.DB) {
	pgMu.Lock()
	defer pgMu.Unlock()
	pgDB = db
}

// SnapshotPG samples PostgreSQL activity/locks once. Never logs SQL text or backend PIDs.
func SnapshotPG() {
	if !Enabled() {
		return
	}
	pgMu.Lock()
	db := pgDB
	pgMu.Unlock()
	if db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()

	fields := PGFields{PgStatStatementsAvail: pgStatStatementsAvailable(ctx, db)}
	row := db.QueryRowContext(ctx, `
SELECT
  COUNT(*) FILTER (WHERE state = 'active')::int,
  COUNT(*) FILTER (WHERE state = 'idle')::int,
  COUNT(*) FILTER (WHERE state = 'idle in transaction')::int,
  COUNT(*) FILTER (WHERE wait_event_type IS NOT NULL AND state = 'active')::int,
  COUNT(*) FILTER (WHERE EXTRACT(EPOCH FROM (now() - xact_start)) > $1)::int
FROM pg_stat_activity
WHERE datname = current_database()
  AND pid <> pg_backend_pid()
`, longTxnThresholdSec)
	_ = row.Scan(
		&fields.ActiveConnectionCount,
		&fields.IdleConnectionCount,
		&fields.IdleInTransactionCount,
		&fields.WaitingConnectionCount,
		&fields.LongTransactionCount,
	)

	_ = db.QueryRowContext(ctx, `
SELECT COUNT(*)::int
FROM pg_locks
WHERE NOT granted
`).Scan(&fields.LockWaitCount)
	fields.BlockedConnectionCount = fields.LockWaitCount

	var waitType, waitEvent string
	_ = db.QueryRowContext(ctx, `
SELECT COALESCE(wait_event_type, ''), COALESCE(wait_event, '')
FROM pg_stat_activity
WHERE datname = current_database()
  AND pid <> pg_backend_pid()
  AND wait_event_type IS NOT NULL
GROUP BY wait_event_type, wait_event
ORDER BY COUNT(*) DESC
LIMIT 1
`).Scan(&waitType, &waitEvent)
	fields.WaitEventTypeTop = normalizeWaitLabel(waitType)
	fields.WaitEventTop = normalizeWaitLabel(waitEvent)

	if fields.PgStatStatementsAvail {
		// Read-only existence probe only; do not export query text.
		var n int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*)::int FROM pg_stat_statements`).Scan(&n); err == nil {
			fields.StatementsSampled = true
		}
	}

	emit(Event{
		Metric:  MetricPGWaitSnapshot,
		Type:    "pg_wait_snapshot",
		Outcome: OutcomeSuccess,
		PG:      &fields,
	})
}

func pgStatStatementsAvailable(ctx context.Context, db *sql.DB) bool {
	pgMu.Lock()
	defer pgMu.Unlock()
	if pgStatStmtKnown {
		return pgStatStmtAvail
	}
	var one int
	err := db.QueryRowContext(ctx, `SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements' LIMIT 1`).Scan(&one)
	pgStatStmtKnown = true
	pgStatStmtAvail = err == nil
	return pgStatStmtAvail
}

func normalizeWaitLabel(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	// Keep only coarse PostgreSQL wait labels (letters/underscore).
	var b strings.Builder
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > 64 {
		return out[:64]
	}
	return out
}

func startPGSamplerLocked() {
	if pgSamplerStop != nil {
		return
	}
	interval := time.Duration(defaultPGSampleMS) * time.Millisecond
	if raw := strings.TrimSpace(os.Getenv("P7_DIAGNOSTIC_PG_SAMPLE_INTERVAL_MS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 500 {
			interval = time.Duration(n) * time.Millisecond
		}
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	pgSamplerStop = stop
	pgSamplerDone = done
	go func() {
		defer close(done)
		t := time.NewTicker(interval)
		defer t.Stop()
		SnapshotPG()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				SnapshotPG()
			}
		}
	}()
}

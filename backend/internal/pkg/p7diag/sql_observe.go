package p7diag

import (
	"database/sql"
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	MetricDBConnectionAcquireDuration = "p7_diag_db_connection_acquire_duration"
	MetricDBQueryDuration             = "p7_diag_db_query_duration"
	MetricDBTransactionDuration       = "p7_diag_db_transaction_duration"
	MetricDBCommitDuration            = "p7_diag_db_commit_duration"
	MetricSQLFingerprint              = "p7_diag_sql_fingerprint"
)

// SQLTiming carries per-operation acquire/query timing without SQL text or identifiers.
type SQLTiming struct {
	ConnectionAcquireMs float64 `json:"connectionAcquireMs,omitempty"`
	QueryExecutionMs    float64 `json:"queryExecutionMs,omitempty"`
	TransactionMs       float64 `json:"transactionMs,omitempty"`
	CommitMs            float64 `json:"commitMs,omitempty"`
	RowsAffected        int64   `json:"rowsAffected,omitempty"`
	QueryError          string  `json:"queryError,omitempty"`
	TransactionState    string  `json:"transactionState,omitempty"`
}

// ObserveSQL records a fixed-operation SQL fingerprint timing event.
func ObserveSQL(route, module, operation, queryKind, tableGroup, outcome string, transactional bool, timing SQLTiming) {
	if !Enabled() || !validRoute(route) || !validModule(module) || !validSQLOperation(operation) || !validQueryKind(queryKind) || !validOutcome(outcome) {
		return
	}
	fp := FingerprintFromParts(module, operation, queryKind, tableGroup, transactional)
	timing.TransactionState = normalizeTransactionState(timing.TransactionState)
	if timing.QueryError != "" {
		switch timing.QueryError {
		case "unique_violation", "deadlock", "timeout", "connection_error", "not_found", "query_error", "expected_conflict":
		default:
			timing.QueryError = "query_error"
		}
	}
	// Fingerprint call counts only real query executions (not commit-only envelopes).
	if timing.QueryExecutionMs > 0 || timing.RowsAffected != 0 || timing.QueryError != "" {
		emit(Event{
			Metric:     MetricSQLFingerprint,
			Type:       "sql_fingerprint",
			Route:      route,
			Operation:  operation,
			Outcome:    outcome,
			DurationMs: timing.QueryExecutionMs,
			SQL:        &fp,
			SQLTiming:  &timing,
		})
	}
	if timing.ConnectionAcquireMs > 0 {
		emit(Event{
			Metric:     MetricDBConnectionAcquireDuration,
			Type:       "db_connection_acquire",
			Route:      route,
			Operation:  operation,
			Outcome:    outcome,
			DurationMs: timing.ConnectionAcquireMs,
			SQL:        &fp,
		})
	}
	if timing.QueryExecutionMs > 0 {
		emit(Event{
			Metric:     MetricDBQueryDuration,
			Type:       "db_query_duration",
			Route:      route,
			Operation:  operation,
			Outcome:    outcome,
			DurationMs: timing.QueryExecutionMs,
			SQL:        &fp,
		})
	}
	if timing.TransactionMs > 0 {
		emit(Event{
			Metric:     MetricDBTransactionDuration,
			Type:       "db_transaction_duration",
			Route:      route,
			Operation:  operation,
			Outcome:    outcome,
			DurationMs: timing.TransactionMs,
			SQL:        &fp,
		})
	}
	if timing.CommitMs > 0 {
		emit(Event{
			Metric:     MetricDBCommitDuration,
			Type:       "db_commit_duration",
			Route:      route,
			Operation:  operation,
			Outcome:    outcome,
			DurationMs: timing.CommitMs,
			SQL:        &fp,
		})
	}
}

// TimedGorm runs fn while approximating connection-acquire wait from sql.DBStats deltas.
func TimedGorm(db *gorm.DB, fn func() error) (SQLTiming, error) {
	timing := SQLTiming{TransactionState: "none"}
	if db == nil {
		err := fn()
		return timing, err
	}
	raw, err := db.DB()
	var before sql.DBStats
	if err == nil && raw != nil {
		before = raw.Stats()
	}
	start := time.Now()
	callErr := fn()
	timing.QueryExecutionMs = durationMs(time.Since(start))
	if err == nil && raw != nil {
		after := raw.Stats()
		waitDelta := after.WaitDuration - before.WaitDuration
		if waitDelta > 0 {
			timing.ConnectionAcquireMs = durationMs(waitDelta)
		}
	}
	if callErr != nil && !errors.Is(callErr, gorm.ErrRecordNotFound) {
		timing.QueryError = normalizeQueryError(callErr)
	}
	return timing, callErr
}

// TimedGormRows is TimedGorm plus rowsAffected.
func TimedGormRows(db *gorm.DB, fn func() (int64, error)) (SQLTiming, error) {
	var rows int64
	timing, err := TimedGorm(db, func() error {
		var callErr error
		rows, callErr = fn()
		return callErr
	})
	timing.RowsAffected = rows
	return timing, err
}

package observability

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// StartDBStatsCollector periodically records sql.DB runtime stats.
func StartDBStatsCollector(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, db *sql.DB, cat *metrics.Catalog, dbRole string, interval time.Duration) {
	if wg == nil || db == nil || cat == nil {
		return
	}
	role := normalizeDBRole(dbRole)
	if interval <= 0 {
		interval = 15 * time.Second
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(interval)
		defer tick.Stop()
		var prevWaitCount int64
		var prevWaitDuration time.Duration
		record := func() {
			st := db.Stats()
			if cat.DBConnectionsOpen != nil {
				cat.DBConnectionsOpen.WithLabelValues(role).Set(float64(st.OpenConnections))
			}
			if cat.DBConnectionsInUse != nil {
				cat.DBConnectionsInUse.WithLabelValues(role).Set(float64(st.InUse))
			}
			if cat.DBConnectionsIdle != nil {
				cat.DBConnectionsIdle.WithLabelValues(role).Set(float64(st.Idle))
			}
			if cat.DBMaxOpenConnections != nil {
				cat.DBMaxOpenConnections.WithLabelValues(role).Set(float64(st.MaxOpenConnections))
			}
			if st.WaitCount > prevWaitCount && cat.DBConnectionWaitCount != nil {
				cat.DBConnectionWaitCount.WithLabelValues(role).Add(float64(st.WaitCount - prevWaitCount))
			}
			if st.WaitDuration > prevWaitDuration && cat.DBConnectionWaitDuration != nil {
				cat.DBConnectionWaitDuration.WithLabelValues(role).Add((st.WaitDuration - prevWaitDuration).Seconds())
			}
			prevWaitCount = st.WaitCount
			prevWaitDuration = st.WaitDuration
		}
		record()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				record()
				if log != nil {
					log.Debug("db_stats_collected", "db_role", role)
				}
			}
		}
	}()
}

// InstrumentedDB wraps sql.DB query and transaction calls with metrics/traces.
type InstrumentedDB struct {
	DB                 *sql.DB
	Metrics            *metrics.Catalog
	Tracer             *tracing.Provider
	Logger             *slog.Logger
	Driver             string
	SlowQueryThreshold time.Duration
}

func (d *InstrumentedDB) QueryContext(ctx context.Context, operation, tableGroup, query string, args ...any) (*sql.Rows, error) {
	if d == nil || d.DB == nil {
		return nil, sql.ErrConnDone
	}
	op, tg, driver := d.safeLabels(operation, tableGroup)
	start := time.Now()
	ctx, span := d.startSpan(ctx, "db.query", op, tg)
	rows, err := d.DB.QueryContext(ctx, query, args...)
	d.finishQuery(span, op, tg, driver, start, err)
	return rows, err
}

func (d *InstrumentedDB) QueryRowContext(ctx context.Context, operation, tableGroup, query string, args ...any) *sql.Row {
	if d == nil || d.DB == nil {
		return nil
	}
	op, tg, driver := d.safeLabels(operation, tableGroup)
	start := time.Now()
	ctx, span := d.startSpan(ctx, "db.query_row", op, tg)
	row := d.DB.QueryRowContext(ctx, query, args...)
	d.finishQuery(span, op, tg, driver, start, nil)
	return row
}

func (d *InstrumentedDB) ExecContext(ctx context.Context, operation, tableGroup, query string, args ...any) (sql.Result, error) {
	if d == nil || d.DB == nil {
		return nil, sql.ErrConnDone
	}
	op, tg, driver := d.safeLabels(operation, tableGroup)
	start := time.Now()
	ctx, span := d.startSpan(ctx, "db.exec", op, tg)
	res, err := d.DB.ExecContext(ctx, query, args...)
	d.finishQuery(span, op, tg, driver, start, err)
	return res, err
}

func (d *InstrumentedDB) BeginTx(ctx context.Context, operation, tableGroup string, opts *sql.TxOptions) (*InstrumentedTx, error) {
	if d == nil || d.DB == nil {
		return nil, sql.ErrConnDone
	}
	op, tg, driver := d.safeLabels(operation, tableGroup)
	start := time.Now()
	ctx, span := d.startSpan(ctx, "db.transaction", op, tg)
	tx, err := d.DB.BeginTx(ctx, opts)
	if err != nil {
		d.finishTransaction(span, op, tg, driver, start, "failure", err)
		return nil, err
	}
	return &InstrumentedTx{Tx: tx, parent: d, operation: op, tableGroup: tg, driver: driver, start: start, span: span}, nil
}

// InstrumentedTx records transaction commit/rollback outcomes.
type InstrumentedTx struct {
	Tx         *sql.Tx
	parent     *InstrumentedDB
	operation  string
	tableGroup string
	driver     string
	start      time.Time
	span       trace.Span
}

func (tx *InstrumentedTx) Commit() error {
	if tx == nil || tx.Tx == nil {
		return sql.ErrTxDone
	}
	err := tx.Tx.Commit()
	tx.parent.finishTransaction(tx.span, tx.operation, tx.tableGroup, tx.driver, tx.start, resultFromErr(err), err)
	return err
}

func (tx *InstrumentedTx) Rollback() error {
	if tx == nil || tx.Tx == nil {
		return sql.ErrTxDone
	}
	err := tx.Tx.Rollback()
	if tx.parent != nil && tx.parent.Metrics != nil && tx.parent.Metrics.DBTransactionRollbacks != nil {
		tx.parent.Metrics.DBTransactionRollbacks.WithLabelValues(tx.operation, tx.tableGroup, resultFromErr(err), tx.driver).Inc()
	}
	tx.parent.finishTransaction(tx.span, tx.operation, tx.tableGroup, tx.driver, tx.start, resultFromErr(err), err)
	return err
}

func (d *InstrumentedDB) startSpan(ctx context.Context, name, operation, tableGroup string) (context.Context, trace.Span) {
	if d == nil || d.Tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	ctx, span := tracing.StartSpan(ctx, d.Tracer.Tracer(), name,
		attribute.String("db.operation", operation),
		attribute.String("db.table_group", tableGroup),
	)
	return ctx, span
}

func (d *InstrumentedDB) finishQuery(span trace.Span, operation, tableGroup, driver string, start time.Time, err error) {
	dur := time.Since(start)
	result := resultFromErr(err)
	if d != nil && d.Metrics != nil {
		if d.Metrics.DBQueryDuration != nil {
			d.Metrics.DBQueryDuration.WithLabelValues(operation, tableGroup, result, driver).Observe(dur.Seconds())
		}
		if err != nil && d.Metrics.DBQueryErrors != nil {
			d.Metrics.DBQueryErrors.WithLabelValues(operation, tableGroup, result, driver).Inc()
		}
	}
	d.logSlow(operation, tableGroup, dur)
	endSpan(span, err)
}

func (d *InstrumentedDB) finishTransaction(span trace.Span, operation, tableGroup, driver string, start time.Time, result string, err error) {
	dur := time.Since(start)
	if d != nil && d.Metrics != nil && d.Metrics.DBTransactionDuration != nil {
		d.Metrics.DBTransactionDuration.WithLabelValues(operation, tableGroup, result, driver).Observe(dur.Seconds())
	}
	d.logSlow(operation, tableGroup, dur)
	endSpan(span, err)
}

func (d *InstrumentedDB) safeLabels(operation, tableGroup string) (string, string, string) {
	op := normalizeOperation(operation)
	tg := normalizeTableGroup(tableGroup)
	driver := strings.TrimSpace(d.Driver)
	if driver == "" {
		driver = "unknown"
	}
	return op, tg, driver
}

func (d *InstrumentedDB) logSlow(operation, tableGroup string, dur time.Duration) {
	if d == nil || d.Logger == nil {
		return
	}
	threshold := d.SlowQueryThreshold
	if threshold <= 0 {
		threshold = 500 * time.Millisecond
	}
	if dur >= threshold {
		d.Logger.Warn("db_slow_query", "operation", operation, "table_group", tableGroup, "duration_ms", dur.Milliseconds())
	}
}

func endSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "database_error")
	}
	span.End()
}

func resultFromErr(err error) string {
	if err == nil {
		return "success"
	}
	if err == context.Canceled || err == context.DeadlineExceeded {
		return "timeout"
	}
	return "failure"
}

func normalizeDBRole(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "primary", "replica", "analytics":
		return raw
	default:
		return "primary"
	}
}

func normalizeOperation(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "select", "insert", "update", "delete", "upsert", "count", "begin", "commit", "rollback", "transaction":
		return raw
	default:
		return "other"
	}
}

func normalizeTableGroup(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "products", "orders", "inventory", "tasks", "webhooks", "alerts", "slo", "settings", "security", "files", "auth":
		return raw
	default:
		return "other"
	}
}

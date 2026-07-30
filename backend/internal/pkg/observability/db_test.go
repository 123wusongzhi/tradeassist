package observability

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
)

func TestDBStatsCollectorAndInstrumentedDB(t *testing.T) {
	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reg := metrics.NewRegistry("test")
	cat, err := metrics.RegisterCatalog(reg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	StartDBStatsCollector(ctx, &wg, nil, raw, cat, "primary", time.Millisecond)
	db := &InstrumentedDB{DB: raw, Metrics: cat, Driver: "sqlite", SlowQueryThreshold: time.Hour}
	if _, err := db.ExecContext(ctx, "insert", "settings", "create table settings (id integer primary key, name text)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "insert", "settings", "insert into settings(name) values (?)", "safe"); err != nil {
		t.Fatal(err)
	}
	rows, err := db.QueryContext(ctx, "select", "settings", "select name from settings")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	tx, err := db.BeginTx(ctx, "transaction", "settings", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	cancel()
	wg.Wait()
	values := reg.SnapshotValues()
	if values["db_query_duration_seconds"] == 0 {
		t.Fatalf("expected query duration samples: %+v", values)
	}
	if values["db_transaction_rollbacks_total"] == 0 {
		t.Fatalf("expected rollback metric: %+v", values)
	}
}

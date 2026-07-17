package operationlog

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newOperationLogTestDB(t testing.TB) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&OperationLog{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestHashChainConcurrentAppendSameScope(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- svc.WriteBackground(context.Background(), WriteOpts{
				TenantID: 7,
				Action:   "login",
				Resource: "auth",
				Status:   "failed",
				Message:  fmt.Sprintf("wrong password %d", i),
			})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	var rows []OperationLog
	if err := db.Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != n {
		t.Fatalf("rows = %d, want %d", len(rows), n)
	}

	prev := ""
	predecessorUse := map[string]int{}
	for i := range rows {
		if rows[i].EntryHash == "" {
			t.Fatalf("row %d missing entry hash", i)
		}
		if rows[i].PrevHash != prev {
			t.Fatalf("row %d prev hash mismatch: got %q want %q", i, rows[i].PrevHash, prev)
		}
		predecessorUse[rows[i].PrevHash]++
		prev = rows[i].EntryHash
	}
	if predecessorUse[""] != 1 {
		t.Fatalf("genesis predecessor count = %d, want 1", predecessorUse[""])
	}
	for predecessor, count := range predecessorUse {
		if predecessor != "" && count != 1 {
			t.Fatalf("predecessor %q used %d times, want 1", predecessor, count)
		}
	}
	count, mismatchAt, err := svc.VerifyChain(context.Background(), 7, time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("verify chain mismatch at %v: %v", mismatchAt, err)
	}
	if count != n {
		t.Fatalf("verify count = %d, want %d", count, n)
	}
}

func TestPreviousHashLookupFailureDoesNotWriteEmptyHash(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}
	forced := errors.New("forced previous hash lookup failure")
	if err := db.Callback().Query().Before("gorm:query").Register("operationlog:test_query_fail", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "operation_logs" {
			tx.AddError(forced)
		}
	}); err != nil {
		t.Fatal(err)
	}
	err := svc.WriteBackground(context.Background(), WriteOpts{TenantID: 9, Action: "login", Resource: "auth", Status: "failed"})
	if !errors.Is(err, forced) {
		t.Fatalf("err = %v, want forced lookup failure", err)
	}
	_ = db.Callback().Query().Remove("operationlog:test_query_fail")

	var count int64
	if err := db.Model(&OperationLog{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rows after lookup failure = %d, want 0", count)
	}
}

func TestOperationLogInsertFailureRollsBackHashChain(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}
	forced := errors.New("forced operation log insert failure")
	if err := db.Callback().Create().Before("gorm:create").Register("operationlog:test_create_fail", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "operation_logs" {
			tx.AddError(forced)
		}
	}); err != nil {
		t.Fatal(err)
	}
	err := svc.WriteBackground(context.Background(), WriteOpts{TenantID: 11, Action: "login", Resource: "auth", Status: "failed"})
	if !errors.Is(err, forced) {
		t.Fatalf("err = %v, want forced insert failure", err)
	}
	_ = db.Callback().Create().Remove("operationlog:test_create_fail")

	var count int64
	if err := db.Model(&OperationLog{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rows after insert failure = %d, want 0", count)
	}
}

func BenchmarkOperationLogHashChainAppend(b *testing.B) {
	db := newOperationLogTestDB(b)
	svc := &Service{DB: db}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := svc.WriteBackground(context.Background(), WriteOpts{
			TenantID: 17,
			Action:   "login",
			Resource: "auth",
			Status:   "failed",
		}); err != nil {
			b.Fatal(err)
		}
	}
}

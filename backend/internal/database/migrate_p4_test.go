package database

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"gorm.io/gorm"
)

func TestMigrateLoginAttemptUniqueIndexMergesDuplicates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:login_attempt_migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE auth_login_attempts (id text primary key, tenant_id integer, account_key text not null, ip_hash text, failed_count integer, locked_until datetime, last_failed_at datetime, created_at datetime, updated_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE INDEX idx_auth_login_attempts_account_key ON auth_login_attempts(account_key)`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	later := now.Add(time.Hour)
	if err := db.Create(&auth.AuthLoginAttempt{ID: uuid.New(), AccountKey: "same", FailedCount: 2, LastFailedAt: &now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&auth.AuthLoginAttempt{ID: uuid.New(), AccountKey: "same", FailedCount: 5, LockedUntil: &later}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateLoginAttemptUniqueIndex(db); err != nil {
		t.Fatal(err)
	}
	var rows []auth.AuthLoginAttempt
	if err := db.Find(&rows).Error; err != nil || len(rows) != 1 || rows[0].FailedCount != 5 || rows[0].LockedUntil == nil {
		t.Fatalf("merged rows=%+v err=%v", rows, err)
	}
	if err := db.Create(&auth.AuthLoginAttempt{ID: uuid.New(), AccountKey: "same"}).Error; err == nil {
		t.Fatal("unique index did not reject duplicate")
	}
	indexes, err := db.Migrator().GetIndexes(&auth.AuthLoginAttempt{})
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range indexes {
		if columns := index.Columns(); len(columns) == 1 && columns[0] == "account_key" {
			if unique, known := index.Unique(); !known || !unique {
				t.Fatalf("account_key index %q is not unique", index.Name())
			}
			return
		}
	}
	t.Fatal("account_key index not found")
}

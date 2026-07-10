package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Advisory lock keys for migration (two int32 keys for pg_advisory_lock).
const (
	migrationLockKey1 = 8837291
	migrationLockKey2 = 20260710
)

// RunMigrateWithLock acquires a PostgreSQL advisory lock, runs AutoMigrate, then releases.
func RunMigrateWithLock(ctx context.Context, db *gorm.DB, timeout time.Duration, run func(*gorm.DB) error) error {
	if db == nil {
		return fmt.Errorf("migration lock: db is nil")
	}
	if run == nil {
		return fmt.Errorf("migration lock: run func is nil")
	}
	driver := ""
	if db.Dialector != nil {
		driver = db.Dialector.Name()
	}
	if driver != "postgres" {
		// MySQL / others: run without advisory lock (single-instance convention).
		return run(db)
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	deadline := time.Now().UTC().Add(timeout)
	for {
		acquired, err := tryAdvisoryLock(ctx, db)
		if err != nil {
			return err
		}
		if acquired {
			defer func() { _ = releaseAdvisoryLock(db) }()
			return run(db)
		}
		if time.Now().UTC().After(deadline) {
			return fmt.Errorf("migration lock: timeout waiting for advisory lock")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func tryAdvisoryLock(ctx context.Context, db *gorm.DB) (bool, error) {
	var ok bool
	err := db.WithContext(ctx).Raw(
		"SELECT pg_try_advisory_lock(?, ?)", migrationLockKey1, migrationLockKey2,
	).Scan(&ok).Error
	return ok, err
}

func releaseAdvisoryLock(db *gorm.DB) error {
	return db.Exec("SELECT pg_advisory_unlock(?, ?)", migrationLockKey1, migrationLockKey2).Error
}

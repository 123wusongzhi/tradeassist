package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"gorm.io/gorm"
)

// migrateP4Security applies Phase P4 auth, audit and file security schema.
func migrateP4Security(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate p4: db is nil")
	}
	if err := migrateLoginAttemptUniqueIndex(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(
		&auth.Tenant{},
		&auth.AuthSession{},
		&auth.AuthRefreshToken{},
		&auth.AuthLoginAttempt{},
		&auth.AuthReauthToken{},
		&admin.AdminUser{},
		&operationlog.OperationLog{},
		&files.FileRecord{},
	); err != nil {
		return err
	}
	return migrateP4Indexes(db)
}

// migrateLoginAttemptUniqueIndex makes the atomic login-guard upsert safe for
// installations created before account_key was unique. It retains the most
// restrictive row for each key before creating the GORM-named unique index.
func migrateLoginAttemptUniqueIndex(db *gorm.DB) error {
	if !db.Migrator().HasTable(&auth.AuthLoginAttempt{}) {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var rows []auth.AuthLoginAttempt
		if err := tx.Order("account_key ASC, failed_count DESC, locked_until DESC, last_failed_at DESC, created_at ASC").Find(&rows).Error; err != nil {
			return err
		}
		seen := map[string]auth.AuthLoginAttempt{}
		for _, row := range rows {
			if keep, ok := seen[row.AccountKey]; !ok {
				seen[row.AccountKey] = row
				continue
			} else {
				if row.FailedCount > keep.FailedCount {
					keep.FailedCount = row.FailedCount
				}
				if keep.LockedUntil == nil || (row.LockedUntil != nil && row.LockedUntil.After(*keep.LockedUntil)) {
					keep.LockedUntil = row.LockedUntil
				}
				if keep.LastFailedAt == nil || (row.LastFailedAt != nil && row.LastFailedAt.After(*keep.LastFailedAt)) {
					keep.LastFailedAt = row.LastFailedAt
				}
				if err := tx.Model(&auth.AuthLoginAttempt{}).Where("id = ?", keep.ID).Updates(map[string]any{"failed_count": keep.FailedCount, "locked_until": keep.LockedUntil, "last_failed_at": keep.LastFailedAt}).Error; err != nil {
					return err
				}
				seen[row.AccountKey] = keep
				if err := tx.Delete(&row).Error; err != nil {
					return err
				}
			}
		}
		indexes, err := tx.Migrator().GetIndexes(&auth.AuthLoginAttempt{})
		if err != nil {
			return err
		}
		for _, index := range indexes {
			columns := index.Columns()
			if len(columns) != 1 || columns[0] != "account_key" {
				continue
			}
			if unique, known := index.Unique(); known && unique {
				return nil
			}
			// Older releases created the same GORM-named index as non-unique.
			// It must be removed before CreateIndex can establish the constraint.
			if err := tx.Migrator().DropIndex(&auth.AuthLoginAttempt{}, index.Name()); err != nil {
				return err
			}
		}
		return tx.Migrator().CreateIndex(&auth.AuthLoginAttempt{}, "AccountKey")
	})
}

func migrateP4Indexes(db *gorm.DB) error {
	type idx struct {
		table string
		name  string
		sql   string
	}
	indexes := []idx{
		{"auth_refresh_tokens", "idx_auth_refresh_family_status", "CREATE INDEX IF NOT EXISTS idx_auth_refresh_family_status ON auth_refresh_tokens (token_family_id, status)"},
		{"auth_sessions", "idx_auth_sessions_user_status", "CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_status ON auth_sessions (user_id, status)"},
		{"auth_login_attempts", "idx_auth_login_account", "CREATE INDEX IF NOT EXISTS idx_auth_login_account ON auth_login_attempts (account_key)"},
		{"operation_logs", "idx_operation_logs_tenant_created", "CREATE INDEX IF NOT EXISTS idx_operation_logs_tenant_created ON operation_logs (tenant_id, created_at)"},
	}
	for _, i := range indexes {
		if !db.Migrator().HasTable(i.table) {
			continue
		}
		if err := db.Exec(i.sql).Error; err != nil {
			return fmt.Errorf("p4 index %s: %w", i.name, err)
		}
	}
	return nil
}

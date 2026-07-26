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
	if err := db.AutoMigrate(
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

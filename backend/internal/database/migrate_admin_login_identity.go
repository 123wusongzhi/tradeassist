package database

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// migrateAdminLoginIdentityIndexes makes the identifiers accepted by the
// global login endpoint unambiguous. Authentication deliberately has no
// tenant selector, so duplicate normalized email/phone values must fail
// startup instead of letting query order choose an account.
func migrateAdminLoginIdentityIndexes(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate admin login identities: db is nil")
	}
	if !db.Migrator().HasTable("admin_users") {
		return nil
	}
	type duplicate struct {
		Value string
		Count int64
	}
	checks := []struct {
		name       string
		expression string
		indexSQL   string
	}{
		{
			name:       "email",
			expression: "LOWER(TRIM(email))",
			indexSQL:   "CREATE UNIQUE INDEX IF NOT EXISTS ux_admin_users_login_email ON admin_users (LOWER(TRIM(email))) WHERE TRIM(email) <> ''",
		},
		{
			name:       "phone",
			expression: "TRIM(phone)",
			indexSQL:   "CREATE UNIQUE INDEX IF NOT EXISTS ux_admin_users_login_phone ON admin_users (TRIM(phone)) WHERE TRIM(phone) <> ''",
		},
	}
	for _, check := range checks {
		var dup duplicate
		query := fmt.Sprintf("SELECT %s AS value, COUNT(*) AS count FROM admin_users WHERE TRIM(%s) <> '' GROUP BY %s HAVING COUNT(*) > 1 LIMIT 1", check.expression, check.name, check.expression)
		if err := db.Raw(query).Scan(&dup).Error; err != nil {
			return fmt.Errorf("migrate admin login %s duplicate check: %w", check.name, err)
		}
		if dup.Count > 1 {
			return fmt.Errorf("migrate admin login identities: duplicate normalized %s %q", check.name, strings.TrimSpace(dup.Value))
		}
		if err := db.Exec(check.indexSQL).Error; err != nil {
			return fmt.Errorf("migrate admin login %s index: %w", check.name, err)
		}
	}
	return nil
}

package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// migrateTenantAdminRole removes historical global-admin grants from
// self-service tenant accounts. Tenant 0 remains the system/bootstrap tenant.
// This intentionally fails startup on an update error so the insecure state is
// never silently retained.
func migrateTenantAdminRole(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate tenant admin role: db is nil")
	}
	if !db.Migrator().HasTable("admin_users") {
		return nil
	}
	if err := db.Model(&struct{}{}).Table("admin_users").
		Where("tenant_id > ? AND LOWER(TRIM(role)) = ?", 0, adminperm.RoleAdmin).
		Update("role", adminperm.RoleTenantAdmin).Error; err != nil {
		return fmt.Errorf("migrate tenant admin role: %w", err)
	}
	return nil
}

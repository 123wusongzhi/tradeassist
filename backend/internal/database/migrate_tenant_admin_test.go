package database

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

func TestMigrateTenantAdminRoleDowngradesOnlyTenantAdmins(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:tenant_admin_migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	global := admin.AdminUser{Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TenantID: 0}
	tenant := admin.AdminUser{Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TenantID: 9}
	if err := db.Create(&global).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateTenantAdminRole(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&global, "id = ?", global.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&tenant, "id = ?", tenant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if global.Role != adminperm.RoleAdmin || tenant.Role != adminperm.RoleTenantAdmin {
		t.Fatalf("roles after migration global=%q tenant=%q", global.Role, tenant.Role)
	}
}

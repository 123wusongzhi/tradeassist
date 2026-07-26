package admin

import (
	"context"
	"log/slog"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"gorm.io/gorm"
)

func testBootstrapDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&AdminUser{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestEnsureBootstrapAdmin_createsAdminRole(t *testing.T) {
	t.Parallel()
	db := testBootstrapDB(t)
	cfg := &config.Config{
		AppEnv:                 config.EnvDevelopment,
		BootstrapAdminEmail:    "admin@example.com",
		BootstrapAdminPassword: "secret",
	}
	if err := EnsureBootstrapAdmin(context.Background(), db, cfg, slog.Default()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	var u AdminUser
	if err := db.Where("email = ?", "admin@example.com").First(&u).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if u.Role != RoleAdmin || u.Status != StatusActive {
		t.Fatalf("got role=%q status=%q", u.Role, u.Status)
	}
}

func TestEnsureBootstrapAdmin_syncsExistingOperatorToAdmin(t *testing.T) {
	t.Parallel()
	db := testBootstrapDB(t)
	existing := AdminUser{
		Username:     NewInternalUsername(),
		Email:        "ops@example.com",
		PasswordHash: "hash",
		Role:         "operator",
		Status:       StatusActive,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg := &config.Config{
		AppEnv:              config.EnvDevelopment,
		BootstrapAdminEmail: "ops@example.com",
	}
	if err := EnsureBootstrapAdmin(context.Background(), db, cfg, slog.Default()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	var u AdminUser
	if err := db.First(&u, "id = ?", existing.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if u.Role != RoleAdmin {
		t.Fatalf("expected admin role, got %q", u.Role)
	}
}

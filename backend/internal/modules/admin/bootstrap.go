package admin

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	// RoleAdmin is the global administrator role with all permissions.
	RoleAdmin = "admin"
	// StatusActive allows login and API access.
	StatusActive = "active"
)

type bootstrapIdentity struct {
	email string
	phone string
}

// EnsureBootstrapAdmin creates the first admin when the table is empty and keeps
// the configured bootstrap account (ADMIN_BOOTSTRAP_EMAIL / PHONE) at admin role.
func EnsureBootstrapAdmin(ctx context.Context, db *gorm.DB, cfg *config.Config, log *slog.Logger) error {
	if db == nil || cfg == nil {
		return fmt.Errorf("admin bootstrap: invalid deps")
	}
	var n int64
	if err := db.WithContext(ctx).Model(&AdminUser{}).Count(&n).Error; err != nil {
		return fmt.Errorf("admin bootstrap count: %w", err)
	}

	id, err := parseBootstrapIdentity(cfg)
	if err != nil {
		if n == 0 {
			return err
		}
		id = bootstrapIdentity{}
	}

	if n == 0 {
		if err := createBootstrapAdmin(ctx, db, cfg, id, log); err != nil {
			return err
		}
	}
	return ensureBootstrapAdminPrivileges(ctx, db, id, log)
}

func parseBootstrapIdentity(cfg *config.Config) (bootstrapIdentity, error) {
	rawEmail := strings.TrimSpace(cfg.BootstrapAdminEmail)
	rawPhone := strings.TrimSpace(cfg.BootstrapAdminPhone)
	hasEmail := rawEmail != ""
	hasPhone := rawPhone != ""

	if !hasEmail && !hasPhone {
		return bootstrapIdentity{}, fmt.Errorf("admin bootstrap: set ADMIN_BOOTSTRAP_EMAIL and/or ADMIN_BOOTSTRAP_PHONE")
	}

	var out bootstrapIdentity
	if hasEmail {
		em, _, ok := ParseLoginAccount(rawEmail)
		if !ok {
			return bootstrapIdentity{}, fmt.Errorf("admin bootstrap: ADMIN_BOOTSTRAP_EMAIL invalid")
		}
		out.email = em
	}
	if hasPhone {
		phone := NormalizePhoneDigits(rawPhone)
		if len(phone) < 10 || len(phone) > 15 {
			return bootstrapIdentity{}, fmt.Errorf("admin bootstrap: invalid ADMIN_BOOTSTRAP_PHONE")
		}
		out.phone = phone
	}
	return out, nil
}

func createBootstrapAdmin(ctx context.Context, db *gorm.DB, cfg *config.Config, id bootstrapIdentity, log *slog.Logger) error {
	password := cfg.BootstrapAdminPassword
	if password == "" {
		if cfg.AppEnv == "production" {
			return fmt.Errorf("admin bootstrap: no admins exist; set ADMIN_BOOTSTRAP_PASSWORD for first boot")
		}
		password = "changeme"
		if log != nil {
			log.Warn("admin_bootstrap_default_password", "hint", "set ADMIN_BOOTSTRAP_PASSWORD for non-dev")
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("admin bootstrap hash: %w", err)
	}

	disp := id.email
	if disp == "" {
		disp = id.phone
	}

	u := AdminUser{
		Username:     NewInternalUsername(),
		Email:        id.email,
		Phone:        id.phone,
		PasswordHash: string(hash),
		DisplayName:  disp,
		Role:         RoleAdmin,
		Status:       StatusActive,
	}

	if err := db.WithContext(ctx).Create(&u).Error; err != nil {
		return fmt.Errorf("admin bootstrap create: %w", err)
	}
	if log != nil {
		log.Info("admin_bootstrapped", "email", id.email, "phone", id.phone, "id", u.ID.String(), "role", RoleAdmin)
	}
	return nil
}

func ensureBootstrapAdminPrivileges(ctx context.Context, db *gorm.DB, id bootstrapIdentity, log *slog.Logger) error {
	if id.email == "" && id.phone == "" {
		return nil
	}
	var u AdminUser
	found := false
	if id.email != "" {
		if err := db.WithContext(ctx).Where("LOWER(TRIM(email)) = ?", id.email).First(&u).Error; err == nil {
			found = true
		}
	}
	if !found && id.phone != "" {
		if err := db.WithContext(ctx).Where("phone = ?", id.phone).First(&u).Error; err == nil {
			found = true
		}
	}
	if !found {
		return nil
	}

	updates := map[string]any{}
	if strings.TrimSpace(strings.ToLower(u.Role)) != RoleAdmin {
		updates["role"] = RoleAdmin
	}
	if st := strings.TrimSpace(strings.ToLower(u.Status)); st == "disabled" || st == "inactive" {
		updates["status"] = StatusActive
	}
	if len(updates) == 0 {
		return nil
	}
	if err := db.WithContext(ctx).Model(&AdminUser{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("admin bootstrap privileges: %w", err)
	}
	if log != nil {
		log.Info("admin_bootstrap_privileges_synced", "email", u.Email, "phone", u.Phone, "id", u.ID.String(), "role", RoleAdmin)
	}
	return nil
}

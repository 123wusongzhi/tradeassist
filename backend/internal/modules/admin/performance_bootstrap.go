package admin

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	PerfSystemAdminEmail = "p7v2-perf-admin@example.invalid"
	PerfTenantAdminEmail = "p7v2-perf-tenant-admin@example.invalid"
	PerfOperatorEmail    = "p7v2-perf-operator@example.invalid"
	PerfReadonlyEmail    = "p7v2-perf-readonly@example.invalid"
	PerfDisabledEmail    = "p7v2-perf-disabled@example.invalid"
)

// PerformanceBootstrapStats summarizes idempotent performance account provisioning.
type PerformanceBootstrapStats struct {
	UsersCreated     int   `json:"usersCreated"`
	UsersUpdated     int   `json:"usersUpdated"`
	GrantsCreated    int   `json:"grantsCreated"`
	GrantsUpdated    int   `json:"grantsUpdated"`
	DuplicateUsers   int   `json:"duplicateUsers"`
	DuplicateRoles   int   `json:"duplicateRoles"`
	DuplicateAssigns int   `json:"duplicateAssignments"`
	DefaultTenantID  int64 `json:"defaultTenantId"`
	ShopsGranted     int   `json:"shopsGranted"`
}

type perfAccountSpec struct {
	email    string
	role     string
	status   string
	tenantID int64
	password string
}

// EnsurePerformanceBootstrap provisions fixed performance test identities.
// Allowed only when APP_ENV=performance and PERFORMANCE_TEST_MODE=true.
func EnsurePerformanceBootstrap(ctx context.Context, db *gorm.DB, cfg *config.Config, log *slog.Logger) (*PerformanceBootstrapStats, error) {
	stats := &PerformanceBootstrapStats{}
	if db == nil || cfg == nil {
		return stats, fmt.Errorf("performance bootstrap: invalid deps")
	}
	if config.IsProduction(cfg.AppEnv) {
		return stats, fmt.Errorf("performance bootstrap: forbidden in production")
	}
	if cfg.AppEnv != config.EnvPerformance || !cfg.P7.PerformanceTestMode {
		return nil, nil
	}

	tenantID := perfDefaultTenantID()
	stats.DefaultTenantID = tenantID

	specs := perfAccountSpecs(cfg)

	for _, spec := range specs {
		if strings.TrimSpace(spec.password) == "" {
			return stats, fmt.Errorf("performance bootstrap: missing password for %s", spec.email)
		}
		created, updated, err := upsertPerfUser(ctx, db, spec)
		if err != nil {
			return stats, err
		}
		if created {
			stats.UsersCreated++
		}
		if updated {
			stats.UsersUpdated++
		}
	}

	var users []AdminUser
	if err := db.WithContext(ctx).
		Where("LOWER(TRIM(email)) IN ?", []string{
			strings.ToLower(PerfOperatorEmail),
			strings.ToLower(PerfReadonlyEmail),
		}).
		Find(&users).Error; err != nil {
		return stats, fmt.Errorf("performance bootstrap load scoped users: %w", err)
	}

	shopIDs, err := perfTenantShopIDs(ctx, db, tenantID)
	if err != nil {
		return stats, err
	}
	stats.ShopsGranted = len(shopIDs)

	for _, u := range users {
		scope := StorePermScopeOperate
		if strings.EqualFold(u.Role, "readonly") {
			scope = StorePermScopeView
		}
		added, err := ensureStoreGrants(ctx, db, u.ID, shopIDs, scope)
		if err != nil {
			return stats, err
		}
		stats.GrantsCreated += added
	}

	if log != nil {
		log.Info("performance_bootstrap_complete",
			"usersCreated", stats.UsersCreated,
			"usersUpdated", stats.UsersUpdated,
			"grantsCreated", stats.GrantsCreated,
			"defaultTenantId", stats.DefaultTenantID,
			"shopsGranted", stats.ShopsGranted,
		)
	}
	return stats, nil
}

// PerformanceBootstrapUserIDs returns performance harness account ids for cache invalidation.
func PerformanceBootstrapUserIDs(ctx context.Context, db *gorm.DB) ([]uuid.UUID, error) {
	if db == nil {
		return nil, fmt.Errorf("performance bootstrap: no db")
	}
	var ids []uuid.UUID
	err := db.WithContext(ctx).Model(&AdminUser{}).
		Where("LOWER(TRIM(email)) LIKE ?", "p7v2-perf-%@example.invalid").
		Pluck("id", &ids).Error
	return ids, err
}

func upsertPerfUser(ctx context.Context, db *gorm.DB, spec perfAccountSpec) (created bool, updated bool, err error) {
	email := strings.ToLower(strings.TrimSpace(spec.email))
	hash, err := bcrypt.GenerateFromPassword([]byte(spec.password), bcrypt.DefaultCost)
	if err != nil {
		return false, false, fmt.Errorf("performance bootstrap hash: %w", err)
	}

	var existing AdminUser
	findErr := db.WithContext(ctx).Where("LOWER(TRIM(email)) = ?", email).First(&existing).Error
	if findErr != nil && findErr != gorm.ErrRecordNotFound {
		return false, false, findErr
	}

	if findErr == gorm.ErrRecordNotFound {
		u := AdminUser{
			TenantID:     spec.tenantID,
			Username:     NewInternalUsername(),
			Email:        email,
			PasswordHash: string(hash),
			DisplayName:  email,
			Role:         spec.role,
			Status:       spec.status,
			TokenVersion: 1,
		}
		if err := db.WithContext(ctx).Create(&u).Error; err != nil {
			return false, false, fmt.Errorf("performance bootstrap create %s: %w", email, err)
		}
		return true, false, nil
	}

	updates := map[string]any{}
	if existing.TenantID != spec.tenantID {
		updates["tenant_id"] = spec.tenantID
	}
	if strings.TrimSpace(strings.ToLower(existing.Role)) != strings.TrimSpace(strings.ToLower(spec.role)) {
		updates["role"] = spec.role
	}
	wantStatus := strings.TrimSpace(strings.ToLower(spec.status))
	curStatus := strings.TrimSpace(strings.ToLower(existing.Status))
	if wantStatus != curStatus {
		updates["status"] = spec.status
	}
	// Performance harness treats env passwords as source of truth across restarts.
	updates["password_hash"] = string(hash)
	if len(updates) > 0 {
		updates["token_version"] = gorm.Expr("token_version + 1")
		if err := db.WithContext(ctx).Model(&AdminUser{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return false, false, fmt.Errorf("performance bootstrap update %s: %w", email, err)
		}
		return false, true, nil
	}
	return false, false, nil
}

func ensureStoreGrants(ctx context.Context, db *gorm.DB, userID uuid.UUID, shopIDs []uuid.UUID, scope string) (int, error) {
	if userID == uuid.Nil || len(shopIDs) == 0 {
		return 0, nil
	}
	scope = NormalizeStorePermScope(scope)
	created := 0
	for _, sid := range shopIDs {
		if sid == uuid.Nil {
			continue
		}
		var n int64
		if err := db.WithContext(ctx).Model(&UserStorePermission{}).
			Where("user_id = ? AND store_id = ?", userID, sid).
			Count(&n).Error; err != nil {
			return created, err
		}
		if n > 0 {
			if err := db.WithContext(ctx).Model(&UserStorePermission{}).
				Where("user_id = ? AND store_id = ?", userID, sid).
				Update("permission_scope", scope).Error; err != nil {
				return created, err
			}
			continue
		}
		row := UserStorePermission{
			UserID:          userID,
			StoreID:         sid,
			Platform:        "mock",
			PermissionScope: scope,
		}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

func perfTenantShopIDs(ctx context.Context, db *gorm.DB, tenantID int64) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := db.WithContext(ctx).
		Table("shops").
		Select("id").
		Where("tenant_id = ?", tenantID).
		Order("created_at ASC, id ASC").
		Limit(32).
		Pluck("id", &ids).Error
	return ids, err
}

func perfAccountSpecs(cfg *config.Config) []perfAccountSpec {
	tenantID := perfDefaultTenantID()
	systemPassword := strings.TrimSpace(cfg.BootstrapAdminPassword)
	if systemPassword == "" {
		systemPassword = perfSystemAdminPassword()
	}
	return []perfAccountSpec{
		{
			email:    perfSystemAdminEmail(),
			role:     RoleAdmin,
			status:   StatusActive,
			tenantID: 0,
			password: systemPassword,
		},
		{
			email:    PerfTenantAdminEmail,
			role:     RoleAdmin,
			status:   StatusActive,
			tenantID: tenantID,
			password: os.Getenv("P7V2_PERF_TENANT_ADMIN_PASSWORD"),
		},
		{
			email:    PerfOperatorEmail,
			role:     "operator",
			status:   StatusActive,
			tenantID: tenantID,
			password: os.Getenv("P7V2_PERF_OPERATOR_PASSWORD"),
		},
		{
			email:    PerfReadonlyEmail,
			role:     "readonly",
			status:   StatusActive,
			tenantID: tenantID,
			password: os.Getenv("P7V2_PERF_READONLY_PASSWORD"),
		},
		{
			email:    PerfDisabledEmail,
			role:     "operator",
			status:   "disabled",
			tenantID: tenantID,
			password: firstNonEmptyEnv("P7V2_PERF_DISABLED_PASSWORD", "P7V2_PERF_OPERATOR_PASSWORD"),
		},
	}
}

func perfDefaultTenantID() int64 {
	raw := strings.TrimSpace(os.Getenv("P7_PERF_DEFAULT_TENANT_ID"))
	if raw == "" {
		return 1
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return 1
	}
	return n
}

func firstNonEmptyEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func perfSystemAdminEmail() string {
	if v := firstNonEmptyEnv("ADMIN_BOOTSTRAP_EMAIL"); v != "" {
		return strings.ToLower(strings.TrimSpace(v))
	}
	return PerfSystemAdminEmail
}

func perfSystemAdminPassword() string {
	if v := firstNonEmptyEnv("P7V2_PERF_ADMIN_PASSWORD", "ADMIN_BOOTSTRAP_PASSWORD"); v != "" {
		return v
	}
	return ""
}

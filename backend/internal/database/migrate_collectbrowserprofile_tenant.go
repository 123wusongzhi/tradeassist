package database

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectbrowserprofile"
	"gorm.io/gorm"
)

// migrateCollectBrowserProfileTenantScope backfills a legacy profile only if
// every usable task reference identifies exactly one non-zero tenant. Profiles
// without a provable business owner remain in the explicit system tenant (0).
func migrateCollectBrowserProfileTenantScope(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&collectbrowserprofile.CollectBrowserProfile{}) {
		return nil
	}
	if err := db.AutoMigrate(&collectbrowserprofile.CollectBrowserProfile{}); err != nil {
		return fmt.Errorf("migrate collect browser profile tenant columns: %w", err)
	}
	var profiles []collectbrowserprofile.CollectBrowserProfile
	if err := db.Where("tenant_id = ?", 0).Find(&profiles).Error; err != nil {
		return fmt.Errorf("list legacy collect browser profiles: %w", err)
	}
	for _, profile := range profiles {
		tenants, err := collectBrowserProfileMigrationTenants(db, profile.ID)
		if err != nil {
			return fmt.Errorf("resolve legacy collect browser profile %s: %w", profile.ID, err)
		}
		if len(tenants) != 1 {
			continue
		}
		for tenantID := range tenants {
			if err := db.Model(&collectbrowserprofile.CollectBrowserProfile{}).Where("id = ? AND tenant_id = ?", profile.ID, 0).Update("tenant_id", tenantID).Error; err != nil {
				return fmt.Errorf("backfill collect browser profile %s: %w", profile.ID, err)
			}
		}
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_collect_browser_profiles_tenant_updated ON collect_browser_profiles (tenant_id, updated_at)").Error; err != nil {
		return fmt.Errorf("create collect browser profile tenant index: %w", err)
	}
	return nil
}

func collectBrowserProfileMigrationTenants(db *gorm.DB, profileID uuid.UUID) (map[int64]struct{}, error) {
	var tasks []collect.CollectTask
	if err := db.Select("tenant_id", "request_options").Where("tenant_id > 0").Find(&tasks).Error; err != nil {
		return nil, err
	}
	tenants := make(map[int64]struct{})
	for _, task := range tasks {
		var options struct {
			ProfileID string `json:"profileId"`
		}
		if len(task.RequestOptions) == 0 || json.Unmarshal(task.RequestOptions, &options) != nil {
			continue
		}
		if id, err := uuid.Parse(strings.TrimSpace(options.ProfileID)); err == nil && id == profileID {
			tenants[task.TenantID] = struct{}{}
		}
	}
	return tenants, nil
}

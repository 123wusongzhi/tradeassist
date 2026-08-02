package database

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectbrowserprofile"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestMigrateCollectBrowserProfileTenantScopeOnlyBackfillsUniqueTaskOwner(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	require.NoError(t, db.AutoMigrate(&collect.CollectTask{}, &collectbrowserprofile.CollectBrowserProfile{}))
	owned := collectbrowserprofile.CollectBrowserProfile{Name: "owned", Domain: "example.com", ProfileKey: "owned", Status: collectbrowserprofile.StatusActive}
	unknown := collectbrowserprofile.CollectBrowserProfile{Name: "unknown", Domain: "example.com", ProfileKey: "unknown", Status: collectbrowserprofile.StatusActive}
	conflicted := collectbrowserprofile.CollectBrowserProfile{Name: "conflicted", Domain: "example.com", ProfileKey: "conflicted", Status: collectbrowserprofile.StatusActive}
	require.NoError(t, db.Create(&owned).Error)
	require.NoError(t, db.Create(&unknown).Error)
	require.NoError(t, db.Create(&conflicted).Error)
	require.NoError(t, db.Create(&collect.CollectTask{TenantID: 41, Source: "custom", SourceURL: "https://example.com", Status: collect.StatusPending, RequestOptions: datatypes.JSON([]byte(`{"profileId":"` + owned.ID.String() + `"}`))}).Error)
	require.NoError(t, db.Create(&collect.CollectTask{TenantID: 41, Source: "custom", SourceURL: "https://example.com", Status: collect.StatusPending, RequestOptions: datatypes.JSON([]byte(`{"profileId":"` + conflicted.ID.String() + `"}`))}).Error)
	require.NoError(t, db.Create(&collect.CollectTask{TenantID: 42, Source: "custom", SourceURL: "https://example.com", Status: collect.StatusPending, RequestOptions: datatypes.JSON([]byte(`{"profileId":"` + conflicted.ID.String() + `"}`))}).Error)

	require.NoError(t, migrateCollectBrowserProfileTenantScope(db))
	var gotOwned, gotUnknown, gotConflicted collectbrowserprofile.CollectBrowserProfile
	require.NoError(t, db.First(&gotOwned, "id = ?", owned.ID).Error)
	require.NoError(t, db.First(&gotUnknown, "id = ?", unknown.ID).Error)
	require.NoError(t, db.First(&gotConflicted, "id = ?", conflicted.ID).Error)
	require.Equal(t, int64(41), gotOwned.TenantID)
	require.Zero(t, gotUnknown.TenantID)
	require.Zero(t, gotConflicted.TenantID)
}

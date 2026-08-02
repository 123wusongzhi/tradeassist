package collect

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectbrowserprofile"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func TestValidateCustomProfileSnapshotReauthorizesTenantAndSnapshot(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&collectbrowserprofile.CollectBrowserProfile{}))
	profile := collectbrowserprofile.CollectBrowserProfile{
		TenantID:   41,
		Name:       "tenant-41",
		Domain:     "example.com",
		ProfileKey: "custom_" + uuid.NewString(),
		Status:     collectbrowserprofile.StatusActive,
	}
	require.NoError(t, db.Create(&profile).Error)
	svc := &Service{Profiles: &collectbrowserprofile.Service{DB: db}}
	ctxFor := func(tenantID int64) context.Context {
		return security.WithTenantContext(context.Background(), security.WorkerTenantContext(tenantID, uuid.Nil))
	}

	got, err := svc.validateCustomProfileSnapshot(
		ctxFor(41),
		"https://www.example.com/item/1",
		profile.ID.String(),
		profile.ProfileKey,
	)
	require.NoError(t, err)
	require.Equal(t, profile.ProfileKey, got)

	_, err = svc.validateCustomProfileSnapshot(
		ctxFor(42),
		"https://www.example.com/item/1",
		profile.ID.String(),
		profile.ProfileKey,
	)
	require.ErrorIs(t, err, errCustomProfileSnapshotInvalid)

	_, err = svc.validateCustomProfileSnapshot(
		ctxFor(41),
		"https://www.example.com/item/1",
		profile.ID.String(),
		"custom_stale_snapshot",
	)
	require.ErrorIs(t, err, errCustomProfileSnapshotInvalid)

	_, err = svc.validateCustomProfileSnapshot(
		ctxFor(41),
		"https://other.example.net/item/1",
		profile.ID.String(),
		profile.ProfileKey,
	)
	require.ErrorIs(t, err, errCustomProfileSnapshotInvalid)
}

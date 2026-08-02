package api

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListingImageFetcherRequiresCleanTenantOwnedFile(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:listing_image_fetcher?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&files.FileRecord{}, &settings.Setting{}))

	root := t.TempDir()
	var pngBytes bytes.Buffer
	require.NoError(t, png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 1, 1))))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "images"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "images", "clean.png"), pngBytes.Bytes(), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "images", "tenant-zero.png"), pngBytes.Bytes(), 0o600))
	for k, v := range map[string]string{"kind": "local", "local_root": root} {
		require.NoError(t, db.Create(&settings.Setting{TenantID: 0, GroupKey: "storage", ItemKey: k, ItemValue: v, ValueType: "string"}).Error)
	}
	clean := &files.FileRecord{TenantID: 1, OriginalName: "clean.png", ObjectKey: "images/clean.png", ContentType: "image/png", StorageKind: "local", SecurityStatus: files.SecurityClean, ScanStatus: files.SecurityClean}
	foreign := &files.FileRecord{TenantID: 2, OriginalName: "foreign.png", ObjectKey: "images/foreign.png", ContentType: "image/png", StorageKind: "local", SecurityStatus: files.SecurityClean, ScanStatus: files.SecurityClean}
	scanning := &files.FileRecord{TenantID: 1, OriginalName: "scanning.png", ObjectKey: "images/scanning.png", ContentType: "image/png", StorageKind: "local", SecurityStatus: files.SecurityScanning, ScanStatus: files.SecurityScanning}
	tenantZero := &files.FileRecord{TenantID: 0, OriginalName: "tenant-zero.png", ObjectKey: "images/tenant-zero.png", ContentType: "image/png", StorageKind: "local", SecurityStatus: files.SecurityClean, ScanStatus: files.SecurityClean}
	require.NoError(t, db.Create(clean).Error)
	require.NoError(t, db.Create(foreign).Error)
	require.NoError(t, db.Create(scanning).Error)
	require.NoError(t, db.Create(tenantZero).Error)

	fetcher := newTikTokListingImageFetcher(db, &settings.Service{DB: db})
	ctx := context.Background()
	for _, key := range []string{foreign.ObjectKey, scanning.ObjectKey} {
		_, _, err := fetcher.FetchProductImageBytes(ctx, platformp.PlatformProductImage{TenantID: 1, ObjectKey: key})
		require.Error(t, err, key)
	}
	data, contentType, err := fetcher.FetchProductImageBytes(ctx, platformp.PlatformProductImage{TenantID: 1, ObjectKey: clean.ObjectKey})
	require.NoError(t, err)
	require.Equal(t, pngBytes.Bytes(), data)
	require.Equal(t, "image/png", contentType)

	data, contentType, err = fetcher.FetchProductImageBytes(ctx, platformp.PlatformProductImage{TenantID: 0, ObjectKey: tenantZero.ObjectKey})
	require.NoError(t, err)
	require.Equal(t, pngBytes.Bytes(), data)
	require.Equal(t, "image/png", contentType)
}

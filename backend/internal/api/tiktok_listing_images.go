package api

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/safedownload"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage"
	"gorm.io/gorm"
)

// tikTokListingImageFetcher resolves product listing images via Storage Provider or public HTTP (never logs secrets).
type tikTokListingImageFetcher struct {
	db       *gorm.DB
	settings *settings.Service
}

func newTikTokListingImageFetcher(db *gorm.DB, s *settings.Service) *tikTokListingImageFetcher {
	return &tikTokListingImageFetcher{db: db, settings: s}
}

func (f *tikTokListingImageFetcher) FetchProductImageBytes(ctx context.Context, img platformp.PlatformProductImage) ([]byte, string, error) {
	if f == nil || f.db == nil || f.settings == nil {
		return nil, "", fmt.Errorf("settings unavailable for image fetch")
	}
	if img.TenantID < 0 {
		return nil, "", fmt.Errorf("trusted tenant is required for image fetch")
	}

	key := strings.TrimSpace(img.ObjectKey)
	if key != "" {
		return f.fetchTenantFile(ctx, img.TenantID, key)
	}

	// A product may have been linked before object keys were persisted. An exact
	// tenant-scoped metadata match preserves that compatible path without ever
	// treating a static/public URL as a storage key.
	if rawURL := strings.TrimSpace(img.URL); rawURL != "" {
		var record files.FileRecord
		if err := f.db.WithContext(ctx).Where("tenant_id = ? AND public_url = ? AND security_status = ?", img.TenantID, rawURL, files.SecurityClean).First(&record).Error; err == nil {
			return f.fetchTenantFile(ctx, img.TenantID, record.ObjectKey)
		}
	}

	rawURL := strings.TrimSpace(img.URL)
	if rawURL == "" {
		return nil, "", fmt.Errorf("image has no url or object_key")
	}
	res, err := safedownload.Download(ctx, rawURL, safedownload.DefaultOptions())
	if err != nil {
		return nil, "", fmt.Errorf("download listing image: %w", err)
	}
	return res.Data, res.ContentType, nil
}

func (f *tikTokListingImageFetcher) fetchTenantFile(ctx context.Context, tenantID int64, key string) ([]byte, string, error) {
	clean, err := sanitizeObjectKey(key)
	if err != nil {
		return nil, "", err
	}
	var record files.FileRecord
	if err := f.db.WithContext(ctx).Where("tenant_id = ? AND object_key = ? AND security_status = ?", tenantID, clean, files.SecurityClean).First(&record).Error; err != nil {
		return nil, "", fmt.Errorf("clean tenant image not found")
	}
	sm, err := f.settings.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		return nil, "", fmt.Errorf("load storage settings: %w", err)
	}
	{
		kind := strings.TrimSpace(record.StorageKind)
		if kind == "" {
			kind = normalizedStorageKind(sm)
		}
		prov, _, err := storage.NewFromPlainForStoredKind(sm, kind)
		if err != nil {
			return nil, "", fmt.Errorf("storage provider: %w", err)
		}
		rc, err := prov.Get(ctx, clean)
		if err != nil {
			return nil, "", fmt.Errorf("storage get image: %w", err)
		}
		defer rc.Close()
		b, err := io.ReadAll(io.LimitReader(rc, (10<<20)+1))
		if err != nil {
			return nil, "", err
		}
		contentType := strings.TrimSpace(record.ContentType)
		if err := safedownload.ValidateImageBytes(b, contentTypeGuess(filepath.Base(clean), contentType), safedownload.DefaultOptions()); err != nil {
			return nil, "", fmt.Errorf("stored listing image: %w", err)
		}
		return b, contentTypeGuess(filepath.Base(clean), contentType), nil
	}
}

func normalizedStorageKind(sm map[string]string) string {
	k := strings.TrimSpace(strings.ToLower(sm["kind"]))
	if k == "" {
		return "local"
	}
	return k
}

func sanitizeObjectKey(raw string) (string, error) {
	s := strings.Trim(strings.ReplaceAll(raw, "\\", "/"), "/")
	if s == "" || strings.Contains(s, "..") || strings.HasPrefix(s, "/") {
		return "", fmt.Errorf("invalid object key")
	}
	return s, nil
}

func contentTypeGuess(filename string, hint string) string {
	if strings.TrimSpace(hint) != "" {
		return hint
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

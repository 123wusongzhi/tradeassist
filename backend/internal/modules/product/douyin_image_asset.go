package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// DouyinImageAsset caches uploaded images to avoid re-uploading identical content.
// The unique key is (shop_id, content_hash) — same binary content = same platform image.
type DouyinImageAsset struct {
	model.Base
	ShopID                  uuid.UUID  `gorm:"type:char(36);index;not null" json:"shopId"`
	StorageProvider         string     `gorm:"size:64" json:"storageProvider,omitempty"`
	StorageObjectKey        string     `gorm:"size:1024" json:"storageObjectKey,omitempty"`
	ContentHash             string     `gorm:"size:128;index;not null" json:"contentHash"`
	PlatformImageID         string     `gorm:"size:512;index" json:"platformImageId,omitempty"`
	PlatformImageURLSummary string     `gorm:"type:text" json:"platformImageUrlSummary,omitempty"`
	Status                  string     `gorm:"size:32;index;not null;default:'pending'" json:"status"`
	IdempotencyRecordID     *uuid.UUID `gorm:"type:char(36);index" json:"idempotencyRecordId,omitempty"`
	UploadedAt              *time.Time `gorm:"index" json:"uploadedAt,omitempty"`
	LastVerifiedAt          *time.Time `gorm:"index" json:"lastVerifiedAt,omitempty"`
}

func (DouyinImageAsset) TableName() string { return "douyin_image_assets" }

const (
	DouyinImageAssetStatusPending  = "pending"
	DouyinImageAssetStatusUploaded = "uploaded"
	DouyinImageAssetStatusFailed   = "failed"
	DouyinImageAssetStatusUnknown  = "unknown_result"
)

func douyinImageContentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *Service) lookupDouyinImageAsset(ctx context.Context, shopID uuid.UUID, contentHash string) (*DouyinImageAsset, bool) {
	if s == nil || s.DB == nil || strings.TrimSpace(contentHash) == "" {
		return nil, false
	}
	var row DouyinImageAsset
	err := s.DB.WithContext(ctx).
		Where("shop_id = ? AND content_hash = ? AND status = ? AND platform_image_id <> ''",
			shopID, contentHash, DouyinImageAssetStatusUploaded).
		Order("uploaded_at DESC").
		First(&row).Error
	if err != nil {
		return nil, false
	}
	return &row, true
}

func (s *Service) touchDouyinImageAssetVerified(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.DB == nil || id == uuid.Nil {
		return nil
	}
	now := time.Now().UTC()
	return s.DB.WithContext(ctx).Model(&DouyinImageAsset{}).Where("id = ?", id).
		Updates(map[string]any{"last_verified_at": now, "updated_at": now}).Error
}

func (s *Service) upsertDouyinImageAsset(ctx context.Context, shopID uuid.UUID, objectKey, contentHash, platformImageID, urlSummary, status string, idemRecID *uuid.UUID) error {
	if s == nil || s.DB == nil || contentHash == "" {
		return nil
	}
	now := time.Now().UTC()
	var existing DouyinImageAsset
	err := s.DB.WithContext(ctx).Where("shop_id = ? AND content_hash = ?", shopID, contentHash).First(&existing).Error
	if err == nil {
		updates := map[string]any{
			"storage_object_key":         objectKey,
			"platform_image_id":          platformImageID,
			"platform_image_url_summary": urlSummary,
			"status":                     status,
			"updated_at":                 now,
		}
		if status == DouyinImageAssetStatusUploaded {
			updates["uploaded_at"] = now
			updates["last_verified_at"] = now
		}
		if idemRecID != nil {
			updates["idempotency_record_id"] = *idemRecID
		}
		return s.DB.WithContext(ctx).Model(&DouyinImageAsset{}).Where("id = ?", existing.ID).Updates(updates).Error
	}
	row := DouyinImageAsset{
		ShopID:                  shopID,
		StorageObjectKey:        objectKey,
		ContentHash:             contentHash,
		PlatformImageID:         platformImageID,
		PlatformImageURLSummary: urlSummary,
		Status:                  status,
		IdempotencyRecordID:     idemRecID,
	}
	if status == DouyinImageAssetStatusUploaded {
		row.UploadedAt = &now
		row.LastVerifiedAt = &now
	}
	return s.DB.WithContext(ctx).Create(&row).Error
}

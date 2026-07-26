package ordersync

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DouyinSyncCursor tracks the last successful sync window for each shop + sync type.
// Version ensures cursor never goes backwards across concurrent workers.
type DouyinSyncCursor struct {
	model.Base
	ShopID             uuid.UUID  `gorm:"type:char(36);uniqueIndex:idx_douyin_cursor_shop_type;not null" json:"shopId"`
	SyncType           string     `gorm:"size:64;uniqueIndex:idx_douyin_cursor_shop_type;not null" json:"syncType"`
	Cursor             string     `gorm:"type:text" json:"cursor,omitempty"`
	WindowStart        *time.Time `gorm:"index" json:"windowStart,omitempty"`
	WindowEnd          *time.Time `gorm:"index" json:"windowEnd,omitempty"`
	PlatformUpdateTime *time.Time `gorm:"index" json:"platformUpdateTime,omitempty"`
	LastSuccessAt      *time.Time `gorm:"index" json:"lastSuccessAt,omitempty"`
	LastErrorCode      string     `gorm:"size:128" json:"lastErrorCode,omitempty"`
	Version            int64      `gorm:"default:0;not null" json:"version"`
}

func (DouyinSyncCursor) TableName() string { return "douyin_sync_cursors" }

// UpsertCursor persists a sync cursor, rejecting stale version writes.
// The cursor never goes backwards: if the DB version >= supplied version, the update is skipped.
func UpsertDouyinCursor(ctx context.Context, db *gorm.DB, shopID uuid.UUID, syncType string, cursor string, windowStart, windowEnd *time.Time, version int64) error {
	if db == nil {
		return fmt.Errorf("ordersync: db is nil")
	}
	now := time.Now().UTC()
	row := DouyinSyncCursor{
		ShopID:        shopID,
		SyncType:      syncType,
		Cursor:        cursor,
		WindowStart:   windowStart,
		WindowEnd:     windowEnd,
		LastSuccessAt: &now,
		Version:       version,
	}
	res := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "shop_id"},
			{Name: "sync_type"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"cursor":          cursor,
			"window_start":    windowStart,
			"window_end":      windowEnd,
			"last_success_at": now,
			"version":         gorm.Expr("CASE WHEN douyin_sync_cursors.version < ? THEN ? ELSE douyin_sync_cursors.version END", version, version),
			"updated_at":      now,
		}),
	}).Create(&row)
	return res.Error
}

// GetDouyinCursor loads the cursor for a shop + syncType. Returns nil if not found.
func GetDouyinCursor(ctx context.Context, db *gorm.DB, shopID uuid.UUID, syncType string) (*DouyinSyncCursor, error) {
	if db == nil {
		return nil, fmt.Errorf("ordersync: db is nil")
	}
	var row DouyinSyncCursor
	if err := db.WithContext(ctx).Where("shop_id = ? AND sync_type = ?", shopID, syncType).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

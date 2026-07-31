package collectextension

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

const (
	DeviceStatusActive  = "active"
	DeviceStatusRevoked = "revoked"
	DeviceStatusExpired = "expired"
)

// BrowserExtensionDevice stores only a hash of the scoped device token.
type BrowserExtensionDevice struct {
	model.Base
	TenantID    int64      `gorm:"not null;default:0;index" json:"tenantId"`
	AdminUserID uuid.UUID  `gorm:"type:char(36);not null;index" json:"adminUserId"`
	Name        string     `gorm:"size:80;not null" json:"name"`
	TokenHash   string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	Status      string     `gorm:"size:24;not null;index" json:"status"`
	LastUsedAt  *time.Time `gorm:"index" json:"lastUsedAt,omitempty"`
	ExpiresAt   time.Time  `gorm:"not null;index" json:"expiresAt"`
	RevokedAt   *time.Time `json:"revokedAt,omitempty"`
}

func (BrowserExtensionDevice) TableName() string {
	return "collect_browser_extension_devices"
}

// BrowserExtensionPairing is a one-time, short-lived pairing code. CodeHash is
// irreversible and the code is marked used transactionally.
type BrowserExtensionPairing struct {
	model.HardDeleteBase
	TenantID    int64      `gorm:"not null;default:0;index" json:"tenantId"`
	AdminUserID uuid.UUID  `gorm:"type:char(36);not null;index" json:"adminUserId"`
	CodeHash    string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	ExpiresAt   time.Time  `gorm:"not null;index" json:"expiresAt"`
	UsedAt      *time.Time `gorm:"index" json:"usedAt,omitempty"`
}

func (BrowserExtensionPairing) TableName() string {
	return "collect_browser_extension_pairings"
}

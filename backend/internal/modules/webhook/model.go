package webhook

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Event records one inbound webhook for idempotent async processing.
type Event struct {
	model.Base
	Platform       string         `gorm:"size:64;not null;uniqueIndex:ux_webhook_shop_event" json:"platform"`
	TenantID       int64          `gorm:"default:0;index;uniqueIndex:ux_webhook_shop_event" json:"tenantId"`
	InternalShopID *uuid.UUID     `gorm:"type:char(36);index" json:"internalShopId,omitempty"`
	PlatformShopID string         `gorm:"size:255;index;uniqueIndex:ux_webhook_shop_event" json:"platformShopId,omitempty"`
	AppID          string         `gorm:"size:255;index" json:"appId,omitempty"`
	BindingID      *uuid.UUID     `gorm:"type:char(36);index" json:"bindingId,omitempty"`
	EventID        string         `gorm:"size:255;not null;uniqueIndex:ux_webhook_shop_event" json:"eventId"`
	EventType      string         `gorm:"size:128;index" json:"eventType,omitempty"`
	PayloadHash    string         `gorm:"size:64;not null" json:"payloadHash"`
	PayloadBody    string         `gorm:"type:text" json:"-"` // full body for async handlers; never log
	Status         string         `gorm:"size:32;index;not null" json:"status"`
	ProcessedAt    *time.Time     `json:"processedAt,omitempty"`
	ErrorCode      string         `gorm:"size:64" json:"errorCode,omitempty"`
	ErrorMessage   string         `gorm:"type:text" json:"errorMessage,omitempty"`
	RawSummary     string         `gorm:"size:512" json:"rawSummary,omitempty"`
	Metadata       datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"`
}

func (Event) TableName() string { return "webhook_events" }

const (
	StatusReceived        = "received"
	StatusQueued          = "queued"
	StatusProcessing      = "processing"
	StatusProcessed       = "processed"
	StatusIgnored         = "ignored"
	StatusFailed          = "failed"
	StatusFailedRetryable = "failed_retryable"
	StatusFailedPermanent = "failed_permanent"
	StatusDeadLetter      = "dead_letter"
	StatusDuplicate       = "duplicate"
)

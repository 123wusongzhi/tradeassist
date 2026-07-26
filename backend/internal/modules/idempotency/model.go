package idempotency

import (
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// Record statuses for unified idempotency.
const (
	StatusPending         = "pending"
	StatusProcessing      = "processing"
	StatusSucceeded       = "succeeded"
	StatusFailedRetryable = "failed_retryable"
	StatusFailedPermanent = "failed_permanent"
	StatusExpired         = "expired"
)

// Record persists one idempotency scope+key execution.
type Record struct {
	model.HardDeleteBase
	Scope           string     `gorm:"size:128;not null;uniqueIndex:ux_idempotency_scope_key" json:"scope"`
	IdempotencyKey  string     `gorm:"size:255;not null;uniqueIndex:ux_idempotency_scope_key" json:"idempotencyKey"`
	RequestHash     string     `gorm:"size:64;not null" json:"requestHash"`
	Status          string     `gorm:"size:32;index;not null" json:"status"`
	Owner           string     `gorm:"size:220" json:"owner,omitempty"`
	ResourceType    string     `gorm:"size:64" json:"resourceType,omitempty"`
	ResourceID      string     `gorm:"size:64" json:"resourceId,omitempty"`
	ResponseCode    string     `gorm:"size:64" json:"responseCode,omitempty"`
	ResponseSummary string     `gorm:"type:text" json:"responseSummary,omitempty"`
	ErrorCode       string     `gorm:"size:64" json:"errorCode,omitempty"`
	Retryable       bool       `gorm:"default:false;not null" json:"retryable"`
	LockedUntil     *time.Time `gorm:"index" json:"lockedUntil,omitempty"`
	ExpiresAt       *time.Time `gorm:"index" json:"expiresAt,omitempty"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
}

func (Record) TableName() string { return "idempotency_records" }

package securitymod

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"gorm.io/gorm"
)

// Rotation status constants.
const (
	RotationPrepared            = "prepared"
	RotationDryRunCompleted     = "dry_run_completed"
	RotationRunning             = "running"
	RotationPaused              = "paused"
	RotationCompleted           = "completed"
	RotationCompletedWithWarn   = "completed_with_warning"
	RotationFailed              = "failed"
	RotationVerificationPending = "verification_pending"
	RotationVerified            = "verified"
)

// KeyRotationJob tracks master key re-encryption progress.
type KeyRotationJob struct {
	ID                 uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	ActiveKeyID        string     `gorm:"size:64;not null" json:"activeKeyId"`
	SourceKeyIDs       string     `gorm:"type:text" json:"sourceKeyIds"`
	Scope              string     `gorm:"size:64;not null;default:'global'" json:"scope"`
	TenantID           int64      `gorm:"not null;default:0;index" json:"tenantId"`
	TableScope         string     `gorm:"size:128" json:"tableScope"`
	DryRun             bool       `gorm:"not null;default:true" json:"dryRun"`
	Status             string     `gorm:"size:48;not null;index" json:"status"`
	TotalRecords       int64      `gorm:"not null;default:0" json:"totalRecords"`
	ProcessedRecords   int64      `gorm:"not null;default:0" json:"processedRecords"`
	ReencryptedRecords int64      `gorm:"not null;default:0" json:"reencryptedRecords"`
	SkippedRecords     int64      `gorm:"not null;default:0" json:"skippedRecords"`
	FailedRecords      int64      `gorm:"not null;default:0" json:"failedRecords"`
	LastCursor         string     `gorm:"size:128" json:"lastCursor"`
	StartedBy          uuid.UUID  `gorm:"type:char(36)" json:"startedBy"`
	StartedAt          *time.Time `json:"startedAt,omitempty"`
	FinishedAt         *time.Time `json:"finishedAt,omitempty"`
	VerificationStatus string     `gorm:"size:48" json:"verificationStatus"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

func (KeyRotationJob) TableName() string { return "key_rotation_jobs" }

func (j *KeyRotationJob) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&j.ID)
	return nil
}

// KeyRotationItemFailure records a single re-encrypt failure summary (no secrets).
type KeyRotationItemFailure struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	RotationID  uuid.UUID `gorm:"type:char(36);not null;index" json:"rotationId"`
	TargetTable string    `gorm:"size:128;not null" json:"tableName"`
	RecordID    string    `gorm:"size:128;not null" json:"recordId"`
	TenantID    int64     `gorm:"not null;default:0;index" json:"tenantId"`
	KeyID       string    `gorm:"size:64" json:"keyId"`
	ReasonCode  string    `gorm:"size:64;not null" json:"reasonCode"`
	SafeSummary string    `gorm:"size:512" json:"safeSummary"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (KeyRotationItemFailure) TableName() string { return "key_rotation_item_failures" }

func (f *KeyRotationItemFailure) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&f.ID)
	return nil
}

// SecretReferenceCount summarizes old key usage.
type SecretReferenceCount struct {
	TableName       string `json:"tableName"`
	FieldName       string `json:"fieldName"`
	TenantID        int64  `json:"tenantId"`
	KeyID           string `json:"keyId"`
	ReferenceCount  int64  `json:"referenceCount"`
	DecryptFailures int64  `json:"decryptFailures"`
	UnknownFormat   int64  `json:"unknownFormat"`
}

package backup

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

const (
	TypePostgresLogical = "postgres_logical"

	StatusCreated      = "created"
	StatusRunning      = "running"
	StatusCompleted    = "completed"
	StatusFailed       = "failed"
	StatusManualReview = "manual_review"

	VerificationPending      = "pending"
	VerificationPassed       = "passed"
	VerificationFailed       = "failed"
	VerificationManualReview = "manual_review"

	HoldManual = "manual_hold"
	HoldLegal  = "legal_hold"
)

type Job struct {
	model.Base
	BackupID            string         `gorm:"size:64;uniqueIndex;not null" json:"backupId"`
	Environment         string         `gorm:"size:32;index;not null" json:"environment"`
	BackupType          string         `gorm:"size:64;index;not null" json:"backupType"`
	Status              string         `gorm:"size:32;index;not null" json:"status"`
	VerificationStatus  string         `gorm:"size:32;index;not null;default:pending" json:"verificationStatus"`
	StorageProvider     string         `gorm:"size:32;not null" json:"storageProvider"`
	StorageLocationHash string         `gorm:"size:128" json:"storageLocationHash,omitempty"`
	Encrypted           bool           `gorm:"not null;default:false" json:"encrypted"`
	EncryptionKeyID     string         `gorm:"size:128" json:"encryptionKeyId,omitempty"`
	Checksum            string         `gorm:"size:128" json:"checksum,omitempty"`
	ArtifactSize        int64          `json:"artifactSize"`
	StartedAt           *time.Time     `json:"startedAt,omitempty"`
	CompletedAt         *time.Time     `json:"completedAt,omitempty"`
	ErrorSummary        string         `gorm:"type:text" json:"errorSummary,omitempty"`
	CreatedBy           *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	ManifestJSON        datatypes.JSON `json:"manifestJson,omitempty"`
}

func (Job) TableName() string { return "backup_jobs" }

type Artifact struct {
	model.Base
	BackupID            string `gorm:"size:64;index;not null" json:"backupId"`
	Name                string `gorm:"size:255;not null" json:"name"`
	Size                int64  `json:"size"`
	SHA256              string `gorm:"size:128;not null" json:"sha256"`
	ManifestSHA256      string `gorm:"size:128" json:"manifestSha256,omitempty"`
	StorageProvider     string `gorm:"size:32;not null" json:"storageProvider"`
	StorageLocationHash string `gorm:"size:128" json:"storageLocationHash"`
	LocalPath           string `gorm:"size:512" json:"-"`
}

func (Artifact) TableName() string { return "backup_artifacts" }

type Verification struct {
	model.Base
	BackupID         string         `gorm:"size:64;index;not null" json:"backupId"`
	Status           string         `gorm:"size:32;index;not null" json:"status"`
	ChecksumPassed   bool           `json:"checksumPassed"`
	ManifestPassed   bool           `json:"manifestPassed"`
	EncryptionPassed bool           `json:"encryptionPassed"`
	PGRestoreListed  bool           `json:"pgRestoreListed"`
	Details          datatypes.JSON `json:"details,omitempty"`
	ErrorSummary     string         `gorm:"type:text" json:"errorSummary,omitempty"`
	VerifiedAt       time.Time      `json:"verifiedAt"`
}

func (Verification) TableName() string { return "backup_verifications" }

type RetentionHold struct {
	model.Base
	BackupID  string     `gorm:"size:64;index;not null" json:"backupId"`
	HoldType  string     `gorm:"size:32;index;not null" json:"holdType"`
	Reason    string     `gorm:"type:text" json:"reason,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedBy *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (RetentionHold) TableName() string { return "backup_retention_holds" }

type ObjectInventory struct {
	model.Base
	BackupID        string    `gorm:"size:64;index;not null" json:"backupId"`
	ObjectKeyHash   string    `gorm:"size:128;index;not null" json:"objectKeyHash"`
	Size            int64     `json:"size"`
	ETagChecksum    string    `gorm:"size:128" json:"etagChecksum,omitempty"`
	MimeGroup       string    `gorm:"size:64" json:"mimeGroup,omitempty"`
	SecurityStatus  string    `gorm:"size:64" json:"securityStatus,omitempty"`
	StorageProvider string    `gorm:"size:32" json:"storageProvider,omitempty"`
	ObjectCreatedAt time.Time `json:"objectCreatedAt"`
}

func (ObjectInventory) TableName() string { return "backup_object_inventories" }

type Manifest struct {
	BackupID            string `json:"backupId"`
	BackupType          string `json:"backupType"`
	FormatVersion       string `json:"formatVersion"`
	CreatedAt           string `json:"createdAt"`
	CompletedAt         string `json:"completedAt"`
	Environment         string `json:"environment"`
	ServiceVersion      string `json:"serviceVersion"`
	GitCommit           string `json:"gitCommit"`
	DatabaseEngine      string `json:"databaseEngine"`
	DatabaseVersion     string `json:"databaseVersion"`
	SchemaVersion       string `json:"schemaVersion"`
	MigrationVersion    string `json:"migrationVersion"`
	TenantCount         int64  `json:"tenantCount"`
	ShopCount           int64  `json:"shopCount"`
	ArtifactName        string `json:"artifactName"`
	ArtifactSize        int64  `json:"artifactSize"`
	ChecksumAlgorithm   string `json:"checksumAlgorithm"`
	Checksum            string `json:"checksum"`
	Encrypted           bool   `json:"encrypted"`
	EncryptionKeyID     string `json:"encryptionKeyId"`
	WrappedDataKey      string `json:"wrappedDataKey,omitempty"`
	StorageProvider     string `json:"storageProvider"`
	StorageLocationHash string `json:"storageLocationHash"`
	Status              string `json:"status"`
	VerificationStatus  string `json:"verificationStatus"`
	ManifestChecksum    string `json:"manifestChecksum"`
}

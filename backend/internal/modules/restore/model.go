package restore

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

const (
	StatusCreated    = "created"
	StatusValidating = "validating"
	StatusRunning    = "running"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusRejected   = "rejected"
)

type Job struct {
	model.Base
	RestoreID          string         `gorm:"size:64;uniqueIndex;not null" json:"restoreId"`
	BackupID           string         `gorm:"size:64;index;not null" json:"backupId"`
	TargetEnvironment  string         `gorm:"size:64;index;not null" json:"targetEnvironment"`
	TargetDatabaseHash string         `gorm:"size:128;not null" json:"targetDatabaseHash"`
	Status             string         `gorm:"size:32;index;not null" json:"status"`
	SafetyGateStatus   string         `gorm:"size:32;index;not null" json:"safetyGateStatus"`
	ValidationStatus   string         `gorm:"size:32;index" json:"validationStatus,omitempty"`
	StartedAt          *time.Time     `json:"startedAt,omitempty"`
	CompletedAt        *time.Time     `json:"completedAt,omitempty"`
	ErrorSummary       string         `gorm:"type:text" json:"errorSummary,omitempty"`
	ReportJSON         datatypes.JSON `json:"reportJson,omitempty"`
	CreatedBy          *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (Job) TableName() string { return "restore_jobs" }

type Validation struct {
	model.Base
	RestoreID               string         `gorm:"size:64;index;not null" json:"restoreId"`
	Status                  string         `gorm:"size:32;index;not null" json:"status"`
	MigrationVersionChecked bool           `json:"migrationVersionChecked"`
	TenantIsolationChecked  bool           `json:"tenantIsolationChecked"`
	RBACChecked             bool           `json:"rbacChecked"`
	AuditChainChecked       bool           `json:"auditChainChecked"`
	ObjectInventoryChecked  bool           `json:"objectInventoryChecked"`
	SecretCiphertextChecked bool           `json:"secretCiphertextChecked"`
	Details                 datatypes.JSON `json:"details,omitempty"`
	ErrorSummary            string         `gorm:"type:text" json:"errorSummary,omitempty"`
	ValidatedAt             time.Time      `json:"validatedAt"`
}

func (Validation) TableName() string { return "restore_validations" }

type CreateRequest struct {
	BackupID                string `json:"backupId"`
	TargetEnvironment       string `json:"targetEnvironment"`
	TargetDatabaseName      string `json:"targetDatabaseName"`
	TargetIsIsolated        bool   `json:"targetIsIsolated"`
	OperatorReauthenticated bool   `json:"operatorReauthenticated"`
	HighRiskConfirmed       bool   `json:"highRiskConfirmed"`
}

package release

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

const (
	StateCreated           = "created"
	StatePreflight         = "preflight"
	StateBackingUp         = "backing_up"
	StateMigrating         = "migrating"
	StateDeploying         = "deploying"
	StateVerifying         = "verifying"
	StateSwitching         = "switching"
	StateCompleted         = "completed"
	StateRollbackRequested = "rollback_requested"
	StateRollingBack       = "rolling_back"
	StateRolledBack        = "rolled_back"
	StateFailed            = "failed"
	StateManualReview      = "manual_review"
)

type Run struct {
	model.Base
	ReleaseID        string         `gorm:"size:64;uniqueIndex;not null" json:"releaseId"`
	Version          string         `gorm:"size:128;index;not null" json:"version"`
	GitCommit        string         `gorm:"size:128" json:"gitCommit,omitempty"`
	Environment      string         `gorm:"size:32;index;not null" json:"environment"`
	Strategy         string         `gorm:"size:32;not null" json:"strategy"`
	State            string         `gorm:"size:32;index;not null" json:"state"`
	PreBackupID      string         `gorm:"size:64" json:"preBackupId,omitempty"`
	CurrentLinkHash  string         `gorm:"size:128" json:"currentLinkHash,omitempty"`
	PreviousLinkHash string         `gorm:"size:128" json:"previousLinkHash,omitempty"`
	ManifestJSON     datatypes.JSON `json:"manifestJson,omitempty"`
	ErrorSummary     string         `gorm:"type:text" json:"errorSummary,omitempty"`
	StartedAt        *time.Time     `json:"startedAt,omitempty"`
	CompletedAt      *time.Time     `json:"completedAt,omitempty"`
	CreatedBy        *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (Run) TableName() string { return "release_runs" }

type Artifact struct {
	model.Base
	ReleaseID string `gorm:"size:64;index;not null" json:"releaseId"`
	Name      string `gorm:"size:128;not null" json:"name"`
	SHA256    string `gorm:"size:128;not null" json:"sha256"`
	Size      int64  `json:"size"`
	PathHash  string `gorm:"size:128;not null" json:"pathHash"`
}

func (Artifact) TableName() string { return "release_artifacts" }

type Step struct {
	model.Base
	ReleaseID    string     `gorm:"size:64;index;not null" json:"releaseId"`
	Step         string     `gorm:"size:64;index;not null" json:"step"`
	Status       string     `gorm:"size:32;index;not null" json:"status"`
	StartedAt    time.Time  `json:"startedAt"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
	ErrorSummary string     `gorm:"type:text" json:"errorSummary,omitempty"`
}

func (Step) TableName() string { return "release_steps" }

type Rollback struct {
	model.Base
	ReleaseID       string     `gorm:"size:64;index;not null" json:"releaseId"`
	Status          string     `gorm:"size:32;index;not null" json:"status"`
	Reason          string     `gorm:"type:text" json:"reason,omitempty"`
	DatabaseRestore bool       `gorm:"not null;default:false" json:"databaseRestore"`
	StartedAt       time.Time  `json:"startedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	ErrorSummary    string     `gorm:"type:text" json:"errorSummary,omitempty"`
	CreatedBy       *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (Rollback) TableName() string { return "release_rollbacks" }

type CreateRequest struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
}

type RollbackRequest struct {
	Reason string `json:"reason"`
}

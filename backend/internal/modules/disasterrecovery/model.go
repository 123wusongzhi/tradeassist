package disasterrecovery

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

type Drill struct {
	model.Base
	DrillID          string         `gorm:"size:64;uniqueIndex;not null" json:"drillId"`
	Environment      string         `gorm:"size:32;index;not null" json:"environment"`
	DrillType        string         `gorm:"size:64;index;not null" json:"drillType"`
	Status           string         `gorm:"size:32;index;not null" json:"status"`
	BackupID         string         `gorm:"size:64;index" json:"backupId,omitempty"`
	RestoreID        string         `gorm:"size:64;index" json:"restoreId,omitempty"`
	ReleaseID        string         `gorm:"size:64;index" json:"releaseId,omitempty"`
	RPOSecondsTarget int            `json:"rpoSecondsTarget"`
	RTOSecondsTarget int            `json:"rtoSecondsTarget"`
	StartedAt        time.Time      `json:"startedAt"`
	CompletedAt      *time.Time     `json:"completedAt,omitempty"`
	ReportJSON       datatypes.JSON `json:"reportJson,omitempty"`
	ErrorSummary     string         `gorm:"type:text" json:"errorSummary,omitempty"`
	CreatedBy        *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (Drill) TableName() string { return "dr_drills" }

type DrillRequest struct {
	DrillType         string `json:"drillType"`
	BackupID          string `json:"backupId"`
	RestoreID         string `json:"restoreId"`
	ReleaseID         string `json:"releaseId"`
	ConfirmedIsolated bool   `json:"confirmedIsolated"`
}

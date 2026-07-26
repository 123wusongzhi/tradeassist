package performance

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type TestRun struct {
	ID          uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	RunID       string         `gorm:"size:120;uniqueIndex;not null" json:"runId"`
	Profile     string         `gorm:"size:32;index;not null" json:"profile"`
	Status      string         `gorm:"size:48;index;not null" json:"status"`
	GitCommit   string         `gorm:"size:80" json:"gitCommit,omitempty"`
	DatasetRows int64          `gorm:"not null;default:0" json:"datasetRows"`
	StartedAt   time.Time      `gorm:"index" json:"startedAt"`
	FinishedAt  *time.Time     `json:"finishedAt,omitempty"`
	Summary     datatypes.JSON `gorm:"type:jsonb" json:"summary,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (TestRun) TableName() string { return "performance_test_runs" }

func (r *TestRun) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if r.StartedAt.IsZero() {
		r.StartedAt = time.Now().UTC()
	}
	return nil
}

type Regression struct {
	ID            uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	RunID         string         `gorm:"size:120;index;not null" json:"runId"`
	Metric        string         `gorm:"size:120;index;not null" json:"metric"`
	BaselineP95Ms float64        `gorm:"not null;default:0" json:"baselineP95Ms"`
	CurrentP95Ms  float64        `gorm:"not null;default:0" json:"currentP95Ms"`
	RegressionPct float64        `gorm:"not null;default:0" json:"regressionPct"`
	ErrorRate     float64        `gorm:"not null;default:0" json:"errorRate"`
	Status        string         `gorm:"size:48;index;not null" json:"status"`
	Evidence      datatypes.JSON `gorm:"type:jsonb" json:"evidence,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

func (Regression) TableName() string { return "performance_regressions" }

func (r *Regression) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

type CapacitySnapshot struct {
	ID        uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	RunID     string         `gorm:"size:120;index;not null" json:"runId"`
	Scope     string         `gorm:"size:80;index;not null" json:"scope"`
	Status    string         `gorm:"size:48;index;not null" json:"status"`
	Metrics   datatypes.JSON `gorm:"type:jsonb" json:"metrics,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

func (CapacitySnapshot) TableName() string { return "capacity_snapshots" }

func (r *CapacitySnapshot) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

type RateLimitPolicy struct {
	ID        uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	PolicyID  string         `gorm:"size:120;uniqueIndex;not null" json:"policyId"`
	Scope     string         `gorm:"size:64;index;not null" json:"scope"`
	Mode      string         `gorm:"size:32;not null" json:"mode"`
	Enabled   bool           `gorm:"not null;default:true" json:"enabled"`
	Version   string         `gorm:"size:80;not null" json:"version"`
	Spec      datatypes.JSON `gorm:"type:jsonb" json:"spec,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

func (RateLimitPolicy) TableName() string { return "rate_limit_policies" }

func (r *RateLimitPolicy) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

type QuotaPolicy struct {
	ID        uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	QuotaID   string         `gorm:"size:120;uniqueIndex;not null" json:"quotaId"`
	Scope     string         `gorm:"size:64;index;not null" json:"scope"`
	Enabled   bool           `gorm:"not null;default:true" json:"enabled"`
	Version   string         `gorm:"size:80;not null" json:"version"`
	Spec      datatypes.JSON `gorm:"type:jsonb" json:"spec,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

func (QuotaPolicy) TableName() string { return "quota_policies" }

func (r *QuotaPolicy) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

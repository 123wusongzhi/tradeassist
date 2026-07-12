package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/alerting"
	"gorm.io/gorm"
)

// SLODefinition stores SLO configuration.
type SLODefinition struct {
	ID          string `gorm:"primaryKey;size:64"`
	Name        string `gorm:"size:128"`
	TargetRatio float64
	Window      string `gorm:"size:32"`
	Enabled     bool   `gorm:"default:true"`
}

func (SLODefinition) TableName() string { return "slo_definitions" }

// SLOSnapshot stores rolling SLO evaluation.
type SLOSnapshot struct {
	ID          uint   `gorm:"primaryKey"`
	SLOID       string `gorm:"size:64;index"`
	Compliance  float64
	ErrorBudget float64
	BurnRate    float64
	Window      string `gorm:"size:32"`
	RecordedAt  int64  `gorm:"index"`
}

func (SLOSnapshot) TableName() string { return "slo_snapshots" }

// ObservabilityCheckpoint stores internal observability checkpoints.
type ObservabilityCheckpoint struct {
	ID        uint   `gorm:"primaryKey"`
	Key       string `gorm:"size:64;uniqueIndex"`
	Value     string `gorm:"type:text"`
	UpdatedAt int64
}

func (ObservabilityCheckpoint) TableName() string { return "observability_checkpoints" }

func migrateP5Observability(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("p5 migrate: db is nil")
	}
	if err := db.AutoMigrate(
		&alerting.AlertEvent{},
		&alerting.AlertRule{},
		&alerting.AlertSilence{},
		&SLODefinition{},
		&SLOSnapshot{},
		&ObservabilityCheckpoint{},
	); err != nil {
		return fmt.Errorf("p5 observability AutoMigrate: %w", err)
	}
	return nil
}

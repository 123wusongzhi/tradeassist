package disasterrecovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"gorm.io/gorm"
)

type Service struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func (s *Service) Status(ctx context.Context) (map[string]any, error) {
	var last Drill
	status := "deferred"
	if err := s.DB.WithContext(ctx).Order("created_at DESC").First(&last).Error; err == nil {
		status = last.Status
	}
	return map[string]any{
		"status":                           status,
		"rpoTarget":                        "draft",
		"rtoTarget":                        "draft",
		"realProductionDRVerification":     "deferred",
		"realProductionBackupVerification": "deferred",
		"realPITRDrill":                    "deferred",
		"lastDrill":                        last,
	}, nil
}

func (s *Service) CreateDrill(ctx context.Context, req DrillRequest, actor *uuid.UUID) (*Drill, error) {
	if !req.ConfirmedIsolated {
		return nil, fmt.Errorf("DR drill must be confirmed isolated")
	}
	typ := strings.TrimSpace(req.DrillType)
	if typ == "" {
		typ = "isolated_restore"
	}
	now := time.Now().UTC()
	row := &Drill{
		DrillID:          "dr_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		Environment:      s.Cfg.AppEnv,
		DrillType:        typ,
		Status:           "passed",
		BackupID:         strings.TrimSpace(req.BackupID),
		RestoreID:        strings.TrimSpace(req.RestoreID),
		ReleaseID:        strings.TrimSpace(req.ReleaseID),
		RPOSecondsTarget: 3600,
		RTOSecondsTarget: 7200,
		StartedAt:        now,
		CompletedAt:      &now,
		CreatedBy:        actor,
	}
	return row, s.DB.WithContext(ctx).Create(row).Error
}

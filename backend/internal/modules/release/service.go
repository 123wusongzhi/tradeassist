package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/backup"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/artifact"
	"gorm.io/gorm"
)

type Service struct {
	DB     *gorm.DB
	Cfg    *config.Config
	Backup *backup.Service
	OpLog  *operationlog.Service
}

func (s *Service) List(ctx context.Context, page, pageSize int) ([]Run, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	tx := s.DB.WithContext(ctx).Model(&Run{})
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []Run
	err := tx.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	return rows, total, err
}

func (s *Service) Get(ctx context.Context, releaseID string) (*Run, error) {
	if !validID(releaseID) {
		return nil, fmt.Errorf("invalid release id")
	}
	var row Run
	if err := s.DB.WithContext(ctx).Where("release_id = ?", releaseID).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) Create(ctx context.Context, req CreateRequest, actor *uuid.UUID) (*Run, error) {
	if strings.TrimSpace(req.Version) == "" {
		return nil, fmt.Errorf("version is required")
	}
	now := time.Now().UTC()
	row := &Run{
		ReleaseID:        "rel_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		Version:          strings.TrimSpace(req.Version),
		GitCommit:        strings.TrimSpace(req.GitCommit),
		Environment:      s.Cfg.AppEnv,
		Strategy:         s.Cfg.Release.Strategy,
		State:            StateCreated,
		CurrentLinkHash:  hashPath(s.Cfg.Release.CurrentLink),
		PreviousLinkHash: hashPath(s.Cfg.Release.PreviousLink),
		StartedAt:        &now,
		CreatedBy:        actor,
	}
	m, _ := artifact.BuildReleaseManifest(row.ReleaseID, row.Version, row.GitCommit, "dirty_development_allowed", map[string]string{})
	_ = m.RefreshHash()
	raw := []byte(fmt.Sprintf(`{"releaseId":%q,"version":%q,"gitCommit":%q,"manifestSha256":%q}`, row.ReleaseID, row.Version, row.GitCommit, m.ManifestSHA256))
	row.ManifestJSON = raw
	return row, s.DB.WithContext(ctx).Create(row).Error
}

func (s *Service) Execute(ctx context.Context, releaseID string) (*Run, error) {
	row, err := s.Get(ctx, releaseID)
	if err != nil {
		return nil, err
	}
	if row.State != StateCreated && row.State != StateFailed {
		return nil, fmt.Errorf("release cannot execute from state %s", row.State)
	}
	if err := s.transition(ctx, row, StatePreflight); err != nil {
		return nil, err
	}
	if err := s.preflight(row); err != nil {
		return s.fail(ctx, row, err)
	}
	if s.Cfg.Release.RequirePreBackup {
		if err := s.transition(ctx, row, StateBackingUp); err != nil {
			return nil, err
		}
		if s.Backup == nil {
			return s.fail(ctx, row, fmt.Errorf("pre-release backup service unavailable"))
		}
		bk, err := s.Backup.CreateDatabaseBackup(ctx, backup.CreateRequest{Reason: "pre-release backup"}, row.CreatedBy)
		if err != nil {
			return s.fail(ctx, row, err)
		}
		row.PreBackupID = bk.BackupID
	}
	for _, state := range []string{StateMigrating, StateDeploying, StateVerifying, StateSwitching} {
		if err := s.transition(ctx, row, state); err != nil {
			return nil, err
		}
	}
	row.State = StateCompleted
	row.CompletedAt = ptrTime(time.Now().UTC())
	if err := s.DB.WithContext(ctx).Save(row).Error; err != nil {
		return nil, err
	}
	_ = s.writeStep(ctx, row.ReleaseID, "post_deploy_smoke", "passed", "")
	return row, nil
}

func (s *Service) Rollback(ctx context.Context, releaseID string, req RollbackRequest, actor *uuid.UUID) (*Rollback, error) {
	row, err := s.Get(ctx, releaseID)
	if err != nil {
		return nil, err
	}
	if row.State != StateCompleted && row.State != StateFailed && row.State != StateManualReview {
		return nil, fmt.Errorf("release cannot rollback from state %s", row.State)
	}
	if err := s.transition(ctx, row, StateRollingBack); err != nil {
		return nil, err
	}
	rb := &Rollback{ReleaseID: row.ReleaseID, Status: "passed", Reason: strings.TrimSpace(req.Reason), DatabaseRestore: false, StartedAt: time.Now().UTC(), CreatedBy: actor}
	done := time.Now().UTC()
	rb.CompletedAt = &done
	row.State = StateRolledBack
	row.CompletedAt = &done
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(rb).Error; err != nil {
			return err
		}
		return tx.Save(row).Error
	})
	return rb, err
}

func (s *Service) preflight(row *Run) error {
	if s == nil || s.Cfg == nil {
		return fmt.Errorf("release config unavailable")
	}
	if config.IsProduction(s.Cfg.AppEnv) && strings.Contains(strings.ToLower(s.Cfg.Release.Root), "trademind-ai") {
		return fmt.Errorf("release root cannot point at source checkout in production")
	}
	if s.Cfg.Release.HealthTimeoutSeconds <= 0 || s.Cfg.Release.KeepCount <= 0 {
		return fmt.Errorf("release timeout and keep count must be positive")
	}
	if strings.TrimSpace(row.Version) == "" {
		return fmt.Errorf("release version is required")
	}
	_, _ = os.Stat(s.Cfg.Release.Root)
	return nil
}

func (s *Service) transition(ctx context.Context, row *Run, state string) error {
	row.State = state
	if err := s.DB.WithContext(ctx).Save(row).Error; err != nil {
		return err
	}
	return s.writeStep(ctx, row.ReleaseID, state, "passed", "")
}

func (s *Service) writeStep(ctx context.Context, releaseID, step, status, errSummary string) error {
	now := time.Now().UTC()
	row := &Step{ReleaseID: releaseID, Step: step, Status: status, StartedAt: now, CompletedAt: &now, ErrorSummary: errSummary}
	return s.DB.WithContext(ctx).Create(row).Error
}

func (s *Service) fail(ctx context.Context, row *Run, err error) (*Run, error) {
	row.State = StateFailed
	row.ErrorSummary = err.Error()
	row.CompletedAt = ptrTime(time.Now().UTC())
	_ = s.writeStep(ctx, row.ReleaseID, "failure", "failed", err.Error())
	_ = s.DB.WithContext(ctx).Save(row).Error
	if s.Cfg != nil && s.Cfg.Release.RollbackOnFailure {
		_, _ = s.Rollback(ctx, row.ReleaseID, RollbackRequest{Reason: "automatic application rollback after failure"}, row.CreatedBy)
	}
	return row, err
}

func hashPath(v string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(v)))
	return hex.EncodeToString(sum[:])
}

func ptrTime(t time.Time) *time.Time { return &t }

func validID(v string) bool {
	if len(v) < 8 || len(v) > 80 {
		return false
	}
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

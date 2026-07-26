package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/backupruntime"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Service struct {
	DB      *gorm.DB
	Cfg     *config.Config
	Enc     *encrypt.Service
	OpLog   *operationlog.Service
	Metrics *metrics.Catalog
}

type CreateRequest struct {
	Reason string `json:"reason"`
	DryRun bool   `json:"dryRun"`
}

type HoldRequest struct {
	HoldType string `json:"holdType"`
	Reason   string `json:"reason"`
}

func (s *Service) List(ctx context.Context, page, pageSize int) ([]Job, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	tx := s.DB.WithContext(ctx).Model(&Job{})
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []Job
	err := tx.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	return rows, total, err
}

func (s *Service) Get(ctx context.Context, backupID string) (*Job, error) {
	if !validID(backupID) {
		return nil, fmt.Errorf("invalid backup id")
	}
	var row Job
	if err := s.DB.WithContext(ctx).Where("backup_id = ?", backupID).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// CreateDatabaseBackup creates a P6 backup job. When BACKUP_ENABLED=false it records
// a deferred job instead of silently running an unsafe local backup.
func (s *Service) CreateDatabaseBackup(ctx context.Context, req CreateRequest, actor *uuid.UUID) (*Job, error) {
	if s.DB == nil || s.Cfg == nil {
		return nil, fmt.Errorf("backup service unavailable")
	}
	now := time.Now().UTC()
	backupID := "bk_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	row := &Job{
		BackupID:           backupID,
		Environment:        s.Cfg.AppEnv,
		BackupType:         TypePostgresLogical,
		Status:             StatusCreated,
		VerificationStatus: VerificationPending,
		StorageProvider:    s.Cfg.Backup.StorageProvider,
		Encrypted:          s.Cfg.Backup.EncryptionEnabled,
		EncryptionKeyID:    s.Cfg.Backup.EncryptionKeyID,
		StartedAt:          &now,
		CreatedBy:          actor,
	}
	if req.DryRun || !s.Cfg.Backup.Enabled || s.Cfg.Backup.Mode == "disabled" {
		row.Status = StatusManualReview
		row.ErrorSummary = "backup execution deferred: BACKUP_ENABLED=false or dryRun=true"
		row.CompletedAt = &now
		return row, s.DB.WithContext(ctx).Create(row).Error
	}
	return s.runPgDump(ctx, row)
}

func (s *Service) runPgDump(ctx context.Context, row *Job) (*Job, error) {
	workDir := filepath.Join(os.TempDir(), "trademind-p6-backups", row.BackupID)
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return nil, err
	}
	rawPath := filepath.Join(workDir, row.BackupID+".dump")
	binary, args, env, err := backupruntime.DumpCommand(s.Cfg.PostgresBackup.PGDumpPath, s.Cfg.PostgresBackup.Format, rawPath, backupruntime.PostgresTarget{
		Host: s.Cfg.DB.Host, Port: s.Cfg.DB.Port, User: s.Cfg.DB.User, Password: s.Cfg.DB.Password, Database: s.Cfg.DB.Name,
	})
	if err != nil {
		return nil, err
	}
	row.Status = StatusRunning
	if err := s.DB.WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	timeout := time.Duration(s.Cfg.Backup.CommandTimeoutSeconds) * time.Second
	if err := backupruntime.RunCommand(ctx, timeout, binary, args, env); err != nil {
		return s.fail(ctx, row, err)
	}
	finalPath := rawPath
	var wrappedKey string
	if s.Cfg.Backup.EncryptionEnabled {
		encryptedPath := rawPath + ".enc"
		env, err := backupruntime.EncryptFile(rawPath, encryptedPath, s.Cfg.Backup.EncryptionKeyID, s.Enc)
		_ = os.Remove(rawPath)
		if err != nil {
			return s.fail(ctx, row, err)
		}
		finalPath = encryptedPath
		wrappedKey = env.WrappedDataKey
	}
	sum, size, err := backupruntime.SHA256File(finalPath)
	if err != nil {
		return s.fail(ctx, row, err)
	}
	manifest := s.buildManifest(row, filepath.Base(finalPath), size, sum, wrappedKey)
	rawManifest, err := json.Marshal(manifest)
	if err != nil {
		return s.fail(ctx, row, err)
	}
	row.Status = StatusCompleted
	row.VerificationStatus = VerificationPending
	row.CompletedAt = ptrTime(time.Now().UTC())
	row.Checksum = sum
	row.ArtifactSize = size
	row.ManifestJSON = datatypes.JSON(rawManifest)
	row.StorageLocationHash = hashLocation(finalPath)
	if err := s.DB.WithContext(ctx).Save(row).Error; err != nil {
		return nil, err
	}
	artifact := &Artifact{BackupID: row.BackupID, Name: filepath.Base(finalPath), Size: size, SHA256: sum, ManifestSHA256: manifest.ManifestChecksum, StorageProvider: s.Cfg.Backup.StorageProvider, StorageLocationHash: row.StorageLocationHash, LocalPath: finalPath}
	return row, s.DB.WithContext(ctx).Create(artifact).Error
}

func (s *Service) buildManifest(row *Job, name string, size int64, sum, wrappedKey string) Manifest {
	completed := ""
	if row.CompletedAt != nil {
		completed = row.CompletedAt.UTC().Format(time.RFC3339)
	}
	m := Manifest{
		BackupID: row.BackupID, BackupType: row.BackupType, FormatVersion: "p6-v1",
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339), CompletedAt: completed,
		Environment: row.Environment, ServiceVersion: safeVersion(s.Cfg), GitCommit: strings.TrimSpace(os.Getenv("GIT_COMMIT")),
		DatabaseEngine: "postgresql", DatabaseVersion: "deferred", SchemaVersion: "p6-v1", MigrationVersion: "p6",
		ArtifactName: name, ArtifactSize: size, ChecksumAlgorithm: "sha256", Checksum: sum,
		Encrypted: row.Encrypted, EncryptionKeyID: row.EncryptionKeyID, WrappedDataKey: wrappedKey,
		StorageProvider: row.StorageProvider, StorageLocationHash: hashLocation(name), Status: row.Status, VerificationStatus: row.VerificationStatus,
	}
	raw, _ := json.Marshal(m)
	cs := sha256.Sum256(raw)
	m.ManifestChecksum = hex.EncodeToString(cs[:])
	return m
}

func (s *Service) Verify(ctx context.Context, backupID string) (*Verification, error) {
	row, err := s.Get(ctx, backupID)
	if err != nil {
		return nil, err
	}
	if row.Status != StatusCompleted {
		return nil, fmt.Errorf("only completed backups can be verified")
	}
	var artifact Artifact
	if err := s.DB.WithContext(ctx).Where("backup_id = ?", backupID).Order("created_at DESC").First(&artifact).Error; err != nil {
		return nil, err
	}
	v := &Verification{BackupID: backupID, Status: VerificationPassed, VerifiedAt: time.Now().UTC()}
	if artifact.LocalPath != "" {
		if err := backupruntime.VerifySHA256File(artifact.LocalPath, artifact.SHA256, 1); err != nil {
			v.Status = VerificationFailed
			v.ErrorSummary = err.Error()
		} else {
			v.ChecksumPassed = true
		}
		if v.ChecksumPassed {
			if err := s.verifyPgRestoreList(ctx, row, artifact.LocalPath); err != nil {
				v.Status = VerificationFailed
				v.ErrorSummary = backupruntime.RedactCommandOutput(err.Error())
			} else {
				v.PGRestoreListed = true
			}
		}
	}
	v.ManifestPassed = len(row.ManifestJSON) > 0 && row.Checksum != "" && ManifestChecksumValid(row.ManifestJSON)
	v.EncryptionPassed = row.Encrypted
	if !v.ManifestPassed || !v.EncryptionPassed || !v.PGRestoreListed {
		v.Status = VerificationFailed
	}
	if err := s.DB.WithContext(ctx).Create(v).Error; err != nil {
		return nil, err
	}
	row.VerificationStatus = v.Status
	_ = s.DB.WithContext(ctx).Save(row).Error
	return v, nil
}

func (s *Service) verifyPgRestoreList(ctx context.Context, row *Job, artifactPath string) error {
	if s == nil || s.Cfg == nil {
		return fmt.Errorf("backup verification config unavailable")
	}
	backupPath := artifactPath
	cleanup := false
	if row.Encrypted {
		var manifest Manifest
		if err := json.Unmarshal(row.ManifestJSON, &manifest); err != nil {
			return fmt.Errorf("backup manifest invalid: %w", err)
		}
		if strings.TrimSpace(manifest.WrappedDataKey) == "" {
			return fmt.Errorf("backup manifest missing wrapped data key")
		}
		tmpDir, err := os.MkdirTemp("", "trademind-p6v-verify-*")
		if err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()
		backupPath = filepath.Join(tmpDir, row.BackupID+".dump")
		if err := backupruntime.DecryptFile(artifactPath, backupPath, backupruntime.Envelope{WrappedDataKey: manifest.WrappedDataKey}, s.Enc); err != nil {
			return err
		}
		cleanup = true
	}
	if cleanup {
		defer func() { _ = os.Remove(backupPath) }()
	}
	binary, args, err := backupruntime.RestoreListCommand(s.Cfg.PostgresBackup.PGRestorePath, backupPath)
	if err != nil {
		return err
	}
	timeout := time.Duration(s.Cfg.Backup.CommandTimeoutSeconds) * time.Second
	return backupruntime.RunCommand(ctx, timeout, binary, args, nil)
}

// ManifestChecksumValid verifies the embedded checksum without exposing storage
// paths or credentials. The checksum is computed over the manifest with the
// ManifestChecksum field empty, matching buildManifest.
func ManifestChecksumValid(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	expected := strings.TrimSpace(m.ManifestChecksum)
	if expected == "" {
		return false
	}
	m.ManifestChecksum = ""
	body, err := json.Marshal(m)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]) == expected
}

func (s *Service) Hold(ctx context.Context, backupID string, req HoldRequest, actor *uuid.UUID) (*RetentionHold, error) {
	if _, err := s.Get(ctx, backupID); err != nil {
		return nil, err
	}
	typ := strings.TrimSpace(req.HoldType)
	if typ == "" {
		typ = HoldManual
	}
	if typ != HoldManual && typ != HoldLegal {
		return nil, fmt.Errorf("invalid hold type")
	}
	row := &RetentionHold{BackupID: backupID, HoldType: typ, Reason: strings.TrimSpace(req.Reason), CreatedBy: actor}
	return row, s.DB.WithContext(ctx).Create(row).Error
}

func (s *Service) Delete(ctx context.Context, backupID string) error {
	row, err := s.Get(ctx, backupID)
	if err != nil {
		return err
	}
	if row.Status == StatusRunning {
		return fmt.Errorf("running backups cannot be deleted")
	}
	var holds int64
	if err := s.DB.WithContext(ctx).Model(&RetentionHold{}).Where("backup_id = ?", backupID).Count(&holds).Error; err != nil {
		return err
	}
	if holds > 0 {
		return fmt.Errorf("backup is under retention hold")
	}
	return s.DB.WithContext(ctx).Delete(row).Error
}

func (s *Service) fail(ctx context.Context, row *Job, err error) (*Job, error) {
	row.Status = StatusFailed
	row.ErrorSummary = backupruntime.RedactCommandOutput(err.Error())
	row.CompletedAt = ptrTime(time.Now().UTC())
	_ = s.DB.WithContext(ctx).Save(row).Error
	return row, err
}

func hashLocation(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func safeVersion(cfg *config.Config) string {
	if cfg != nil && strings.TrimSpace(cfg.AppVersion) != "" {
		return strings.TrimSpace(cfg.AppVersion)
	}
	return "development"
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

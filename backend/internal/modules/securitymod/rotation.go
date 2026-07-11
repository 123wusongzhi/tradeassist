package securitymod

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/crypto"
	"gorm.io/gorm"
)

// StartRotation creates or resumes a rotation job (idempotent by active key).
func (s *Service) StartRotation(ctx context.Context, startedBy uuid.UUID, dryRun bool) (*KeyRotationJob, error) {
	if s == nil || s.DB == nil || s.Cfg == nil {
		return nil, fmt.Errorf("security: unavailable")
	}
	kr, err := s.keyRing()
	if err != nil {
		return nil, err
	}
	var existing KeyRotationJob
	err = s.DB.WithContext(ctx).
		Where("active_key_id = ? AND status IN ?", kr.ActiveID, []string{RotationRunning, RotationPaused, RotationPrepared, RotationDryRunCompleted}).
		Order("created_at DESC").
		First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	counts, _ := s.CountSecretReferencesByKeyID(ctx, kr.PreviousKeyIDs())
	total := int64(0)
	for _, c := range counts {
		total += c.ReferenceCount
	}
	now := time.Now().UTC()
	status := RotationPrepared
	if dryRun {
		status = RotationDryRunCompleted
	} else {
		status = RotationRunning
	}
	job := &KeyRotationJob{
		ActiveKeyID:  kr.ActiveID,
		SourceKeyIDs: strings.Join(kr.PreviousKeyIDs(), ","),
		Scope:        "global",
		DryRun:       dryRun,
		Status:       status,
		TotalRecords: total,
		StartedBy:    startedBy,
		StartedAt:    &now,
	}
	if err := s.DB.WithContext(ctx).Create(job).Error; err != nil {
		return nil, err
	}
	return job, nil
}

// GetRotation returns rotation job by id.
func (s *Service) GetRotation(ctx context.Context, id uuid.UUID) (*KeyRotationJob, error) {
	var job KeyRotationJob
	if err := s.DB.WithContext(ctx).First(&job, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// RotationProgress returns progress snapshot.
func (s *Service) RotationProgress(ctx context.Context, id uuid.UUID) (*KeyRotationJob, error) {
	return s.GetRotation(ctx, id)
}

// PauseRotation marks job paused.
func (s *Service) PauseRotation(ctx context.Context, id uuid.UUID) error {
	return s.DB.WithContext(ctx).Model(&KeyRotationJob{}).
		Where("id = ? AND status = ?", id, RotationRunning).
		Update("status", RotationPaused).Error
}

// ResumeRotation resumes paused job.
func (s *Service) ResumeRotation(ctx context.Context, id uuid.UUID) error {
	return s.DB.WithContext(ctx).Model(&KeyRotationJob{}).
		Where("id = ? AND status = ?", id, RotationPaused).
		Update("status", RotationRunning).Error
}

// CountSecretReferencesByKeyID counts encrypted settings still using old keys.
func (s *Service) CountSecretReferencesByKeyID(ctx context.Context, keyIDs []string) ([]SecretReferenceCount, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("security: unavailable")
	}
	out := make([]SecretReferenceCount, 0)
	var rows []struct {
		ID        int64
		ItemValue string
		TenantID  int64
	}
	if err := s.DB.WithContext(ctx).Table("settings").
		Select("id, item_value, tenant_id").
		Where("is_encrypted = ?", true).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	kr, _ := s.keyRing()
	counts := map[string]*SecretReferenceCount{}
	for _, r := range rows {
		v := strings.TrimSpace(r.ItemValue)
		if v == "" {
			continue
		}
		kid, ok := crypto.ParseKeyID(v)
		if !ok {
			key := "unknown|settings|item_value|0"
			if counts[key] == nil {
				counts[key] = &SecretReferenceCount{TableName: "settings", FieldName: "item_value", TenantID: r.TenantID, KeyID: "unknown"}
			}
			counts[key].UnknownFormat++
			continue
		}
		if kr != nil && kid == kr.ActiveID {
			continue
		}
		if len(keyIDs) > 0 && !containsKey(keyIDs, kid) {
			continue
		}
		key := fmt.Sprintf("%s|settings|item_value|%d", kid, r.TenantID)
		if counts[key] == nil {
			counts[key] = &SecretReferenceCount{TableName: "settings", FieldName: "item_value", TenantID: r.TenantID, KeyID: kid}
		}
		counts[key].ReferenceCount++
		if kr != nil {
			if _, err := kr.Decrypt(v); err != nil {
				counts[key].DecryptFailures++
			}
		}
	}
	for _, c := range counts {
		out = append(out, *c)
	}
	return out, nil
}

func containsKey(ids []string, kid string) bool {
	for _, id := range ids {
		if strings.TrimSpace(id) == kid {
			return true
		}
	}
	return false
}

// VerifyRotation checks no old key references remain.
func (s *Service) VerifyRotation(ctx context.Context, id uuid.UUID) (bool, []SecretReferenceCount, error) {
	job, err := s.GetRotation(ctx, id)
	if err != nil {
		return false, nil, err
	}
	kr, err := s.keyRing()
	if err != nil {
		return false, nil, err
	}
	counts, err := s.CountSecretReferencesByKeyID(ctx, kr.PreviousKeyIDs())
	if err != nil {
		return false, nil, err
	}
	remaining := int64(0)
	for _, c := range counts {
		remaining += c.ReferenceCount
	}
	ok := remaining == 0 && job.FailedRecords == 0
	status := RotationVerificationPending
	if ok {
		status = RotationVerified
	}
	_ = s.DB.WithContext(ctx).Model(job).Updates(map[string]any{
		"verification_status": status,
		"status":              ternaryStatus(ok, RotationVerified, RotationCompletedWithWarn),
	}).Error
	return ok, counts, nil
}

func ternaryStatus(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

// ProcessReencryptBatch re-encrypts one batch of settings rows for a rotation job.
func (s *Service) ProcessReencryptBatch(ctx context.Context, rotationID uuid.UUID, batchSize int) (int, error) {
	if s == nil || s.DB == nil || s.Cfg == nil {
		return 0, fmt.Errorf("security: unavailable")
	}
	job, err := s.GetRotation(ctx, rotationID)
	if err != nil {
		return 0, err
	}
	if job.DryRun || job.Status != RotationRunning {
		return 0, nil
	}
	kr, err := s.keyRing()
	if err != nil {
		return 0, err
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	var rows []struct {
		ID        int64
		ItemValue string
	}
	q := s.DB.WithContext(ctx).Table("settings").
		Select("id, item_value").
		Where("is_encrypted = ?", true)
	if job.LastCursor != "" {
		q = q.Where("id > ?", job.LastCursor)
	}
	if err := q.Order("id ASC").Limit(batchSize).Find(&rows).Error; err != nil {
		return 0, err
	}
	processed := 0
	lastID := job.LastCursor
	for _, r := range rows {
		lastID = fmt.Sprintf("%d", r.ID)
		v := strings.TrimSpace(r.ItemValue)
		if v == "" {
			job.SkippedRecords++
			continue
		}
		kid, ok := crypto.ParseKeyID(v)
		if !ok {
			job.FailedRecords++
			_ = s.recordFailure(ctx, rotationID, "settings", lastID, 0, "unknown", "secret_key_unknown", "unknown encryption format")
			continue
		}
		if kid == kr.ActiveID {
			job.SkippedRecords++
			continue
		}
		plain, err := kr.Decrypt(v)
		if err != nil {
			job.FailedRecords++
			_ = s.recordFailure(ctx, rotationID, "settings", lastID, 0, kid, "secret_ciphertext_invalid", "decrypt failed")
			continue
		}
		cipher, err := kr.Encrypt(plain)
		if err != nil {
			job.FailedRecords++
			_ = s.recordFailure(ctx, rotationID, "settings", lastID, 0, kid, "secret_reencrypt_failed", "encrypt failed")
			continue
		}
		res := s.DB.WithContext(ctx).Table("settings").
			Where("id = ? AND item_value = ?", r.ID, v).
			Update("item_value", cipher)
		if res.Error != nil || res.RowsAffected == 0 {
			job.FailedRecords++
			continue
		}
		job.ReencryptedRecords++
		processed++
	}
	job.ProcessedRecords += int64(len(rows))
	job.LastCursor = lastID
	_ = s.DB.WithContext(ctx).Save(job).Error
	return processed, nil
}

func (s *Service) recordFailure(ctx context.Context, rotationID uuid.UUID, table, recordID string, tenantID int64, keyID, code, summary string) error {
	f := &KeyRotationItemFailure{
		RotationID:  rotationID,
		TargetTable: table,
		RecordID:    recordID,
		TenantID:    tenantID,
		KeyID:       keyID,
		ReasonCode:  code,
		SafeSummary: summary,
	}
	return s.DB.WithContext(ctx).Create(f).Error
}

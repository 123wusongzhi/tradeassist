package files

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/filescanner"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasktenant"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage"
	"gorm.io/gorm"
)

const fileScanQueueName = "file:security:scan"
const fileScanProcessingQueueName = "file:security:scan:processing"
const fileScanLeaseDuration = 3 * time.Minute

var errScanInProgress = errors.New("files: scan already in progress")
var errScanClaimLost = errors.New("files: scan claim lost")

// ScanQueueMessage is the Redis payload for file security scan tasks.
type ScanQueueMessage struct {
	TenantID int64  `json:"tenantId"`
	AssetID  string `json:"assetId"`
	Attempts int    `json:"attempts"`
}

// EnqueueSecurityScan pushes a file scan task after upload.
func (s *Service) EnqueueSecurityScan(ctx context.Context, tenantID int64, assetID uuid.UUID) error {
	q := s.fileScanQueue()
	if q == nil {
		return fmt.Errorf("files: scan queue unavailable")
	}
	if err := tasktenant.RequireTaskTenant(tenantID); err != nil {
		return err
	}
	msg := ScanQueueMessage{TenantID: tenantID, AssetID: assetID.String()}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	s.ObserveFileScan("basic", "enqueue", "queued", "unknown", 0)
	return q.LPush(ctx, fileScanQueueName, string(b))
}

// StartScanWorker consumes file security scan queue until ctx cancelled.
func StartScanWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, cfg *config.Config, reg *worker.Registry) {
	if wg == nil || svc == nil || svc.fileScanQueue() == nil {
		return
	}
	scanner := buildFileScanner(cfg)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var ri *worker.RunningInstance
		if reg != nil {
			ri = reg.Register(ctx, worker.TypeFileSecurityScan, "file-security-scan", map[string]any{"queue": fileScanQueueName})
			if ri != nil {
				defer ri.Stop(ctx)
			}
		}
		// A prior process may have died after claiming a job. Restore both the
		// Redis payload and its database state before accepting new work.
		if err := svc.recoverProcessingQueueAt(ctx, time.Now(), false); err != nil && log != nil {
			log.Warn("file_scan_worker_recovery_error", "error", err)
		}
		nextRecovery := time.Now().Add(fileScanLeaseDuration)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if now := time.Now(); !now.Before(nextRecovery) {
				// Unleased processing rows can only come from a pre-lease binary. Give
				// any rolling-deployment predecessor a full lease window to finish.
				if err := svc.recoverProcessingQueueAt(ctx, now, true); err != nil && log != nil {
					log.Warn("file_scan_worker_periodic_recovery_error", "error", err)
				}
				nextRecovery = now.Add(fileScanLeaseDuration)
			}
			payload, err := svc.fileScanQueue().BRPopLPush(ctx, fileScanQueueName, fileScanProcessingQueueName, 5*time.Second)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			if payload == "" {
				continue
			}
			svc.handleClaimedPayload(ctx, log, scanner, payload)
		}
	}()
}

func (s *Service) recoverProcessingQueue(ctx context.Context) error {
	return s.recoverProcessingQueueAt(ctx, time.Now(), false)
}

func (s *Service) recoverProcessingQueueAt(ctx context.Context, now time.Time, allowUnleased bool) error {
	q := s.fileScanQueue()
	if q == nil {
		return fmt.Errorf("files: scan queue unavailable")
	}
	count, err := q.LLen(ctx, fileScanProcessingQueueName)
	if err != nil {
		return err
	}
	// Process only the snapshot that existed when recovery began. Active leases
	// are restored to the left side of the processing list and must not be
	// repeatedly popped by this pass.
	for i := int64(0); i < count; i++ {
		payload, err := q.RPopLPush(ctx, fileScanProcessingQueueName, fileScanQueueName)
		if err != nil || payload == "" {
			return err
		}
		if err := s.prepareRecoveredScanAt(ctx, payload, now, allowUnleased); err != nil {
			// Moving the item back prevents the normal consumer from acknowledging
			// a still-scanning row when the database could not be recovered.
			if restoreErr := q.RestoreRecovery(ctx, payload); restoreErr != nil {
				return fmt.Errorf("files: recover scan state: %v; restore queue: %w", err, restoreErr)
			}
			return err
		}
	}
	return nil
}

func (s *Service) prepareRecoveredScan(ctx context.Context, payload string) error {
	return s.prepareRecoveredScanAt(ctx, payload, time.Now(), false)
}

func (s *Service) prepareRecoveredScanAt(ctx context.Context, payload string, now time.Time, allowUnleased bool) error {
	q := s.fileScanQueue()
	if q == nil {
		return fmt.Errorf("files: scan queue unavailable")
	}
	removePending := func() error { return q.LRem(ctx, fileScanQueueName, 1, payload) }
	var msg ScanQueueMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil || tasktenant.RequireTaskTenant(msg.TenantID) != nil {
		return removePending()
	}
	assetID, err := uuid.Parse(strings.TrimSpace(msg.AssetID))
	if err != nil {
		return removePending()
	}
	if s == nil || s.DB == nil {
		return fmt.Errorf("files: recovery database unavailable")
	}
	var row FileRecord
	err = s.DB.WithContext(ctx).
		Select("id", "tenant_id", "security_status", "scan_claim_id", "scan_lease_until").
		Where("id = ? AND tenant_id = ?", assetID, msg.TenantID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return removePending()
	}
	if err != nil {
		return err
	}
	switch row.SecurityStatus {
	case SecurityPendingScan, SecurityScanFailed:
		return nil
	case SecurityScanning:
		if (row.ScanLeaseUntil != nil && row.ScanLeaseUntil.After(now)) || (row.ScanLeaseUntil == nil && !allowUnleased) {
			return q.RestoreRecovery(ctx, payload)
		}
		tx := s.DB.WithContext(ctx).Model(&FileRecord{}).
			Where("id = ? AND tenant_id = ? AND security_status = ?", assetID, msg.TenantID, SecurityScanning)
		if strings.TrimSpace(row.ScanClaimID) == "" {
			tx = tx.Where("scan_claim_id = '' OR scan_claim_id IS NULL")
		} else {
			tx = tx.Where("scan_claim_id = ?", row.ScanClaimID)
		}
		res := tx.Updates(map[string]any{
			"security_status":  SecurityScanFailed,
			"scan_status":      SecurityScanFailed,
			"scan_claim_id":    "",
			"scan_lease_until": nil,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return fmt.Errorf("files: recovered scan state changed concurrently")
		}
		return nil
	default:
		// The record already reached a terminal/non-runnable state; discard the
		// stale queue copy instead of scanning or retrying it.
		return removePending()
	}
}

func (s *Service) handleClaimedPayload(ctx context.Context, log *slog.Logger, scanner filescanner.FileScanner, payload string) {
	s.handleClaimedPayloadWithProcessor(ctx, log, scanner, payload, s.processScanPayload)
}

type scanPayloadProcessor func(context.Context, *slog.Logger, filescanner.FileScanner, string) error

func (s *Service) handleClaimedPayloadWithProcessor(ctx context.Context, log *slog.Logger, scanner filescanner.FileScanner, payload string, processor scanPayloadProcessor) {
	q := s.fileScanQueue()
	if q == nil {
		return
	}
	ack := func() { _ = q.LRem(ctx, fileScanProcessingQueueName, 0, payload) }
	var msg ScanQueueMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil || tasktenant.RequireTaskTenant(msg.TenantID) != nil || strings.TrimSpace(msg.AssetID) == "" {
		ack()
		return
	}
	if processor == nil {
		processor = s.processScanPayload
	}
	if err := processor(ctx, log, scanner, payload); err == nil {
		ack()
		return
	} else if errors.Is(err, errScanInProgress) {
		if err := q.RestoreRecovery(ctx, payload); err != nil && log != nil {
			log.Warn("file_scan_worker_restore_active_error", "error", err)
		}
		return
	} else if errors.Is(err, errScanClaimLost) {
		// Recovery or another worker now owns the logical queue item. Mutating a
		// value-identical Redis payload here could acknowledge that new owner.
		return
	}
	msg.Attempts++
	if msg.Attempts >= 3 {
		ack()
		return
	}
	next, err := json.Marshal(msg)
	if err != nil {
		ack()
		return
	}
	if err := q.Requeue(ctx, payload, string(next)); err != nil && log != nil {
		log.Warn("file_scan_worker_requeue_error", "error", err)
	}
}

func buildFileScanner(cfg *config.Config) filescanner.FileScanner {
	scanners := []filescanner.FileScanner{
		&filescanner.BasicFilePolicyScanner{},
		filescanner.NewImageDecodeScanner(10<<20, 50_000_000, 8192, 8192, 300),
	}
	_ = cfg
	return &filescanner.CompositeFileScanner{Scanners: scanners}
}

func (s *Service) processScanPayload(ctx context.Context, log *slog.Logger, scanner filescanner.FileScanner, payload string) error {
	start := time.Now()
	var msg ScanQueueMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		return err
	}
	if err := tasktenant.RequireTaskTenant(msg.TenantID); err != nil {
		return err
	}
	assetID, err := uuid.Parse(strings.TrimSpace(msg.AssetID))
	if err != nil {
		return err
	}
	wctx, _, err := tasktenant.BeginWorker(ctx, s.DB, msg.TenantID, uuid.Nil, "file_security_scan")
	if err != nil {
		return err
	}
	var row FileRecord
	if err := repository.FindByID(wctx, s.DB, &row, msg.TenantID, assetID); err != nil {
		return err
	}
	if row.SecurityStatus == SecurityScanning {
		// An active lease owns this logical payload. Restore a single processing
		// marker instead of making it immediately claimable again.
		return errScanInProgress
	}
	if row.SecurityStatus != SecurityPendingScan && row.SecurityStatus != SecurityScanFailed {
		return nil
	}
	mimeGroup := mimeGroup(row.ContentType)
	if !row.CreatedAt.IsZero() {
		s.ObserveFileScan("basic", "queue_age", "claimed", mimeGroup, time.Since(row.CreatedAt))
	}
	s.ObserveFileScan("basic", "claim", "claimed", mimeGroup, 0)
	next, err := TransitionSecurityStatus(row.SecurityStatus, SecurityScanning)
	if err != nil {
		return err
	}
	claimID := uuid.NewString()
	leaseUntil := time.Now().Add(fileScanLeaseDuration)
	claim := s.DB.WithContext(wctx).Model(&FileRecord{}).
		Where("id = ? AND tenant_id = ? AND security_status = ?", assetID, msg.TenantID, row.SecurityStatus).
		Updates(map[string]any{
			"security_status":  next,
			"scan_status":      next,
			"scan_claim_id":    claimID,
			"scan_lease_until": &leaseUntil,
		})
	if claim.Error != nil {
		return claim.Error
	}
	if claim.RowsAffected != 1 {
		var current FileRecord
		if err := s.DB.WithContext(wctx).Select("security_status").Where("id = ? AND tenant_id = ?", assetID, msg.TenantID).First(&current).Error; err != nil {
			return err
		}
		if current.SecurityStatus == SecurityScanning {
			return errScanInProgress
		}
		return nil
	}
	scanCtx, cancel := context.WithTimeout(wctx, fileScanLeaseDuration)
	defer cancel()
	result, scanErr := s.runScanner(scanCtx, scanner, &row)
	if scanErr != nil {
		if err := s.markScanFailedIfOwned(wctx, assetID, msg.TenantID, claimID); err != nil {
			return err
		}
		s.ObserveFileScan("basic", "result", "failure", mimeGroup, time.Since(start))
		return scanErr
	}
	if err := s.renewScanLease(wctx, assetID, msg.TenantID, claimID); err != nil {
		return err
	}
	final := mapResultStatus(result.Status)
	updates := map[string]any{
		"security_status":  final,
		"scan_status":      final,
		"scan_claim_id":    "",
		"scan_lease_until": nil,
	}
	var rollbackPublic, finalizeQuarantine func() error
	if final == SecurityClean {
		publicKey, publicURL, rollback, finalize, err := s.promoteCleanObject(scanCtx, &row)
		if err != nil {
			if markErr := s.markScanFailedIfOwned(wctx, assetID, msg.TenantID, claimID); markErr != nil {
				return markErr
			}
			return fmt.Errorf("files: promote clean object: %w", err)
		}
		updates["object_key"] = publicKey
		updates["public_url"] = publicURL
		rollbackPublic, finalizeQuarantine = rollback, finalize
	}
	update := s.DB.WithContext(wctx).Model(&FileRecord{}).
		Where("id = ? AND tenant_id = ? AND security_status = ? AND scan_claim_id = ?", assetID, msg.TenantID, SecurityScanning, claimID).
		Updates(updates)
	if update.Error != nil || update.RowsAffected != 1 {
		if rollbackPublic != nil {
			_ = rollbackPublic()
		}
		if update.Error != nil {
			return update.Error
		}
		return fmt.Errorf("files: scan record changed before finalization")
	}
	if finalizeQuarantine != nil {
		_ = finalizeQuarantine()
	}
	s.ObserveFileScan("basic", "result", final, mimeGroup, time.Since(start))
	if final == SecurityScanFailed {
		return fmt.Errorf("files: scanner returned scan_failed")
	}
	return nil
}

func (s *Service) renewScanLease(ctx context.Context, assetID uuid.UUID, tenantID int64, claimID string) error {
	now := time.Now()
	nextLease := now.Add(fileScanLeaseDuration)
	res := s.DB.WithContext(ctx).Model(&FileRecord{}).
		Where("id = ? AND tenant_id = ? AND security_status = ? AND scan_claim_id = ? AND scan_lease_until > ?", assetID, tenantID, SecurityScanning, claimID, now).
		Update("scan_lease_until", &nextLease)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errScanClaimLost
	}
	return nil
}

func (s *Service) markScanFailedIfOwned(ctx context.Context, assetID uuid.UUID, tenantID int64, claimID string) error {
	res := s.DB.WithContext(ctx).Model(&FileRecord{}).
		Where("id = ? AND tenant_id = ? AND security_status = ? AND scan_claim_id = ?", assetID, tenantID, SecurityScanning, claimID).
		Updates(map[string]any{
			"security_status":  SecurityScanFailed,
			"scan_status":      SecurityScanFailed,
			"scan_claim_id":    "",
			"scan_lease_until": nil,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errScanClaimLost
	}
	return nil
}

// promoteCleanObject copies first and deletes quarantine only after the public
// object exists. A failed promotion leaves the database non-public.
func (s *Service) promoteCleanObject(ctx context.Context, row *FileRecord) (string, string, func() error, func() error, error) {
	if row == nil || !strings.HasPrefix(row.ObjectKey, "quarantine/") {
		return "", "", nil, nil, fmt.Errorf("invalid quarantine key")
	}
	plain, err := s.Settings.PlainByGroup(ctx, globalStorageSettingsTenantID, "storage")
	if err != nil {
		return "", "", nil, nil, err
	}
	prov, _, err := storage.NewFromPlainForStoredKind(plain, row.StorageKind)
	if err != nil {
		return "", "", nil, nil, err
	}
	src, err := prov.Get(ctx, row.ObjectKey)
	if err != nil {
		return "", "", nil, nil, err
	}
	defer src.Close()
	key := strings.TrimPrefix(row.ObjectKey, "quarantine/")
	if err := prov.Put(ctx, key, src, row.Size, row.ContentType); err != nil {
		return "", "", nil, nil, err
	}
	url, err := prov.GetURL(ctx, key)
	if err != nil {
		_ = prov.Delete(ctx, key)
		return "", "", nil, nil, err
	}
	return key, url, func() error { return prov.Delete(ctx, key) }, func() error { return prov.Delete(ctx, row.ObjectKey) }, nil
}

func mapResultStatus(st string) string {
	switch strings.TrimSpace(strings.ToLower(st)) {
	case filescanner.ResultClean:
		return SecurityClean
	case filescanner.ResultRejected:
		return SecurityRejected
	case filescanner.ResultQuarantined:
		return SecurityQuarantined
	default:
		return SecurityScanFailed
	}
}

func (s *Service) runScanner(ctx context.Context, scanner filescanner.FileScanner, row *FileRecord) (filescanner.ScanResult, error) {
	if s == nil || s.Settings == nil || row == nil {
		return filescanner.ScanResult{}, fmt.Errorf("files: scan unavailable")
	}
	plain, err := s.Settings.PlainByGroup(ctx, globalStorageSettingsTenantID, "storage")
	if err != nil {
		return filescanner.ScanResult{}, err
	}
	prov, _, err := storage.NewFromPlainForStoredKind(plain, row.StorageKind)
	if err != nil {
		return filescanner.ScanResult{}, err
	}
	data, err := prov.Get(ctx, row.ObjectKey)
	if err != nil {
		return filescanner.ScanResult{}, err
	}
	defer data.Close()
	tmp, err := os.CreateTemp("", "tm-scan-*")
	if err != nil {
		return filescanner.ScanResult{}, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, data); err != nil {
		_ = tmp.Close()
		return filescanner.ScanResult{}, err
	}
	_ = tmp.Close()
	sum := sha256.Sum256([]byte(row.ObjectKey))
	hash := hex.EncodeToString(sum[:])
	in := filescanner.ScanInput{
		TenantID:      row.TenantID,
		AssetID:       row.ID.String(),
		ObjectKey:     row.ObjectKey,
		MimeType:      row.ContentType,
		Size:          row.Size,
		ContentHash:   hash,
		LocalTempPath: filepath.Clean(tmpPath),
	}
	scanCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return scanner.Scan(scanCtx, in)
}

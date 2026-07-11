package files

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
)

const fileScanQueueName = "file:security:scan"

// ScanQueueMessage is the Redis payload for file security scan tasks.
type ScanQueueMessage struct {
	TenantID int64  `json:"tenantId"`
	AssetID  string `json:"assetId"`
}

// EnqueueSecurityScan pushes a file scan task after upload.
func (s *Service) EnqueueSecurityScan(ctx context.Context, tenantID int64, assetID uuid.UUID) error {
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return nil
	}
	if err := tasktenant.RequireTaskTenant(tenantID); err != nil {
		return err
	}
	msg := ScanQueueMessage{TenantID: tenantID, AssetID: assetID.String()}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return s.Redis.LPush(ctx, fileScanQueueName, string(b)).Err()
}

// StartScanWorker consumes file security scan queue until ctx cancelled.
func StartScanWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, cfg *config.Config, reg *worker.Registry) {
	if wg == nil || svc == nil || svc.Redis == nil || svc.Redis.Client == nil {
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
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			res, err := svc.Redis.BRPop(ctx, 5*time.Second, fileScanQueueName).Result()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			if len(res) < 2 {
				continue
			}
			if err := svc.processScanPayload(ctx, log, scanner, res[1]); err != nil && log != nil {
				log.Warn("file_scan_worker_error", "error", err)
			}
		}
	}()
}

func buildFileScanner(cfg *config.Config) filescanner.FileScanner {
	scanners := []filescanner.FileScanner{
		&filescanner.BasicFilePolicyScanner{},
		&filescanner.ImageDecodeScanner{},
	}
	_ = cfg
	return &filescanner.CompositeFileScanner{Scanners: scanners}
}

func (s *Service) processScanPayload(ctx context.Context, log *slog.Logger, scanner filescanner.FileScanner, payload string) error {
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
	if row.SecurityStatus != SecurityPendingScan && row.SecurityStatus != SecurityScanFailed {
		return nil
	}
	next, err := TransitionSecurityStatus(row.SecurityStatus, SecurityScanning)
	if err != nil {
		return err
	}
	if err := s.DB.WithContext(wctx).Model(&FileRecord{}).Where("id = ? AND tenant_id = ?", assetID, msg.TenantID).
		Update("security_status", next).Error; err != nil {
		return err
	}
	result, scanErr := s.runScanner(wctx, scanner, &row)
	if scanErr != nil {
		_ = s.DB.WithContext(wctx).Model(&FileRecord{}).Where("id = ?", assetID).
			Updates(map[string]any{"security_status": SecurityScanFailed, "scan_status": SecurityScanFailed})
		return scanErr
	}
	final := mapResultStatus(result.Status)
	_ = s.DB.WithContext(wctx).Model(&FileRecord{}).Where("id = ? AND tenant_id = ?", assetID, msg.TenantID).
		Updates(map[string]any{"security_status": final, "scan_status": final})
	return nil
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
	plain, err := s.Settings.PlainByGroup(ctx, row.TenantID, "storage")
	if err != nil {
		return filescanner.ScanResult{}, err
	}
	prov, _, err := storage.NewFromPlain(plain)
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

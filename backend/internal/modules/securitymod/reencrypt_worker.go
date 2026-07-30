package securitymod

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
)

// StartReencryptWorker polls running rotation jobs and processes re-encrypt batches.
func StartReencryptWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, reg *worker.Registry) {
	if wg == nil || svc == nil || svc.DB == nil {
		return
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		var ri *worker.RunningInstance
		if reg != nil {
			ri = reg.Register(ctx, worker.TypeSecuritySecretReencrypt, "security-reencrypt", nil)
			if ri != nil {
				defer ri.Stop(ctx)
			}
		}
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCtx := security.WorkerSystemContext(0, uuid.Nil, "security_secret_reencrypt")
				if err := processRunningRotations(runCtx, log, svc); err != nil && log != nil {
					log.Warn("security_reencrypt_worker_error", "error", err)
				}
			}
		}
	}()
}

func processRunningRotations(ctx context.Context, log *slog.Logger, svc *Service) error {
	var jobs []KeyRotationJob
	if err := svc.DB.WithContext(ctx).
		Where("status = ? AND dry_run = ?", RotationRunning, false).
		Order("created_at ASC").
		Limit(5).
		Find(&jobs).Error; err != nil {
		return err
	}
	for i := range jobs {
		n, err := svc.ProcessReencryptBatch(ctx, jobs[i].ID, 50)
		if err != nil && log != nil {
			log.Warn("security_reencrypt_batch_failed", "rotationId", jobs[i].ID.String(), "error", err)
			continue
		}
		if n > 0 && log != nil {
			log.Info("security_reencrypt_batch_done", "rotationId", jobs[i].ID.String(), "processed", n)
		}
	}
	return nil
}

package shop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

type ozonCategorySyncQueueMessage struct {
	RunID string `json:"runId"`
}

// OzonCategorySyncStart is returned when a category refresh is accepted.
// With Redis it normally contains a pending run; without Redis the same run is
// processed inline as a development fallback and is already terminal.
type OzonCategorySyncStart struct {
	Stats *OzonCategoryStats   `json:"stats,omitempty"`
	Run   *OzonCategorySyncRun `json:"run,omitempty"`
	RunID uuid.UUID            `json:"runId,omitempty"`
}

// StartOzonCategorySync validates the tenant-owned Ozon shop, persists a
// traceable run and enqueues it. Redis-less development keeps the exact run
// lifecycle but executes it inline so the feature remains usable.
func (s *Service) StartOzonCategorySync(ctx context.Context, tenantID int64, shopID uuid.UUID) (*OzonCategorySyncStart, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	ctx = security.WithTenantContext(ctx, &security.TenantContext{TenantID: tenantID})
	resolvedShopID, _, err := s.resolveOzonShopAndAuth(ctx, tenantID, shopID)
	if err != nil {
		return nil, err
	}
	if !s.DB.Migrator().HasTable(&OzonCategorySyncRun{}) || !s.DB.Migrator().HasTable(&OzonCategoryChange{}) {
		stats, syncErr := s.SyncOzonCategories(ctx, tenantID, resolvedShopID)
		return &OzonCategorySyncStart{Stats: stats}, syncErr
	}
	var active OzonCategorySyncRun
	if err := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND shop_id = ? AND status IN ?", tenantID, resolvedShopID, []string{OzonCategorySyncPending, OzonCategorySyncRunning}).
		Order("created_at DESC").First(&active).Error; err == nil {
		stats, statsErr := s.OzonCategoryStats(ctx, tenantID)
		if statsErr != nil {
			return nil, statsErr
		}
		return &OzonCategorySyncStart{Stats: stats, Run: &active, RunID: active.ID}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	run := OzonCategorySyncRun{
		TenantID: tenantID,
		ShopID:   resolvedShopID,
		Status:   OzonCategorySyncPending,
	}
	if err := s.DB.WithContext(ctx).Create(&run).Error; err != nil {
		return nil, err
	}

	if s.Redis != nil && s.Redis.Client != nil {
		raw, marshalErr := json.Marshal(ozonCategorySyncQueueMessage{RunID: run.ID.String()})
		if marshalErr != nil {
			return nil, marshalErr
		}
		if err := s.Redis.LPush(ctx, OzonCategorySyncQueueName, string(raw)).Err(); err != nil {
			finished := time.Now().UTC()
			_ = s.DB.WithContext(ctx).Model(&OzonCategorySyncRun{}).
				Where("id = ? AND tenant_id = ? AND status = ?", run.ID, tenantID, OzonCategorySyncPending).
				Updates(map[string]any{
					"status":        OzonCategorySyncFailedState,
					"finished_at":   &finished,
					"error_code":    OzonCategorySyncFailed,
					"error_message": "failed to enqueue category sync",
				}).Error
			return nil, fmt.Errorf("enqueue ozon category sync: %w", err)
		}
	} else if err := s.ProcessOzonCategorySyncRun(ctx, run.ID); err != nil {
		return nil, err
	}

	saved, err := s.GetOzonCategorySyncRun(ctx, tenantID, run.ID)
	if err != nil {
		return nil, err
	}
	stats, err := s.OzonCategoryStats(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return &OzonCategorySyncStart{Stats: stats, Run: saved, RunID: saved.ID}, nil
}

// ProcessOzonCategorySyncRun claims one pending run. The conditional update is
// the at-most-once boundary for duplicate Redis deliveries.
func (s *Service) ProcessOzonCategorySyncRun(ctx context.Context, runID uuid.UUID) error {
	if s == nil || s.DB == nil || runID == uuid.Nil {
		return fmt.Errorf("invalid ozon category sync run")
	}
	tenant := security.FromContext(ctx)
	if tenant == nil || tenant.TenantID < 0 {
		return fmt.Errorf("ozon category sync tenant context required")
	}
	var run OzonCategorySyncRun
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", runID, tenant.TenantID).First(&run).Error; err != nil {
		return err
	}
	started := time.Now().UTC()
	claimed := s.DB.WithContext(ctx).Model(&OzonCategorySyncRun{}).
		Where("id = ? AND tenant_id = ? AND status = ?", runID, tenant.TenantID, OzonCategorySyncPending).
		Updates(map[string]any{
			"status":        OzonCategorySyncRunning,
			"started_at":    &started,
			"finished_at":   nil,
			"error_code":    "",
			"error_message": "",
		})
	if claimed.Error != nil {
		return claimed.Error
	}
	if claimed.RowsAffected == 0 {
		return nil
	}

	workerCtx := ctx
	_, auth, err := s.resolveOzonShopAndAuth(workerCtx, run.TenantID, run.ShopID)
	if err == nil {
		_, err = s.syncOzonCategoriesRun(workerCtx, auth, &run)
	}
	if err == nil {
		return nil
	}

	finished := time.Now().UTC()
	updateErr := s.DB.WithContext(workerCtx).Model(&OzonCategorySyncRun{}).
		Where("id = ? AND tenant_id = ? AND status = ?", run.ID, run.TenantID, OzonCategorySyncRunning).
		Updates(map[string]any{
			"status":        OzonCategorySyncFailedState,
			"finished_at":   &finished,
			"error_code":    OzonCategorySyncFailed,
			"error_message": truncateOzonSyncError(err.Error()),
		}).Error
	if updateErr != nil {
		return fmt.Errorf("ozon category sync failed: %v; persist failure: %w", err, updateErr)
	}
	return err
}

func truncateOzonSyncError(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= 500 {
		return string(runes)
	}
	return string(runes[:500]) + "…"
}

// StartOzonCategorySyncWorker consumes the dedicated queue with one or more
// bounded workers. It performs only Ozon read calls and local cache updates.
func StartOzonCategorySyncWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, concurrency int, reg *worker.Registry) {
	if svc == nil || svc.Redis == nil || svc.Redis.Client == nil {
		return
	}
	if concurrency < 1 {
		concurrency = 1
	}
	for slot := 1; slot <= concurrency; slot++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			var instance *worker.RunningInstance
			if reg != nil {
				instance = reg.Register(ctx, worker.TypeOzonCategorySync, fmt.Sprintf("ozon-category-sync-%d", slot), map[string]any{"queue": OzonCategorySyncQueueName})
				if instance != nil {
					defer instance.Stop(context.Background())
				}
			}
			runOzonCategorySyncWorker(ctx, log, svc, slot)
		}(slot)
	}
}

func runOzonCategorySyncWorker(ctx context.Context, log *slog.Logger, svc *Service, slot int) {
	if slot == 1 {
		recoverStaleOzonCategorySyncRuns(ctx, log, svc)
	}
	for {
		result, err := svc.Redis.BRPop(ctx, 5*time.Second, OzonCategorySyncQueueName).Result()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		if len(result) < 2 {
			continue
		}
		var msg ozonCategorySyncQueueMessage
		if err := json.Unmarshal([]byte(result[1]), &msg); err != nil {
			if log != nil {
				log.Warn("ozon_category_sync_bad_message", "worker", slot)
			}
			continue
		}
		runID, err := uuid.Parse(strings.TrimSpace(msg.RunID))
		if err != nil || runID == uuid.Nil {
			if log != nil {
				log.Warn("ozon_category_sync_bad_run_id", "worker", slot)
			}
			continue
		}

		jobCtx := ctx
		var run OzonCategorySyncRun
		if err := svc.DB.WithContext(jobCtx).Where("id = ?", runID).First(&run).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) && log != nil {
				log.Warn("ozon_category_sync_load_failed", "worker", slot, "runId", runID.String(), "error", err)
			}
			continue
		}
		workerTenant := security.WorkerTenantContext(run.TenantID, uuid.Nil)
		workerTenant.ShopScope = []uuid.UUID{run.ShopID}
		jobCtx = security.WithTenantContext(jobCtx, workerTenant)
		jobCtx = security.WithSystemContext(jobCtx, &security.SystemContext{Operation: "ozon_category_sync"})
		if err := svc.ProcessOzonCategorySyncRun(jobCtx, runID); err != nil && log != nil {
			log.Warn("ozon_category_sync_failed", "worker", slot, "runId", runID.String(), "error", err)
		}
	}
}

const ozonCategorySyncStaleAfter = 10 * time.Minute

// recoverStaleOzonCategorySyncRuns performs one bounded recovery pass on
// worker startup. A conditional running→pending update prevents competing
// workers from re-enqueueing the same run; succeeded runs are never touched.
func recoverStaleOzonCategorySyncRuns(ctx context.Context, log *slog.Logger, svc *Service) {
	if svc == nil || svc.DB == nil || svc.Redis == nil || svc.Redis.Client == nil {
		return
	}
	cutoff := time.Now().UTC().Add(-ozonCategorySyncStaleAfter)
	var stale []OzonCategorySyncRun
	if err := svc.DB.WithContext(ctx).Where("status = ? AND started_at IS NOT NULL AND started_at < ?", OzonCategorySyncRunning, cutoff).Order("started_at ASC").Limit(20).Find(&stale).Error; err != nil {
		if log != nil {
			log.Warn("ozon_category_sync_stale_scan_failed", "error", err)
		}
		return
	}
	for _, run := range stale {
		res := svc.DB.WithContext(ctx).Model(&OzonCategorySyncRun{}).Where("id = ? AND tenant_id = ? AND status = ?", run.ID, run.TenantID, OzonCategorySyncRunning).Updates(map[string]any{"status": OzonCategorySyncPending, "started_at": nil, "error_code": "", "error_message": ""})
		if res.Error != nil || res.RowsAffected == 0 {
			continue
		}
		raw, err := json.Marshal(ozonCategorySyncQueueMessage{RunID: run.ID.String()})
		if err != nil {
			continue
		}
		if err := svc.Redis.LPush(ctx, OzonCategorySyncQueueName, string(raw)).Err(); err != nil {
			finished := time.Now().UTC()
			_ = svc.DB.WithContext(ctx).Model(&OzonCategorySyncRun{}).
				Where("id = ? AND tenant_id = ? AND status = ?", run.ID, run.TenantID, OzonCategorySyncPending).
				Updates(map[string]any{
					"status":        OzonCategorySyncFailedState,
					"finished_at":   &finished,
					"error_code":    OzonCategorySyncFailed,
					"error_message": "failed to re-enqueue stale category sync",
				}).Error
			if log != nil {
				log.Warn("ozon_category_sync_stale_requeue_failed", "runId", run.ID.String(), "error", err)
			}
		}
	}
}

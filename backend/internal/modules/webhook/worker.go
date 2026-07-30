package webhook

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
)

// StartWorker polls DB for status=queued webhook events until ctx is cancelled.
func StartWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, cfg *config.Config, reg *worker.Registry) {
	if wg == nil || svc == nil {
		return
	}
	interval := 3 * time.Second
	if cfg != nil && cfg.WebhookWorkerIntervalSeconds > 0 {
		interval = time.Duration(cfg.WebhookWorkerIntervalSeconds) * time.Second
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		var ri *worker.RunningInstance
		if reg != nil {
			ri = reg.Register(ctx, worker.TypeWebhook, "webhook-poll", map[string]any{"poll": "webhook_events"})
			if ri != nil {
				defer ri.Stop(ctx)
			}
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		if log != nil {
			log.Info("webhook_worker_started", "interval_sec", int(interval.Seconds()))
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				n, err := svc.ProcessQueuedEvents(runCtx, 20)
				cancel()
				if err != nil && log != nil {
					log.Warn("webhook_worker_poll_failed", "error", err)
				} else if n > 0 && log != nil {
					log.Info("webhook_worker_processed", "count", n)
				}
			}
		}
	}()
}

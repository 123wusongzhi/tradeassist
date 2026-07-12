package observabilitymod

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"gorm.io/gorm"
)

const (
	SLOStatusAchieved         = "achieved"
	SLOStatusViolated         = "violated"
	SLOStatusInsufficientData = "insufficient_data"
)

// EnsureDefaultSLOs seeds code-level SLO definitions idempotently.
func EnsureDefaultSLOs(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	defs := []database.SLODefinition{
		{ID: "api_availability", Name: "API availability", TargetRatio: 0.995, Window: "1h", Enabled: true},
		{ID: "api_latency", Name: "API latency", TargetRatio: 0.99, Window: "1h", Enabled: true},
		{ID: "worker_success", Name: "Worker success", TargetRatio: 0.99, Window: "1h", Enabled: true},
		{ID: "webhook_processing", Name: "Webhook processing", TargetRatio: 0.99, Window: "1h", Enabled: true},
		{ID: "provider_success", Name: "Provider success", TargetRatio: 0.98, Window: "1h", Enabled: true},
		{ID: "order_sync_freshness", Name: "Order sync freshness", TargetRatio: 0.98, Window: "1h", Enabled: true},
		{ID: "file_scan_completion", Name: "File scan completion", TargetRatio: 0.99, Window: "1h", Enabled: true},
		{ID: "audit_write_success", Name: "Audit write success", TargetRatio: 0.999, Window: "1h", Enabled: true},
	}
	for _, def := range defs {
		var count int64
		if err := db.WithContext(ctx).Model(&database.SLODefinition{}).Where("id = ?", def.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			if err := db.WithContext(ctx).Create(&def).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// EvaluateSLOs writes snapshots for enabled SLOs from aggregate samples.
func EvaluateSLOs(ctx context.Context, db *gorm.DB, cat *metrics.Catalog, samples map[string]float64) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("slo evaluator unavailable")
	}
	var defs []database.SLODefinition
	if err := db.WithContext(ctx).Where("enabled = ?", true).Find(&defs).Error; err != nil {
		return 0, err
	}
	now := time.Now().UTC().Unix()
	written := 0
	for _, def := range defs {
		total, errors := sloInputs(def.ID, samples)
		compliance, remaining, burn, status := calculateSLO(total, errors, def.TargetRatio)
		snap := database.SLOSnapshot{
			SLOID:       def.ID,
			Compliance:  compliance,
			ErrorBudget: remaining,
			BurnRate:    burn,
			Window:      normalizeWindow(def.Window),
			Status:      status,
			RecordedAt:  now,
		}
		if err := db.WithContext(ctx).Create(&snap).Error; err != nil {
			return written, err
		}
		if cat != nil {
			cat.ObserveSLO(def.ID, snap.Window, compliance, remaining, burn)
		}
		written++
	}
	return written, nil
}

func StartSLOEvaluatorWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, db *gorm.DB, cat *metrics.Catalog, interval time.Duration, sample func() map[string]float64) {
	if wg == nil || db == nil || sample == nil {
		return
	}
	if interval <= 0 {
		interval = time.Minute
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if n, err := EvaluateSLOs(ctx, db, cat, sample()); err != nil && log != nil {
					log.Warn("slo_evaluator_failed", "error", err)
				} else if log != nil {
					log.Debug("slo_evaluator_completed", "snapshots", n)
				}
			}
		}
	}()
}

func sloInputs(id string, samples map[string]float64) (float64, float64) {
	if samples == nil {
		return 0, 0
	}
	if total := samples["slo:"+id+":total"]; total > 0 {
		return total, samples["slo:"+id+":errors"]
	}
	switch id {
	case "api_availability", "api_latency":
		return samples["http_server_requests_total"], samples["http_server_panics_total"]
	case "worker_success":
		total := samples["tasks_completed_total"] + samples["tasks_failed_total"] + samples["tasks_dead_letter_total"]
		return total, samples["tasks_failed_total"] + samples["tasks_dead_letter_total"]
	case "webhook_processing":
		total := samples["webhook_events_processed_total"] + samples["webhook_payload_rejected_total"]
		return total, samples["webhook_payload_rejected_total"] + samples["webhook_shop_resolution_failures_total"]
	case "provider_success":
		return samples["provider_requests_total"], samples["provider_request_timeouts_total"] + samples["provider_contract_mismatches_total"]
	case "order_sync_freshness":
		return samples["order_sync_runs_total"], samples["order_sync_failures_total"]
	case "file_scan_completion":
		return samples["file_scan_tasks_total"], samples["file_scan_failures_total"] + samples["file_scan_stuck_total"]
	case "audit_write_success":
		total := samples["security_events_total"] + samples["audit_chain_mismatch_total"]
		return total, samples["audit_chain_mismatch_total"]
	default:
		return 0, 0
	}
}

func calculateSLO(total, errors, target float64) (float64, float64, float64, string) {
	if total <= 0 || target <= 0 || target >= 1 {
		return 0, 0, 0, SLOStatusInsufficientData
	}
	if errors < 0 {
		errors = 0
	}
	if errors > total {
		errors = total
	}
	errorRatio := errors / total
	compliance := 1 - errorRatio
	allowed := 1 - target
	remaining := (allowed - errorRatio) / allowed
	remaining = math.Max(0, math.Min(1, remaining))
	burn := errorRatio / allowed
	status := SLOStatusAchieved
	if compliance < target {
		status = SLOStatusViolated
	}
	return compliance, remaining, burn, status
}

func normalizeWindow(raw string) string {
	switch raw {
	case "1h", "6h", "24h", "7d", "30d":
		return raw
	default:
		return "1h"
	}
}

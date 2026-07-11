package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"gorm.io/gorm"
)

// ProcessEvent runs async business handling for one durable webhook event.
// Uses idempotency key webhook-process:{platform}:{eventId}. Unknown platforms use a noop handler.
func (s *Service) ProcessEvent(ctx context.Context, platform, eventID string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("webhook: unavailable")
	}
	platform = strings.TrimSpace(platform)
	eventID = strings.TrimSpace(eventID)
	if platform == "" || eventID == "" {
		return fmt.Errorf("platform and eventId required")
	}

	var ev Event
	if err := s.DB.WithContext(ctx).Where("platform = ? AND event_id = ?", platform, eventID).First(&ev).Error; err != nil {
		return err
	}
	if ev.Status == StatusProcessed || ev.Status == StatusIgnored || ev.Status == StatusDuplicate {
		return nil
	}

	key := idempotency.WebhookProcess(platform, eventID)
	owner := "webhook-process"
	var idemJob *webhookAcquire
	if s.Idempotency != nil {
		reqHash := idempotency.HashRequest([]byte(ev.PayloadHash))
		res, acqErr := s.Idempotency.Acquire(ctx, idempotency.ScopeWebhook, key, reqHash, owner, idempotency.DefaultLease)
		decision, rec, _ := idempotency.Classify(res, acqErr)
		switch decision {
		case idempotency.DecisionAlreadySucceeded:
			return nil
		case idempotency.DecisionInProgress:
			return nil
		case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
			return newCodeError(CodeKeyConflict, 409, CodeKeyConflict)
		case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
			if rec == nil && res != nil {
				rec = res.Record
			}
			if rec != nil {
				idemJob = &webhookAcquire{RecordID: rec.ID, Owner: owner}
			}
		default:
			if acqErr != nil {
				return acqErr
			}
		}
	}

	now := s.now()
	claim := s.DB.WithContext(ctx).Model(&Event{}).
		Where("id = ? AND status IN ?", ev.ID, []string{StatusQueued, StatusReceived, StatusFailedRetryable}).
		Updates(map[string]any{
			"status":     StatusProcessing,
			"updated_at": now,
		})
	if claim.Error != nil {
		s.failProcess(ctx, idemJob, "WEBHOOK_CLAIM_FAILED", true)
		return claim.Error
	}
	if claim.RowsAffected == 0 {
		if idemJob != nil && s.Idempotency != nil {
			_ = s.Idempotency.Complete(ctx, idemJob.RecordID, idemJob.Owner, idempotency.CompleteResult{
				ResponseCode: "WEBHOOK_ALREADY_CLAIMED",
				ResourceType: "webhook_event",
				ResourceID:   eventID,
			})
		}
		return nil
	}

	if err := s.handlePlatformEvent(ctx, &ev); err != nil {
		_ = s.markFailed(ctx, ev.ID, StatusFailedRetryable, "WEBHOOK_PROCESS_FAILED", err.Error())
		s.failProcess(ctx, idemJob, "WEBHOOK_PROCESS_FAILED", true)
		return err
	}

	processedAt := s.now()
	if err := s.DB.WithContext(ctx).Model(&Event{}).Where("id = ?", ev.ID).Updates(map[string]any{
		"status":        StatusProcessed,
		"processed_at":  processedAt,
		"error_code":    "",
		"error_message": "",
		"updated_at":    processedAt,
	}).Error; err != nil {
		s.failProcess(ctx, idemJob, "WEBHOOK_MARK_PROCESSED_FAILED", true)
		return err
	}

	if idemJob != nil && s.Idempotency != nil {
		summary, _ := json.Marshal(map[string]string{"eventId": eventID, "status": StatusProcessed})
		_ = s.Idempotency.Complete(ctx, idemJob.RecordID, idemJob.Owner, idempotency.CompleteResult{
			ResponseCode:    "WEBHOOK_PROCESSED",
			ResponseSummary: string(summary),
			ResourceType:    "webhook_event",
			ResourceID:      eventID,
		})
	}
	return nil
}

// ProcessQueuedEvents claims and processes up to limit queued webhook events (DB poll worker).
func (s *Service) ProcessQueuedEvents(ctx context.Context, limit int) (int, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("webhook: unavailable")
	}
	if limit <= 0 {
		limit = 20
	}
	var rows []Event
	err := s.DB.WithContext(ctx).
		Where("status = ?", StatusQueued).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return 0, err
	}
	done := 0
	for i := range rows {
		if err := s.ProcessEvent(ctx, rows[i].Platform, rows[i].EventID); err != nil {
			continue
		}
		done++
	}
	return done, nil
}

func (s *Service) handlePlatformEvent(_ context.Context, ev *Event) error {
	// MVP: no Douyin/business handlers — noop marks processed for all platforms including internal-test.
	_ = ev
	return nil
}

func (s *Service) markFailed(ctx context.Context, id uuid.UUID, status, code, msg string) error {
	return s.DB.WithContext(ctx).Model(&Event{}).Where("id = ?", id).Updates(map[string]any{
		"status":        status,
		"error_code":    code,
		"error_message": truncateSummary(msg),
		"updated_at":    s.now(),
	}).Error
}

func (s *Service) failProcess(ctx context.Context, job *webhookAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}

// LoadEventByPlatformEventID loads a durable event row.
func (s *Service) LoadEventByPlatformEventID(ctx context.Context, platform, eventID string) (*Event, error) {
	var ev Event
	if err := s.DB.WithContext(ctx).Where("platform = ? AND event_id = ?", platform, eventID).First(&ev).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &ev, nil
}

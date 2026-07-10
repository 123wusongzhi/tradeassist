package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MaxPayloadBytes limits inbound webhook body size.
const MaxPayloadBytes = 1 << 20 // 1 MB

const (
	errWebhookInProgress  = "WEBHOOK_IN_PROGRESS"
	errWebhookKeyConflict = "WEBHOOK_KEY_CONFLICT"
)

// Service handles webhook idempotency and fast ACK.
type Service struct {
	DB          *gorm.DB
	Idempotency *idempotency.Service
}

// IngestRequest is normalized webhook input.
type IngestRequest struct {
	Platform  string
	EventID   string
	Payload   json.RawMessage
	Timestamp time.Time
}

// IngestResult describes ACK outcome.
type IngestResult struct {
	EventID   string `json:"eventId"`
	Status    string `json:"status"`
	Duplicate bool   `json:"duplicate"`
}

type webhookAcquire struct {
	RecordID uuid.UUID
	Owner    string
}

// Ingest stores webhook event idempotently and returns fast ACK metadata.
func (s *Service) Ingest(ctx context.Context, req IngestRequest) (*IngestResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("webhook: unavailable")
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		return nil, fmt.Errorf("platform is required")
	}
	eventID := strings.TrimSpace(req.EventID)
	if eventID == "" {
		eventID = hashPayload(req.Payload)
	}
	if len(req.Payload) > MaxPayloadBytes {
		return nil, fmt.Errorf("payload too large")
	}
	hash := hashPayload(req.Payload)
	key := idempotency.Webhook(platform, eventID)
	reqHash := idempotency.HashRequest(req.Payload)
	owner := "webhook-ingest"

	var existing Event
	err := s.DB.WithContext(ctx).
		Where("platform = ? AND event_id = ?", platform, eventID).
		First(&existing).Error
	if err == nil {
		return &IngestResult{EventID: eventID, Status: existing.Status, Duplicate: true}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var idemJob *webhookAcquire
	if s.Idempotency != nil {
		res, acqErr := s.Idempotency.Acquire(ctx, idempotency.ScopeWebhook, key, reqHash, owner, idempotency.DefaultLease)
		decision, rec, _ := idempotency.Classify(res, acqErr)
		switch decision {
		case idempotency.DecisionAlreadySucceeded:
			if row, loadErr := s.loadExistingEvent(ctx, platform, eventID); loadErr == nil {
				return &IngestResult{EventID: eventID, Status: row.Status, Duplicate: true}, nil
			}
			return &IngestResult{EventID: eventID, Status: StatusReceived, Duplicate: true}, nil
		case idempotency.DecisionInProgress:
			return nil, fmt.Errorf("%s", errWebhookInProgress)
		case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
			return nil, fmt.Errorf("%s", errWebhookKeyConflict)
		case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
			if rec == nil && res != nil {
				rec = res.Record
			}
			if rec != nil {
				idemJob = &webhookAcquire{RecordID: rec.ID, Owner: owner}
			}
		default:
			if acqErr != nil {
				return nil, acqErr
			}
		}
	}

	ev := Event{
		Platform:    platform,
		EventID:     eventID,
		PayloadHash: hash,
		Status:      StatusReceived,
		RawSummary:  truncateSummary(string(req.Payload)),
	}
	if err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "platform"}, {Name: "event_id"}},
		DoNothing: true,
	}).Create(&ev).Error; err != nil {
		s.failWebhookIngest(ctx, idemJob, "WEBHOOK_STORE_FAILED", true)
		return nil, err
	}

	if err := s.DB.WithContext(ctx).
		Where("platform = ? AND event_id = ?", platform, eventID).
		First(&ev).Error; err != nil {
		s.failWebhookIngest(ctx, idemJob, "WEBHOOK_STORE_FAILED", true)
		return nil, err
	}

	if idemJob != nil && s.Idempotency != nil {
		summary, _ := json.Marshal(map[string]string{"eventId": eventID, "status": ev.Status})
		if err := s.Idempotency.Complete(ctx, idemJob.RecordID, idemJob.Owner, idempotency.CompleteResult{
			ResponseCode:    "WEBHOOK_RECEIVED",
			ResponseSummary: string(summary),
			ResourceType:    "webhook_event",
			ResourceID:      eventID,
		}); err != nil {
			return nil, err
		}
	}

	return &IngestResult{EventID: eventID, Status: ev.Status, Duplicate: false}, nil
}

func (s *Service) loadExistingEvent(ctx context.Context, platform, eventID string) (*Event, error) {
	var row Event
	if err := s.DB.WithContext(ctx).Where("platform = ? AND event_id = ?", platform, eventID).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) failWebhookIngest(ctx context.Context, job *webhookAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}

func hashPayload(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func truncateSummary(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

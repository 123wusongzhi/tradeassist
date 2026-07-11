package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Service handles webhook idempotency, ingest, and async processing.
type Service struct {
	DB              *gorm.DB
	Idempotency     *idempotency.Service
	Verifiers       *Registry
	MaxPayloadBytes int64
	MaxClockSkew    time.Duration
	AppEnv          string
	Now             func() time.Time
}

// IngestRequest is normalized webhook input.
type IngestRequest struct {
	Platform  string
	EventType string
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

func (s *Service) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *Service) maxPayload() int64 {
	if s != nil && s.MaxPayloadBytes > 0 {
		return s.MaxPayloadBytes
	}
	return 512 * 1024
}

func (s *Service) maxSkew() time.Duration {
	if s != nil && s.MaxClockSkew > 0 {
		return s.MaxClockSkew
	}
	return 300 * time.Second
}

// ValidateTimestamp rejects missing (when required) and out-of-window timestamps.
func (s *Service) ValidateTimestamp(ts time.Time, required bool) error {
	if ts.IsZero() {
		if required {
			return newCodeError(CodeTimestampMissing, http.StatusUnauthorized, CodeTimestampMissing)
		}
		return nil
	}
	now := s.now()
	delta := now.Sub(ts)
	if delta < 0 {
		delta = -delta
	}
	if delta > s.maxSkew() {
		return newCodeError(CodeTimestampExpired, http.StatusUnauthorized, CodeTimestampExpired)
	}
	return nil
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
	if len(req.Payload) > int(s.maxPayload()) {
		return nil, newCodeError(CodePayloadTooLarge, http.StatusRequestEntityTooLarge, CodePayloadTooLarge)
	}
	if err := s.ValidateTimestamp(req.Timestamp, !req.Timestamp.IsZero()); err != nil {
		return nil, err
	}
	eventID := strings.TrimSpace(req.EventID)
	if eventID == "" {
		eventID = hashPayload(req.Payload)
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
			return &IngestResult{EventID: eventID, Status: StatusDuplicate, Duplicate: true}, nil
		case idempotency.DecisionInProgress:
			return nil, newCodeError(CodeInProgress, http.StatusConflict, CodeInProgress)
		case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
			return nil, newCodeError(CodeKeyConflict, http.StatusConflict, CodeKeyConflict)
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

	meta, _ := json.Marshal(map[string]string{
		"eventType": strings.TrimSpace(req.EventType),
	})
	ev := Event{
		Platform:    platform,
		EventID:     eventID,
		EventType:   strings.TrimSpace(req.EventType),
		PayloadHash: hash,
		PayloadBody: string(req.Payload),
		Status:      StatusQueued,
		RawSummary:  truncateSummary(string(req.Payload)),
		Metadata:    datatypes.JSON(meta),
	}
	createRes := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "platform"}, {Name: "event_id"}},
		DoNothing: true,
	}).Create(&ev)
	if createRes.Error != nil {
		s.failWebhookIngest(ctx, idemJob, CodeStoreFailed, true)
		return nil, createRes.Error
	}

	if err := s.DB.WithContext(ctx).
		Where("platform = ? AND event_id = ?", platform, eventID).
		First(&ev).Error; err != nil {
		s.failWebhookIngest(ctx, idemJob, CodeStoreFailed, true)
		return nil, err
	}

	// Concurrent insert: another writer won ON CONFLICT DoNothing.
	if createRes.RowsAffected == 0 {
		if idemJob != nil && s.Idempotency != nil {
			_ = s.Idempotency.Complete(ctx, idemJob.RecordID, idemJob.Owner, idempotency.CompleteResult{
				ResponseCode:    "WEBHOOK_DUPLICATE",
				ResponseSummary: `{"duplicate":true}`,
				ResourceType:    "webhook_event",
				ResourceID:      eventID,
			})
		}
		return &IngestResult{EventID: eventID, Status: ev.Status, Duplicate: true}, nil
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

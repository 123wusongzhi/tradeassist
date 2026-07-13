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
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Service handles webhook idempotency, ingest, and async processing.
type Service struct {
	DB              *gorm.DB
	Idempotency     *idempotency.Service
	Verifiers       *Registry
	ShopResolver    WebhookShopResolver
	OrderHandler    OrderEventHandler
	Metrics         *metrics.Catalog
	MaxPayloadBytes int64
	MaxClockSkew    time.Duration
	AppEnv          string
	Now             func() time.Time
}

// IngestRequest is normalized webhook input.
type IngestRequest struct {
	Platform     string
	EventType    string
	EventID      string
	Payload      json.RawMessage
	Timestamp    time.Time
	ResolvedShop *ResolvedWebhookShop
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
	start := time.Now()
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("webhook: unavailable")
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		return nil, fmt.Errorf("platform is required")
	}
	if len(req.Payload) > int(s.maxPayload()) {
		s.ObserveWebhook(platform, req.EventType, "payload_rejected", "failure", CodePayloadTooLarge, 0)
		return nil, newCodeError(CodePayloadTooLarge, http.StatusRequestEntityTooLarge, CodePayloadTooLarge)
	}
	if err := s.ValidateTimestamp(req.Timestamp, !req.Timestamp.IsZero()); err != nil {
		s.ObserveWebhook(platform, req.EventType, "replay_rejected", "failure", CodeTimestampExpired, 0)
		return nil, err
	}
	eventID := strings.TrimSpace(req.EventID)
	if eventID == "" {
		eventID = hashPayload(req.Payload)
	}
	hash := hashPayload(req.Payload)
	key := webhookIngestKey(platform, eventID, req.ResolvedShop)
	reqHash := idempotency.HashRequest(req.Payload)
	owner := "webhook-ingest"

	var existing Event
	err := s.eventScopeQuery(ctx, platform, eventID, req.ResolvedShop).First(&existing).Error
	if err == nil {
		s.ObserveWebhook(platform, req.EventType, "duplicate", "duplicate", "", 0)
		s.ObserveWebhook(platform, req.EventType, "request", "success", "", time.Since(start))
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
			if row, loadErr := s.loadExistingEvent(ctx, platform, eventID, req.ResolvedShop); loadErr == nil {
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

	meta := eventMetadata(req.EventType, req.ResolvedShop)
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
	applyResolvedShopToEvent(&ev, req.ResolvedShop)
	if ev.TenantID <= 0 {
		ev.TenantID = devTestWebhookTenant(platform, s.AppEnv)
	}
	createRes := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "platform"}, {Name: "tenant_id"}, {Name: "platform_shop_id"}, {Name: "event_id"}},
		DoNothing: true,
	}).Create(&ev)
	if createRes.Error != nil {
		s.failWebhookIngest(ctx, idemJob, CodeStoreFailed, true)
		s.ObserveWebhook(platform, req.EventType, "request", "failure", CodeStoreFailed, time.Since(start))
		return nil, createRes.Error
	}

	if err := s.eventScopeQuery(ctx, platform, eventID, req.ResolvedShop).First(&ev).Error; err != nil {
		s.failWebhookIngest(ctx, idemJob, CodeStoreFailed, true)
		s.ObserveWebhook(platform, req.EventType, "request", "failure", CodeStoreFailed, time.Since(start))
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
		s.ObserveWebhook(platform, req.EventType, "duplicate", "duplicate", "", 0)
		s.ObserveWebhook(platform, req.EventType, "request", "success", "", time.Since(start))
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

	s.ObserveWebhook(platform, req.EventType, "persisted", "success", "", 0)
	s.ObserveWebhook(platform, req.EventType, "request", "success", "", time.Since(start))
	return &IngestResult{EventID: eventID, Status: ev.Status, Duplicate: false}, nil
}

func (s *Service) loadExistingEvent(ctx context.Context, platform, eventID string, resolved *ResolvedWebhookShop) (*Event, error) {
	var row Event
	if err := s.eventScopeQuery(ctx, platform, eventID, resolved).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) eventScopeQuery(ctx context.Context, platform, eventID string, resolved *ResolvedWebhookShop) *gorm.DB {
	q := s.DB.WithContext(ctx).Where("platform = ? AND event_id = ?", platform, eventID)
	if resolved != nil && strings.TrimSpace(resolved.PlatformShopID) != "" {
		q = q.Where("tenant_id = ? AND platform_shop_id = ?", resolved.TenantID, strings.TrimSpace(resolved.PlatformShopID))
	} else {
		q = q.Where("tenant_id = ? AND platform_shop_id = ?", devTestWebhookTenant(platform, s.AppEnv), "")
	}
	return q
}

// devTestWebhookTenant assigns a positive tenant for dev-only internal-test webhooks without shop binding.
func devTestWebhookTenant(platform, appEnv string) int64 {
	if platform == PlatformInternalTest && !config.IsProduction(appEnv) {
		return 1
	}
	return 0
}

func webhookIngestKey(platform, eventID string, resolved *ResolvedWebhookShop) string {
	if resolved != nil && strings.TrimSpace(resolved.PlatformShopID) != "" {
		return idempotency.WebhookScoped(platform, resolved.TenantID, resolved.PlatformShopID, eventID)
	}
	return idempotency.Webhook(platform, eventID)
}

func webhookProcessKey(ev *Event) string {
	if ev != nil && strings.TrimSpace(ev.PlatformShopID) != "" {
		return idempotency.WebhookProcessScoped(ev.Platform, ev.TenantID, ev.PlatformShopID, ev.EventID)
	}
	if ev == nil {
		return idempotency.WebhookProcess("", "")
	}
	return idempotency.WebhookProcess(ev.Platform, ev.EventID)
}

func applyResolvedShopToEvent(ev *Event, resolved *ResolvedWebhookShop) {
	if ev == nil || resolved == nil {
		return
	}
	ev.TenantID = resolved.TenantID
	if resolved.InternalShopID != uuid.Nil {
		id := resolved.InternalShopID
		ev.InternalShopID = &id
	}
	ev.PlatformShopID = strings.TrimSpace(resolved.PlatformShopID)
	ev.AppID = strings.TrimSpace(resolved.AppID)
	if resolved.BindingID != uuid.Nil {
		id := resolved.BindingID
		ev.BindingID = &id
	}
}

func eventMetadata(eventType string, resolved *ResolvedWebhookShop) []byte {
	m := map[string]any{"eventType": strings.TrimSpace(eventType)}
	if resolved != nil {
		m["tenantId"] = resolved.TenantID
		m["internalShopId"] = resolved.InternalShopID.String()
		m["platformShopId"] = resolved.PlatformShopID
		m["appId"] = resolved.AppID
		m["bindingId"] = resolved.BindingID.String()
		m["authorizationStatus"] = resolved.AuthorizationStatus
		m["contractStatus"] = resolved.ContractStatus
		if resolved.TestFallback {
			m["test_fallback"] = true
		}
	}
	b, _ := json.Marshal(m)
	return b
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

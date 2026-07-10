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

	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MaxPayloadBytes limits inbound webhook body size.
const MaxPayloadBytes = 1 << 20 // 1 MB

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
	scope := "webhook"
	key := idempotency.Webhook(platform, eventID)

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
		return nil, err
	}

	if s.Idempotency != nil {
		reqHash := idempotency.HashRequest(req.Payload)
		_, _ = s.Idempotency.Acquire(ctx, scope, key, reqHash, "webhook-ingest", idempotency.DefaultLease)
	}

	return &IngestResult{EventID: eventID, Status: StatusReceived, Duplicate: false}, nil
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

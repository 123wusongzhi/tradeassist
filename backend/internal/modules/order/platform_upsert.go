package order

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Platform order upsert source channels.
const (
	UpsertSourcePolling        = "polling"
	UpsertSourceWebhook        = "webhook"
	UpsertSourceManualSync     = "manual_sync"
	UpsertSourceReconciliation = "reconciliation"
)

// PlatformOrderUpsertInput is the unified entry for webhook and polling order writes.
type PlatformOrderUpsertInput struct {
	TenantID          int64
	Platform          string
	ShopID            uuid.UUID
	PlatformOrderID   string
	PlatformUpdatedAt *time.Time
	PlatformRevision  string
	EventType         string
	EventID           string
	Source            string
	NormalizedOrder   SyncedOrderPayload
	RequestID         string
}

// PlatformOrderUpsertResult describes one unified upsert outcome.
type PlatformOrderUpsertResult struct {
	OrderID      uuid.UUID
	Created      bool
	Updated      bool
	StaleIgnored bool
	Replayed     bool
	ResponseCode string
}

// UpsertPlatformOrder is the single business entry for platform order import (webhook + polling).
func (s *Service) UpsertPlatformOrder(ctx context.Context, input PlatformOrderUpsertInput) (*PlatformOrderUpsertResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	platformKey := strings.TrimSpace(input.Platform)
	if platformKey == "" {
		return nil, fmt.Errorf("platform is required")
	}
	ext := strings.TrimSpace(input.PlatformOrderID)
	if ext == "" {
		return nil, fmt.Errorf("platform order id is required")
	}
	if input.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id is required")
	}

	p := input.NormalizedOrder
	p.ExternalOrderID = ext
	if p.PlatformUpdatedAt == nil {
		p.PlatformUpdatedAt = input.PlatformUpdatedAt
	}
	if strings.TrimSpace(p.PlatformRevision) == "" {
		p.PlatformRevision = strings.TrimSpace(input.PlatformRevision)
	}
	if strings.TrimSpace(p.PlatformRevision) == "" && p.PlatformUpdatedAt != nil {
		p.PlatformRevision = revisionFromTime(*p.PlatformUpdatedAt)
	}

	outcome, err := s.importSyncedOrderWithIdempotency(ctx, input.ShopID, platformKey, p, importMeta{
		source:            strings.TrimSpace(input.Source),
		eventType:         strings.TrimSpace(input.EventType),
		eventID:           strings.TrimSpace(input.EventID),
		platformRevision:  strings.TrimSpace(p.PlatformRevision),
		platformUpdatedAt: p.PlatformUpdatedAt,
	})
	if err != nil {
		return nil, err
	}
	res := &PlatformOrderUpsertResult{
		OrderID:      outcome.orderID,
		Created:      outcome.isCreate,
		Updated:      !outcome.isCreate && !outcome.replayed && !outcome.staleIgnored,
		StaleIgnored: outcome.staleIgnored,
		Replayed:     outcome.replayed,
	}
	if outcome.staleIgnored {
		res.ResponseCode = codeOrderStaleUpdateIgnored
	} else if outcome.replayed {
		res.ResponseCode = codeOrderImportSucceeded
	} else {
		res.ResponseCode = codeOrderImportSucceeded
	}
	return res, nil
}

// UpsertPlatformOrders batch helper used by polling sync.
func (s *Service) UpsertPlatformOrders(ctx context.Context, shopID uuid.UUID, platform string, source string, payloads []SyncedOrderPayload) (orderIDs []uuid.UUID, success int, failed int, created int, updated int, err error) {
	for _, p := range payloads {
		res, upErr := s.UpsertPlatformOrder(ctx, PlatformOrderUpsertInput{
			Platform:        platform,
			ShopID:          shopID,
			PlatformOrderID: strings.TrimSpace(p.ExternalOrderID),
			Source:          source,
			NormalizedOrder: p,
		})
		if upErr != nil {
			failed++
			continue
		}
		success++
		if res.Created {
			created++
		} else if res.Updated {
			updated++
		}
		if res.OrderID != uuid.Nil {
			orderIDs = append(orderIDs, res.OrderID)
		}
	}
	return orderIDs, success, failed, created, updated, nil
}

func revisionFromTime(t time.Time) string {
	return fmt.Sprintf("t:%d", t.UTC().Unix())
}

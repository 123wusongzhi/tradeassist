package order

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
)

const (
	orderImportOwner            = "order-import"
	codeOrderStaleUpdateIgnored = "ORDER_STALE_UPDATE_IGNORED"
	codeOrderImportSucceeded    = "ORDER_IMPORT_SUCCEEDED"
	codeOrderImportInProgress   = "ORDER_IMPORT_IN_PROGRESS"
	codeOrderImportKeyConflict  = "ORDER_IMPORT_KEY_CONFLICT"
)

type syncedOrderImportOutcome struct {
	orderID      uuid.UUID
	isCreate     bool
	staleIgnored bool
	replayed     bool
}

type orderImportHashPayload struct {
	Status            string  `json:"status"`
	PaymentStatus     string  `json:"paymentStatus"`
	FulfillmentStatus string  `json:"fulfillmentStatus"`
	TotalAmount       float64 `json:"totalAmount"`
	OrderedAt         string  `json:"orderedAt,omitempty"`
	PaidAt            string  `json:"paidAt,omitempty"`
	ShippedAt         string  `json:"shippedAt,omitempty"`
	DeliveredAt       string  `json:"deliveredAt,omitempty"`
}

func orderImportRequestHash(p SyncedOrderPayload) string {
	payload := orderImportHashPayload{
		Status:            normalizeSyncedOrderStatus(p.Status),
		PaymentStatus:     normalizeSyncedPaymentStatus(p.PaymentStatus),
		FulfillmentStatus: normalizeSyncedFulfillmentStatus(p.FulfillmentStatus),
		TotalAmount:       p.TotalAmount,
		OrderedAt:         formatTimeRFC(p.OrderedAt),
		PaidAt:            formatTimeRFC(p.PaidAt),
		ShippedAt:         formatTimeRFC(p.ShippedAt),
		DeliveredAt:       formatTimeRFC(p.DeliveredAt),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return idempotency.HashRequest([]byte(fmt.Sprintf("%v", payload)))
	}
	return idempotency.HashRequest(b)
}

func orderLifecycleRank(status, paymentStatus, fulfillmentStatus string) int {
	st := strings.TrimSpace(strings.ToLower(status))
	ps := strings.TrimSpace(strings.ToLower(paymentStatus))
	fs := strings.TrimSpace(strings.ToLower(fulfillmentStatus))

	switch st {
	case StatusRefunded:
		return 90
	case StatusCancelled:
		return 85
	case StatusClosed:
		return 80
	case StatusDelivered:
		return 70
	case StatusShipped:
		return 60
	case StatusProcessing:
		return 50
	case StatusPaid:
		return 40
	case StatusPending:
		// fall through to payment/fulfillment hints
	}

	switch ps {
	case PaymentRefunded:
		return 90
	case PaymentPartiallyRefunded:
		return 55
	case PaymentPaid:
		if rank := fulfillmentLifecycleRank(fs); rank > 40 {
			return rank
		}
		return 40
	}

	if rank := fulfillmentLifecycleRank(fs); rank > 0 {
		return rank
	}
	return 10
}

func fulfillmentLifecycleRank(fs string) int {
	switch fs {
	case FulfillmentReturned:
		return 75
	case FulfillmentFulfilled:
		return 65
	case FulfillmentPartial:
		return 45
	default:
		return 0
	}
}

func orderLifecycleTimestamp(o *Order) time.Time {
	if o == nil {
		return time.Time{}
	}
	var max time.Time
	for _, t := range []*time.Time{o.OrderedAt, o.PaidAt, o.ShippedAt, o.DeliveredAt} {
		if t == nil || t.IsZero() {
			continue
		}
		utc := t.UTC()
		if max.IsZero() || utc.After(max) {
			max = utc
		}
	}
	return max
}

func payloadLifecycleTimestamp(p SyncedOrderPayload) time.Time {
	var max time.Time
	for _, t := range []*time.Time{p.OrderedAt, p.PaidAt, p.ShippedAt, p.DeliveredAt} {
		if t == nil || t.IsZero() {
			continue
		}
		utc := t.UTC()
		if max.IsZero() || utc.After(max) {
			max = utc
		}
	}
	return max
}

func isStaleSyncedUpdate(existing *Order, p SyncedOrderPayload) bool {
	if existing == nil {
		return false
	}
	incStatus := normalizeSyncedOrderStatus(p.Status)
	incPayment := normalizeSyncedPaymentStatus(p.PaymentStatus)
	incFulfillment := normalizeSyncedFulfillmentStatus(p.FulfillmentStatus)

	existRank := orderLifecycleRank(existing.Status, existing.PaymentStatus, existing.FulfillmentStatus)
	incRank := orderLifecycleRank(incStatus, incPayment, incFulfillment)
	if existRank > incRank {
		return true
	}
	if existRank < incRank {
		return false
	}

	existT := orderLifecycleTimestamp(existing)
	incT := payloadLifecycleTimestamp(p)
	if !existT.IsZero() && !incT.IsZero() && incT.Before(existT) {
		return true
	}
	return false
}

func (s *Service) findExistingSyncedOrder(ctx context.Context, shopID uuid.UUID, platformKey, ext string) (*Order, error) {
	var existing Order
	err := s.DB.WithContext(ctx).
		Where("shop_id = ? AND platform = ? AND external_order_id = ?", shopID, platformKey, ext).
		First(&existing).Error
	if err != nil {
		return nil, err
	}
	return &existing, nil
}

func (s *Service) importSyncedOrderWithIdempotency(ctx context.Context, shopID uuid.UUID, platformKey string, p SyncedOrderPayload) (syncedOrderImportOutcome, error) {
	var zero syncedOrderImportOutcome
	ext := strings.TrimSpace(p.ExternalOrderID)
	if ext == "" {
		return zero, fmt.Errorf("external order id is required")
	}

	if s.Idempotency == nil {
		orderID, isCreate, err := s.upsertSingleSyncedOrder(ctx, shopID, platformKey, p)
		if err != nil {
			return zero, err
		}
		return syncedOrderImportOutcome{orderID: orderID, isCreate: isCreate}, nil
	}

	key := idempotency.OrderImport(platformKey, shopID.String(), ext)
	reqHash := orderImportRequestHash(p)
	res, err := s.Idempotency.Acquire(ctx, idempotency.ScopeOrderImport, key, reqHash, orderImportOwner, idempotency.DefaultLease)
	decision, rec, classifyErr := idempotency.Classify(res, err)

	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		rid := ""
		if res != nil {
			rid = res.ResourceID
		}
		if rid == "" && rec != nil {
			rid = rec.ResourceID
		}
		if rid == "" {
			return zero, fmt.Errorf("%s", codeOrderImportInProgress)
		}
		oid, perr := uuid.Parse(rid)
		if perr != nil {
			return zero, fmt.Errorf("%s", codeOrderImportInProgress)
		}
		return syncedOrderImportOutcome{orderID: oid, replayed: true}, nil
	case idempotency.DecisionInProgress:
		return zero, fmt.Errorf("%s", codeOrderImportInProgress)
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		if classifyErr != nil {
			return zero, classifyErr
		}
		return zero, fmt.Errorf("%s", codeOrderImportKeyConflict)
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if res == nil || res.Record == nil {
			return zero, fmt.Errorf("idempotency: missing record")
		}
		recordID := res.Record.ID

		existing, findErr := s.findExistingSyncedOrder(ctx, shopID, platformKey, ext)
		if findErr == nil && isStaleSyncedUpdate(existing, p) {
			summary, _ := json.Marshal(map[string]string{
				"orderId": existing.ID.String(),
				"reason":  codeOrderStaleUpdateIgnored,
			})
			if err := s.Idempotency.Complete(ctx, recordID, orderImportOwner, idempotency.CompleteResult{
				ResponseCode:    codeOrderStaleUpdateIgnored,
				ResponseSummary: string(summary),
				ResourceType:    "order",
				ResourceID:      existing.ID.String(),
			}); err != nil {
				return zero, err
			}
			return syncedOrderImportOutcome{
				orderID:      existing.ID,
				staleIgnored: true,
			}, nil
		}

		orderID, isCreate, upErr := s.upsertSingleSyncedOrder(ctx, shopID, platformKey, p)
		if upErr != nil {
			_ = s.Idempotency.Fail(ctx, recordID, orderImportOwner, upErr.Error(), true)
			return zero, upErr
		}
		summary, _ := json.Marshal(map[string]string{"orderId": orderID.String()})
		if err := s.Idempotency.Complete(ctx, recordID, orderImportOwner, idempotency.CompleteResult{
			ResponseCode:    codeOrderImportSucceeded,
			ResponseSummary: string(summary),
			ResourceType:    "order",
			ResourceID:      orderID.String(),
		}); err != nil {
			return zero, err
		}
		return syncedOrderImportOutcome{orderID: orderID, isCreate: isCreate}, nil
	default:
		if classifyErr != nil {
			return zero, classifyErr
		}
		return zero, err
	}
}

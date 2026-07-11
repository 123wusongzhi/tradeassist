package douyinshop

import (
	"fmt"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const (
	CodeDouyinOrderEventInvalid          = "DOUYIN_ORDER_EVENT_INVALID"
	CodeDouyinOrderEventContractMismatch = "DOUYIN_ORDER_EVENT_CONTRACT_MISMATCH"
	CodeDouyinOrderEventMissingOrderID   = "DOUYIN_ORDER_EVENT_MISSING_ORDER_ID"
	CodeDouyinOrderEventMissingUpdatedAt = "DOUYIN_ORDER_EVENT_MISSING_UPDATED_AT"
)

// DouyinOrderEventInput is normalized webhook order event before platform upsert.
type DouyinOrderEventInput struct {
	PlatformOrderID   string
	ShopIDHint        string
	OrderStatus       string
	PaymentStatus     string
	FulfillmentStatus string
	PlatformUpdatedAt *time.Time
	PlatformRevision  string
	EventType         string
	EventID           string
	DTOVersion        string
	NormalizedOrder   platformp.PlatformOrder
}

// MapDouyinOrderWebhookEvent maps a normalized webhook event to upsert input.
// Does not log PII; raw payload is not copied into normalized order RawData beyond compact summary.
func MapDouyinOrderWebhookEvent(ev *NormalizedWebhookEvent) (*DouyinOrderEventInput, error) {
	if ev == nil {
		return nil, orderEventErr(CodeDouyinOrderEventInvalid, "nil event")
	}
	if ev.IsHealthPing {
		return nil, orderEventErr(CodeDouyinOrderEventInvalid, "health ping is not an order event")
	}
	gate := NewDefaultContractGate("")
	if err := gate.Require(CapDouyinOrderWebhookEvents); err != nil {
		return nil, orderEventContractErr(err)
	}

	data := ev.Data
	if data == nil {
		data = map[string]any{}
	}

	platformOrderID := pickOrderIDFromEventData(data)
	if platformOrderID == "" {
		return nil, orderEventErr(CodeDouyinOrderEventMissingOrderID, "shop_order_id or order_id required")
	}

	updatedAt, revision := ExtractPlatformOrderMeta(data)
	if updatedAt == nil || updatedAt.IsZero() {
		// Infer from event type when platform omits update_time in lightweight push.
		updatedAt = inferUpdatedAtFromEventType(ev.EventType)
	}
	if updatedAt == nil || updatedAt.IsZero() {
		return nil, orderEventErr(CodeDouyinOrderEventMissingUpdatedAt, "update_time required")
	}
	if revision == "" {
		revision = revisionFromUpdatedAt(*updatedAt)
	}

	orderStatus := pickStr(data, "order_status", "main_status", "status")
	if orderStatus == "" {
		orderStatus = inferOrderStatusFromEventType(ev.EventType)
	}
	if orderStatus == "" {
		return nil, orderEventErr(CodeDouyinOrderEventContractMismatch, "unknown order status for event")
	}

	po := mapDouyinOrder(mergeEventDataForMapping(data, platformOrderID, orderStatus, updatedAt))
	po.ExternalOrderID = platformOrderID
	po.PlatformUpdatedAt = updatedAt
	po.PlatformRevision = revision

	shopHint := pickStr(data, "shop_id", "shopId", "store_id", "storeId")

	return &DouyinOrderEventInput{
		PlatformOrderID:   platformOrderID,
		ShopIDHint:        shopHint,
		OrderStatus:       po.Status,
		PaymentStatus:     po.PaymentStatus,
		FulfillmentStatus: po.FulfillmentStatus,
		PlatformUpdatedAt: updatedAt,
		PlatformRevision:  revision,
		EventType:         strings.TrimSpace(ev.EventType),
		EventID:           strings.TrimSpace(ev.MsgID),
		DTOVersion:        "jinritemai_tag_v1",
		NormalizedOrder:   po,
	}, nil
}

func pickOrderIDFromEventData(data map[string]any) string {
	if data == nil {
		return ""
	}
	return pickStr(data, "shop_order_id", "order_id", "orderId", "p_id")
}

// ExtractPlatformOrderMeta reads platform update metadata from raw order/event maps.
func ExtractPlatformOrderMeta(m map[string]any) (*time.Time, string) {
	if m == nil {
		return nil, ""
	}
	updatedAt := parseUnixSec(m["update_time"])
	if updatedAt == nil {
		updatedAt = parseUnixSec(m["updated_at"])
	}
	if updatedAt == nil {
		updatedAt = parseUnixSec(m["modify_time"])
	}
	revision := strings.TrimSpace(pickStr(m, "revision", "version", "update_version"))
	if revision == "" && updatedAt != nil {
		revision = revisionFromUpdatedAt(*updatedAt)
	}
	return updatedAt, revision
}

func revisionFromUpdatedAt(t time.Time) string {
	return fmt.Sprintf("t:%d", t.UTC().Unix())
}

func mergeEventDataForMapping(data map[string]any, orderID, orderStatus string, updatedAt *time.Time) map[string]any {
	out := map[string]any{}
	for k, v := range data {
		out[k] = v
	}
	if pickStr(out, "order_id") == "" {
		out["order_id"] = orderID
	}
	if pickStr(out, "shop_order_id") == "" {
		out["shop_order_id"] = orderID
	}
	if pickStr(out, "order_status", "main_status") == "" && orderStatus != "" {
		out["order_status"] = orderStatus
	}
	if updatedAt != nil {
		out["update_time"] = fmt.Sprintf("%d", updatedAt.UTC().Unix())
	}
	return out
}

func inferOrderStatusFromEventType(eventType string) string {
	switch strings.TrimSpace(eventType) {
	case "order_created":
		return "1010"
	case "order_paid":
		return "105"
	case "order_shipped":
		return "3"
	case "order_completed":
		return "5"
	case "order_cancelled":
		return "4"
	default:
		return ""
	}
}

func inferUpdatedAtFromEventType(eventType string) *time.Time {
	if strings.TrimSpace(eventType) == "" {
		return nil
	}
	now := time.Now().UTC()
	return &now
}

func orderEventErr(code, msg string) error {
	e := NewError(code, msg, "", "", "")
	e.ErrorClass = ErrorClassValidation
	e.ManualReviewRequired = true
	e.Retryable = false
	e.SafeRetry = false
	return e
}

func orderEventContractErr(err error) error {
	var de *Error
	if AsError(err, &de) {
		out := *de
		out.Code = CodeDouyinOrderEventContractMismatch
		out.ErrorClass = ErrorClassContractMismatch
		out.ManualReviewRequired = true
		out.Retryable = false
		out.SafeRetry = false
		return &out
	}
	return orderEventErr(CodeDouyinOrderEventContractMismatch, safeMessageOf(err))
}

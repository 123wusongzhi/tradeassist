package douyinshop

import (
	"context"
	"fmt"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// MethodOrderDetail is the official jinritemai OpenAPI method for single order detail.
// Source: https://op.jinritemai.com/docs/guide-docs/154/2034 (order.orderDetail)
const MethodOrderDetail = "order.orderDetail"

// OrderDetail is the normalized P3 order detail response.
type OrderDetail struct {
	platformp.PlatformOrder
	// Additional detail fields not available in order list
	BuyerNote   string         `json:"buyerNote,omitempty"`
	PaymentInfo map[string]any `json:"paymentInfo,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

// GetOrderDetail fetches a single order by shop_order_id via order.orderDetail.
// Only field names confirmed in Douyin OpenAPI docs are used.
// If response parse fails with unexpected shape, returns CodeDouyinContractMismatch.
func (c *Client) GetOrderDetail(ctx context.Context, shopOrderID string) (*OrderDetail, error) {
	shopOrderID = strings.TrimSpace(shopOrderID)
	if shopOrderID == "" {
		return nil, NewError(CodeDouyinValidationFailed, "shop_order_id is required for order.orderDetail", "", "", "")
	}

	params := map[string]any{
		"shop_order_id": shopOrderID,
	}

	var raw map[string]any
	if err := c.Do(ctx, MethodOrderDetail, params, &raw); err != nil {
		return nil, mapOrderDetailError(err)
	}

	detail, err := parseOrderDetailRaw(raw)
	if err != nil {
		return nil, NewError(CodeDouyinContractMismatch,
			fmt.Sprintf("order.orderDetail response shape mismatch: %v", err),
			"", "contract_mismatch", "")
	}
	return detail, nil
}

func parseOrderDetailRaw(raw map[string]any) (*OrderDetail, error) {
	if raw == nil {
		return nil, fmt.Errorf("nil response")
	}
	// Attempt to parse from raw using existing mapDouyinOrder helper
	po := mapDouyinOrder(raw)
	if po.ExternalOrderID == "" {
		// Try nested "order" key — some API versions wrap data
		if inner, ok := raw["order"].(map[string]any); ok && inner != nil {
			po = mapDouyinOrder(inner)
			raw = inner
		}
	}
	if po.ExternalOrderID == "" {
		return nil, fmt.Errorf("could not parse platform_order_id from response")
	}
	detail := &OrderDetail{
		PlatformOrder: po,
		Raw:           sanitizeRawMap(raw),
	}
	if v, ok := raw["buyer_note"].(string); ok {
		detail.BuyerNote = strings.TrimSpace(v)
	}
	if v, ok := raw["pay_info"].(map[string]any); ok {
		detail.PaymentInfo = sanitizeRawMap(v)
	}
	return detail, nil
}

// ParseOrderDetailRawForTest is an exported wrapper for use in tests only.
func ParseOrderDetailRawForTest(raw map[string]any) (*OrderDetail, error) {
	return parseOrderDetailRaw(raw)
}

func mapOrderDetailError(err error) *Error {
	var de *Error
	if AsError(err, &de) {
		switch de.Code {
		case CodeDouyinAuthExpired, CodeDouyinPermissionDenied, CodeDouyinRateLimited:
			return de
		}
		de.Code = CodeDouyinOrderDetailFailed
		return de
	}
	return NewError(CodeDouyinOrderDetailFailed, "order.orderDetail failed", "", safeMessageOf(err), "")
}

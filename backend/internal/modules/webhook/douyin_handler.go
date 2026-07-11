package webhook

import (
	"context"
	"log/slog"

	douyinshop "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

// OrderEventHandler receives normalized order events from Douyin webhooks.
// Implementations may upsert orders, trigger sync tasks, etc.
type OrderEventHandler interface {
	HandleDouyinOrderEvent(ctx context.Context, ev *douyinshop.NormalizedWebhookEvent) error
}

// douyinEventDispatcher routes normalized Douyin events to typed handlers.
type douyinEventDispatcher struct {
	OrderHandler OrderEventHandler
}

// DispatchDouyinEvent routes a normalized event to the appropriate handler.
// Unknown event types are safely ACK'd with a warning log — never dropped silently.
func (d *douyinEventDispatcher) DispatchDouyinEvent(ctx context.Context, ev *douyinshop.NormalizedWebhookEvent) error {
	if ev == nil {
		return nil
	}
	if ev.IsHealthPing {
		slog.InfoContext(ctx, "douyin webhook health ping received")
		return nil
	}
	switch ev.EventType {
	case "order_created", "order_paid", "order_shipped", "order_completed", "order_cancelled":
		if d.OrderHandler != nil {
			return d.OrderHandler.HandleDouyinOrderEvent(ctx, ev)
		}
		slog.WarnContext(ctx, "douyin order event received but no OrderEventHandler configured",
			"eventType", ev.EventType, "msgId", ev.MsgID)
		return nil
	case "inventory_alert":
		slog.InfoContext(ctx, "douyin inventory_alert received — handler not implemented in P3",
			"msgId", ev.MsgID)
		return nil
	case "product_status_changed":
		slog.InfoContext(ctx, "douyin product_status_changed received — handler not implemented in P3",
			"msgId", ev.MsgID)
		return nil
	default:
		// Unknown tag — safe ACK with warning
		slog.WarnContext(ctx, "douyin webhook unknown event type — safe ACK",
			"eventType", ev.EventType, "tag", ev.Tag, "msgId", ev.MsgID)
		return nil
	}
}

// HandleDouyinPlatformEvent is the entry point called from processor.handlePlatformEvent
// for platforms "douyin_shop" and "douyin".
// It parses the raw event payload and dispatches to the event dispatcher.
func (s *Service) HandleDouyinPlatformEvent(ctx context.Context, ev *Event) error {
	if ev == nil {
		return nil
	}
	payload := []byte(ev.PayloadBody)
	if len(payload) == 0 {
		payload = []byte(ev.RawSummary)
	}

	// Try standard Douyin envelope first
	if env, err := douyinshop.ParseDouyinWebhookEnvelope(payload); err == nil && env.Event != "" {
		normalized := douyinshop.NormalizeDouyinEnvelope(env, payload)
		dispatcher := &douyinEventDispatcher{}
		return dispatcher.DispatchDouyinEvent(ctx, normalized)
	}

	// Try jinritemai array push
	if items, err := douyinshop.ParseJinriteimaiPushEnvelope(payload); err == nil && len(items) > 0 {
		dispatcher := &douyinEventDispatcher{}
		for _, item := range items {
			normalized := douyinshop.NormalizeJinriteimaiItem(item, payload)
			if err := dispatcher.DispatchDouyinEvent(ctx, normalized); err != nil {
				return err
			}
		}
		return nil
	}

	// Unrecognized shape — safe ACK
	slog.WarnContext(ctx, "douyin webhook payload did not match any known envelope format — safe ACK",
		"platform", ev.Platform, "eventId", ev.EventID)
	return nil
}

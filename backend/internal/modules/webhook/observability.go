package webhook

import (
	"strings"
	"time"
)

// ObserveWebhook records webhook metrics through the shared P5 catalog.
func (s *Service) ObserveWebhook(platform, eventType, event, result, errorClass string, dur time.Duration) {
	if s == nil || s.Metrics == nil {
		return
	}
	s.Metrics.ObserveWebhook(safeWebhookPlatform(platform), eventGroup(eventType), event, result, errorClass, dur)
}

func safeWebhookPlatform(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "douyin", "douyin_shop", "internal_test":
		return v
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func eventGroup(eventType string) string {
	v := strings.ToLower(strings.TrimSpace(eventType))
	switch {
	case strings.Contains(v, "order"):
		return "order"
	case strings.Contains(v, "product"):
		return "product"
	case strings.Contains(v, "refund"):
		return "refund"
	case v == "":
		return "unknown"
	default:
		return "other"
	}
}

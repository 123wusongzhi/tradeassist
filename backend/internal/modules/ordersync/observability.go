package ordersync

import (
	"strings"
	"time"
)

// ObserveOrder records order sync metrics through the shared P5 catalog.
func (s *Service) ObserveOrder(platform, source, event, result, errorClass string, count int, dur time.Duration, cursorLag time.Duration) {
	if s == nil || s.Metrics == nil {
		return
	}
	s.Metrics.ObserveOrder(safeOrderPlatform(platform), safeOrderSource(source), event, result, errorClass, count, dur, cursorLag)
}

func safeOrderPlatform(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "douyin_shop", "tiktok", "shopee", "lazada", "amazon", "manual":
		return v
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func safeOrderSource(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "polling", "webhook", "manual", "reconciliation":
		return v
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func sourceFromMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case ModeManual:
		return "manual"
	case ModeFull, ModeIncremental:
		return "polling"
	default:
		return "unknown"
	}
}

func classifyOrderSyncError(msg string) string {
	msg = strings.ToLower(strings.TrimSpace(msg))
	switch {
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline"):
		return "provider_timeout"
	case strings.Contains(msg, "database"), strings.Contains(msg, "sql"), strings.Contains(msg, "db"):
		return "database"
	case msg == "":
		return "unknown"
	default:
		return "error"
	}
}

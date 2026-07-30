package inventory

import (
	"strings"
	"time"
)

// ObserveInventory records inventory metrics through the shared P5 catalog.
func (s *Service) ObserveInventory(platform, operation, event, result, errorClass string, count int, dur time.Duration) {
	if s == nil || s.Metrics == nil {
		return
	}
	s.Metrics.ObserveInventory(safeInventoryPlatform(platform), safeInventoryOperation(operation), event, result, errorClass, count, dur)
}

func safeInventoryPlatform(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "local", "douyin_shop", "tiktok", "shopee", "lazada", "amazon":
		return v
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func safeInventoryOperation(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "adjust", "deduct", "compensate", "query", "push":
		return v
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func classifyInventoryError(msg string) string {
	msg = strings.ToLower(strings.TrimSpace(msg))
	switch {
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline"):
		return "provider_timeout"
	case strings.Contains(msg, "conflict"), strings.Contains(msg, "version"):
		return "version_conflict"
	case strings.Contains(msg, "database"), strings.Contains(msg, "sql"), strings.Contains(msg, "db"):
		return "database"
	case msg == "":
		return "unknown"
	default:
		return "error"
	}
}

func isInventoryTimeout(msg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg))
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline")
}

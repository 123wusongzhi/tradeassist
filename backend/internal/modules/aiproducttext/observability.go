package aiproducttext

import (
	"strings"
	"time"
)

// ObserveAIText records AI text metrics through the shared P5 catalog.
func (s *Service) ObserveAIText(provider, operation, event, result, errorClass string, dur time.Duration) {
	if s == nil || s.Metrics == nil {
		return
	}
	s.Metrics.ObserveAIText(safeAITextProvider(provider), safeAITextOperation(operation), event, result, errorClass, dur)
}

func safeAITextProvider(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "internal", "configured", "openai", "dashscope", "deepseek", "mock":
		return v
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func safeAITextOperation(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case OpTitle, OpDescription, "batch", "batch_create", "apply", "undo":
		return v
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func classifyAITextError(msg string) string {
	msg = strings.ToLower(strings.TrimSpace(msg))
	switch {
	case isAITextTimeout(msg):
		return "provider_timeout"
	case strings.Contains(msg, "config"), strings.Contains(msg, "missing"):
		return "environment_blocked"
	case msg == "":
		return "unknown"
	default:
		return "provider_failure"
	}
}

func isAITextTimeout(msg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg))
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline")
}

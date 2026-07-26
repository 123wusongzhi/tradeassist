package aiproductimage

import (
	"strings"
	"time"
)

// ObserveAIImage records AI image metrics through the shared P5 catalog.
func (s *Service) ObserveAIImage(provider, operation, event, result, errorClass string, dur time.Duration) {
	if s == nil || s.Metrics == nil {
		return
	}
	s.Metrics.ObserveAIImage(safeAIImageProvider(provider), safeAIImageOperation(operation), event, result, errorClass, dur)
}

func safeAIImageProvider(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "internal", "configured", "dashscope_image", "removebg", "mock":
		return v
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func safeAIImageOperation(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case OpQualityCheck, OpRemoveWatermark, OpRemoveLogo, OpWhiteBackground, OpOptimizeBackground, OpTranslateText, OpSelectBestMain:
		return v
	case "batch", "batch_create", "generation", "apply":
		return v
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func classifyAIImageError(msg string) string {
	msg = strings.ToLower(strings.TrimSpace(msg))
	switch {
	case isAIImageTimeout(msg):
		return "provider_timeout"
	case strings.Contains(msg, "config"), strings.Contains(msg, "missing"):
		return "environment_blocked"
	case msg == "":
		return "unknown"
	default:
		return "provider_failure"
	}
}

func isAIImageTimeout(msg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg))
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") || strings.Contains(msg, CodeProviderTimeout)
}

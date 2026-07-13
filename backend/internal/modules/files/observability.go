package files

import (
	"strings"
	"time"
)

// ObserveFileScan records file scan metrics through the shared P5 catalog.
func (s *Service) ObserveFileScan(scanner, event, result, mimeGroup string, dur time.Duration) {
	if s == nil || s.Metrics == nil {
		return
	}
	s.Metrics.ObserveFileScan(safeFileScanner(scanner), event, result, mimeGroup, dur)
}

func safeFileScanner(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "basic", "composite":
		return v
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func mimeGroup(contentType string) string {
	v := strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case strings.HasPrefix(v, "image/"):
		return "image"
	case strings.HasPrefix(v, "text/"):
		return "text"
	case strings.Contains(v, "pdf"):
		return "pdf"
	case strings.Contains(v, "zip") || strings.Contains(v, "archive"):
		return "archive"
	case v == "":
		return "unknown"
	default:
		return "other"
	}
}

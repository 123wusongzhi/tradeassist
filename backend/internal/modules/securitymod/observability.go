package securitymod

import "strings"

// ObserveSecurity records security metrics through the shared P5 catalog.
func (s *Service) ObserveSecurity(module, event, result, severity string) {
	if s == nil || s.Metrics == nil {
		return
	}
	s.Metrics.ObserveSecurity(safeSecurityModule(module), safeSecurityEvent(event), result, safeSecuritySeverity(severity))
}

func safeSecurityModule(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "auth", "shop", "tenant", "operationlog", "security", "files":
		return v
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func safeSecurityEvent(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "authorization_denied", "tenant_access_denied", "shop_access_denied", "system_context_denied", "idor_attempt", "csrf_rejected", "origin_rejected", "open_redirect_rejected", "audit_chain_mismatch", "security_event":
		return v
	case "":
		return "security_event"
	default:
		return "security_event"
	}
}

func safeSecuritySeverity(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "info", "warning", "critical":
		return v
	case "":
		return "warning"
	default:
		return "warning"
	}
}

package files

import (
	"fmt"
	"strings"
)

// Security status values.
const (
	SecurityUploaded    = "uploaded"
	SecurityPendingScan = "pending_scan"
	SecurityScanning    = "scanning"
	SecurityClean       = "clean"
	SecurityRejected    = "rejected"
	SecurityQuarantined = "quarantined"
	SecurityScanFailed  = "scan_failed"
	SecurityDeleted     = "deleted"
)

var allowedTransitions = map[string]map[string]struct{}{
	SecurityUploaded:    {SecurityPendingScan: {}},
	SecurityPendingScan: {SecurityScanning: {}},
	SecurityScanning:    {SecurityClean: {}, SecurityRejected: {}, SecurityQuarantined: {}, SecurityScanFailed: {}},
	SecurityScanFailed:  {SecurityPendingScan: {}},
	SecurityClean:       {SecurityQuarantined: {}},
}

// CanTransition reports whether a security status change is allowed.
func CanTransition(from, to string) bool {
	from = strings.TrimSpace(strings.ToLower(from))
	to = strings.TrimSpace(strings.ToLower(to))
	if from == to {
		return true
	}
	next, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	_, ok = next[to]
	return ok
}

// TransitionSecurityStatus validates and returns the next status.
func TransitionSecurityStatus(current, next string) (string, error) {
	current = strings.TrimSpace(strings.ToLower(current))
	next = strings.TrimSpace(strings.ToLower(next))
	if next == "" {
		return "", fmt.Errorf("empty security status")
	}
	if !CanTransition(current, next) {
		return "", fmt.Errorf("invalid security status transition: %s -> %s", current, next)
	}
	return next, nil
}

// IsAccessible reports whether file can be downloaded by ordinary users.
func IsAccessible(status string) bool {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case SecurityClean:
		return true
	default:
		return false
	}
}

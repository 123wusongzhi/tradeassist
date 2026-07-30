package order

import (
	"fmt"
	"strings"
	"time"
)

// ValidateOrderStateTransition returns false when incoming would illegally regress lifecycle.
// Cancel/refund from platform may regress; other backward moves are blocked unless same revision replay.
func ValidateOrderStateTransition(currentStatus, currentPayment, currentFulfillment, incomingStatus, incomingPayment, incomingFulfillment string) bool {
	curRank := orderLifecycleRank(
		normalizeSyncedOrderStatus(currentStatus),
		normalizeSyncedPaymentStatus(currentPayment),
		normalizeSyncedFulfillmentStatus(currentFulfillment),
	)
	incRank := orderLifecycleRank(
		normalizeSyncedOrderStatus(incomingStatus),
		normalizeSyncedPaymentStatus(incomingPayment),
		normalizeSyncedFulfillmentStatus(incomingFulfillment),
	)
	if incRank >= curRank {
		return true
	}
	// Platform may explicitly cancel/refund after ship in some channels.
	incSt := normalizeSyncedOrderStatus(incomingStatus)
	if incSt == StatusCancelled || incSt == StatusRefunded {
		return true
	}
	return false
}

// isStalePlatformUpdate compares revision and platformUpdatedAt before lifecycle heuristics.
func isStalePlatformUpdate(existing *Order, revision string, updatedAt *time.Time, p SyncedOrderPayload) bool {
	if existing == nil {
		return false
	}
	incRev := strings.TrimSpace(revision)
	if incRev == "" {
		incRev = strings.TrimSpace(p.PlatformRevision)
	}
	existRev := strings.TrimSpace(existing.PlatformRevision)
	if existRev != "" && incRev != "" {
		if existRev == incRev {
			return true // duplicate revision — treat as stale/no-op for downstream side effects
		}
		if platformRevisionOlder(incRev, existRev) {
			return true
		}
	}

	incAt := updatedAt
	if incAt == nil || incAt.IsZero() {
		incAt = p.PlatformUpdatedAt
	}
	existAt := existing.PlatformUpdatedAt
	if existAt != nil && !existAt.IsZero() && incAt != nil && !incAt.IsZero() {
		if incAt.Before(existAt.UTC()) {
			return true
		}
		if incAt.Equal(existAt.UTC()) && existRev != "" && incRev != "" && existRev == incRev {
			return true
		}
	}

	return isStaleSyncedUpdate(existing, p)
}

func platformRevisionOlder(incoming, existing string) bool {
	incoming = strings.TrimSpace(incoming)
	existing = strings.TrimSpace(existing)
	if incoming == "" || existing == "" {
		return false
	}
	if strings.HasPrefix(incoming, "t:") && strings.HasPrefix(existing, "t:") {
		var inSec, exSec int64
		_, _ = fmt.Sscanf(incoming, "t:%d", &inSec)
		_, _ = fmt.Sscanf(existing, "t:%d", &exSec)
		return inSec < exSec
	}
	return incoming < existing
}

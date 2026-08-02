package files

import "testing"

func TestScanFailedCanBeRetried(t *testing.T) {
	if !CanTransition(SecurityScanFailed, SecurityScanning) {
		t.Fatal("scan_failed must be retryable")
	}
}

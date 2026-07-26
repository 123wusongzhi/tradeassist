package filescanner

import (
	"context"
	"time"
)

// Scan status constants.
const (
	ResultClean       = "clean"
	ResultRejected    = "rejected"
	ResultQuarantined = "quarantined"
	ResultScanFailed  = "scan_failed"
)

// ScanInput carries validated file metadata for scanning.
type ScanInput struct {
	TenantID      int64
	AssetID       string
	ObjectKey     string
	MimeType      string
	Size          int64
	ContentHash   string
	LocalTempPath string
}

// ScanResult is the outcome of a file scan.
type ScanResult struct {
	Status         string
	ReasonCode     string
	SafeSummary    string
	ScannerVersion string
	ScannedAt      time.Time
}

// FileScanner scans uploaded content for policy violations.
type FileScanner interface {
	Name() string
	Scan(ctx context.Context, input ScanInput) (ScanResult, error)
}

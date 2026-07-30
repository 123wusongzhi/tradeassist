package filescanner

import "context"

// CompositeFileScanner runs scanners in order; first non-clean result wins.
type CompositeFileScanner struct {
	Scanners []FileScanner
}

func (c *CompositeFileScanner) Name() string { return "composite" }

func (c *CompositeFileScanner) Scan(ctx context.Context, input ScanInput) (ScanResult, error) {
	for _, sc := range c.Scanners {
		if sc == nil {
			continue
		}
		res, err := sc.Scan(ctx, input)
		if err != nil {
			return ScanResult{
				Status:         ResultScanFailed,
				ReasonCode:     "scanner_error",
				SafeSummary:    sc.Name() + " failed",
				ScannerVersion: sc.Name(),
				ScannedAt:      res.ScannedAt,
			}, nil
		}
		if res.Status != ResultClean {
			return res, nil
		}
	}
	return ScanResult{Status: ResultClean, ReasonCode: "ok", SafeSummary: "all scanners passed"}, nil
}

// BasicFilePolicyScanner rejects non-image files in production paths.
type BasicFilePolicyScanner struct {
	AllowNonImage bool
}

func (b *BasicFilePolicyScanner) Name() string { return "basic_policy" }

func (b *BasicFilePolicyScanner) Scan(ctx context.Context, input ScanInput) (ScanResult, error) {
	if b != nil && b.AllowNonImage {
		return ScanResult{Status: ResultClean, ReasonCode: "non_image_allowed"}, nil
	}
	ct := input.MimeType
	if len(ct) >= 6 && ct[:6] == "image/" {
		return ScanResult{Status: ResultClean, ReasonCode: "image_type"}, nil
	}
	return ScanResult{
		Status:      ResultRejected,
		ReasonCode:  "non_image_rejected",
		SafeSummary: "non-image files rejected by policy",
	}, nil
}

// NoopScanner marks files clean — only for dev/test.
type NoopScanner struct{}

func (n *NoopScanner) Name() string { return "noop" }

func (n *NoopScanner) Scan(ctx context.Context, input ScanInput) (ScanResult, error) {
	return ScanResult{Status: ResultClean, ReasonCode: "noop", SafeSummary: "noop scanner"}, nil
}

package imagetask

import (
	"context"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/pkg/safedownload"
)

func TestResolveSourceRejectsPrivateRawURL(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1/internal.png",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/internal.png",
	} {
		_, _, err := (&Service{}).ResolveSource(context.Background(), 7, nil, raw)
		if err == nil || (!strings.Contains(err.Error(), safedownload.ErrPrivateHost) && !strings.Contains(err.Error(), safedownload.ErrPrivateIP) && !strings.Contains(err.Error(), safedownload.ErrMetadataEndpoint)) {
			t.Fatalf("expected private source URL %q to be rejected, got %v", raw, err)
		}
	}
}

func TestScoreImageHTTPRejectsPrivateRawURL(t *testing.T) {
	_, err := (&Service{}).ScoreImageHTTP(context.Background(), ScoreImageRequest{
		TenantID:       7,
		SourceImageURL: "http://127.0.0.1/admin",
	})
	if err == nil || !strings.Contains(err.Error(), safedownload.ErrPrivateHost) {
		t.Fatalf("expected private scoring URL to be rejected, got %v", err)
	}
}

func TestPayloadFromDataURLRejectsOversizedInputBeforeDecode(t *testing.T) {
	raw := "data:image/png;base64," + strings.Repeat("A", int((maxTranslateImageBytes*4/3)+17))
	if _, err := payloadFromDataURL(raw); err == nil || !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("expected oversized data URL rejection, got %v", err)
	}
}

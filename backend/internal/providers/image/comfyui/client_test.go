package comfyui

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/pkg/safedownload"
)

func excessiveWidthPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 9000, 1))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDownloadViewRejectsExcessiveImageDimensionsBeforeDecode(t *testing.T) {
	payload := excessiveWidthPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	client, err := NewClient(Options{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = client.downloadView(t.Context(), "oversized.png", "", "output")
	if err == nil || !strings.Contains(err.Error(), safedownload.ErrImageDimensions) {
		t.Fatalf("expected dimension rejection, got %v", err)
	}
}

func TestEncodeAsPNGRejectsExcessiveImageDimensions(t *testing.T) {
	_, err := encodeAsPNG(excessiveWidthPNG(t))
	if err == nil || !strings.Contains(err.Error(), safedownload.ErrImageDimensions) {
		t.Fatalf("expected dimension rejection, got %v", err)
	}
}

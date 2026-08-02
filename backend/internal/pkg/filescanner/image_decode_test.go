package filescanner

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestImageDecodeScannerZeroValueUsesSafeDefaults(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "image-*.png")
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	s := &ImageDecodeScanner{}
	got, err := s.Scan(context.Background(), ScanInput{LocalTempPath: f.Name(), MimeType: "image/png"})
	if err != nil || got.Status != ResultClean {
		t.Fatalf("zero-value scan = %#v, %v", got, err)
	}
}

func TestImageDecodeScannerRejectsInvalidLimits(t *testing.T) {
	s := &ImageDecodeScanner{MaxBytes: -1}
	if err := s.Normalize(); err == nil {
		t.Fatal("expected invalid limits error")
	}
}

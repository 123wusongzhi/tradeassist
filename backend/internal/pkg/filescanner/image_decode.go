package filescanner

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/image/webp"
)

const ImageDecodeScannerVersion = "image-decode-v1"

// ImageDecodeScanner validates image magic bytes, decode, dimensions and size.
type ImageDecodeScanner struct {
	MaxBytes      int64
	MaxPixels     int64
	MaxWidth      int
	MaxHeight     int
	MaxAnimFrames int
	AllowedMIME   map[string]struct{}
}

// NewImageDecodeScanner returns production-safe image scanner defaults.
func NewImageDecodeScanner(maxBytes, maxPixels int64, maxW, maxH, maxFrames int) *ImageDecodeScanner {
	allowed := map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
		"image/gif":  {},
	}
	return &ImageDecodeScanner{
		MaxBytes:      maxBytes,
		MaxPixels:     maxPixels,
		MaxWidth:      maxW,
		MaxHeight:     maxH,
		MaxAnimFrames: maxFrames,
		AllowedMIME:   allowed,
	}
}

// Normalize applies safe limits to a zero-value scanner and rejects invalid
// caller-provided limits before scanning any content.
func (s *ImageDecodeScanner) Normalize() error {
	if s == nil {
		return fmt.Errorf("image scanner is nil")
	}
	defaults := NewImageDecodeScanner(10<<20, 50_000_000, 8192, 8192, 300)
	if s.MaxBytes == 0 {
		s.MaxBytes = defaults.MaxBytes
	}
	if s.MaxPixels == 0 {
		s.MaxPixels = defaults.MaxPixels
	}
	if s.MaxWidth == 0 {
		s.MaxWidth = defaults.MaxWidth
	}
	if s.MaxHeight == 0 {
		s.MaxHeight = defaults.MaxHeight
	}
	if s.MaxAnimFrames == 0 {
		s.MaxAnimFrames = defaults.MaxAnimFrames
	}
	if s.AllowedMIME == nil {
		s.AllowedMIME = defaults.AllowedMIME
	}
	if s.MaxBytes < 1 || s.MaxPixels < 1 || s.MaxWidth < 1 || s.MaxHeight < 1 || s.MaxAnimFrames < 1 {
		return fmt.Errorf("image scanner limits must be positive")
	}
	return nil
}

func (s *ImageDecodeScanner) Name() string { return "image_decode" }

func (s *ImageDecodeScanner) Scan(ctx context.Context, input ScanInput) (ScanResult, error) {
	if err := s.Normalize(); err != nil {
		return ScanResult{}, err
	}
	now := time.Now().UTC()
	fail := func(code, summary string) (ScanResult, error) {
		return ScanResult{
			Status:         ResultRejected,
			ReasonCode:     code,
			SafeSummary:    summary,
			ScannerVersion: ImageDecodeScannerVersion,
			ScannedAt:      now,
		}, nil
	}
	if input.LocalTempPath == "" {
		return ScanResult{
			Status:         ResultScanFailed,
			ReasonCode:     "missing_content",
			SafeSummary:    "scanner input missing",
			ScannerVersion: ImageDecodeScannerVersion,
			ScannedAt:      now,
		}, nil
	}
	f, err := os.Open(input.LocalTempPath)
	if err != nil {
		return ScanResult{
			Status:         ResultScanFailed,
			ReasonCode:     "read_failed",
			SafeSummary:    "unable to read file for scan",
			ScannerVersion: ImageDecodeScannerVersion,
			ScannedAt:      now,
		}, nil
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, s.MaxBytes+1))
	if err != nil {
		return ScanResult{
			Status:         ResultScanFailed,
			ReasonCode:     "read_failed",
			SafeSummary:    "unable to read file for scan",
			ScannerVersion: ImageDecodeScannerVersion,
			ScannedAt:      now,
		}, nil
	}
	if int64(len(data)) > s.MaxBytes {
		return fail("file_too_large", "file exceeds maximum allowed size")
	}
	ct := strings.ToLower(strings.TrimSpace(strings.Split(input.MimeType, ";")[0]))
	if _, ok := s.AllowedMIME[ct]; !ok {
		return fail("mime_not_allowed", "image mime type not in allowlist")
	}
	cfg, decFormat, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		if _, werr := webp.DecodeConfig(bytes.NewReader(data)); werr != nil {
			return fail("decode_failed", "file is not a decodable image")
		}
		decFormat = "webp"
	}
	if s.MaxWidth > 0 && cfg.Width > s.MaxWidth {
		return fail("width_exceeded", "image width exceeds limit")
	}
	if s.MaxHeight > 0 && cfg.Height > s.MaxHeight {
		return fail("height_exceeded", "image height exceeds limit")
	}
	pixels := int64(cfg.Width) * int64(cfg.Height)
	if s.MaxPixels > 0 && pixels > s.MaxPixels {
		return fail("pixels_exceeded", "image total pixels exceed limit")
	}
	if decFormat == "gif" && s.MaxAnimFrames > 0 {
		if int64(len(data)) > s.MaxBytes/2 && pixels > s.MaxPixels/2 {
			return fail("gif_suspicious", "animated gif exceeds policy")
		}
	}
	_, _, err = image.Decode(bytes.NewReader(data))
	if err != nil {
		if _, werr := webp.Decode(bytes.NewReader(data)); werr != nil {
			return fail("decode_failed", "full decode failed")
		}
	}
	return ScanResult{
		Status:         ResultClean,
		ReasonCode:     "ok",
		SafeSummary:    "image passed decode policy",
		ScannerVersion: ImageDecodeScannerVersion,
		ScannedAt:      now,
	}, nil
}

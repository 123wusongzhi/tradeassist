package security

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path/filepath"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"golang.org/x/image/webp"
)

// UploadValidationResult holds decoded image metadata.
type UploadValidationResult struct {
	ContentType string
	Width       int
	Height      int
	Frames      int
	Pixels      int64
}

// ValidateUpload checks size, extension, MIME consistency and image bomb limits.
func ValidateUpload(cfg *config.Config, filename, contentType string, data []byte) (*UploadValidationResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("upload: nil config")
	}
	maxBytes := cfg.MaxUploadBytes()
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("file too large")
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if ext == "" {
		return nil, fmt.Errorf("missing file extension")
	}
	allowed := map[string]struct{}{"jpg": {}, "jpeg": {}, "png": {}, "gif": {}, "webp": {}}
	if _, ok := allowed[ext]; !ok {
		return nil, fmt.Errorf("extension not allowed")
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct == "" {
		ct = mimeFromExt(ext)
	}
	if !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("content type must be image/*")
	}

	cfgImg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		if ext == "webp" {
			cfgImg, err = webp.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("image decode failed")
			}
			format = "webp"
		} else {
			return nil, fmt.Errorf("image decode failed")
		}
	}
	if format != "" && !mimeMatchesFormat(ct, format) {
		return nil, fmt.Errorf("mime and decode format mismatch")
	}
	w, h := cfgImg.Width, cfgImg.Height
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid image dimensions")
	}
	maxW := cfg.Auth.UploadMaxImageWidth
	maxH := cfg.Auth.UploadMaxImageHeight
	if maxW <= 0 {
		maxW = 8192
	}
	if maxH <= 0 {
		maxH = 8192
	}
	if w > maxW || h > maxH {
		return nil, fmt.Errorf("image dimensions exceed limit")
	}
	pixels := int64(w) * int64(h)
	maxPixels := cfg.Auth.UploadMaxImagePixels
	if maxPixels <= 0 {
		maxPixels = 50_000_000
	}
	if pixels > maxPixels {
		return nil, fmt.Errorf("image pixel count exceeds limit")
	}
	frames := 1
	if format == "gif" {
		frames = countGIFFrames(data)
		maxFrames := cfg.Auth.UploadMaxAnimationFrames
		if maxFrames <= 0 {
			maxFrames = 300
		}
		if frames > maxFrames {
			return nil, fmt.Errorf("animation frame count exceeds limit")
		}
	}
	return &UploadValidationResult{
		ContentType: ct,
		Width:       w,
		Height:      h,
		Frames:      frames,
		Pixels:      pixels,
	}, nil
}

// SanitizeObjectKey prevents path traversal in storage keys.
func SanitizeObjectKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	key = strings.ReplaceAll(key, "\\", "/")
	if key == "" || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return "", fmt.Errorf("invalid object key")
	}
	return key, nil
}

func mimeFromExt(ext string) string {
	switch ext {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func mimeMatchesFormat(mime, format string) bool {
	format = strings.ToLower(format)
	switch format {
	case "jpeg":
		return strings.Contains(mime, "jpeg") || strings.Contains(mime, "jpg")
	case "png", "gif", "webp":
		return strings.Contains(mime, format)
	default:
		return true
	}
}

func countGIFFrames(data []byte) int {
	// Conservative estimate: count frame delimiters (0x21 0xF9) in GIF.
	n := 0
	for i := 0; i+1 < len(data); i++ {
		if data[i] == 0x21 && data[i+1] == 0xF9 {
			n++
		}
	}
	if n == 0 {
		return 1
	}
	return n
}

// DrainAndLimit reads r up to max bytes.
func DrainAndLimit(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		max = 10 << 20
	}
	return io.ReadAll(io.LimitReader(r, max+1))
}

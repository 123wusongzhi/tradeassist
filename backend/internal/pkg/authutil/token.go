package authutil

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// HashToken returns SHA-256 hex digest with optional pepper.
func HashToken(token, pepper string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(strings.TrimSpace(token)))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(strings.TrimSpace(pepper)))
	return hex.EncodeToString(h.Sum(nil))
}

// NewOpaqueToken generates a URL-safe random token.
func NewOpaqueToken(byteLen int) (string, error) {
	if byteLen < 16 {
		byteLen = 32
	}
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("token: rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashIP returns truncated hash of client IP.
func HashIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:8])
}

// SummarizeUserAgent returns short browser label.
func SummarizeUserAgent(ua string) string {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return "unknown"
	}
	low := strings.ToLower(ua)
	switch {
	case strings.Contains(low, "edg/"):
		return "Edge"
	case strings.Contains(low, "chrome/") && !strings.Contains(low, "chromium"):
		return "Chrome"
	case strings.Contains(low, "firefox/"):
		return "Firefox"
	case strings.Contains(low, "safari/") && !strings.Contains(low, "chrome"):
		return "Safari"
	default:
		if len(ua) > 48 {
			return ua[:48] + "…"
		}
		return ua
	}
}

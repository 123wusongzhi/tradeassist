package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
)

// PlatformInternalTest is the HMAC test platform (dev/test only).
const PlatformInternalTest = "internal-test"

// TestHMACSecret is the shared secret for the internal-test verifier (never use in production).
const TestHMACSecret = "trademind-internal-test-webhook-secret"

// VerifyInput is the normalized signature verification request.
type VerifyInput struct {
	Platform  string
	Headers   http.Header
	RawBody   []byte
	Timestamp time.Time
	Nonce     string
	Signature string
}

// SignatureVerifier validates inbound webhook authenticity.
type SignatureVerifier interface {
	Verify(ctx context.Context, input VerifyInput) error
}

// Registry maps platform → SignatureVerifier.
type Registry struct {
	verifiers map[string]SignatureVerifier
	appEnv    string
}

// NewRegistry builds a verifier registry from config.
func NewRegistry(cfg *config.Config) *Registry {
	appEnv := config.EnvDevelopment
	enableTest := false
	if cfg != nil {
		appEnv = config.NormalizeEnv(cfg.AppEnv)
		enableTest = cfg.WebhookEnableTestVerifier
	}
	r := &Registry{
		verifiers: make(map[string]SignatureVerifier),
		appEnv:    appEnv,
	}
	if enableTest && (appEnv == config.EnvDevelopment || appEnv == config.EnvTest || (appEnv == config.EnvPerformance && cfg != nil && cfg.P7.PerformanceTestMode)) {
		r.verifiers[PlatformInternalTest] = &HMACSHA256TestVerifier{Secret: []byte(TestHMACSecret)}
	}
	return r
}

// Register adds or replaces a platform verifier.
func (r *Registry) Register(platform string, v SignatureVerifier) {
	if r == nil || v == nil {
		return
	}
	platform = strings.TrimSpace(strings.ToLower(platform))
	if platform == "" {
		return
	}
	if r.verifiers == nil {
		r.verifiers = make(map[string]SignatureVerifier)
	}
	r.verifiers[platform] = v
}

// Verify dispatches to the platform verifier.
func (r *Registry) Verify(ctx context.Context, input VerifyInput) error {
	platform := strings.TrimSpace(strings.ToLower(input.Platform))
	if platform == "" {
		return newCodeError(CodeVerifierNotConfigured, http.StatusUnauthorized, CodeVerifierNotConfigured)
	}
	if config.IsProduction(r.appEnv) && platform == PlatformInternalTest {
		return newCodeError(CodeSignatureBypassForbidden, http.StatusForbidden, CodeSignatureBypassForbidden)
	}
	if r == nil || r.verifiers == nil {
		return newCodeError(CodeVerifierNotConfigured, http.StatusUnauthorized, CodeVerifierNotConfigured)
	}
	v, ok := r.verifiers[platform]
	if !ok || v == nil {
		return newCodeError(CodeVerifierNotConfigured, http.StatusUnauthorized, CodeVerifierNotConfigured)
	}
	return v.Verify(ctx, input)
}

// HMACSHA256TestVerifier validates X-Webhook-Signature as hex HMAC-SHA256(secret, "{unix}.{body}").
type HMACSHA256TestVerifier struct {
	Secret []byte
}

func (v *HMACSHA256TestVerifier) Verify(_ context.Context, input VerifyInput) error {
	sig := strings.TrimSpace(input.Signature)
	if sig == "" {
		sig = extractSignatureHeader(input.Headers)
	}
	if sig == "" {
		return newCodeError(CodeSignatureMissing, http.StatusUnauthorized, CodeSignatureMissing)
	}
	if input.Timestamp.IsZero() {
		return newCodeError(CodeTimestampMissing, http.StatusUnauthorized, CodeTimestampMissing)
	}
	secret := v.Secret
	if len(secret) == 0 {
		secret = []byte(TestHMACSecret)
	}
	expected := computeTestHMAC(secret, input.Timestamp, input.RawBody)
	got := normalizeSignatureHex(sig)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(got)) != 1 {
		return newCodeError(CodeSignatureInvalid, http.StatusUnauthorized, CodeSignatureInvalid)
	}
	return nil
}

func computeTestHMAC(secret []byte, ts time.Time, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(formatTimestampPayload(ts)))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func formatTimestampPayload(ts time.Time) string {
	return strconv.FormatInt(ts.Unix(), 10)
}

// SignTestPayload returns the hex HMAC for tests / internal-test clients.
func SignTestPayload(secret []byte, ts time.Time, body []byte) string {
	if len(secret) == 0 {
		secret = []byte(TestHMACSecret)
	}
	return computeTestHMAC(secret, ts, body)
}

func extractSignatureHeader(h http.Header) string {
	if h == nil {
		return ""
	}
	if v := strings.TrimSpace(h.Get("X-Webhook-Signature")); v != "" {
		return v
	}
	if v := strings.TrimSpace(h.Get("X-TradeMind-Signature")); v != "" {
		return v
	}
	return ""
}

func extractTimestampHeader(h http.Header) string {
	if h == nil {
		return ""
	}
	if v := strings.TrimSpace(h.Get("X-Webhook-Timestamp")); v != "" {
		return v
	}
	if v := strings.TrimSpace(h.Get("X-TradeMind-Timestamp")); v != "" {
		return v
	}
	return ""
}

func extractNonceHeader(h http.Header) string {
	if h == nil {
		return ""
	}
	if v := strings.TrimSpace(h.Get("X-Webhook-Nonce")); v != "" {
		return v
	}
	return strings.TrimSpace(h.Get("X-TradeMind-Nonce"))
}

func normalizeSignatureHex(sig string) string {
	sig = strings.TrimSpace(sig)
	lower := strings.ToLower(sig)
	for _, prefix := range []string{"sha256=", "v1="} {
		if strings.HasPrefix(lower, prefix) {
			return strings.ToLower(strings.TrimSpace(sig[len(prefix):]))
		}
	}
	return strings.ToLower(sig)
}

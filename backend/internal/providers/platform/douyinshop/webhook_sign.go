package douyinshop

import (
	"context"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// DouyinWebhookSignatureHeader is the header carrying the Douyin webhook signature.
// Douyin Open Platform docs describe both X-Douyin-Signature and X-Sign variants.
// Both are checked for maximum compatibility.
const (
	DouyinWebhookSignatureHeader  = "X-Douyin-Signature"
	DouyinWebhookSignatureHeader2 = "X-Sign"
)

// DouyinWebhookSecretProvider resolves the app secret for webhook signature verification.
type DouyinWebhookSecretProvider interface {
	GetWebhookSecret(ctx context.Context) (string, error)
}

// StaticSecretProvider wraps a pre-loaded secret (used in tests / settings-backed resolver).
type StaticSecretProvider struct {
	Secret string
}

func (p *StaticSecretProvider) GetWebhookSecret(_ context.Context) (string, error) {
	if p == nil || strings.TrimSpace(p.Secret) == "" {
		return "", NewError(CodeDouyinNotConfigured, "douyin webhook app_secret not configured", "", "", "")
	}
	return strings.TrimSpace(p.Secret), nil
}

// DouyinSignatureVerifier verifies Douyin Open Platform webhook signatures.
//
// Algorithm (from Douyin Open Platform webhook documentation):
//
//	SHA1( appSecret + rawBody )  →  hex lowercase
//	Compare with X-Douyin-Signature (case-insensitive hex)
//
// Reference: https://op.jinritemai.com/docs/guide-docs/7/52
type DouyinSignatureVerifier struct {
	SecretProvider DouyinWebhookSecretProvider
}

// Verify checks the Douyin webhook signature.
func (v *DouyinSignatureVerifier) Verify(ctx context.Context, rawBody []byte, headers http.Header) error {
	if v == nil || v.SecretProvider == nil {
		return NewError(CodeDouyinNotConfigured, "douyin webhook verifier not configured", "", "", "")
	}
	sig := extractDouyinSignature(headers)
	if sig == "" {
		return NewError(CodeDouyinValidationFailed, "missing douyin webhook signature header", "", "signature_missing", "")
	}
	secret, err := v.SecretProvider.GetWebhookSecret(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(secret) == "" {
		return NewError(CodeDouyinNotConfigured, "douyin webhook app_secret is empty", "", "", "")
	}
	expected := computeDouyinWebhookSig(secret, rawBody)
	got := strings.ToLower(strings.TrimSpace(sig))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(got)) != 1 {
		return NewError(CodeDouyinValidationFailed, "douyin webhook signature mismatch", "", "signature_invalid", "")
	}
	return nil
}

// computeDouyinWebhookSig computes SHA1(appSecret + rawBody) as hex lowercase.
func computeDouyinWebhookSig(appSecret string, rawBody []byte) string {
	h := sha1.New()
	_, _ = h.Write([]byte(appSecret))
	_, _ = h.Write(rawBody)
	return hex.EncodeToString(h.Sum(nil))
}

// ComputeDouyinWebhookSig is exported for testing.
func ComputeDouyinWebhookSig(appSecret string, rawBody []byte) string {
	return computeDouyinWebhookSig(appSecret, rawBody)
}

func extractDouyinSignature(h http.Header) string {
	if h == nil {
		return ""
	}
	if v := strings.TrimSpace(h.Get(DouyinWebhookSignatureHeader)); v != "" {
		return v
	}
	return strings.TrimSpace(h.Get(DouyinWebhookSignatureHeader2))
}

// VerifyDouyinWebhookSignature is a standalone convenience function for inline use.
func VerifyDouyinWebhookSignature(ctx context.Context, appSecret string, rawBody []byte, headers http.Header) error {
	v := &DouyinSignatureVerifier{
		SecretProvider: &StaticSecretProvider{Secret: appSecret},
	}
	return v.Verify(ctx, rawBody, headers)
}

// WebhookSignatureSummary describes the current verifier state.
func WebhookSignatureSummary(secretConfigured bool) string {
	if secretConfigured {
		return fmt.Sprintf("webhook signature verifier ready (SHA1)")
	}
	return "webhook signature verifier blocked: app_secret not configured (blocked_by_config)"
}

package douyinshop_test

import (
	"context"
	"net/http"
	"testing"

	douyinshop "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

func TestComputeDouyinWebhookSig(t *testing.T) {
	appSecret := "test-app-secret"
	body := []byte(`{"event":"order_created","content":{}}`)
	sig := douyinshop.ComputeDouyinWebhookSig(appSecret, body)
	if len(sig) != 40 {
		t.Errorf("expected 40-char hex SHA1, got %d chars: %s", len(sig), sig)
	}
	// Deterministic
	sig2 := douyinshop.ComputeDouyinWebhookSig(appSecret, body)
	if sig != sig2 {
		t.Errorf("signature not deterministic: %s != %s", sig, sig2)
	}
	// Different secret → different sig
	sigOther := douyinshop.ComputeDouyinWebhookSig("other-secret", body)
	if sig == sigOther {
		t.Error("different secrets produced same signature")
	}
}

func TestDouyinSignatureVerifier_Valid(t *testing.T) {
	appSecret := "test-app-secret"
	body := []byte(`{"event":"order_created"}`)
	expectedSig := douyinshop.ComputeDouyinWebhookSig(appSecret, body)

	h := http.Header{}
	h.Set(douyinshop.DouyinWebhookSignatureHeader, expectedSig)

	v := &douyinshop.DouyinSignatureVerifier{
		SecretProvider: &douyinshop.StaticSecretProvider{Secret: appSecret},
	}
	if err := v.Verify(context.Background(), body, h); err != nil {
		t.Errorf("expected valid signature, got error: %v", err)
	}
}

func TestDouyinSignatureVerifier_Invalid(t *testing.T) {
	appSecret := "test-app-secret"
	body := []byte(`{"event":"order_created"}`)

	h := http.Header{}
	h.Set(douyinshop.DouyinWebhookSignatureHeader, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")

	v := &douyinshop.DouyinSignatureVerifier{
		SecretProvider: &douyinshop.StaticSecretProvider{Secret: appSecret},
	}
	if err := v.Verify(context.Background(), body, h); err == nil {
		t.Error("expected signature mismatch error, got nil")
	}
}

func TestDouyinSignatureVerifier_MissingHeader(t *testing.T) {
	appSecret := "test-app-secret"
	body := []byte(`{"event":"order_created"}`)
	h := http.Header{}

	v := &douyinshop.DouyinSignatureVerifier{
		SecretProvider: &douyinshop.StaticSecretProvider{Secret: appSecret},
	}
	err := v.Verify(context.Background(), body, h)
	if err == nil {
		t.Error("expected missing signature error, got nil")
	}
	var de *douyinshop.Error
	if douyinshop.AsError(err, &de) && de.Code == douyinshop.CodeDouyinValidationFailed {
		return
	}
	t.Errorf("unexpected error type: %v", err)
}

func TestDouyinSignatureVerifier_EmptySecret(t *testing.T) {
	body := []byte(`{"event":"order_created"}`)
	h := http.Header{}
	h.Set(douyinshop.DouyinWebhookSignatureHeader, "anything")

	v := &douyinshop.DouyinSignatureVerifier{
		SecretProvider: &douyinshop.StaticSecretProvider{Secret: ""},
	}
	err := v.Verify(context.Background(), body, h)
	if err == nil {
		t.Error("expected not-configured error for empty secret, got nil")
	}
}

func TestWebhookSignatureSummary(t *testing.T) {
	s := douyinshop.WebhookSignatureSummary(true)
	if s == "" {
		t.Error("expected non-empty summary")
	}
	sBlocked := douyinshop.WebhookSignatureSummary(false)
	if sBlocked == "" {
		t.Error("expected non-empty blocked summary")
	}
}

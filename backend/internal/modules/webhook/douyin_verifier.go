package webhook

import (
	"context"
	"net/http"
	"strings"

	douyinshop "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

// DouyinVerifier adapts douyinshop.DouyinSignatureVerifier to SignatureVerifier.
type DouyinVerifier struct {
	inner  *douyinshop.DouyinSignatureVerifier
	gate   *douyinshop.DefaultContractGate
	appEnv string
}

// NewDouyinVerifier builds a SignatureVerifier for Douyin webhook pushes.
// If secret is empty, Verify returns CodeVerifierNotConfigured.
func NewDouyinVerifier(secret string) SignatureVerifier {
	return NewDouyinVerifierWithEnv(secret, "")
}

// NewDouyinVerifierWithEnv applies production contract gate for webhook signature v1.
func NewDouyinVerifierWithEnv(secret, appEnv string) SignatureVerifier {
	secretProvider := &douyinshop.StaticSecretProvider{Secret: strings.TrimSpace(secret)}
	return &DouyinVerifier{
		inner:  &douyinshop.DouyinSignatureVerifier{SecretProvider: secretProvider},
		gate:   douyinshop.NewDefaultContractGate(appEnv),
		appEnv: strings.TrimSpace(appEnv),
	}
}

// Verify implements SignatureVerifier using SHA1(appSecret + rawBody) contract v1.
func (v *DouyinVerifier) Verify(ctx context.Context, input VerifyInput) error {
	if v == nil || v.inner == nil {
		return newCodeError(CodeVerifierNotConfigured, http.StatusUnauthorized, CodeVerifierNotConfigured)
	}
	if v.gate != nil {
		if err := v.gate.Require(douyinshop.CapDouyinWebhookSignatureV1); err != nil {
			var de *douyinshop.Error
			if douyinshop.AsError(err, &de) {
				return newCodeError(CodeSignatureContractPending, http.StatusForbidden, de.Code)
			}
			return newCodeError(CodeSignatureContractPending, http.StatusForbidden, douyinshop.CodeDouyinContractVerificationRequired)
		}
	}
	err := v.inner.Verify(ctx, input.RawBody, input.Headers)
	if err != nil {
		var de *douyinshop.Error
		if douyinshop.AsError(err, &de) {
			switch de.Code {
			case douyinshop.CodeDouyinNotConfigured:
				return newCodeError(CodeVerifierNotConfigured, http.StatusUnauthorized, CodeVerifierNotConfigured)
			case douyinshop.CodeDouyinValidationFailed:
				if strings.Contains(de.Message, "missing") {
					return newCodeError(CodeSignatureMissing, http.StatusUnauthorized, CodeSignatureMissing)
				}
				return newCodeError(CodeSignatureInvalid, http.StatusUnauthorized, CodeSignatureInvalid)
			}
		}
		return newCodeError(CodeSignatureInvalid, http.StatusUnauthorized, CodeSignatureInvalid)
	}
	return nil
}

// WebhookSignatureCapabilityStatus exposes contract gate status for config status center.
func WebhookSignatureCapabilityStatus(appEnv string) douyinshop.ContractCapabilityStatus {
	return douyinshop.NewDefaultContractGate(appEnv).Status(douyinshop.CapDouyinWebhookSignatureV1)
}

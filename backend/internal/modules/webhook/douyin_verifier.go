package webhook

import (
	"context"
	"net/http"
	"strings"

	douyinshop "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

// DouyinVerifier adapts douyinshop.DouyinSignatureVerifier to SignatureVerifier.
type DouyinVerifier struct {
	inner *douyinshop.DouyinSignatureVerifier
}

// NewDouyinVerifier builds a SignatureVerifier for Douyin webhook pushes.
// If secret is empty, Verify returns CodeVerifierNotConfigured.
func NewDouyinVerifier(secret string) SignatureVerifier {
	secretProvider := &douyinshop.StaticSecretProvider{Secret: strings.TrimSpace(secret)}
	return &DouyinVerifier{
		inner: &douyinshop.DouyinSignatureVerifier{SecretProvider: secretProvider},
	}
}

// Verify implements SignatureVerifier using SHA1(appSecret + rawBody).
func (v *DouyinVerifier) Verify(ctx context.Context, input VerifyInput) error {
	if v == nil || v.inner == nil {
		return newCodeError(CodeVerifierNotConfigured, http.StatusUnauthorized, CodeVerifierNotConfigured)
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

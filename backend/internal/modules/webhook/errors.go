package webhook

import "fmt"

// Error codes for webhook HTTP ingest and verification.
const (
	CodeSignatureMissing         = "WEBHOOK_SIGNATURE_MISSING"
	CodeSignatureInvalid         = "WEBHOOK_SIGNATURE_INVALID"
	CodeVerifierNotConfigured    = "WEBHOOK_VERIFIER_NOT_CONFIGURED"
	CodeSignatureBypassForbidden = "WEBHOOK_SIGNATURE_BYPASS_FORBIDDEN"
	CodeTimestampExpired         = "WEBHOOK_TIMESTAMP_EXPIRED"
	CodeTimestampMissing         = "WEBHOOK_TIMESTAMP_MISSING"
	CodePayloadTooLarge          = "WEBHOOK_PAYLOAD_TOO_LARGE"
	CodeInvalidContentType       = "WEBHOOK_INVALID_CONTENT_TYPE"
	CodeInvalidJSON              = "WEBHOOK_INVALID_JSON"
	CodeInProgress               = "WEBHOOK_IN_PROGRESS"
	CodeKeyConflict              = "WEBHOOK_KEY_CONFLICT"
	CodeStoreFailed              = "WEBHOOK_STORE_FAILED"
)

// CodeError is a webhook domain error with a stable code for HTTP mapping.
type CodeError struct {
	Code       string
	HTTPStatus int
	Message    string
}

func (e *CodeError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func newCodeError(code string, httpStatus int, msg string) *CodeError {
	if msg == "" {
		msg = code
	}
	return &CodeError{Code: code, HTTPStatus: httpStatus, Message: msg}
}

func AsCodeError(err error) (*CodeError, bool) {
	if err == nil {
		return nil, false
	}
	if ce, ok := err.(*CodeError); ok {
		return ce, true
	}
	return nil, false
}

func wrapCode(code string, httpStatus int, cause error) error {
	msg := code
	if cause != nil {
		msg = fmt.Sprintf("%s: %v", code, cause)
	}
	return newCodeError(code, httpStatus, msg)
}

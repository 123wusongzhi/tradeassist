package customerchat

import "errors"

const (
	ErrCodeCustomerMessageUnknownResult = "CUSTOMER_MESSAGE_UNKNOWN_RESULT"
	ErrCodeCustomerMessageInProgress    = "IDEMPOTENCY_IN_PROGRESS"
)

// PlatformSendError carries customer platform send failure semantics for API mapping.
type PlatformSendError struct {
	Code                 string
	Message              string
	ManualReviewRequired bool
	SafeRetry            bool
}

func (e *PlatformSendError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

func asPlatformSendError(err error) (*PlatformSendError, bool) {
	var pe *PlatformSendError
	if errors.As(err, &pe) && pe != nil {
		return pe, true
	}
	return nil, false
}

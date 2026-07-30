package douyinshop

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/safefields"
)

const (
	CodeDouyinAPIError                      = "DOUYIN_API_ERROR"
	CodeDouyinAuthExpired                   = "DOUYIN_AUTH_EXPIRED"
	CodeDouyinTokenRefreshFailed            = "DOUYIN_TOKEN_REFRESH_FAILED"
	CodeDouyinPermissionDenied              = "DOUYIN_PERMISSION_DENIED"
	CodeDouyinRateLimited                   = "DOUYIN_RATE_LIMITED"
	CodeDouyinRequestTimeout                = "DOUYIN_REQUEST_TIMEOUT"
	CodeDouyinResponseParseFailed           = "DOUYIN_RESPONSE_PARSE_FAILED"
	CodeDouyinShopInfoFailed                = "DOUYIN_SHOP_INFO_FAILED"
	CodeDouyinStoreNotAuthorized            = "DOUYIN_STORE_NOT_AUTHORIZED"
	CodeDouyinCategoryMissing               = "DOUYIN_CATEGORY_MISSING"
	CodeDouyinRequiredAttrMissing           = "DOUYIN_REQUIRED_ATTR_MISSING"
	CodeDouyinMainImageNotUploaded          = "DOUYIN_MAIN_IMAGE_NOT_UPLOADED"
	CodeDouyinCreateProductFailed           = "DOUYIN_CREATE_PRODUCT_FAILED"
	CodeDouyinProductPayloadInvalid         = "DOUYIN_PRODUCT_PAYLOAD_INVALID"
	CodeDouyinProductDetailFailed           = "DOUYIN_PRODUCT_DETAIL_FAILED"
	CodeDouyinProductNotFound               = "DOUYIN_PRODUCT_NOT_FOUND"
	CodeDouyinProductDetailPermissionDenied = "DOUYIN_PRODUCT_DETAIL_PERMISSION_DENIED"
	CodeDouyinSKUBindingSyncFailed          = "DOUYIN_SKU_BINDING_SYNC_FAILED"
	CodeDouyinSKUBindingUnmatched           = "DOUYIN_SKU_BINDING_UNMATCHED"
	CodeDouyinSKUBindingAmbiguous           = "DOUYIN_SKU_BINDING_AMBIGUOUS"
	CodeDouyinSKUManualBindFailed           = "DOUYIN_SKU_MANUAL_BIND_FAILED"
	CodeDouyinSKUManualUnbindFailed         = "DOUYIN_SKU_MANUAL_UNBIND_FAILED"
	CodeDouyinPlatformSKUIDMissing          = "DOUYIN_PLATFORM_SKU_ID_MISSING"
	CodeDouyinSKUBindingConflict            = "DOUYIN_SKU_BINDING_CONFLICT"
	CodeDouyinSKUBindingRequired            = "DOUYIN_SKU_BINDING_REQUIRED"
	CodeDouyinOrderSyncFailed               = "DOUYIN_ORDER_SYNC_FAILED"
	CodeDouyinOrderListFailed               = "DOUYIN_ORDER_LIST_FAILED"
	CodeDouyinOrderDetailFailed             = "DOUYIN_ORDER_DETAIL_FAILED"
	CodeDouyinOrderParseFailed              = "DOUYIN_ORDER_PARSE_FAILED"
	CodeDouyinOrderPermissionDenied         = "DOUYIN_ORDER_PERMISSION_DENIED"
	CodeDouyinOrderRateLimited              = "DOUYIN_ORDER_RATE_LIMITED"
	CodeDouyinOrderAmountInvalid            = "DOUYIN_ORDER_AMOUNT_INVALID"
	CodeUnknownDouyinOrderError             = "UNKNOWN_DOUYIN_ORDER_ERROR"
	CodeDouyinProductNotBound               = "DOUYIN_PRODUCT_NOT_BOUND"
	CodeDouyinSKUNotBound                   = "DOUYIN_SKU_NOT_BOUND"
	CodeDouyinStockInvalid                  = "DOUYIN_STOCK_INVALID"
	CodeDouyinInventorySyncNotReady         = "DOUYIN_INVENTORY_SYNC_NOT_READY"
	CodeDouyinInventorySyncFailed           = "DOUYIN_INVENTORY_SYNC_FAILED"
	CodeDouyinInventoryPermissionDenied     = "DOUYIN_INVENTORY_PERMISSION_DENIED"
	CodeDouyinInventoryRateLimited          = "DOUYIN_INVENTORY_RATE_LIMITED"
	CodeDouyinInventoryRecentlySynced       = "DOUYIN_INVENTORY_RECENTLY_SYNCED"
	CodeUnknownDouyinInventoryError         = "UNKNOWN_DOUYIN_INVENTORY_ERROR"
	CodeUnknownDouyinError                  = "UNKNOWN_DOUYIN_ERROR"

	// P3 additions
	CodeDouyinUnknownResult                = "DOUYIN_UNKNOWN_RESULT"
	CodeDouyinNotConfigured                = "DOUYIN_NOT_CONFIGURED"
	CodeDouyinReauthorizationRequired      = "DOUYIN_REAUTHORIZATION_REQUIRED"
	CodeDouyinContractMismatch             = "DOUYIN_CONTRACT_MISMATCH"
	CodeDouyinContractVerificationRequired = "DOUYIN_CONTRACT_VERIFICATION_REQUIRED"
	CodeDouyinContractVersionUnsupported   = "DOUYIN_CONTRACT_VERSION_UNSUPPORTED"
	CodeDouyinMethodNotConfirmed           = "DOUYIN_METHOD_NOT_CONFIRMED"
	CodeDouyinScopeNotConfirmed            = "DOUYIN_SCOPE_NOT_CONFIRMED"
	CodeDouyinResponseSchemaMismatch       = "DOUYIN_RESPONSE_SCHEMA_MISMATCH"
	CodeDouyinManualConfirmationRequired   = "DOUYIN_MANUAL_CONFIRMATION_REQUIRED"
	CodeDouyinValidationFailed             = "DOUYIN_VALIDATION_FAILED"
	CodeDouyinTimeout                      = "DOUYIN_TIMEOUT"
	CodeDouyinResourceNotFound             = "DOUYIN_RESOURCE_NOT_FOUND"
	CodeDouyinTokenVersionConflict         = "DOUYIN_TOKEN_VERSION_CONFLICT"
	CodeDouyinTokenRefreshInProgress       = "DOUYIN_TOKEN_REFRESH_IN_PROGRESS"
	CodeDouyinOAuthStateMissing            = "DOUYIN_OAUTH_STATE_MISSING"
	CodeDouyinOAuthStateExpired            = "DOUYIN_OAUTH_STATE_EXPIRED"
	CodeDouyinOAuthStateAlreadyUsed        = "DOUYIN_OAUTH_STATE_ALREADY_USED"
	CodeDouyinOAuthRedirectNotAllowed      = "DOUYIN_OAUTH_REDIRECT_NOT_ALLOWED"

	// Error class constants for ErrorClass field.
	ErrorClassAuthError        = "auth_error"
	ErrorClassRateLimited      = "rate_limited"
	ErrorClassTimeout          = "timeout"
	ErrorClassUnknownResult    = "unknown_result"
	ErrorClassContractMismatch = "contract_mismatch"
	ErrorClassValidation       = "validation"
	ErrorClassPermission       = "permission"
	ErrorClassNotFound         = "not_found"
	ErrorClassNetwork          = "network"
	ErrorClassSystem           = "system"
)

type Error struct {
	Code             string
	Message          string
	PlatformCode     string
	PlatformMessage  string
	RequestID        string
	Retryable        bool
	RateLimited      bool
	PermissionDenied bool
	AuthExpired      bool
	// P3 fields
	SafeRetry            bool   // true = re-send is safe (idempotent read-side or idempotent write with confirmed dedup)
	ManualReviewRequired bool   // operator must check platform before retry
	UnknownResult        bool   // write sent, outcome unknown (timeout/network after write)
	ErrorClass           string // auth_error, rate_limited, timeout, unknown_result, etc.
	RetryAfter           int64  // seconds to wait before retry (from Retry-After header)
}

func NewError(code, msg, platformCode, platformMsg, requestID string) *Error {
	e := &Error{
		Code:            strings.TrimSpace(code),
		Message:         strings.TrimSpace(msg),
		PlatformCode:    strings.TrimSpace(platformCode),
		PlatformMessage: SanitizeErrorText(platformMsg),
		RequestID:       strings.TrimSpace(requestID),
	}
	if e.Code == "" {
		e.Code = CodeUnknownDouyinError
	}
	if e.Message == "" {
		e.Message = e.Code
	}
	switch e.Code {
	case CodeDouyinAuthExpired:
		e.AuthExpired = true
	case CodeDouyinPermissionDenied:
		e.PermissionDenied = true
	case CodeDouyinRateLimited:
		e.RateLimited = true
		e.Retryable = true
	case CodeDouyinRequestTimeout, CodeDouyinCreateProductFailed, CodeDouyinProductDetailFailed,
		CodeDouyinOrderSyncFailed, CodeDouyinOrderListFailed, CodeDouyinOrderDetailFailed,
		CodeDouyinInventorySyncFailed, CodeDouyinSKUBindingSyncFailed:
		e.Retryable = true
	case CodeDouyinOrderRateLimited, CodeDouyinInventoryRateLimited:
		e.RateLimited = true
		e.Retryable = true
	case CodeDouyinOrderPermissionDenied, CodeDouyinInventoryPermissionDenied:
		e.PermissionDenied = true
	case CodeDouyinProductPayloadInvalid, CodeDouyinCategoryMissing, CodeDouyinRequiredAttrMissing, CodeDouyinMainImageNotUploaded,
		CodeDouyinProductNotBound, CodeDouyinSKUNotBound, CodeDouyinStockInvalid, CodeDouyinInventorySyncNotReady,
		CodeDouyinProductNotFound, CodeDouyinProductDetailPermissionDenied,
		CodeDouyinSKUBindingUnmatched, CodeDouyinSKUBindingAmbiguous,
		CodeDouyinSKUManualBindFailed, CodeDouyinSKUManualUnbindFailed, CodeDouyinPlatformSKUIDMissing,
		CodeDouyinSKUBindingConflict, CodeDouyinSKUBindingRequired,
		CodeDouyinGrayReleaseNotEnabled, CodeDouyinShopNotInGrayList, CodeDouyinWriteOperationDisabled:
		e.Retryable = false
	// P3 codes
	case CodeDouyinUnknownResult:
		e.UnknownResult = true
		e.SafeRetry = false
		e.ManualReviewRequired = true
		e.ErrorClass = ErrorClassUnknownResult
	case CodeDouyinNotConfigured:
		e.Retryable = false
		e.ErrorClass = ErrorClassSystem
	case CodeDouyinReauthorizationRequired:
		e.AuthExpired = true
		e.ErrorClass = ErrorClassAuthError
	case CodeDouyinContractMismatch, CodeDouyinContractVerificationRequired,
		CodeDouyinContractVersionUnsupported, CodeDouyinMethodNotConfirmed,
		CodeDouyinScopeNotConfirmed, CodeDouyinResponseSchemaMismatch:
		e.Retryable = false
		e.SafeRetry = false
		e.ManualReviewRequired = true
		e.ErrorClass = ErrorClassContractMismatch
	case CodeDouyinManualConfirmationRequired:
		e.SafeRetry = false
		e.ManualReviewRequired = true
		e.ErrorClass = ErrorClassUnknownResult
	case CodeDouyinValidationFailed:
		e.Retryable = false
		e.ErrorClass = ErrorClassValidation
	case CodeDouyinTimeout:
		e.Retryable = true
		e.ErrorClass = ErrorClassTimeout
	case CodeDouyinResourceNotFound:
		e.Retryable = false
		e.ErrorClass = ErrorClassNotFound
	case CodeDouyinTokenVersionConflict:
		e.Retryable = false
		e.ErrorClass = ErrorClassAuthError
	case CodeDouyinTokenRefreshInProgress:
		e.Retryable = true
		e.ErrorClass = ErrorClassAuthError
	}
	// Set ErrorClass from existing flags if not already set by P3 switch
	if e.ErrorClass == "" {
		switch {
		case e.AuthExpired:
			e.ErrorClass = ErrorClassAuthError
		case e.RateLimited:
			e.ErrorClass = ErrorClassRateLimited
		case e.PermissionDenied:
			e.ErrorClass = ErrorClassPermission
		case e.UnknownResult:
			e.ErrorClass = ErrorClassUnknownResult
		}
	}
	return e
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{e.Code}
	if e.PlatformCode != "" {
		parts = append(parts, "platformCode="+e.PlatformCode)
	}
	if e.RequestID != "" {
		parts = append(parts, "requestId="+e.RequestID)
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	return strings.Join(parts, " ")
}

func AsError(err error, target **Error) bool {
	return errors.As(err, target)
}

func SanitizeErrorText(raw string) string {
	return safefields.RedactString(raw)
}

func sanitizeRawMap(in map[string]any) map[string]any {
	return safefields.RedactMap(in)
}

func MapHTTPError(status int, requestID string) *Error {
	switch status {
	case http.StatusUnauthorized:
		return NewError(CodeDouyinAuthExpired, "douyin authorization expired", fmt.Sprint(status), "unauthorized", requestID)
	case http.StatusForbidden:
		return NewError(CodeDouyinPermissionDenied, "douyin permission denied", fmt.Sprint(status), "forbidden", requestID)
	case http.StatusTooManyRequests:
		return NewError(CodeDouyinRateLimited, "douyin openapi rate limited", fmt.Sprint(status), "rate limited", requestID)
	case http.StatusGatewayTimeout, http.StatusRequestTimeout:
		return NewError(CodeDouyinRequestTimeout, "douyin openapi request timeout", fmt.Sprint(status), "timeout", requestID)
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		e := NewError(CodeDouyinAPIError, "douyin openapi temporary error", fmt.Sprint(status), "", requestID)
		e.Retryable = true
		return e
	default:
		return NewError(CodeDouyinAPIError, "douyin openapi http error", fmt.Sprint(status), "", requestID)
	}
}

func MapPlatformError(platformCode, platformMsg, requestID string) *Error {
	pc := strings.TrimSpace(platformCode)
	pm := SanitizeErrorText(platformMsg)
	low := strings.ToLower(pc + " " + strings.TrimSpace(platformMsg))
	switch {
	case strings.Contains(low, "rate") || strings.Contains(low, "limit") || strings.Contains(low, "frequency"):
		return NewError(CodeDouyinRateLimited, "douyin openapi rate limited", pc, pm, requestID)
	case strings.Contains(low, "permission") || strings.Contains(low, "forbid") || strings.Contains(low, "unauthoriz"):
		return NewError(CodeDouyinPermissionDenied, "douyin permission denied", pc, pm, requestID)
	case strings.Contains(low, "refresh") && (strings.Contains(low, "expire") || strings.Contains(low, "invalid") || strings.Contains(low, "fail")):
		return NewError(CodeDouyinAuthExpired, "douyin authorization expired", pc, pm, requestID)
	case strings.Contains(low, "access_token") || strings.Contains(low, "token expired") || strings.Contains(low, "invalid token"):
		return NewError(CodeDouyinAuthExpired, "douyin authorization expired", pc, pm, requestID)
	default:
		return NewError(CodeDouyinAPIError, "douyin openapi error", pc, pm, requestID)
	}
}

// ClassifyError returns an ErrorClass string for the given error (may be non-douyin).
func ClassifyError(err error) string {
	if err == nil {
		return ""
	}
	var de *Error
	if errors.As(err, &de) {
		if de.ErrorClass != "" {
			return de.ErrorClass
		}
		switch {
		case de.AuthExpired:
			return ErrorClassAuthError
		case de.RateLimited:
			return ErrorClassRateLimited
		case de.PermissionDenied:
			return ErrorClassPermission
		case de.UnknownResult:
			return ErrorClassUnknownResult
		}
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return ErrorClassTimeout
	case strings.Contains(msg, "rate") || strings.Contains(msg, "limit"):
		return ErrorClassRateLimited
	case strings.Contains(msg, "auth") || strings.Contains(msg, "token") || strings.Contains(msg, "unauthorized"):
		return ErrorClassAuthError
	default:
		return ErrorClassSystem
	}
}

func platformCodeOf(err error) string {
	var de *Error
	if errors.As(err, &de) {
		return de.PlatformCode
	}
	return ""
}

func requestIDOf(err error) string {
	var de *Error
	if errors.As(err, &de) {
		return de.RequestID
	}
	return ""
}

func safeMessageOf(err error) string {
	var de *Error
	if errors.As(err, &de) {
		if de.PlatformMessage != "" {
			return de.PlatformMessage
		}
		return de.Message
	}
	if err == nil {
		return ""
	}
	return SanitizeErrorText(err.Error())
}

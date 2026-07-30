package inventorysyncp9

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

var (
	ErrAuthenticationRequired = errors.New("authentication_required")
	ErrRetryNotAllowed        = errors.New("retry_not_allowed")
)

type apiErrorSpec struct {
	status int
	code   string
	msg    string
	biz    int
}

func apiError(err error) apiErrorSpec {
	spec := apiErrorSpec{status: http.StatusInternalServerError, code: "internal_error", msg: "internal error", biz: response.CodeInternalError}
	var idemErr *idempotency.OpError
	switch {
	case err == nil:
		return apiErrorSpec{status: http.StatusOK, code: "ok", msg: "ok", biz: response.CodeOK}
	case errors.Is(err, ErrAuthenticationRequired):
		return apiErrorSpec{status: http.StatusUnauthorized, code: "authentication_required", msg: "authentication required", biz: response.CodeUnauthorized}
	case errors.Is(err, ErrValidation), pagination.ErrorCode(err) != "":
		return apiErrorSpec{status: http.StatusBadRequest, code: ErrCodeValidation, msg: "validation error", biz: response.CodeBadRequest}
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrTenantMismatch):
		return apiErrorSpec{status: http.StatusNotFound, code: ErrCodeNotFound, msg: "not found", biz: response.CodeNotFound}
	case errors.Is(err, ErrPermissionDenied):
		return apiErrorSpec{status: http.StatusForbidden, code: ErrCodePermissionDenied, msg: "permission denied", biz: response.CodePermissionDenied}
	case errors.Is(err, ErrProductionCapabilityForbidden), errors.Is(err, ErrProviderCapabilityForbidden):
		return apiErrorSpec{status: http.StatusForbidden, code: ErrCodeProductionCapabilityForbidden, msg: "provider mode is not allowed", biz: response.CodePermissionDenied}
	case errors.Is(err, ErrRevisionConflict):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeRevisionConflict, msg: "revision conflict", biz: response.CodeBadRequest}
	case errors.Is(err, ErrRetryNotAllowed):
		return apiErrorSpec{status: http.StatusConflict, code: "retry_not_allowed", msg: "run is not retryable", biz: response.CodeBadRequest}
	case errors.Is(err, ErrIdempotencyPayloadConflict), errors.As(err, &idemErr) && idemErr.Code == idempotency.ErrCodeKeyConflict:
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeIdempotencyPayloadConflict, msg: "idempotency payload conflict", biz: response.CodeBadRequest}
	case errors.As(err, &idemErr) && idemErr.Code == idempotency.ErrCodeInProgress:
		return apiErrorSpec{status: http.StatusConflict, code: "idempotency_in_progress", msg: "request is already in progress", biz: response.CodeBadRequest}
	case errors.Is(err, ErrStateConflict), errors.Is(err, ErrSyncRunAlreadyRunning), errors.Is(err, ErrManualBindingAlreadyResolved), errors.Is(err, ErrBindingConflict), errors.Is(err, ErrManualBindingAlreadyPending):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeStateConflict, msg: "state conflict", biz: response.CodeBadRequest}
	case errors.Is(err, ErrProviderNotRegistered), errors.Is(err, ErrProviderCursorInvalid), errors.Is(err, ErrProviderCursorLoop), errors.Is(err, ErrProviderPageInvalid), errors.Is(err, ErrProviderPageLimitExceeded):
		return apiErrorSpec{status: http.StatusBadRequest, code: ErrCodeValidation, msg: "validation error", biz: response.CodeBadRequest}
	}
	return spec
}

func apiRespondError(c *gin.Context, err error) {
	spec := apiError(err)
	response.JSON(c, spec.status, spec.biz, spec.msg, gin.H{"errorCode": spec.code})
}

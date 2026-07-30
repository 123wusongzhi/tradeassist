package operationtask

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

type apiErrorSpec struct {
	status int
	code   string
	msg    string
	biz    int
}

func apiError(err error) apiErrorSpec {
	spec := apiErrorSpec{status: http.StatusInternalServerError, code: "internal_error", msg: "internal error", biz: response.CodeInternalError}
	switch {
	case err == nil:
		return apiErrorSpec{status: http.StatusOK, code: "ok", msg: "ok", biz: response.CodeOK}
	case errors.Is(err, ErrValidation):
		return apiErrorSpec{status: http.StatusBadRequest, code: ErrCodeValidation, msg: "validation error", biz: response.CodeBadRequest}
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrTenantMismatch):
		return apiErrorSpec{status: http.StatusNotFound, code: ErrCodeNotFound, msg: "not found", biz: response.CodeNotFound}
	case errors.Is(err, ErrPermissionDenied), errors.Is(err, ErrExecutionModeForbidden):
		return apiErrorSpec{status: http.StatusForbidden, code: ErrCodePermissionDenied, msg: "permission denied", biz: response.CodePermissionDenied}
	case errors.Is(err, ErrRevisionConflict):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeRevisionConflict, msg: "revision conflict", biz: response.CodeBadRequest}
	case errors.Is(err, ErrInvalidTransition):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeInvalidTransition, msg: "invalid transition", biz: response.CodeBadRequest}
	case errors.Is(err, ErrStateConflict):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeStateConflict, msg: "state conflict", biz: response.CodeBadRequest}
	case errors.Is(err, ErrDraftNotLatest):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeDraftNotLatest, msg: "draft is not latest", biz: response.CodeBadRequest}
	case errors.Is(err, ErrDraftVersionMismatch):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeDraftVersionMismatch, msg: "draft version mismatch", biz: response.CodeBadRequest}
	case errors.Is(err, ErrDraftHashMismatch):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeDraftHashMismatch, msg: "draft hash mismatch", biz: response.CodeBadRequest}
	case errors.Is(err, ErrApprovalIdemConflict):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeApprovalIdemConflict, msg: "idempotency payload conflict", biz: response.CodeBadRequest}
	case errors.Is(err, ErrDuplicateRequest), errors.Is(err, ErrDuplicateIdempotencyKey), errors.Is(err, ErrDuplicateDraftVersion), errors.Is(err, ErrDuplicateApprovalIdem), errors.Is(err, ErrDuplicateExecutionIdem):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeDuplicateRequest, msg: "duplicate request", biz: response.CodeBadRequest}
	case errors.Is(err, ErrReferenceMismatch), errors.Is(err, ErrDraftBindingConflict):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeDraftBindingConflict, msg: "draft binding conflict", biz: response.CodeBadRequest}
	case errors.Is(err, ErrExecutionInProgress):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeExecutionInProgress, msg: "execution already in progress", biz: response.CodeBadRequest}
	case errors.Is(err, ErrIdemPayloadConflict):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeIdemPayloadConflict, msg: "idempotency payload conflict", biz: response.CodeBadRequest}
	case errors.Is(err, ErrRetryLimitExceeded):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeRetryLimitExceeded, msg: "retry limit exceeded", biz: response.CodeBadRequest}
	case errors.Is(err, ErrFinalizeConflict), errors.Is(err, ErrResultUnknown):
		return apiErrorSpec{status: http.StatusConflict, code: ErrCodeFinalizeConflict, msg: "state conflict", biz: response.CodeBadRequest}
	}
	return spec
}

func apiRespondError(c *gin.Context, err error) {
	spec := apiError(err)
	response.JSON(c, spec.status, spec.biz, spec.msg, gin.H{"errorCode": spec.code})
}

package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

// clientSafeError is implemented by domain errors that may be returned to an
// API caller without exposing internal state, SQL details, or provider secrets.
type clientSafeError interface {
	error
	HTTPStatus() int
	SafeMessage() string
}

type clientSafeDataError interface {
	SafeData() any
}

// OK writes a success envelope with HTTP 200.
func OK(c *gin.Context, data any) {
	JSON(c, http.StatusOK, CodeOK, "ok", data)
}

// Fail writes an error envelope; pick HTTP status and business code to match rules.
func Fail(c *gin.Context, httpStatus, bizCode int, msg string) {
	if msg == "" {
		msg = "error"
	}
	JSON(c, httpStatus, bizCode, msg, nil)
}

// JSON writes the unified API body; use for custom cases.
func JSON(c *gin.Context, httpStatus, bizCode int, msg string, data any) {
	tid, _ := c.Get(ctxkey.TraceID)
	trace, _ := tid.(string)
	c.JSON(httpStatus, Envelope{
		Code:    bizCode,
		Message: msg,
		Data:    data,
		TraceID: trace,
	})
}

// HandleError maps errors to HTTP + business codes while keeping internal and
// provider details out of the response envelope.
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		Fail(c, http.StatusNotFound, CodeNotFound, "资源不存在")
		return
	}
	var safeErr clientSafeError
	if errors.As(err, &safeErr) {
		status := safeErr.HTTPStatus()
		if status < 400 || status > 599 {
			status = http.StatusInternalServerError
		}
		message := safeErr.SafeMessage()
		if message == "" {
			message = http.StatusText(status)
		}
		var data any
		var dataErr clientSafeDataError
		if errors.As(err, &dataErr) {
			data = dataErr.SafeData()
		}
		JSON(c, status, businessCodeForHTTPStatus(status), message, data)
		return
	}
	Fail(c, http.StatusInternalServerError, CodeInternalError, "internal error")
}

func businessCodeForHTTPStatus(status int) int {
	switch status {
	case http.StatusBadRequest, http.StatusConflict, http.StatusUnprocessableEntity:
		return CodeBadRequest
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusForbidden:
		return CodePermissionDenied
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusBadGateway:
		return CodeBadGateway
	case http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return CodeServiceUnavailable
	default:
		return CodeInternalError
	}
}

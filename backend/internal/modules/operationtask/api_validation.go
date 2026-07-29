package operationtask

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httpapi"
	"gorm.io/datatypes"
)

const (
	apiMaxJSONBodyBytes = int64(1 << 20)
	apiMaxPayloadBytes  = 256 << 10
	apiMaxTitleLength   = 200
	apiMaxSummaryLength = 1000
	apiMaxReasonLength  = 1000
	apiMaxCommentLength = 2000
	apiMaxCursorLength  = 2048
	apiMaxIdemLength    = 128
	apiMaxLimit         = 100
)

var apiIdempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)

func apiBindJSON(c *gin.Context, dst any) error {
	if err := httpapi.BindStrictJSON(c, dst, apiMaxJSONBodyBytes); err != nil {
		return ErrValidation
	}
	return nil
}

func apiIdempotencyKey(c *gin.Context) (string, error) {
	key := ""
	if c != nil {
		key = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	}
	if key == "" || len(key) > apiMaxIdemLength || !apiIdempotencyKeyPattern.MatchString(key) {
		return "", ErrValidation
	}
	return key, nil
}

func apiRequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if v, ok := c.Get(ctxkey.TraceID); ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return strings.TrimSpace(c.GetHeader("X-Request-ID"))
}

func apiActorID(c *gin.Context) (uuid.UUID, error) {
	if c == nil {
		return uuid.Nil, ErrPermissionDenied
	}
	v, ok := c.Get(ctxkey.AdminID)
	if !ok {
		return uuid.Nil, ErrPermissionDenied
	}
	s, ok := v.(string)
	if !ok {
		return uuid.Nil, ErrPermissionDenied
	}
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.Nil, ErrPermissionDenied
	}
	return id, nil
}

func apiUUIDParam(c *gin.Context, name string) (uuid.UUID, error) {
	if c == nil {
		return uuid.Nil, ErrValidation
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param(name)))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, ErrValidation
	}
	return id, nil
}

func apiJSONPayload(raw json.RawMessage) (datatypes.JSON, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || len(raw) > apiMaxPayloadBytes || !json.Valid(raw) || trimmed == "null" {
		return nil, ErrValidation
	}
	payload := datatypes.JSON([]byte(trimmed))
	if payloadHasSecret(payload) {
		return nil, ErrValidation
	}
	return payload, nil
}

func apiValidateCreateTaskRequest(req CreateTaskRequest) (datatypes.JSON, error) {
	payload, err := apiJSONPayload(req.Payload)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(req.Title)) == 0 || len(strings.TrimSpace(req.Title)) > apiMaxTitleLength {
		return nil, ErrValidation
	}
	if len(strings.TrimSpace(req.Summary)) > apiMaxSummaryLength {
		return nil, ErrValidation
	}
	if !allowedOperationTaskSources[strings.ToLower(strings.TrimSpace(req.SourceType))] ||
		!allowedOperationTaskTypes[strings.ToLower(strings.TrimSpace(req.TaskType))] ||
		!allowedPlatforms[strings.ToLower(strings.TrimSpace(req.Platform))] {
		return nil, ErrValidation
	}
	priority := strings.ToLower(strings.TrimSpace(req.Priority))
	if priority != "" && !allowedPriorities[priority] {
		return nil, ErrValidation
	}
	return payload, nil
}

func apiValidateReason(reason string, required bool) error {
	reason = strings.TrimSpace(reason)
	if required && reason == "" {
		return ErrValidation
	}
	if len(reason) > apiMaxReasonLength {
		return ErrValidation
	}
	return nil
}

func apiValidateComment(comment string) error {
	if len(strings.TrimSpace(comment)) > apiMaxCommentLength {
		return ErrValidation
	}
	return nil
}

func apiValidateLimitCursor(limit int, cursor string) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > apiMaxLimit {
		limit = apiMaxLimit
	}
	if len(strings.TrimSpace(cursor)) > apiMaxCursorLength {
		return 0, ErrValidation
	}
	return limit, nil
}

func decodeSafeJSON(raw datatypes.JSON) any {
	redacted := redactSafeJSON(raw)
	var out any
	if len(redacted) == 0 || json.Unmarshal(redacted, &out) != nil {
		return map[string]any{}
	}
	return out
}

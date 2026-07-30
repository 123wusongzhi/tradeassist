package inventorysyncp9

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httpapi"
)

const (
	apiMaxJSONBodyBytes = int64(1 << 20)
	apiMaxLimit         = 100
	apiMaxCursorLength  = 512
	apiMaxCommentLength = 2000
	apiMaxReasonLength  = 256
)

var apiIdempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)

var allowedManualStatuses = map[string]bool{
	ManualBindingStatusPending: true, ManualBindingStatusConfirmed: true,
	ManualBindingStatusRejected: true, ManualBindingStatusCancelled: true,
}

var forbiddenProductionProviderModes = map[string]bool{
	"production": true, "prod": true, "real": true, "live": true, "online": true, "remote": true, "oauth": true,
}

func apiBindJSON(c *gin.Context, dst any) error {
	if err := httpapi.BindStrictJSON(c, dst, apiMaxJSONBodyBytes); err != nil {
		return ErrValidation
	}
	return nil
}

func apiIdempotencyKeyHash(c *gin.Context) (string, error) {
	key := ""
	if c != nil {
		key = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	}
	if !apiIdempotencyKeyPattern.MatchString(key) {
		return "", ErrValidation
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:]), nil
}

func apiRequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	value, ok := c.Get(ctxkey.TraceID)
	if !ok {
		return ""
	}
	requestID, _ := value.(string)
	return strings.TrimSpace(requestID)
}

func apiActorID(c *gin.Context) (uuid.UUID, error) {
	if c == nil {
		return uuid.Nil, ErrAuthenticationRequired
	}
	value, ok := c.Get(ctxkey.AdminID)
	if !ok {
		return uuid.Nil, ErrAuthenticationRequired
	}
	raw, ok := value.(string)
	if !ok {
		return uuid.Nil, ErrAuthenticationRequired
	}
	actorID, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil || actorID == uuid.Nil {
		return uuid.Nil, ErrAuthenticationRequired
	}
	return actorID, nil
}

func apiUUIDParam(c *gin.Context, name string) (uuid.UUID, error) {
	if c == nil {
		return uuid.Nil, ErrValidation
	}
	value, err := uuid.Parse(strings.TrimSpace(c.Param(name)))
	if err != nil || value == uuid.Nil {
		return uuid.Nil, ErrValidation
	}
	return value, nil
}

func apiOptionalUUIDQuery(c *gin.Context, name string) (*uuid.UUID, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	value, err := uuid.Parse(raw)
	if err != nil || value == uuid.Nil {
		return nil, ErrValidation
	}
	return &value, nil
}

func apiLimitCursor(c *gin.Context) (int, string, error) {
	if c == nil {
		return 0, "", ErrValidation
	}
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return 0, "", ErrValidation
		}
		limit = parsed
	}
	if limit > apiMaxLimit {
		limit = apiMaxLimit
	}
	cursor := strings.TrimSpace(c.Query("cursor"))
	if len(cursor) > apiMaxCursorLength {
		return 0, "", ErrValidation
	}
	return limit, cursor, nil
}

func apiValidateReason(reason string, required bool) error {
	reason = strings.TrimSpace(reason)
	if (required && reason == "") || len(reason) > apiMaxReasonLength {
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

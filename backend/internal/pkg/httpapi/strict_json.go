package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const DefaultMaxJSONBodyBytes = int64(1 << 20)

var ErrInvalidJSONRequest = errors.New("invalid_json_request")

// BindStrictJSON applies the shared API JSON contract: application/json only,
// a bounded body, unknown-field rejection, and exactly one JSON value.
func BindStrictJSON(c *gin.Context, dst any, maxBytes int64) error {
	if c == nil || dst == nil || c.Request == nil || c.Request.Body == nil {
		return ErrInvalidJSONRequest
	}
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(c.GetHeader("Content-Type")))
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		return ErrInvalidJSONRequest
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxJSONBodyBytes
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return ErrInvalidJSONRequest
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrInvalidJSONRequest
	}
	return nil
}

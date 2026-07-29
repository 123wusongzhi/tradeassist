package httpapi

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBindStrictJSONContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type payload struct {
		Name string `json:"name"`
	}
	tests := []struct {
		name        string
		contentType string
		body        string
		wantErr     bool
	}{
		{name: "valid", contentType: "application/json; charset=utf-8", body: `{"name":"safe"}`},
		{name: "missing content type", body: `{"name":"safe"}`, wantErr: true},
		{name: "unknown field", contentType: "application/json", body: `{"name":"safe","tenantId":1}`, wantErr: true},
		{name: "multiple values", contentType: "application/json", body: `{"name":"safe"} {}`, wantErr: true},
		{name: "oversized", contentType: "application/json", body: `{"name":"too-long"}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.body))
			if tt.contentType != "" {
				ctx.Request.Header.Set("Content-Type", tt.contentType)
			}
			var got payload
			maxBytes := int64(1024)
			if tt.name == "oversized" {
				maxBytes = 8
			}
			err := BindStrictJSON(ctx, &got, maxBytes)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidJSONRequest)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "safe", got.Name)
		})
	}
}

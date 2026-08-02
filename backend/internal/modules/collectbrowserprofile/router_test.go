package collectbrowserprofile

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

func TestRoutesFailClosedWithoutDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name    string
		handler *Handler
	}{
		{name: "nil service", handler: &Handler{}},
		{name: "nil database", handler: &Handler{Svc: &Service{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			Register(router.Group("/api/v1"), tc.handler)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/collect/browser-profiles", nil))

			require.Equal(t, http.StatusInternalServerError, rec.Code)
			var envelope response.Envelope
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
			require.Equal(t, response.CodeInternalError, envelope.Code)
			require.Equal(t, "browser profiles unavailable", envelope.Message)
		})
	}
}

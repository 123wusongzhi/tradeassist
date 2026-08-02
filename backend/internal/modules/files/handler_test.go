package files

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

func TestWriteHandlersFailClosedWhenDatabaseUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{Svc: &Service{}}

	for _, tc := range []struct {
		name    string
		method  string
		path    string
		handler gin.HandlerFunc
	}{
		{name: "upload", method: http.MethodPost, path: "/files/upload", handler: h.Upload},
		{name: "delete", method: http.MethodDelete, path: "/files/id", handler: h.Delete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Handle(tc.method, tc.path, tc.handler)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))

			require.Equal(t, http.StatusInternalServerError, w.Code)
			var body response.Envelope
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, response.CodeInternalError, body.Code)
		})
	}
}

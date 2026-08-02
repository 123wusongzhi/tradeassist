package aiproductimage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

func TestRegisterUnavailableDependenciesFailClosed(t *testing.T) {
	for _, svc := range []*Service{nil, {}} {
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set(ctxkey.TenantID, int64(1)) })
		Register(r, &Handler{Svc: svc})
		for _, tc := range []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/products/ai-images/batches"},
			{http.MethodPost, "/products/ai-images/batches/check"},
		} {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("svc=%#v path=%s status=%d body=%s", svc, tc.path, rec.Code, rec.Body.String())
			}
			var body response.Envelope
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != response.CodeInternalError || body.Message != "aiproductimage unavailable" {
				t.Fatalf("svc=%#v path=%s envelope=%+v err=%v", svc, tc.path, body, err)
			}
		}
	}
}

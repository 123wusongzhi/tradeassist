package imagetask

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

func TestRegisterUnavailableDependenciesFailClosed(t *testing.T) {
	for _, svc := range []*Service{nil, {}} {
		r := gin.New()
		Register(r.Group("/api/v1"), &Handler{Svc: svc})
		for _, path := range []string{"/api/v1/image/tasks", "/api/v1/ai/image/score"} {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("svc=%#v path=%s status=%d body=%s", svc, path, rec.Code, rec.Body.String())
			}
			var body response.Envelope
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != response.CodeInternalError || body.Message != "imagetask unavailable" {
				t.Fatalf("svc=%#v path=%s envelope=%+v err=%v", svc, path, body, err)
			}
		}
	}
}

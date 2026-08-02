package customerchat

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterFailsClosedWhenServiceOrDBUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, handler := range map[string]*Handler{
		"service":  {Svc: nil},
		"database": {Svc: &Service{}},
	} {
		t.Run(name, func(t *testing.T) {
			r := gin.New()
			Register(r.Group("/api/v1"), handler)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/customer/conversations", nil))
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
			}
		})
	}
}

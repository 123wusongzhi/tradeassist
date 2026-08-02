package securitymod

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestRegisterRoutesFailsClosedWhenServiceOrDBUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, handler := range map[string]*Handler{
		"database": {Svc: &Service{}},
		"service":  {DB: &gorm.DB{}},
	} {
		t.Run(name, func(t *testing.T) {
			r := gin.New()
			RegisterRoutes(r.Group("/api/v1"), handler)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/security/overview", nil))
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
			}
		})
	}
}

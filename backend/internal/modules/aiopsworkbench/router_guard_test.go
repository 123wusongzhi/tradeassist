package aiopsworkbench

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestRegisterFailsClosedWhenServiceOrDBUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, handler := range map[string]*Handler{
		"service":  {Svc: nil},
		"database": {Svc: &Service{}},
	} {
		t.Run(name, func(t *testing.T) {
			r := gin.New()
			Register(r, handler)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ai/operation-workbench/summary", nil))
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
			}
		})
	}
}

func TestRegisterStillRejectsMissingTenantContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, &Handler{Svc: &Service{DB: &gorm.DB{}}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ai/operation-workbench/summary", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

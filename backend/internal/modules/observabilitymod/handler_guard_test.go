package observabilitymod

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterFailsClosedWhenDatabaseUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, &Handler{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/observability/overview", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

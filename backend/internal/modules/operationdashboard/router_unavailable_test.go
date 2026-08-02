package operationdashboard

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDashboardRouterReturnsInternalErrorWhenUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, h := range []*Handler{{}, {Svc: &Service{}}} {
		router := gin.New()
		Register(router.Group("/api/v1"), h)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/overview", nil))
		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.Contains(t, w.Body.String(), "dashboard unavailable")
	}
}

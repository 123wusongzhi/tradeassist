package productpublish

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProductPublishRouterReturnsInternalErrorWhenUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, h := range []*Handler{{}, {Svc: &Service{}}} {
		router := gin.New()
		Register(router.Group("/api/v1"), h)

		for _, request := range []*http.Request{
			httptest.NewRequest(http.MethodGet, "/api/v1/products/id/publish-targets", nil),
			httptest.NewRequest(http.MethodPost, "/api/v1/products/id/publish", nil),
		} {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, request)
			require.Equal(t, http.StatusInternalServerError, w.Code)
			require.Contains(t, w.Body.String(), "product publish unavailable")
		}
	}
}

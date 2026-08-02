package orderexception

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOrderExceptionRouterReturnsInternalErrorWhenUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, h := range []*Handler{{}, {Svc: &Service{}}} {
		router := gin.New()
		Register(router.Group("/api/v1"), h)

		for _, request := range []*http.Request{
			httptest.NewRequest(http.MethodGet, "/api/v1/orders/exceptions", nil),
			httptest.NewRequest(http.MethodPost, "/api/v1/orders/exceptions/order/id/handle", nil),
		} {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, request)
			require.Equal(t, http.StatusInternalServerError, w.Code)
			require.Contains(t, w.Body.String(), "order exception unavailable")
		}
	}
}

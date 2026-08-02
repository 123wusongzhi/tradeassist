package alerting

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAlertHandlersFailClosedWithoutGlobalAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:alert_handler_access?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{Svc: NewService(db, time.Second, true)}

	tests := []struct {
		method string
		path   string
		handle gin.HandlerFunc
	}{
		{method: http.MethodGet, path: "/observability/alerts", handle: h.List},
		{method: http.MethodPost, path: "/observability/alerts/id/ack", handle: h.Ack},
		{method: http.MethodPost, path: "/observability/alerts/id/silence", handle: h.Silence},
	}
	for _, tc := range tests {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			router := gin.New()
			router.Handle(tc.method, tc.path, tc.handle)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
			if res.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusForbidden, res.Body.String())
			}
		})
	}
}

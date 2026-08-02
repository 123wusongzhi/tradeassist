package worker

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestWorkerMonitorFailsClosedWithoutGlobalAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:worker_monitor_access?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/workers/monitor", (&Handler{DB: db}).Monitor)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/workers/monitor", nil))
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusForbidden, res.Body.String())
	}
}

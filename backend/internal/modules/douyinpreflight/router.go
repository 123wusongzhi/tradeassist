package douyinpreflight

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts Douyin production preflight routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	requireGlobalAdmin := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "douyin preflight routes unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireGlobalAdmin(c, h.Svc.DB) {
			c.Abort()
			return
		}
	}
	write := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "douyin preflight routes unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermConfigManage) {
			c.Abort()
			return
		}
	}
	g.POST("/platform/douyin/production-preflight", requireGlobalAdmin, write, h.Run)
	g.GET("/platform/douyin/production-preflight/latest", requireGlobalAdmin, h.GetLatest)
}

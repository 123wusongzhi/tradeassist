package collectbrowserprofile

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts JWT-protected browser profile routes under /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	profiles := g.Group("/collect/browser-profiles")
	profiles.Use(func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "browser profiles unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermCollectProfileManage) {
			c.Abort()
		}
	})
	profiles.GET("", h.List)
	profiles.POST("", h.Create)
	profiles.POST("/:id/open-login", h.OpenLogin)
	profiles.POST("/:id/check", h.Check)
	profiles.POST("/:id/disable", h.Disable)
	profiles.POST("/:id/enable", h.Enable)
	profiles.DELETE("/:id", h.Delete)
}

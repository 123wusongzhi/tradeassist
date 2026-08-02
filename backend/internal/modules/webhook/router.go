package webhook

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// RegisterPublic mounts the unauthenticated webhook receiver under /api/v1.
func RegisterPublic(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/webhooks/:platform/:eventType", h.Receive)
}

// Register mounts authenticated webhook operations routes.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	read := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "webhooks unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermOrderView) {
			c.Abort()
		}
	}
	g.GET("/webhook-events", read, h.ListEvents)
}

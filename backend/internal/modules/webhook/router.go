package webhook

import "github.com/gin-gonic/gin"

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
	g.GET("/webhook-events", h.ListEvents)
}

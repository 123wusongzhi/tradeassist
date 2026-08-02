package customerchat

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts authenticated routes on g (already under /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	registerCustomerRoutes(g.Group("/customer"), h)
	registerCustomerRoutes(g.Group("/customer-service"), h)
}

func registerCustomerRoutes(c *gin.RouterGroup, h *Handler) {
	if c == nil || h == nil {
		return
	}
	available := func(c *gin.Context) bool {
		if h.Svc != nil && h.Svc.DB != nil {
			return true
		}
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		c.Abort()
		return false
	}
	read := func(c *gin.Context) {
		if !available(c) {
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermCustomerView) {
			c.Abort()
		}
	}
	write := func(c *gin.Context) {
		if !available(c) {
			return
		}
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermCustomerOperate) {
			c.Abort()
		}
	}
	c.Use(read)
	c.GET("/dashboard", h.GetDashboard)
	c.GET("/conversations", h.ListConversations)
	c.POST("/conversations", write, h.CreateConversation)

	c.GET("/conversations/:id/messages", h.ListMessages)
	c.POST("/conversations/:id/messages", write, h.CreateMessage)
	c.POST("/conversations/:id/mark-replied", write, h.MarkReplied)
	c.POST("/conversations/:id/ai/generate-reply", write, h.GenerateReply)
	c.POST("/conversations/:id/ai-suggestions", write, h.GenerateAISuggestion)
	c.GET("/conversations/:id/ai-suggestions", h.ListSuggestions)
	c.POST("/conversations/:id/send-platform-message", write, h.SendPlatformMessage)

	c.GET("/conversations/:id", h.GetConversation)
	c.PUT("/conversations/:id", write, h.UpdateConversation)
	c.DELETE("/conversations/:id", write, h.DeleteConversation)

	c.PUT("/reply-suggestions/:id", write, h.UpdateSuggestion)
	c.POST("/reply-suggestions/:id/accept", write, h.AcceptSuggestion)
	c.POST("/reply-suggestions/:id/discard", write, h.DiscardSuggestion)
	c.POST("/ai-suggestions/:id/apply", write, h.ApplySuggestion)
	c.POST("/ai-suggestions/:id/reject", write, h.RejectSuggestion)
}

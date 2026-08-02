package operationdashboard

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts dashboard routes under an authenticated group.
func Register(g *gin.RouterGroup, h *Handler) {
	requireProductView := func(c *gin.Context) {
		if h == nil || h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "dashboard unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermProductView) {
			c.Abort()
		}
	}
	g.GET("/dashboard/product-operations", requireProductView, h.ProductOperations)
	g.GET("/dashboard/overview", requireProductView, h.Overview)
	g.GET("/dashboard/todos", requireProductView, h.Todos)
	g.GET("/dashboard/health", requireProductView, h.Health)
}

package pricing

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts pricing routes on g (already under /api/v1, authenticated).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	read := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "pricing unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermProductView) {
			c.Abort()
		}
	}
	g.POST("/pricing/calculate", read, h.Calculate)
	write := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "pricing unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermProductWrite) {
			c.Abort()
		}
	}
	g.POST("/products/:id/pricing/apply", write, h.ApplyProduct)
	g.POST("/products/pricing/batch-apply", write, h.BatchApply)
}

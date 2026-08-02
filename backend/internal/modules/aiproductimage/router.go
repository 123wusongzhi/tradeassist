package aiproductimage

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts JWT-protected AI product image batch routes under /products/ai-images.
func Register(r gin.IRouter, h *Handler) {
	if h == nil {
		return
	}
	g := r.Group("/products/ai-images")
	g.Use(func(c *gin.Context) {
		ctx, _, err := withTenantContext(c)
		if err != nil {
			response.Fail(c, 403, response.CodeForbidden, "租户上下文无效")
			c.Abort()
			return
		}
		c.Request = c.Request.WithContext(ctx)
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "aiproductimage unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermProductView) {
			c.Abort()
			return
		}
	})
	write := g.Group("")
	write.Use(func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "aiproductimage unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermAIImageApply) {
			c.Abort()
		}
	})
	write.POST("/batches/check", h.CheckBatch)
	write.POST("/batches", h.CreateBatch)
	g.GET("/batches", h.ListBatches)
	g.GET("/batches/:id", h.GetBatch)
	write.POST("/batches/:id/retry-failed", h.RetryFailed)
	write.POST("/batches/:id/cancel-pending", h.CancelPending)
	write.POST("/batches/:id/apply-selected", h.ApplySelected)
	write.POST("/batches/:id/undo-applied", h.UndoApplied)
	write.POST("/items/:id/regenerate", h.RegenerateItem)
	write.POST("/items/:id/apply", h.ApplyItem)
	write.POST("/items/:id/reject", h.RejectItem)
}

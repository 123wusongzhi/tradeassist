package aioperationbatch

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts JWT-protected ai batch routes (caller wraps with BearerAuth).
func Register(r gin.IRouter, h *Handler) {
	if h == nil {
		return
	}
	g := r.Group("/ai/batches")
	g.Use(func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "ai batches unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermProductView) {
			c.Abort()
		}
	})
	write := func(perm string) gin.HandlerFunc {
		return func(c *gin.Context) {
			if h.Svc == nil || h.Svc.DB == nil {
				response.Fail(c, 500, response.CodeInternalError, "ai batches unavailable")
				c.Abort()
				return
			}
			if !adminperm.RequireWrite(c, h.Svc.DB, perm) {
				c.Abort()
			}
		}
	}
	g.POST("/product-text", write(adminperm.PermAITextApply), h.CreateProductText)
	g.POST("/product-images", write(adminperm.PermAIImageApply), h.CreateProductImages)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.GET("/:id/tasks", h.Tasks)
	g.POST("/:id/retry-failed", write(adminperm.PermProductWrite), h.RetryFailed)
	g.POST("/:id/apply-results", write(adminperm.PermProductWrite), h.ApplyResults)
}

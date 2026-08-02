package orderexception

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts routes under /orders/exceptions (parent group already covers /api/v1).
func Register(parent *gin.RouterGroup, h *Handler) {
	if parent == nil || h == nil {
		return
	}
	g := parent.Group("/orders/exceptions")
	read := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "order exception unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermOrderView) {
			c.Abort()
			return
		}
	}
	write := func(p string) gin.HandlerFunc {
		return func(c *gin.Context) {
			if h.Svc == nil || h.Svc.DB == nil {
				response.Fail(c, 500, response.CodeInternalError, "order exception unavailable")
				c.Abort()
				return
			}
			if !adminperm.RequireWrite(c, h.Svc.DB, p) {
				c.Abort()
				return
			}
		}
	}
	g.GET("", read, h.List)
	g.GET("/:sourceType/:sourceId", read, h.Detail)
	g.POST("/:sourceType/:sourceId/handle", write(adminperm.PermOrderOperate), h.Handle)
	g.POST("/:sourceType/:sourceId/ignore", write(adminperm.PermOrderOperate), h.Ignore)
	g.DELETE("/:sourceType/:sourceId/mark", write(adminperm.PermOrderOperate), h.Unmark)
	g.POST("/:sourceType/:sourceId/bind-sku", write(adminperm.PermSKUBind), h.BindSKU)
	g.POST("/:sourceType/:sourceId/retry-deduct", write(adminperm.PermInventoryOperate), h.RetryDeduct)
	g.POST("/:sourceType/:sourceId/retry-inventory-sync", write(adminperm.PermInventoryOperate), h.RetryInventorySync)
}

package order

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts authenticated routes (already under Bearer /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	read := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermOrderView) {
			c.Abort()
		}
	}
	g.GET("/order-item-sku-matches", read, h.ListGlobalSKUMatches)
	g.POST("/order-items/:itemId/bind-sku", h.PostBindOrderItemSKU)

	o := g.Group("/orders")
	o.GET("", read, h.List)
	o.POST("", h.Create)

	o.POST("/:id/items", h.PostItem)
	o.PUT("/:id/items/:itemId", h.PutItem)
	o.DELETE("/:id/items/:itemId", h.DeleteItem)

	o.POST("/:id/deduct-inventory", h.PostDeductInventory)
	o.POST("/:id/restore-inventory", h.PostRestoreInventory)
	o.GET("/:id/inventory-effects", read, h.GetOrderInventoryEffects)
	o.GET("/:id/sku-matches", read, h.GetOrderSKUMatches)
	o.POST("/:id/match-skus", h.PostMatchOrderSKUs)

	o.PUT("/:id/shipments/:shipmentId", h.PutShipment)
	o.DELETE("/:id/shipments/:shipmentId", h.DeleteShipment)

	o.GET("/:id", read, h.Get)
	o.PUT("/:id", h.Update)
	o.DELETE("/:id", h.Delete)
}

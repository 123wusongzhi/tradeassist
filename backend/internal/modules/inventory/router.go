package inventory

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts inventory + inventory sync REST routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	read := func(perm string) gin.HandlerFunc {
		return func(c *gin.Context) {
			if h.Svc == nil || h.Svc.DB == nil {
				response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
				c.Abort()
				return
			}
			if !adminperm.RequirePermission(c, h.Svc.DB, perm) {
				c.Abort()
			}
		}
	}
	inventoryRead := read(adminperm.PermInventoryView)
	syncRead := read(adminperm.PermInventorySyncRead)
	g.POST("/products/:id/skus/:skuId/adjust-stock", h.AdjustStock)
	g.GET("/products/:id/skus/:skuId/inventory-logs", inventoryRead, h.ListSKULogs)
	g.GET("/products/:id/publication-skus", inventoryRead, h.ListPublicationSkuRows)

	g.POST("/product-publication-skus/:id/sync-inventory", h.SyncPublicationSku)
	g.POST("/products/:id/sync-inventory", h.BatchSyncProduct)

	g.GET("/inventory", inventoryRead, h.ListCenter)
	g.GET("/inventory/logs", inventoryRead, h.ListGlobalLogs)
	g.GET("/inventory/effects", inventoryRead, h.ListGlobalOrderEffects)
	g.GET("/inventory/alerts", inventoryRead, h.ListAlerts)
	g.POST("/inventory/stock-settings/batch-preview", h.BatchPreviewStockSettings)
	g.POST("/inventory/stock-settings/batch-update", h.BatchUpdateStockSettings)

	g.GET("/inventory-sync/tasks", syncRead, h.ListTasks)
	g.GET("/inventory-sync/tasks/:id", syncRead, h.GetTask)
	g.POST("/inventory-sync/tasks/:id/retry", h.RetryTask)

	g.POST("/inventory-sync/batches/retry-failed-tasks", h.RetryInventorySyncTasksBatch)
	g.POST("/inventory-sync/batches", h.CreateInventorySyncBatch)
	g.GET("/inventory-sync/batches", syncRead, h.ListInventorySyncBatches)
	g.GET("/inventory-sync/batches/:id/tasks", syncRead, h.ListInventorySyncBatchTasks)
	g.GET("/inventory-sync/batches/:id", syncRead, h.GetInventorySyncBatch)
	g.POST("/inventory-sync/batches/:id/retry-failed", h.RetryInventorySyncBatchFailed)
}

package ordersync

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Register mounts order-sync routes on authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	write := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "order sync unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermOrderOperate) {
			c.Abort()
		}
	}
	read := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "order sync unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermOrderView) {
			c.Abort()
		}
	}
	shopOperate := func(c *gin.Context) {
		if h.Svc.Shops == nil {
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "order sync unavailable")
			c.Abort()
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
		if err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
			c.Abort()
			return
		}
		// Resolve the tenant-scoped row first. This prevents a global/store grant
		// from reaching credentials or queue work for another tenant's shop.
		if _, err := h.Svc.Shops.TenantShop(c, id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
			} else {
				response.HandleError(c, err)
			}
			c.Abort()
			return
		}
		if !adminperm.RequireStoreOperate(c, h.Svc.DB, id) {
			c.Abort()
		}
	}
	g.POST("/shops/:id/sync-orders", write, shopOperate, h.SyncShopOrders)

	og := g.Group("/order-sync")
	og.GET("/tasks", read, h.ListTasks)
	og.GET("/tasks/:id", read, h.GetTask)
	og.POST("/tasks/:id/retry", write, h.RetryTask)
}

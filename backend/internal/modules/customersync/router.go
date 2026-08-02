package customersync

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

// Register mounts customer message sync routes on authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	write := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "customer message sync unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermCustomerOperate) {
			c.Abort()
		}
	}
	read := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "customer message sync unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermCustomerView) {
			c.Abort()
		}
	}
	shopOperate := func(c *gin.Context) {
		if h.Svc.Shops == nil {
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "customer message sync unavailable")
			c.Abort()
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
		if err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
			c.Abort()
			return
		}
		// Keep tenant lookup ahead of credential/provider/queue work and preserve
		// the store grant 404/403 anti-enumeration behavior.
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
	g.POST("/shops/:id/sync-customer-messages", write, shopOperate, h.SyncShopCustomerMessages)
	cg := g.Group("/customer/message-sync")
	cg.GET("/tasks", read, h.ListTasks)
	cg.GET("/tasks/:id", read, h.GetTask)
	cg.POST("/tasks/:id/retry", write, h.RetryTask)
}

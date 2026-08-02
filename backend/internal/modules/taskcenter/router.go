package taskcenter

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts task center routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	tc := g.Group("/task-center")
	read := tc.Group("", requireTaskPermission(h, false))
	read.GET("/failure-categories", h.FailureCategories)
	read.GET("/alert-notifications", h.ListAlertNotifications)
	read.GET("/alerts", h.ListAlerts)
	read.GET("/failures", h.ListFailures)
	read.GET("/summary", h.Summary)
	read.GET("/failures/:taskType/:id", h.GetFailure)
	write := tc.Group("", requireTaskPermission(h, true))
	write.POST("/alerts/:id/notify", h.NotifyAlert)
	write.POST("/alerts/:id/handle", h.HandleAlert)
	write.POST("/alerts/:id/ignore", h.IgnoreAlert)
	write.DELETE("/alerts/:id/mark", h.Unmark)
	write.POST("/failures/batch-retry", h.BatchRetry)
	write.POST("/failures/batch-ignore", h.BatchIgnore)
	write.POST("/failures/batch-handle", h.BatchHandle)
	write.POST("/failures/:taskType/:id/generate-alert", h.GenerateAlertFromFailure)
	write.POST("/failures/:taskType/:id/retry", h.Retry)
	write.POST("/failures/:taskType/:id/ignore", h.Ignore)
	write.POST("/failures/:taskType/:id/handle", h.Handle)
	write.DELETE("/failures/:taskType/:id/mark", h.Unmark)
	tc.POST("/alerts/scan", requireTaskScanPermission(h), h.ScanAlerts)
}

func requireTaskPermission(h *Handler, write bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !checkTaskPermission(c, h, write) {
			c.Abort()
			return
		}
		c.Next()
	}
}

func checkTaskPermission(c *gin.Context, h *Handler, write bool) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return false
	}
	if _, err := adminperm.TenantIDFromGin(c); err != nil {
		response.HandleError(c, err)
		return false
	}
	if write {
		return adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermTaskRetry)
	}
	return adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermTaskRetry)
}

func requireTaskScanPermission(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !checkTaskPermission(c, h, true) {
			c.Abort()
			return
		}
		tid, err := adminperm.TenantIDFromGin(c)
		p, perr := adminperm.LoadPrincipal(c, h.Svc.DB)
		if err != nil || perr != nil || tid != 0 || p == nil || !p.IsAdmin() {
			adminperm.DenyPermission(c)
			c.Abort()
			return
		}
		c.Next()
	}
}

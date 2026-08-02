package collect

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts authenticated collect routes on g (already under /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	write := g.Group("")
	write.Use(func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermProductWrite) {
			c.Abort()
		}
	})
	read := g.Group("")
	read.Use(func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermProductView) {
			c.Abort()
		}
	})
	monitor := g.Group("")
	monitor.Use(func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireGlobalAdmin(c, h.Svc.DB) {
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermSettingsManage) {
			c.Abort()
		}
	})
	read.GET("/collect/providers", h.ListProviders)
	read.GET("/collect/engines/status", h.EnginesStatus)
	write.POST("/collect/tasks", h.Create)
	read.GET("/collect/tasks", h.List)
	monitor.GET("/collect/monitor", h.Monitor)
	read.GET("/collect/tasks/:id/events", h.ListTaskEvents)
	read.GET("/collect/tasks/:id", h.Get)
	write.POST("/collect/tasks/:id/retry", h.Retry)

	write.POST("/collect/batches", h.CreateBatch)
	read.GET("/collect/batches", h.ListBatches)
	read.GET("/collect/batches/:id/tasks", h.ListBatchTasks)
	read.GET("/collect/batches/:id", h.GetBatch)
	write.POST("/collect/batches/:id/retry-failed", h.RetryBatchFailed)

	read.GET("/collector/providers/1688/auth-status", h.Get1688AuthStatus)
	write.POST("/collector/providers/1688/open-login-browser", h.Open1688LoginBrowser)
	read.GET("/collector/providers/pinduoduo/auth-status", h.GetPinduoduoAuthStatus)
	write.POST("/collector/providers/pinduoduo/check-login", h.CheckPinduoduoLogin)
	write.POST("/collect/providers/pinduoduo/check-login", h.CheckPinduoduoLogin)
	write.POST("/collector/providers/pinduoduo/open-login-browser", h.OpenPinduoduoLoginBrowser)

	write.POST("/collector/providers/taobao_tmall/check-login", h.CheckTaobaoTmallLogin)
	write.POST("/collect/providers/taobao_tmall/check-login", h.CheckTaobaoTmallLogin)
	write.POST("/collector/providers/taobao_tmall/open-login-browser", h.OpenTaobaoTmallLoginBrowser)
	write.POST("/collect/providers/taobao_tmall/open-login-browser", h.OpenTaobaoTmallLoginBrowser)
}

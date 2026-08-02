package imagetask

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts /api/v1/image/tasks and /api/v1/ai/image/* routes on an authenticated group.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	imageRead := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "imagetask unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermProductView) {
			c.Abort()
		}
	}
	g.GET("/image/providers", imageRead, h.ListProviders)

	rg := g.Group("/image/tasks", imageRead)
	imageWrite := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "imagetask unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermAIImageApply) {
			c.Abort()
		}
	}
	productWrite := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "imagetask unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermProductWrite) {
			c.Abort()
		}
	}
	rg.POST("", imageWrite, h.Create)
	rg.GET("", h.List)
	rg.GET("/monitor", h.Monitor)
	rg.GET("/:id", h.Get)
	rg.GET("/:id/items", h.ListItems)
	rg.POST("/:id/apply", imageWrite, h.Apply)
	rg.DELETE("/:id/items/:itemId", imageWrite, h.DeleteItem)
	rg.POST("/:id/retry", imageWrite, h.Retry)
	rg.GET("/:id/translate-edit-state", h.GetTranslateEditState)
	rg.POST("/:id/manual-render", imageWrite, h.ManualRenderTranslate)

	ai := g.Group("/ai/image", imageRead)
	ai.POST("/tasks", imageWrite, h.Create)
	ai.GET("/tasks", h.List)
	ai.GET("/tasks/:id", h.Get)
	ai.GET("/tasks/:id/translate-edit-state", h.GetTranslateEditState)
	ai.POST("/tasks/:id/manual-render", imageWrite, h.ManualRenderTranslate)
	ai.POST("/task-items/:id/save-to-product", productWrite, h.SaveItemToProduct)
	ai.POST("/task-items/:id/set-as-main", productWrite, h.SetItemAsMain)
	ai.POST("/score", productWrite, h.ScoreImage)

	g.POST("/products/:id/images/select-best-main", productWrite, h.SelectBestMainForProduct)
}

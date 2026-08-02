package collectruleai

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts AI collect rule routes (must register before /collect/rules/:id).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	write := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.Settings == nil || h.Svc.Settings.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "collect rule ai unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireWrite(c, h.Svc.Settings.DB, adminperm.PermSettingsManage) {
			c.Abort()
		}
	}
	g.POST("/collect/rules/ai-generate", write, h.Generate)
	g.POST("/collect/rules/ai-generate-and-save", write, h.GenerateAndSave)
}

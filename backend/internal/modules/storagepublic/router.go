package storagepublic

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts storage public access routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	requireGlobalAdmin := func(c *gin.Context) {
		if h.DB == nil {
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "storage public routes unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireGlobalAdmin(c, h.DB) {
			c.Abort()
			return
		}
	}
	write := func(c *gin.Context) {
		if h.DB == nil {
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "storage public routes unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireWrite(c, h.DB, adminperm.PermConfigManage) {
			c.Abort()
			return
		}
	}
	g.POST("/storage/test-public-access", requireGlobalAdmin, write, h.TestPublicAccess)
	g.POST("/settings/storage/public-check", requireGlobalAdmin, write, h.TestPublicAccess)
	g.GET("/settings/storage/public-check/latest", requireGlobalAdmin, h.GetLatestPublicCheck)
}

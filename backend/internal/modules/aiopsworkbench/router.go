package aiopsworkbench

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
)

// Register mounts JWT-protected AI operation workbench routes.
func Register(r gin.IRouter, h *Handler) {
	if h == nil {
		return
	}
	g := r.Group("/ai/operation-workbench")
	g.Use(func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "workbench unavailable")
			c.Abort()
			return
		}
		if !hasTenantContext(c.Request.Context()) {
			response.Fail(c, 403, response.CodeForbidden, "tenant context required")
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermProductView) {
			c.Abort()
			return
		}
		c.Next()
	})
	g.GET("/summary", h.Summary)
	g.GET("/todos", h.ListTodos)
	g.GET("/todos/:id", h.GetTodo)
	g.POST("/todos/refresh", h.RefreshTodos)
}

// hasTenantContext accepts an explicit system tenant (0), but never treats it
// as a wildcard. The request context is populated only by trusted auth middleware.
func hasTenantContext(ctx context.Context) bool {
	tc := security.FromContext(ctx)
	return tc != nil && tc.TenantID >= 0
}

package disasterrecovery

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

type Handler struct {
	Svc *Service
}

func Register(r gin.IRouter, h *Handler) {
	if r == nil || h == nil {
		return
	}
	g := r.Group("/ops/dr")
	g.GET("/status", h.Status)
	g.POST("/drills", h.CreateDrill)
}

func (h *Handler) Status(c *gin.Context) {
	if !adminperm.RequireGlobalAdmin(c, h.Svc.DB) {
		return
	}
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermDRRead) {
		return
	}
	out, err := h.Svc.Status(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) CreateDrill(c *gin.Context) {
	if !adminperm.RequireGlobalAdmin(c, h.Svc.DB) {
		return
	}
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermDRExecute) {
		return
	}
	var req DrillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid drill request")
		return
	}
	row, err := h.Svc.CreateDrill(c.Request.Context(), req, currentAdminID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

func currentAdminID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if id, err := uuid.Parse(s); err == nil {
				return &id
			}
		}
	}
	return nil
}

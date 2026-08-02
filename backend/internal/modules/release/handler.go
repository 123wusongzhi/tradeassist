package release

import (
	"net/http"
	"strconv"

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
	g := r.Group("/ops/releases")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.POST("/:id/execute", h.Execute)
	g.POST("/:id/rollback", h.Rollback)
}

func (h *Handler) List(c *gin.Context) {
	if !adminperm.RequireGlobalAdmin(c, h.Svc.DB) {
		return
	}
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermReleaseRead) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	items, total, err := h.Svc.List(c.Request.Context(), page, pageSize)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (h *Handler) Create(c *gin.Context) {
	if !adminperm.RequireGlobalAdmin(c, h.Svc.DB) {
		return
	}
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermReleaseCreate) {
		return
	}
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid release request")
		return
	}
	row, err := h.Svc.Create(c.Request.Context(), req, currentAdminID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *Handler) Get(c *gin.Context) {
	if !adminperm.RequireGlobalAdmin(c, h.Svc.DB) {
		return
	}
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermReleaseRead) {
		return
	}
	row, err := h.Svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "release not found")
		return
	}
	response.OK(c, row)
}

func (h *Handler) Execute(c *gin.Context) {
	if !adminperm.RequireGlobalAdmin(c, h.Svc.DB) {
		return
	}
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermReleaseExecute) {
		return
	}
	row, err := h.Svc.Execute(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *Handler) Rollback(c *gin.Context) {
	if !adminperm.RequireGlobalAdmin(c, h.Svc.DB) {
		return
	}
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermReleaseRollback) {
		return
	}
	var req RollbackRequest
	_ = c.ShouldBindJSON(&req)
	row, err := h.Svc.Rollback(c.Request.Context(), c.Param("id"), req, currentAdminID(c))
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

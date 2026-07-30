package backup

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
	g := r.Group("/ops/backups")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.POST("/:id/verify", h.Verify)
	g.POST("/:id/hold", h.Hold)
	g.DELETE("/:id", h.Delete)
}

func (h *Handler) List(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupRead) {
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
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupCreate) {
		return
	}
	var req CreateRequest
	_ = c.ShouldBindJSON(&req)
	row, err := h.Svc.CreateDatabaseBackup(c.Request.Context(), req, currentAdminID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *Handler) Get(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupRead) {
		return
	}
	row, err := h.Svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "backup not found")
		return
	}
	response.OK(c, row)
}

func (h *Handler) Verify(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupVerify) {
		return
	}
	row, err := h.Svc.Verify(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *Handler) Hold(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupHold) {
		return
	}
	var req HoldRequest
	_ = c.ShouldBindJSON(&req)
	row, err := h.Svc.Hold(c.Request.Context(), c.Param("id"), req, currentAdminID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *Handler) Delete(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupDelete) {
		return
	}
	if err := h.Svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
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

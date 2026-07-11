package exportmod

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler exposes export job APIs.
type Handler struct {
	Svc *Service
}

// RegisterRoutes mounts /api/v1/exports routes.
func RegisterRoutes(authed *gin.RouterGroup, h *Handler) {
	if authed == nil || h == nil {
		return
	}
	g := authed.Group("/exports")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
}

func (h *Handler) List(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermExportRead) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.Svc.ListJobs(c, page, ps)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows, "total": total, "page": page, "pageSize": ps})
}

func (h *Handler) Create(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermExportCreate) {
		return
	}
	var body struct {
		ExportType string         `json:"exportType"`
		ShopID     string         `json:"shopId"`
		MaskedPII  *bool          `json:"maskedPii"`
		Filters    map[string]any `json:"filters"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	var shopID *uuid.UUID
	if sid := strings.TrimSpace(body.ShopID); sid != "" {
		u, err := uuid.Parse(sid)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid shopId")
			return
		}
		shopID = &u
	}
	masked := true
	if body.MaskedPII != nil {
		masked = *body.MaskedPII
	}
	row, err := h.Svc.CreateJob(c, CreateJobInput{
		ExportType: strings.TrimSpace(body.ExportType),
		ShopID:     shopID,
		MaskedPII:  masked,
		Filters:    body.Filters,
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, row)
}

func (h *Handler) Get(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermExportRead) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	row, err := h.Svc.GetJob(c, id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, row)
}

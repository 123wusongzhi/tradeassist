package securitymod

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler serves /api/v1/security/* endpoints.
type Handler struct {
	Svc *Service
	DB  *gorm.DB
}

// RegisterRoutes mounts security routes on authed group.
func RegisterRoutes(authed *gin.RouterGroup, h *Handler) {
	if authed == nil || h == nil {
		return
	}
	g := authed.Group("/security")
	g.POST("/keys/rotation/prepare", h.RotationPrepare)
	g.GET("/keys/rotation/status", h.RotationStatus)
	g.POST("/keys/rotation/start", h.RotationStart)
	g.GET("/audit/integrity/status", h.AuditIntegrityStatus)
	g.POST("/audit/integrity/verify", h.AuditIntegrityVerify)
}

func (h *Handler) RotationPrepare(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermSecurityKeyRotate) {
		return
	}
	var body struct {
		ConfirmPhrase string `json:"confirmPhrase"`
	}
	_ = c.ShouldBindJSON(&body)
	if strings.TrimSpace(body.ConfirmPhrase) != "ROTATE-KEYS-DRY-RUN" {
		response.Fail(c, 400, response.CodeBadRequest, "AUTH_REAUTHENTICATION_REQUIRED")
		return
	}
	out, err := h.Svc.PrepareRotation(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) RotationStatus(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermSecurityKeyRotate) {
		return
	}
	out, err := h.Svc.PrepareRotation(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) RotationStart(c *gin.Context) {
	h.RotationPrepare(c)
}

func (h *Handler) AuditIntegrityStatus(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermAuditRead) {
		return
	}
	n, err := h.Svc.VerifyAuditIntegrity(c.Request.Context(), 7)
	if err != nil {
		response.OK(c, gin.H{"ok": false, "checked": n})
		return
	}
	response.OK(c, gin.H{"ok": true, "checked": n})
}

func (h *Handler) AuditIntegrityVerify(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermAuditRead) {
		return
	}
	var body struct {
		Days int `json:"days"`
	}
	_ = c.ShouldBindJSON(&body)
	n, err := h.Svc.VerifyAuditIntegrity(c.Request.Context(), body.Days)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "audit chain verification failed")
		return
	}
	response.OK(c, gin.H{"ok": true, "checked": n})
}

package securitymod

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
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
	g.GET("/overview", h.SecurityOverview)
	g.POST("/keys/rotation/prepare", h.RotationPrepare)
	g.GET("/keys/rotation/status", h.RotationStatus)
	g.POST("/keys/rotation/start", h.RotationStart)
	g.GET("/keys/rotation/:id", h.RotationGet)
	g.GET("/keys/rotation/:id/progress", h.RotationProgress)
	g.POST("/keys/rotation/:id/pause", h.RotationPause)
	g.POST("/keys/rotation/:id/resume", h.RotationResume)
	g.POST("/keys/rotation/:id/verify", h.RotationVerify)
	g.GET("/keys/references", h.KeyReferences)
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
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermSecurityKeyRotate) {
		return
	}
	var body struct {
		ConfirmPhrase string `json:"confirmPhrase"`
	}
	_ = c.ShouldBindJSON(&body)
	if strings.TrimSpace(body.ConfirmPhrase) != "ROTATE-KEYS-START" {
		response.Fail(c, 400, response.CodeBadRequest, "AUTH_REAUTHENTICATION_REQUIRED")
		return
	}
	uid, _ := c.Get(ctxkey.AdminID)
	startedBy, _ := uuid.Parse(strings.TrimSpace(uid.(string)))
	job, err := h.Svc.StartRotation(c.Request.Context(), startedBy, false)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, job)
}

func (h *Handler) RotationGet(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermSecurityKeyRotate) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	job, err := h.Svc.GetRotation(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, job)
}

func (h *Handler) RotationProgress(c *gin.Context) {
	h.RotationGet(c)
}

func (h *Handler) RotationPause(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermSecurityKeyRotate) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.PauseRotation(c.Request.Context(), id); err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"paused": true})
}

func (h *Handler) RotationResume(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermSecurityKeyRotate) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.ResumeRotation(c.Request.Context(), id); err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"resumed": true})
}

func (h *Handler) RotationVerify(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermSecurityKeyRotate) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	ok, counts, err := h.Svc.VerifyRotation(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": ok, "references": counts})
}

func (h *Handler) KeyReferences(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermSecurityKeyRotate) {
		return
	}
	kr, err := h.Svc.keyRing()
	if err != nil {
		response.HandleError(c, err)
		return
	}
	counts, err := h.Svc.CountSecretReferencesByKeyID(c.Request.Context(), kr.PreviousKeyIDs())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": counts})
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

func (h *Handler) SecurityOverview(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermConfigRead) {
		return
	}
	out, err := h.Svc.SecurityOverview(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if idStr, ok := c.Get(ctxkey.AdminID); ok {
		if uid, parseErr := uuid.Parse(strings.TrimSpace(idStr.(string))); parseErr == nil && uid != uuid.Nil {
			var sessionCount int64
			_ = h.DB.Model(&auth.AuthSession{}).Where("user_id = ? AND status = ?", uid, auth.SessionStatusActive).Count(&sessionCount).Error
			out["activeSessionCount"] = sessionCount
		}
	}
	response.OK(c, out)
}

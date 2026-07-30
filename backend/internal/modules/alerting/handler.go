package alerting

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler exposes alert management APIs.
type Handler struct {
	Svc *Service
	DB  interface { /* gorm */
	}
}

// Register mounts alerting routes under authed group.
func Register(r gin.IRouter, h *Handler) {
	if r == nil || h == nil || h.Svc == nil {
		return
	}
	g := r.Group("/observability/alerts")
	g.GET("", h.List)
	g.POST("/:id/ack", h.Ack)
	g.POST("/:id/silence", h.Silence)
}

// List returns recent alerts.
func (h *Handler) List(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermAlertsRead) {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []AlertEvent
	q := h.Svc.DB.Order("last_seen_at DESC").Limit(limit)
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if err := q.Find(&rows).Error; err != nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, gin.H{"items": rows})
}

// Ack acknowledges an alert.
func (h *Handler) Ack(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermAlertsAck) {
		return
	}
	id := c.Param("id")
	if err := h.Svc.Acknowledge(c.Request.Context(), id); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"id": id, "status": StatusAcknowledged})
}

type silenceReq struct {
	Reason        string `json:"reason"`
	DurationHours int    `json:"durationHours"`
}

// Silence silences an alert.
func (h *Handler) Silence(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermAlertsSilence) {
		return
	}
	id := c.Param("id")
	var req silenceReq
	_ = c.ShouldBindJSON(&req)
	hours := req.DurationHours
	if hours <= 0 {
		hours = 4
	}
	by, _ := c.Get(ctxkey.AdminID)
	byStr, _ := by.(string)
	if byStr == "" {
		byStr = "system"
	}
	until := time.Now().UTC().Add(time.Duration(hours) * time.Hour)
	if err := h.Svc.Silence(c.Request.Context(), id, req.Reason, byStr, until); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"id": id, "status": StatusSilenced, "expiresAt": until})
}

func newUUID() string {
	return uuid.New().String()
}

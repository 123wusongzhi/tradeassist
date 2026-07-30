package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authcookie"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/p7diag"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/gorm"
)

// Handler serves auth HTTP API.
type Handler struct {
	LoginSvc *LoginService
	Sessions *SessionService
	Admins   *admin.Store
	OpLog    *operationlog.Service
	Redis    *rdb.Client
	Settings *settings.Service
	DB       *gorm.DB
	Cfg      *config.Config
}

type loginBody struct {
	Account  string `json:"account" binding:"required,min=1,max=128"`
	Password string `json:"password" binding:"required,min=1,max=128"`
}

// Login POST /api/v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	totalStart := time.Now()
	totalOutcome := p7diag.OutcomeSuccess
	defer func() {
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "total", totalOutcome, totalStart)
	}()
	if h == nil || h.LoginSvc == nil {
		totalOutcome = p7diag.OutcomeError
		response.Fail(c, 500, response.CodeInternalError, "auth unavailable")
		return
	}
	var body loginBody
	stageStart := time.Now()
	if err := c.ShouldBindJSON(&body); err != nil {
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "request_read", p7diag.OutcomeExpectedRejection, stageStart)
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "request_decode", p7diag.OutcomeExpectedRejection, stageStart)
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "json_decode", p7diag.OutcomeExpectedRejection, stageStart)
		totalOutcome = p7diag.OutcomeExpectedRejection
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "request_read", p7diag.OutcomeSuccess, stageStart)
	p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "request_decode", p7diag.OutcomeSuccess, stageStart)
	p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "json_decode", p7diag.OutcomeSuccess, stageStart)
	stageStart = time.Now()
	account := strings.TrimSpace(body.Account)
	p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "input_normalize", p7diag.OutcomeSuccess, stageStart)
	if account == "" {
		totalOutcome = p7diag.OutcomeExpectedRejection
		response.Fail(c, 400, response.CodeBadRequest, "account is required")
		return
	}
	res, err := h.LoginSvc.Login(c.Request.Context(), account, body.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		totalOutcome = p7diag.OutcomeExpectedRejection
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "invalid_decision", p7diag.OutcomeExpectedRejection, time.Now())
		if h.OpLog != nil {
			stageStart = time.Now()
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Username: account,
				Action:   "login",
				Resource: "auth",
				Status:   "failed",
				Message:  err.Error(),
			})
			// security_audit / operation_log / transaction_* stages are emitted inside OpLog.Write.
			p7diag.ObserveAuditWrite(p7diag.RouteAuthInvalidLogin, "security_audit", p7diag.OutcomeSuccess, stageStart)
			p7diag.Count(p7diag.RouteAuthInvalidLogin, "securityAuditWriteCount", 1)
			p7diag.Count(p7diag.RouteAuthInvalidLogin, "operationLogWriteCount", 1)
		}
		code := response.CodeUnauthorized
		msg := err.Error()
		if msg == ErrAccountTemporarilyLocked || msg == ErrTooManyAttempts {
			code = response.CodeForbidden
		}
		stageStart = time.Now()
		response.Fail(c, 401, code, msg)
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "response_encode", p7diag.OutcomeExpectedRejection, stageStart)
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "response_write", p7diag.OutcomeExpectedRejection, stageStart)
		return
	}
	uid, perr := uuid.Parse(res.User.ID)
	if perr == nil && h.OpLog != nil {
		stageStart = time.Now()
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: &uid,
			Username:    res.User.Username,
			Action:      "login",
			Resource:    "auth",
			Status:      "success",
		})
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "operation_log", p7diag.OutcomeSuccess, stageStart)
	}
	out := gin.H{
		"token":     res.Token,
		"expiresAt": res.ExpiresAt,
		"user":      res.User,
		"sessionMode": func() string {
			if h.Cfg != nil {
				return h.Cfg.Auth.SessionMode
			}
			return config.AuthSessionModeLegacy
		}(),
	}
	if h.Cfg != nil && h.Cfg.UsesSecureSession() {
		SetRefreshCookieResponse(c, h.Cfg, res.RefreshToken, h.Cfg.RefreshTokenTTL())
	} else if res.RefreshToken != "" {
		out["refreshToken"] = res.RefreshToken
	}
	if h.Cfg != nil && h.Cfg.Auth.SessionMode == config.AuthSessionModeLegacy {
		out["deprecatedSessionMode"] = true
	}
	stageStart = time.Now()
	response.OK(c, out)
	p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "response_encode", p7diag.OutcomeSuccess, stageStart)
	p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "response_write", p7diag.OutcomeSuccess, stageStart)
}

// Profile GET /api/v1/auth/profile
func (h *Handler) Profile(c *gin.Context) {
	if h == nil || h.Admins == nil {
		response.Fail(c, 500, response.CodeInternalError, "auth unavailable")
		return
	}
	idStr, ok := c.Get(ctxkey.AdminID)
	if !ok {
		response.Fail(c, 401, response.CodeUnauthorized, "unauthorized")
		return
	}
	s, _ := idStr.(string)
	uid, err := uuid.Parse(s)
	if err != nil {
		response.Fail(c, 401, response.CodeUnauthorized, "unauthorized")
		return
	}
	u, err := h.Admins.ByID(c.Request.Context(), uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 401, response.CodeUnauthorized, "unauthorized")
			return
		}
		response.HandleError(c, err)
		return
	}
	dn := u.DisplayName
	if dn == "" {
		dn = u.LoginLabel()
	}
	p, _ := adminperm.LoadPrincipal(c, h.DB)
	perms := adminperm.PermissionsForRole(strings.TrimSpace(u.Role))
	storePerms := make([]gin.H, 0)
	if p != nil && !p.IsAdmin() {
		perms = p.Permissions
		for _, g := range p.StoreGrants {
			storePerms = append(storePerms, gin.H{
				"storeId":         g.StoreID.String(),
				"platform":        g.Platform,
				"permissionScope": g.PermissionScope,
			})
		}
	}
	response.OK(c, gin.H{
		"id":               u.ID.String(),
		"username":         u.LoginLabel(),
		"email":            u.Email,
		"phone":            u.Phone,
		"displayName":      dn,
		"role":             strings.TrimSpace(u.Role),
		"status":           strings.TrimSpace(u.Status),
		"permissions":      perms,
		"storePermissions": storePerms,
		"createdAt":        u.CreatedAt,
		"updatedAt":        u.UpdatedAt,
	})
}

// Logout POST /api/v1/auth/logout — revokes server session when refresh cookie/body present.
func (h *Handler) Logout(c *gin.Context) {
	raw := authcookie.ReadRefresh(c)
	if raw == "" {
		var body struct {
			RefreshToken string `json:"refreshToken"`
		}
		_ = c.ShouldBindJSON(&body)
		raw = strings.TrimSpace(body.RefreshToken)
	}
	if h.Sessions != nil && raw != "" {
		_ = h.Sessions.RevokeByRefreshToken(c.Request.Context(), raw)
	}
	ClearSessionCookies(c, h.Cfg)
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "logout",
			Resource: "auth",
			Status:   "success",
		})
	}
	response.OK(c, gin.H{"ok": true})
}

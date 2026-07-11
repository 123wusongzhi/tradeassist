package auth

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authcookie"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
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
	if h == nil || h.LoginSvc == nil {
		response.Fail(c, 500, response.CodeInternalError, "auth unavailable")
		return
	}
	var body loginBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	account := strings.TrimSpace(body.Account)
	if account == "" {
		response.Fail(c, 400, response.CodeBadRequest, "account is required")
		return
	}
	res, err := h.LoginSvc.Login(c.Request.Context(), account, body.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Username: account,
				Action:   "login",
				Resource: "auth",
				Status:   "failed",
				Message:  err.Error(),
			})
		}
		code := response.CodeUnauthorized
		msg := err.Error()
		if msg == ErrAccountTemporarilyLocked || msg == ErrTooManyAttempts {
			code = response.CodeForbidden
		}
		response.Fail(c, 401, code, msg)
		return
	}
	uid, perr := uuid.Parse(res.User.ID)
	if perr == nil && h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: &uid,
			Username:    res.User.Username,
			Action:      "login",
			Resource:    "auth",
			Status:      "success",
		})
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
	response.OK(c, out)
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

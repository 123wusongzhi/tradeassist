package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authcookie"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

const refreshCookieName = authcookie.RefreshCookieName

// SessionHandler serves session management HTTP API.
type SessionHandler struct {
	Cfg      *config.Config
	Sessions *SessionService
	OpLog    *operationlog.Service
	DB       *gorm.DB
}

// Refresh POST /api/v1/auth/refresh
func (h *SessionHandler) Refresh(c *gin.Context) {
	if h == nil || h.Sessions == nil || h.Cfg == nil {
		response.Fail(c, 500, response.CodeInternalError, "auth unavailable")
		return
	}
	raw := authcookie.ReadRefresh(c)
	if raw == "" {
		var body struct {
			RefreshToken string `json:"refreshToken"`
		}
		_ = c.ShouldBindJSON(&body)
		raw = strings.TrimSpace(body.RefreshToken)
	}
	if raw == "" {
		response.Fail(c, 401, response.CodeUnauthorized, ErrRefreshTokenRevoked)
		return
	}
	res, err := h.Sessions.RotateRefresh(c.Request.Context(), raw, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		msg := err.Error()
		response.Fail(c, 401, response.CodeUnauthorized, msg)
		return
	}
	if h.Cfg.UsesSecureSession() {
		authcookie.SetRefresh(c, h.Cfg, res.RefreshToken, time.Now().UTC().Add(h.Cfg.RefreshTokenTTL()))
	}
	out := gin.H{
		"token":     res.AccessToken,
		"expiresAt": res.AccessExp.Unix(),
	}
	if !h.Cfg.UsesSecureSession() {
		out["refreshToken"] = res.RefreshToken
	}
	response.OK(c, out)
}

// ListSessions GET /api/v1/auth/sessions
func (h *SessionHandler) ListSessions(c *gin.Context) {
	if h == nil || h.Sessions == nil || h.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "auth unavailable")
		return
	}
	uid, ok := currentUserID(c)
	if !ok {
		response.Fail(c, 401, response.CodeUnauthorized, ErrAuthenticationRequired)
		return
	}
	rows, err := h.Sessions.ListSessions(c.Request.Context(), uid)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	currentSID, _ := c.Get(ctxkey.SessionID)
	current, _ := currentSID.(string)
	items := make([]gin.H, 0, len(rows))
	for _, s := range rows {
		items = append(items, gin.H{
			"id":               s.ID.String(),
			"deviceSummary":    s.DeviceSummary,
			"browserSummary":   s.BrowserSummary,
			"createdAt":        s.CreatedAt,
			"lastActivityAt":   s.LastActivityAt,
			"status":           s.Status,
			"isCurrent":        s.ID.String() == current,
			"userAgentSummary": s.UserAgentSummary,
		})
	}
	response.OK(c, gin.H{"items": items})
}

// DeleteSession DELETE /api/v1/auth/sessions/:id
func (h *SessionHandler) DeleteSession(c *gin.Context) {
	if h == nil || h.Sessions == nil {
		response.Fail(c, 500, response.CodeInternalError, "auth unavailable")
		return
	}
	uid, ok := currentUserID(c)
	if !ok {
		response.Fail(c, 401, response.CodeUnauthorized, ErrAuthenticationRequired)
		return
	}
	sid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid session id")
		return
	}
	if err := h.Sessions.RevokeSession(c.Request.Context(), sid, uid, "user_revoke"); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "session not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "session_revoke",
			Resource:   "auth_session",
			ResourceID: sid.String(),
			Status:     "success",
		})
	}
	currentSID, _ := c.Get(ctxkey.SessionID)
	if current, ok := currentSID.(string); ok && current == sid.String() {
		authcookie.Clear(c, h.Cfg)
	}
	response.OK(c, gin.H{"ok": true})
}

// RevokeOthers POST /api/v1/auth/sessions/revoke-others
func (h *SessionHandler) RevokeOthers(c *gin.Context) {
	if h == nil || h.Sessions == nil {
		response.Fail(c, 500, response.CodeInternalError, "auth unavailable")
		return
	}
	uid, ok := currentUserID(c)
	if !ok {
		response.Fail(c, 401, response.CodeUnauthorized, ErrAuthenticationRequired)
		return
	}
	currentSID, _ := c.Get(ctxkey.SessionID)
	cur := uuid.Nil
	if s, ok := currentSID.(string); ok {
		cur, _ = uuid.Parse(s)
	}
	n, err := h.Sessions.RevokeOtherSessions(c.Request.Context(), uid, cur)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "session_revoke_others",
			Resource: "auth_session",
			Status:   "success",
			Message:  "revoked " + string(rune('0'+min64(n, 9))),
		})
	}
	response.OK(c, gin.H{"revoked": n})
}

// LogoutAll POST /api/v1/auth/logout-all
func (h *SessionHandler) LogoutAll(c *gin.Context) {
	if h == nil || h.Sessions == nil {
		response.Fail(c, 500, response.CodeInternalError, "auth unavailable")
		return
	}
	uid, ok := currentUserID(c)
	if !ok {
		response.Fail(c, 401, response.CodeUnauthorized, ErrAuthenticationRequired)
		return
	}
	n, err := h.Sessions.RevokeAllUserSessions(c.Request.Context(), uid, "logout_all")
	if err != nil {
		response.HandleError(c, err)
		return
	}
	authcookie.Clear(c, h.Cfg)
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "logout_all",
			Resource: "auth",
			Status:   "success",
		})
	}
	response.OK(c, gin.H{"revoked": n})
}

func currentUserID(c *gin.Context) (uuid.UUID, bool) {
	idStr, ok := c.Get(ctxkey.AdminID)
	if !ok {
		return uuid.Nil, false
	}
	s, _ := idStr.(string)
	uid, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil || uid == uuid.Nil {
		return uuid.Nil, false
	}
	return uid, true
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// SetRefreshCookieResponse sets cookie when secure session mode is enabled.
func SetRefreshCookieResponse(c *gin.Context, cfg *config.Config, refresh string, ttl time.Duration) {
	if cfg != nil && cfg.UsesSecureSession() {
		authcookie.SetRefresh(c, cfg, refresh, time.Now().UTC().Add(ttl))
	}
}

// ClearSessionCookies clears refresh cookie on logout.
func ClearSessionCookies(c *gin.Context, cfg *config.Config) {
	authcookie.Clear(c, cfg)
}

// SafeRedirectQuery validates redirect query param.
func SafeRedirectQuery(c *gin.Context, cfg *config.Config, raw string) (string, error) {
	allowed := []string{}
	if cfg != nil {
		if u := strings.TrimSpace(cfg.AdminPublicURL); u != "" {
			if parsed, err := parseHost(u); err == nil {
				allowed = append(allowed, parsed)
			}
		}
	}
	return security.SafeRedirect(raw, allowed)
}

func parseHost(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.HasPrefix(raw, "http") {
		// use net/url in production code path via security package
		return strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://"), nil
	}
	return raw, nil
}

// EnsureMethodNotGet rejects state-changing GET requests.
func EnsureMethodNotGet(c *gin.Context) bool {
	if c.Request.Method == http.MethodGet {
		response.Fail(c, 405, response.CodeBadRequest, "method not allowed")
		return false
	}
	return true
}

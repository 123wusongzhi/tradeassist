package auth

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type registerBody struct {
	Email           string `json:"email" binding:"required,email,max=128"`
	Code            string `json:"code" binding:"required,len=6"`
	Password        string `json:"password" binding:"required,min=6,max=128"`
	ConfirmPassword string `json:"confirmPassword" binding:"required,eqfield=Password"`
}

func (h *Handler) Register(c *gin.Context) {
	if h.Redis == nil {
		response.Fail(c, 503, response.CodeInternalError, "redis unavailable")
		return
	}
	var body registerBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	emailAddr := strings.ToLower(strings.TrimSpace(body.Email))
	if IsWeakPassword(h.Cfg, body.Password) {
		response.Fail(c, 400, response.CodeBadRequest, "password does not meet security requirements")
		return
	}

	// Verify code
	codeKey := fmt.Sprintf("email_code:register:%s", emailAddr)
	storedCode, err := h.Redis.Get(c.Request.Context(), codeKey).Result()
	if err != nil || storedCode != body.Code {
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Username: emailAddr,
				Action:   "register",
				Resource: "auth",
				Status:   "failed",
				Message:  "invalid verification code",
			})
		}
		response.Fail(c, 400, response.CodeBadRequest, "invalid verification code")
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		response.Fail(c, 500, response.CodeInternalError, "password error")
		return
	}

	// Tenant and its first administrator must be created together. Tenant 0 is
	// reserved for legacy/system data and must never receive a public account.
	u := admin.AdminUser{
		Base:         model.Base{},
		Username:     admin.NewInternalUsername(),
		Email:        emailAddr,
		DisplayName:  emailAddr,
		PasswordHash: string(hash),
		Role:         adminperm.RoleTenantAdmin,
		Status:       "active",
	}

	if h.Admins == nil || h.Admins.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "auth unavailable")
		return
	}
	if err := h.Admins.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		tenant := &Tenant{Name: emailAddr}
		if err := tx.Create(tenant).Error; err != nil {
			return err
		}
		u.TenantID = tenant.ID
		return tx.Create(&u).Error
	}); err != nil {
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Username: emailAddr,
				Action:   "register",
				Resource: "auth",
				Status:   "failed",
				Message:  "email already registered",
			})
		}
		response.Fail(c, 400, response.CodeBadRequest, "email already registered")
		return
	}

	// Delete code after successful registration
	h.Redis.Del(c.Request.Context(), codeKey)

	// Log success
	if h.OpLog != nil {
		uid := u.ID
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: &uid,
			Username:    u.Email,
			Action:      "register",
			Resource:    "auth",
			Status:      "success",
		})
	}

	// Auto login
	res, err := h.LoginSvc.Login(c.Request.Context(), emailAddr, body.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		response.Fail(c, 500, response.CodeInternalError, "auto login failed")
		return
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

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"gorm.io/gorm"
)

// LoginService handles credential checks and token issuance.
type LoginService struct {
	Cfg      *config.Config
	Admins   *admin.Store
	Sessions *SessionService
}

// LoginResult is returned to HTTP layer.
type LoginResult struct {
	Token        string
	RefreshToken string
	ExpiresAt    int64 // unix seconds
	User         userView
}

type userView struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	DisplayName string `json:"displayName"`
}

// Login verifies credentials and returns tokens (session or legacy).
func (s *LoginService) Login(ctx context.Context, account, password, ip, userAgent string) (*LoginResult, error) {
	if s == nil || s.Admins == nil || s.Cfg == nil {
		return nil, fmt.Errorf("auth: misconfigured")
	}
	if s.Sessions != nil && (s.Cfg.UsesSecureSession() || s.Cfg.Auth.SessionMode == config.AuthSessionModeSecure) {
		res, err := s.Sessions.CreateSession(ctx, account, password, ip, userAgent)
		if err != nil {
			return nil, err
		}
		return &LoginResult{
			Token:        res.AccessToken,
			RefreshToken: res.RefreshToken,
			ExpiresAt:    res.AccessExp.Unix(),
			User:         res.User,
		}, nil
	}
	return s.legacyLogin(ctx, account, password)
}

func (s *LoginService) legacyLogin(ctx context.Context, account, password string) (*LoginResult, error) {
	u, err := s.Admins.ByLoginAccount(ctx, account)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(ErrInvalidCredentials)
		}
		return nil, err
	}
	if err := admin.CheckPassword(u.PasswordHash, password); err != nil {
		return nil, errors.New(ErrInvalidCredentials)
	}
	if st := strings.TrimSpace(strings.ToLower(u.Status)); st == "disabled" || st == "inactive" {
		return nil, errors.New(ErrUserDisabled)
	}
	label := u.LoginLabel()
	token, exp, err := LegacyMintToken(s.Cfg, u.ID, label)
	if err != nil {
		return nil, err
	}
	dn := u.DisplayName
	if dn == "" {
		dn = label
	}
	return &LoginResult{
		Token:     token,
		ExpiresAt: exp.Unix(),
		User: userView{
			ID:          u.ID.String(),
			Username:    label,
			Email:       u.Email,
			Phone:       u.Phone,
			DisplayName: dn,
		},
	}, nil
}

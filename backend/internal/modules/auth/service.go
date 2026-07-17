package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/p7diag"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// LoginService handles credential checks and token issuance.
type LoginService struct {
	Cfg      *config.Config
	Admins   *admin.Store
	Sessions *SessionService
	Metrics  *metrics.Catalog
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
	s.ObserveAuth("login_attempt", "success", "attempt", "password")
	if s.Sessions != nil && (s.Cfg.UsesSecureSession() || s.Cfg.Auth.SessionMode == config.AuthSessionModeSecure) {
		res, err := s.Sessions.CreateSession(ctx, account, password, ip, userAgent)
		if err != nil {
			s.ObserveAuth("login_failure", "failure", classifyAuthReason(err), "password")
			return nil, err
		}
		s.ObserveAuth("login_success", "success", "success", "password")
		return &LoginResult{
			Token:        res.AccessToken,
			RefreshToken: res.RefreshToken,
			ExpiresAt:    res.AccessExp.Unix(),
			User:         res.User,
		}, nil
	}
	res, err := s.legacyLogin(ctx, account, password)
	if err != nil {
		s.ObserveAuth("login_failure", "failure", classifyAuthReason(err), "password")
		return nil, err
	}
	s.ObserveAuth("login_success", "success", "success", "password")
	return res, nil
}

func (s *LoginService) legacyLogin(ctx context.Context, account, password string) (*LoginResult, error) {
	stageStart := time.Now()
	var u *admin.AdminUser
	var db *gorm.DB
	if s.Admins != nil {
		db = s.Admins.DB
	}
	timing, err := p7diag.TimedGorm(db, func() error {
		var lookupErr error
		u, lookupErr = s.Admins.ByLoginAccount(ctx, account)
		return lookupErr
	})
	outcome := authOutcome(err)
	p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "account_lookup", outcome, stageStart)
	p7diag.ObserveDBOperation(p7diag.RouteAuthInvalidLogin, "account_lookup", outcome, stageStart)
	p7diag.ObserveSQL(p7diag.RouteAuthInvalidLogin, "auth", "auth.account_lookup", "select", "admin_users", outcome, false, timing)
	p7diag.Count(p7diag.RouteAuthInvalidLogin, "accountLookupCount", 1)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			p7diag.Path(p7diag.RouteAuthInvalidLogin, "account_missing")
			p7diag.Path(p7diag.RouteAuthInvalidLogin, p7diag.PathUnknownAccount)
			p7diag.Count(p7diag.RouteAuthInvalidLogin, "unknownAccountQueryCount", 1)
			p7diag.ObservePasswordVerify(p7diag.PathUnknownAccount, p7diag.PasswordAlgoBcrypt, bcrypt.DefaultCost, 0, time.Now())
			return nil, errors.New(ErrInvalidCredentials)
		}
		return nil, err
	}
	stageStart = time.Now()
	cost := bcrypt.DefaultCost
	if c, cerr := bcrypt.Cost([]byte(u.PasswordHash)); cerr == nil {
		cost = c
	}
	if err := admin.CheckPassword(u.PasswordHash, password); err != nil {
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "password_verify", p7diag.OutcomeExpectedRejection, stageStart)
		p7diag.ObservePasswordVerify(p7diag.PathKnownWrongPassword, p7diag.PasswordAlgoBcrypt, cost, 1, stageStart)
		p7diag.Count(p7diag.RouteAuthInvalidLogin, "passwordVerifyCount", 1)
		p7diag.Count(p7diag.RouteAuthInvalidLogin, "wrongPasswordQueryCount", 1)
		p7diag.Path(p7diag.RouteAuthInvalidLogin, "wrong_password")
		p7diag.Path(p7diag.RouteAuthInvalidLogin, p7diag.PathKnownWrongPassword)
		return nil, errors.New(ErrInvalidCredentials)
	}
	p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "password_verify", p7diag.OutcomeSuccess, stageStart)
	p7diag.ObservePasswordVerify(p7diag.PathSuccessVerify, p7diag.PasswordAlgoBcrypt, cost, 1, stageStart)
	p7diag.Count(p7diag.RouteAuthInvalidLogin, "passwordVerifyCount", 1)
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

func authOutcome(err error) string {
	if err == nil {
		return p7diag.OutcomeSuccess
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p7diag.OutcomeExpectedRejection
	}
	return p7diag.OutcomeError
}

package auth

import (
	"strings"
)

// ObserveAuth records auth metrics through the shared P5 catalog.
func (s *SessionService) ObserveAuth(event, result, reason, authMode string) {
	if s == nil || s.Metrics == nil {
		return
	}
	s.Metrics.ObserveAuth(event, result, safeAuthReason(reason), safeAuthMode(authMode))
}

// ObserveAuth records auth metrics through the shared P5 catalog.
func (s *LoginService) ObserveAuth(event, result, reason, authMode string) {
	if s == nil {
		return
	}
	if s.Metrics != nil {
		s.Metrics.ObserveAuth(event, result, safeAuthReason(reason), safeAuthMode(authMode))
		return
	}
	if s.Sessions != nil {
		s.Sessions.ObserveAuth(event, result, reason, authMode)
	}
}

func safeAuthReason(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "attempt", "success", "invalid_credentials", "rate_limited", "account_locked", "refresh_reused", "reuse_detected", "revoked", "expired", "unknown_kid", "created", "user_revoke", "logout", "logout_all":
		return v
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func safeAuthMode(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "password", "refresh_token", "jwt", "session":
		return v
	case "":
		return "password"
	default:
		return "other"
	}
}

func classifyAuthReason(err error) string {
	if err == nil {
		return "success"
	}
	switch strings.TrimSpace(err.Error()) {
	case ErrInvalidCredentials:
		return "invalid_credentials"
	case ErrTooManyAttempts:
		return "rate_limited"
	case ErrAccountTemporarilyLocked:
		return "account_locked"
	case ErrRefreshTokenReused:
		return "refresh_reused"
	case ErrRefreshTokenExpired:
		return "expired"
	case ErrRefreshTokenRevoked, ErrSessionRevoked:
		return "revoked"
	default:
		return "other"
	}
}

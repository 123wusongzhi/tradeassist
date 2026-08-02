package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	AuthSessionModeSecure              = "secure_session"
	AuthSessionModeLegacy              = "legacy_local_storage"
	ErrCodeInsecureAuthConfig          = "INSECURE_AUTH_CONFIGURATION"
	ErrCodeInsecureLegacyAuthForbidden = "INSECURE_LEGACY_AUTH_MODE_FORBIDDEN"
	ErrCodeInsecureCookieConfig        = "INSECURE_COOKIE_CONFIGURATION"
	ErrCodeKeyringConfigInvalid        = "KEYRING_CONFIGURATION_INVALID"
)

// AuthConfig holds Phase P4 authentication and session settings.
type AuthConfig struct {
	SessionMode                  string
	AccessTokenTTLMinutes        int
	RefreshTokenTTLDays          int
	SecureCookie                 bool
	CookieDomain                 string
	CookieSameSite               string
	LoginMaxAttempts             int
	LoginWindowMinutes           int
	AccountLockMinutes           int
	LoginIPRateLimit             int
	RefreshRateLimit             int
	PasswordMinLength            int
	PasswordRequireChangeOnReset bool
	JWTActiveKeyID               string
	JWTActiveSecret              string
	JWTPreviousKeyID             string
	JWTPreviousSecret            string
	JWTRotationGraceMinutes      int
	JWTRotationStartedAt         string
	AppMasterActiveKeyID         string
	AppMasterActiveKey           string
	AppMasterPreviousKeys        string
	UploadMaxFiles               int
	UploadMaxImagePixels         int64
	UploadMaxImageWidth          int
	UploadMaxImageHeight         int
	UploadMaxAnimationFrames     int
}

// AccessTokenTTL returns configured access token lifetime.
func (c *Config) AccessTokenTTL() time.Duration {
	if c == nil {
		return 15 * time.Minute
	}
	if c.Auth.AccessTokenTTLMinutes > 0 {
		return time.Duration(c.Auth.AccessTokenTTLMinutes) * time.Minute
	}
	if c.Auth.SessionMode == AuthSessionModeLegacy {
		return time.Duration(c.JWTExpHrs) * time.Hour
	}
	return 15 * time.Minute
}

// RefreshTokenTTL returns configured refresh token lifetime.
func (c *Config) RefreshTokenTTL() time.Duration {
	if c == nil {
		return 7 * 24 * time.Hour
	}
	days := c.Auth.RefreshTokenTTLDays
	if days <= 0 {
		days = 7
	}
	return time.Duration(days) * 24 * time.Hour
}

// UsesSecureSession reports whether refresh tokens use HttpOnly cookies.
func (c *Config) UsesSecureSession() bool {
	if c == nil {
		return false
	}
	return strings.TrimSpace(c.Auth.SessionMode) == AuthSessionModeSecure
}

// AuthLoginMaxAttempts returns max failed attempts before lockout.
func (c *Config) AuthLoginMaxAttempts() int {
	if c == nil || c.Auth.LoginMaxAttempts <= 0 {
		return 5
	}
	return c.Auth.LoginMaxAttempts
}

func (c *Config) AuthLoginWindowMinutes() int {
	if c == nil || c.Auth.LoginWindowMinutes <= 0 {
		return 15
	}
	return c.Auth.LoginWindowMinutes
}

func (c *Config) AuthAccountLockMinutes() int {
	if c == nil || c.Auth.AccountLockMinutes <= 0 {
		return 30
	}
	return c.Auth.AccountLockMinutes
}

func (c *Config) AuthPasswordMinLength() int {
	if c == nil || c.Auth.PasswordMinLength <= 0 {
		return 8
	}
	return c.Auth.PasswordMinLength
}

func loadAuthConfig(appEnv string) AuthConfig {
	mode := strings.TrimSpace(os.Getenv("AUTH_SESSION_MODE"))
	if mode == "" {
		// Secure server-side sessions are the safe default in every environment.
		// Legacy local-storage transport remains available only when a developer
		// opts in explicitly, and is still forbidden in staging/production.
		mode = AuthSessionModeSecure
	}
	return AuthConfig{
		SessionMode:                  mode,
		AccessTokenTTLMinutes:        atoiOrDefault(os.Getenv("AUTH_ACCESS_TOKEN_TTL_MINUTES"), 15),
		RefreshTokenTTLDays:          atoiOrDefault(os.Getenv("AUTH_REFRESH_TOKEN_TTL_DAYS"), 7),
		SecureCookie:                 envBool(os.Getenv("AUTH_SECURE_COOKIE"), IsStagingOrProduction(appEnv)),
		CookieDomain:                 strings.TrimSpace(os.Getenv("AUTH_COOKIE_DOMAIN")),
		CookieSameSite:               firstNonEmpty(os.Getenv("AUTH_COOKIE_SAME_SITE"), "Lax"),
		LoginMaxAttempts:             atoiOrDefault(os.Getenv("AUTH_LOGIN_MAX_ATTEMPTS"), 5),
		LoginWindowMinutes:           atoiOrDefault(os.Getenv("AUTH_LOGIN_WINDOW_MINUTES"), 15),
		AccountLockMinutes:           atoiOrDefault(os.Getenv("AUTH_ACCOUNT_LOCK_MINUTES"), 30),
		LoginIPRateLimit:             atoiOrDefault(os.Getenv("AUTH_LOGIN_IP_RATE_LIMIT"), 30),
		RefreshRateLimit:             atoiOrDefault(os.Getenv("AUTH_REFRESH_RATE_LIMIT"), 60),
		PasswordMinLength:            atoiOrDefault(os.Getenv("AUTH_PASSWORD_MIN_LENGTH"), 8),
		PasswordRequireChangeOnReset: envBool(os.Getenv("AUTH_PASSWORD_REQUIRE_CHANGE_ON_ADMIN_RESET"), true),
		JWTActiveKeyID:               strings.TrimSpace(os.Getenv("JWT_ACTIVE_KEY_ID")),
		JWTActiveSecret:              strings.TrimSpace(os.Getenv("JWT_ACTIVE_SECRET")),
		JWTPreviousKeyID:             strings.TrimSpace(os.Getenv("JWT_PREVIOUS_KEY_ID")),
		JWTPreviousSecret:            strings.TrimSpace(os.Getenv("JWT_PREVIOUS_SECRET")),
		JWTRotationGraceMinutes:      atoiOrDefault(os.Getenv("JWT_ROTATION_GRACE_MINUTES"), 60),
		JWTRotationStartedAt:         strings.TrimSpace(os.Getenv("JWT_ROTATION_STARTED_AT")),
		AppMasterActiveKeyID:         strings.TrimSpace(os.Getenv("APP_MASTER_ACTIVE_KEY_ID")),
		AppMasterActiveKey:           strings.TrimSpace(os.Getenv("APP_MASTER_ACTIVE_KEY")),
		AppMasterPreviousKeys:        strings.TrimSpace(os.Getenv("APP_MASTER_PREVIOUS_KEYS")),
		UploadMaxFiles:               atoiOrDefault(os.Getenv("UPLOAD_MAX_FILES"), 10),
		UploadMaxImagePixels:         int64(atoiOrDefault(os.Getenv("UPLOAD_MAX_IMAGE_PIXELS"), 50_000_000)),
		UploadMaxImageWidth:          atoiOrDefault(os.Getenv("UPLOAD_MAX_IMAGE_WIDTH"), 8192),
		UploadMaxImageHeight:         atoiOrDefault(os.Getenv("UPLOAD_MAX_IMAGE_HEIGHT"), 8192),
		UploadMaxAnimationFrames:     atoiOrDefault(os.Getenv("UPLOAD_MAX_ANIMATION_FRAMES"), 300),
	}
}

func (c *Config) validateAuthSecurity() error {
	if c == nil {
		return nil
	}
	// Load normalizes an empty value to the environment default. Keep Validate
	// equally safe for programmatically-built Config values used by tests and
	// embedders, but reject every non-enumerated value instead of silently
	// falling into the legacy path.
	mode := strings.TrimSpace(c.Auth.SessionMode)
	if mode == "" {
		mode = AuthSessionModeSecure
		c.Auth.SessionMode = mode
	}
	if mode != AuthSessionModeSecure && mode != AuthSessionModeLegacy {
		return fmt.Errorf("%s: AUTH_SESSION_MODE must be %q or %q", ErrCodeInsecureAuthConfig, AuthSessionModeSecure, AuthSessionModeLegacy)
	}
	if IsStagingOrProduction(c.AppEnv) {
		if mode == AuthSessionModeLegacy {
			return fmt.Errorf("%s: AUTH_SESSION_MODE=legacy_local_storage forbidden in staging/production", ErrCodeInsecureLegacyAuthForbidden)
		}
		if !c.Auth.SecureCookie {
			return fmt.Errorf("%s: AUTH_SECURE_COOKIE must be true in staging/production", ErrCodeInsecureCookieConfig)
		}
		if c.Auth.AccessTokenTTLMinutes <= 0 || c.Auth.AccessTokenTTLMinutes > 24*60 {
			return fmt.Errorf("%s: AUTH_ACCESS_TOKEN_TTL_MINUTES must be between 1 and 1440", ErrCodeInsecureAuthConfig)
		}
		activeSecret := strings.TrimSpace(c.Auth.JWTActiveSecret)
		if activeSecret == "" {
			activeSecret = strings.TrimSpace(c.JWTSecret)
		}
		if isInsecureSecret(activeSecret) {
			return fmt.Errorf("%s: effective JWT signing key must be a strong unique value", ErrCodeKeyringConfigInvalid)
		}
		previousKeyID := strings.TrimSpace(c.Auth.JWTPreviousKeyID)
		previousSecret := strings.TrimSpace(c.Auth.JWTPreviousSecret)
		if previousKeyID != "" && previousSecret != "" && c.Auth.JWTRotationGraceMinutes > 0 && isInsecureSecret(previousSecret) {
			return fmt.Errorf("%s: previous JWT verification key must be a strong unique value", ErrCodeKeyringConfigInvalid)
		}
		if previousKeyID != "" && previousSecret != "" && c.Auth.JWTRotationGraceMinutes > 0 {
			started, err := time.Parse(time.RFC3339, strings.TrimSpace(c.Auth.JWTRotationStartedAt))
			if err != nil || !strings.HasSuffix(strings.TrimSpace(c.Auth.JWTRotationStartedAt), "Z") || started.After(time.Now().UTC()) {
				return fmt.Errorf("%s: JWT_ROTATION_STARTED_AT must be a non-future RFC3339 UTC timestamp when a previous JWT key is configured", ErrCodeKeyringConfigInvalid)
			}
		}
	}
	activeKey := strings.TrimSpace(c.Auth.AppMasterActiveKey)
	if activeKey == "" {
		activeKey = strings.TrimSpace(c.MasterKey)
	}
	if IsProduction(c.AppEnv) && activeKey == "" {
		return fmt.Errorf("%s: APP_MASTER_KEY or APP_MASTER_ACTIVE_KEY required", ErrCodeKeyringConfigInvalid)
	}
	return nil
}

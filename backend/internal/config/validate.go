package config

import (
	"fmt"
	"strings"
)

// Config error codes (API / logs).
const (
	ErrCodeConfigRequired            = "CONFIG_REQUIRED"
	ErrCodeConfigInvalid             = "CONFIG_INVALID"
	ErrCodeConfigInsecureDefault     = "CONFIG_INSECURE_DEFAULT"
	ErrCodeProductionDevRouteEnabled = "PRODUCTION_DEV_ROUTE_ENABLED"
	ErrCodeStorageProviderInvalid    = "STORAGE_PROVIDER_INVALID"
	ErrCodeStoragePublicBaseInvalid  = "STORAGE_PUBLIC_BASE_INVALID"
	ErrCodeSecretKeyRequired         = "SECRET_KEY_REQUIRED"
	ErrCodeDatabaseNotReady          = "DATABASE_NOT_READY"
	ErrCodeRedisNotReady             = "REDIS_NOT_READY"
)

const defaultJWTSecret = "change-me-in-development"

var insecureJWTSecrets = map[string]struct{}{
	defaultJWTSecret:          {},
	"change-me-in-production": {},
	"changeme":                {},
	"secret":                  {},
	"jwt-secret":              {},
	"trademind":               {},
	"your-secret-key":         {},
	"your_jwt_secret":         {},
}

// Validate checks profile-specific rules after env load. Production errors are fatal.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("%s: config is nil", ErrCodeConfigRequired)
	}
	c.AppEnv = NormalizeEnv(c.AppEnv)

	if strings.TrimSpace(c.DB.User) == "" || strings.TrimSpace(c.DB.Name) == "" {
		return fmt.Errorf("%s: DB_USER and DB_NAME are required", ErrCodeConfigRequired)
	}

	if !IsProduction(c.AppEnv) {
		return c.validateNonProduction()
	}
	return c.validateProduction()
}

func (c *Config) validateNonProduction() error {
	if c.EnableDemoSeed && c.EnableDevRoutes {
		// allowed in dev/demo
	}
	return nil
}

func (c *Config) validateProduction() error {
	if c.JWTSecret == defaultJWTSecret || isInsecureSecret(c.JWTSecret) {
		return fmt.Errorf("%s: JWT_SECRET must be set to a strong unique value in production", ErrCodeConfigInsecureDefault)
	}
	if strings.TrimSpace(c.MasterKey) == "" {
		return fmt.Errorf("%s: APP_MASTER_KEY is required in production", ErrCodeSecretKeyRequired)
	}
	if strings.TrimSpace(c.APIPublicURL) == "" {
		return fmt.Errorf("%s: API_PUBLIC_URL is required in production", ErrCodeConfigRequired)
	}
	if strings.TrimSpace(c.AdminPublicURL) == "" {
		return fmt.Errorf("%s: ADMIN_PUBLIC_URL is required in production", ErrCodeConfigRequired)
	}
	if c.EnableDemoSeed {
		return fmt.Errorf("%s: ENABLE_DEMO_SEED must be false in production", ErrCodeProductionDevRouteEnabled)
	}
	if c.EnableDevRoutes {
		return fmt.Errorf("%s: ENABLE_DEV_ROUTES must be false in production", ErrCodeProductionDevRouteEnabled)
	}
	if c.EnableDebugEndpoints {
		return fmt.Errorf("%s: ENABLE_DEBUG_ENDPOINTS must be false in production", ErrCodeProductionDevRouteEnabled)
	}
	if c.EnableSwagger {
		return fmt.Errorf("%s: ENABLE_SWAGGER must be false in production", ErrCodeProductionDevRouteEnabled)
	}
	if strings.TrimSpace(c.BootstrapAdminPassword) == "" {
		return fmt.Errorf("%s: ADMIN_BOOTSTRAP_PASSWORD is required in production when bootstrapping admin", ErrCodeConfigRequired)
	}
	if isWeakBootstrapPassword(c.BootstrapAdminPassword) {
		return fmt.Errorf("%s: ADMIN_BOOTSTRAP_PASSWORD is too weak for production", ErrCodeConfigInsecureDefault)
	}
	return nil
}

func isInsecureSecret(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if _, ok := insecureJWTSecrets[strings.ToLower(s)]; ok {
		return true
	}
	if len(s) < 32 {
		return true
	}
	return false
}

func isWeakBootstrapPassword(p string) bool {
	p = strings.TrimSpace(p)
	if len(p) < 12 {
		return true
	}
	weak := []string{"password", "admin123", "changeme", "12345678", "trademind"}
	lower := strings.ToLower(p)
	for _, w := range weak {
		if lower == w {
			return true
		}
	}
	return false
}

// ProductionDangerousRoutesAllowed reports whether dev/demo seed routes may register.
func (c *Config) ProductionDangerousRoutesAllowed() bool {
	if c == nil {
		return true
	}
	if IsProduction(c.AppEnv) {
		return false
	}
	if c.EnableDevRoutes || c.EnableDemoSeed {
		return true
	}
	// Legacy: dev routes registered when not production (existing behavior).
	return !IsProduction(c.AppEnv)
}

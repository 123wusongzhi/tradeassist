package config

import (
	"fmt"
	"strings"
)

// Config error codes (API / logs).
const (
	ErrCodeConfigRequired                     = "CONFIG_REQUIRED"
	ErrCodeConfigInvalid                      = "CONFIG_INVALID"
	ErrCodeConfigInsecureDefault              = "CONFIG_INSECURE_DEFAULT"
	ErrCodeProductionDevRouteEnabled          = "PRODUCTION_DEV_ROUTE_ENABLED"
	ErrCodeStorageProviderInvalid             = "STORAGE_PROVIDER_INVALID"
	ErrCodeStoragePublicBaseInvalid           = "STORAGE_PUBLIC_BASE_INVALID"
	ErrCodeSecretKeyRequired                  = "SECRET_KEY_REQUIRED"
	ErrCodeDatabaseNotReady                   = "DATABASE_NOT_READY"
	ErrCodeRedisNotReady                      = "REDIS_NOT_READY"
	ErrCodeProductionWebhookFallbackForbidden = "PRODUCTION_WEBHOOK_FALLBACK_FORBIDDEN"
)

const defaultJWTSecret = "change-me-in-development"

const storageLocalForbiddenMsg = "staging/production 环境禁止使用 local storage，请配置 COS、OSS、S3、R2 或其他生产对象存储。"

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

	if err := c.validateStorageProvider(); err != nil {
		return err
	}
	if err := c.validateCORS(); err != nil {
		return err
	}
	if err := c.validateWebhookFallback(); err != nil {
		return err
	}
	if err := c.validateAuthSecurity(); err != nil {
		return err
	}

	if !IsProduction(c.AppEnv) {
		return c.validateNonProduction()
	}
	return c.validateProduction()
}

func (c *Config) validateWebhookFallback() error {
	if !IsStagingOrProduction(c.AppEnv) {
		return nil
	}
	if strings.TrimSpace(c.DouyinWebhookTestShopBindingID) != "" || c.EnableDouyinWebhookDemoFallback {
		return fmt.Errorf("%s: douyin webhook test/demo fallback is forbidden in staging/production", ErrCodeProductionWebhookFallbackForbidden)
	}
	return nil
}

func (c *Config) validateStorageProvider() error {
	env := NormalizeEnv(c.AppEnv)
	provider := strings.ToLower(strings.TrimSpace(c.StorageProvider))
	if provider == "" {
		provider = "local"
	}
	if !AllowsLocalStorage(env) && provider == "local" {
		return fmt.Errorf("%s: %s", ErrCodeStorageProviderInvalid, storageLocalForbiddenMsg)
	}
	allowed := map[string]struct{}{
		"local": {}, "cos": {}, "oss": {}, "s3": {}, "r2": {}, "minio": {},
	}
	if _, ok := allowed[provider]; !ok {
		return fmt.Errorf("%s: unknown storage provider %q", ErrCodeStorageProviderInvalid, provider)
	}
	return nil
}

func (c *Config) validateCORS() error {
	if !IsStagingOrProduction(c.AppEnv) {
		return nil
	}
	if len(c.CORSAllowedOrigins) == 0 {
		return fmt.Errorf("%s: CORS_ALLOWED_ORIGINS is required in staging/production", ErrCodeConfigRequired)
	}
	for _, o := range c.CORSAllowedOrigins {
		o = strings.TrimSpace(o)
		if o == "*" {
			if c.CORSAllowCredentials {
				return CORSError("wildcard origin not allowed with credentials")
			}
			return CORSError("wildcard origin not allowed in staging/production")
		}
	}
	return nil
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

// AllowsLocalStorageProvider reports whether the configured STORAGE_PROVIDER is permitted for APP_ENV.
func (c *Config) AllowsLocalStorageProvider() bool {
	if c == nil {
		return true
	}
	provider := strings.ToLower(strings.TrimSpace(c.StorageProvider))
	if provider == "" {
		provider = "local"
	}
	if provider != "local" {
		return true
	}
	return AllowsLocalStorage(c.AppEnv)
}

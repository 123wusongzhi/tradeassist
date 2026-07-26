package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const ErrCodeProductionTenantFallbackForbidden = "PRODUCTION_TENANT_FALLBACK_FORBIDDEN"

// TenantConfig holds Phase P4.1 tenant isolation settings.
type TenantConfig struct {
	EnableDevDefaultTenant  bool
	DevDefaultTenantID      int64
	EnableDemoDefaultTenant bool
	DemoDefaultTenantID     int64
	PrivateDownloadURLTTL   int
	ExportDownloadURLTTL    int
	SensitiveDownloadURLTTL int
}

func loadTenantConfig(appEnv string) TenantConfig {
	return TenantConfig{
		EnableDevDefaultTenant:  envBool(os.Getenv("ENABLE_DEV_DEFAULT_TENANT"), false),
		DevDefaultTenantID:      int64(atoiOrDefault(os.Getenv("DEV_DEFAULT_TENANT_ID"), 0)),
		EnableDemoDefaultTenant: envBool(os.Getenv("ENABLE_DEMO_DEFAULT_TENANT"), false),
		DemoDefaultTenantID:     int64(atoiOrDefault(os.Getenv("DEMO_DEFAULT_TENANT_ID"), 0)),
		PrivateDownloadURLTTL:   atoiOrDefault(os.Getenv("PRIVATE_DOWNLOAD_URL_TTL_SECONDS"), 300),
		ExportDownloadURLTTL:    atoiOrDefault(os.Getenv("EXPORT_DOWNLOAD_URL_TTL_SECONDS"), 3600),
		SensitiveDownloadURLTTL: atoiOrDefault(os.Getenv("SENSITIVE_DOWNLOAD_URL_TTL_SECONDS"), 120),
	}
}

func (c *Config) validateTenantIsolation() error {
	if c == nil {
		return nil
	}
	if IsStagingOrProduction(c.AppEnv) {
		if c.Tenant.EnableDevDefaultTenant || c.Tenant.EnableDemoDefaultTenant {
			return fmt.Errorf("%s: tenant fallback flags forbidden in staging/production", ErrCodeProductionTenantFallbackForbidden)
		}
		if c.Tenant.DevDefaultTenantID > 0 || c.Tenant.DemoDefaultTenantID > 0 {
			return fmt.Errorf("%s: default tenant id env vars forbidden in staging/production", ErrCodeProductionTenantFallbackForbidden)
		}
	}
	if IsProduction(c.AppEnv) {
		maxTTL := 86400
		priv := c.Tenant.PrivateDownloadURLTTL
		if priv <= 0 {
			priv = 300
		}
		exp := c.Tenant.ExportDownloadURLTTL
		if exp <= 0 {
			exp = 3600
		}
		sens := c.Tenant.SensitiveDownloadURLTTL
		if sens <= 0 {
			sens = 120
		}
		for _, ttl := range []struct {
			name string
			v    int
		}{
			{"PRIVATE_DOWNLOAD_URL_TTL_SECONDS", priv},
			{"EXPORT_DOWNLOAD_URL_TTL_SECONDS", exp},
			{"SENSITIVE_DOWNLOAD_URL_TTL_SECONDS", sens},
		} {
			if ttl.v <= 0 || ttl.v > maxTTL {
				return fmt.Errorf("%s: %s must be between 1 and %d in production", ErrCodeConfigInvalid, ttl.name, maxTTL)
			}
		}
	}
	return nil
}

// ResolveRequestTenantID returns trusted tenant id for HTTP requests.
func (c *Config) ResolveRequestTenantID(rawTenantID int64) (int64, string, error) {
	if rawTenantID > 0 {
		return rawTenantID, "access_token", nil
	}
	if c == nil {
		return 0, "", fmt.Errorf("TENANT_CONTEXT_MISSING")
	}
	env := NormalizeEnv(c.AppEnv)
	if env == EnvDevelopment || env == EnvTest {
		if c.Tenant.EnableDevDefaultTenant && c.Tenant.DevDefaultTenantID > 0 {
			return c.Tenant.DevDefaultTenantID, "dev_tenant_fallback", nil
		}
	}
	if env == EnvDemo {
		if c.Tenant.EnableDemoDefaultTenant && c.Tenant.DemoDefaultTenantID > 0 {
			return c.Tenant.DemoDefaultTenantID, "dev_tenant_fallback", nil
		}
	}
	if IsStagingOrProduction(env) {
		return 0, "", fmt.Errorf("%s", ErrCodeProductionTenantFallbackForbidden)
	}
	// Local development: legacy single-tenant rows often use tenant_id=0.
	return 0, "legacy_dev_zero", nil
}

// TenantFallbackEnabled reports whether explicit dev/demo tenant fallback is on.
func (c *Config) TenantFallbackEnabled() bool {
	if c == nil {
		return false
	}
	env := NormalizeEnv(c.AppEnv)
	if env == EnvDevelopment || env == EnvTest {
		return c.Tenant.EnableDevDefaultTenant && c.Tenant.DevDefaultTenantID > 0
	}
	if env == EnvDemo {
		return c.Tenant.EnableDemoDefaultTenant && c.Tenant.DemoDefaultTenantID > 0
	}
	return false
}

// ParseTenantIDEnv parses a tenant id from string env value.
func ParseTenantIDEnv(v string) int64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

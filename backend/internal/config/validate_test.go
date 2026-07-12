package config

import (
	"os"
	"strings"
	"testing"
)

func TestValidate_productionBlocksDefaults(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AppEnv:             EnvProduction,
		JWTSecret:          defaultJWTSecret,
		MasterKey:          "",
		APIPublicURL:       "",
		AdminPublicURL:     "",
		EnableDemoSeed:     true,
		EnableDevRoutes:    true,
		StorageProvider:    "cos",
		CORSAllowedOrigins: []string{"https://admin.example.com"},
		DB: DBConfig{
			Driver: "postgres",
			User:   "u",
			Name:   "db",
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected production validation failure")
	}
	msg := err.Error()
	for _, code := range []string{ErrCodeConfigInsecureDefault, ErrCodeSecretKeyRequired, ErrCodeConfigRequired, ErrCodeProductionDevRouteEnabled, ErrCodeInsecureCookieConfig, ErrCodeInsecureAuthConfig} {
		if strings.Contains(msg, code) {
			return
		}
	}
	t.Fatalf("unexpected error: %v", err)
}

func TestValidate_developmentAllowsDefaults(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AppEnv:    EnvDevelopment,
		JWTSecret: defaultJWTSecret,
		DB: DBConfig{
			Driver: "postgres",
			User:   "u",
			Name:   "db",
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development should allow defaults: %v", err)
	}
}

func productionP4Auth() AuthConfig {
	return AuthConfig{
		SessionMode:           AuthSessionModeSecure,
		SecureCookie:          true,
		AccessTokenTTLMinutes: 15,
		RefreshTokenTTLDays:   7,
	}
}

func TestValidate_productionRequiresStrongJWT(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AppEnv:                 EnvProduction,
		JWTSecret:              strings.Repeat("a", 48),
		MasterKey:              strings.Repeat("b", 64),
		APIPublicURL:           "https://api.example.com",
		AdminPublicURL:         "https://admin.example.com",
		BootstrapAdminPassword: "StrongPass!2026",
		StorageProvider:        "cos",
		CORSAllowedOrigins:     []string{"https://admin.example.com"},
		Auth:                   productionP4Auth(),
		Observability:          ValidProductionObservability(),
		DB: DBConfig{
			Driver: "postgres",
			User:   "u",
			Name:   "db",
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}
}

func TestValidate_productionRejectsDouyinWebhookFallback(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AppEnv:                         EnvProduction,
		JWTSecret:                      strings.Repeat("a", 48),
		MasterKey:                      strings.Repeat("b", 64),
		APIPublicURL:                   "https://api.example.com",
		AdminPublicURL:                 "https://admin.example.com",
		BootstrapAdminPassword:         "StrongPass!2026",
		StorageProvider:                "cos",
		CORSAllowedOrigins:             []string{"https://admin.example.com"},
		DouyinWebhookTestShopBindingID: "11111111-1111-1111-1111-111111111111",
		DB: DBConfig{
			Driver: "postgres",
			User:   "u",
			Name:   "db",
		},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), ErrCodeProductionWebhookFallbackForbidden) {
		t.Fatalf("expected webhook fallback rejection, got %v", err)
	}
}

func TestValidate_stagingRejectsDouyinWebhookDemoFallback(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AppEnv:                          EnvStaging,
		JWTSecret:                       defaultJWTSecret,
		StorageProvider:                 "cos",
		CORSAllowedOrigins:              []string{"https://admin.example.com"},
		EnableDouyinWebhookDemoFallback: true,
		DB: DBConfig{
			Driver: "postgres",
			User:   "u",
			Name:   "db",
		},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), ErrCodeProductionWebhookFallbackForbidden) {
		t.Fatalf("expected webhook fallback rejection, got %v", err)
	}
}

func TestRedactedSummary_noSecrets(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AppEnv:    EnvDevelopment,
		JWTSecret: "super-secret-jwt-key-should-not-appear",
		MasterKey: "master-key-secret",
		DB: DBConfig{
			Driver:   "postgres",
			Host:     "127.0.0.1",
			Port:     5432,
			User:     "trademind",
			Password: "db-password-secret",
			Name:     "trademind",
		},
	}
	sum := cfg.RedactedSummary()
	s := sum.String()
	if strings.Contains(s, "super-secret") || strings.Contains(s, "db-password") || strings.Contains(s, "master-key") {
		t.Fatalf("summary leaked secrets: %s", s)
	}
	if !sum.JWTSecretConfigured {
		t.Fatal("expected jwt configured flag")
	}
}

func TestLoad_productionFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", strings.Repeat("x", 48))
	t.Setenv("APP_MASTER_KEY", strings.Repeat("y", 64))
	t.Setenv("API_PUBLIC_URL", "https://api.example.com")
	t.Setenv("ADMIN_PUBLIC_URL", "https://admin.example.com")
	t.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "StrongPass!2026")
	t.Setenv("DB_USER", "u")
	t.Setenv("DB_NAME", "db")
	t.Setenv("ENABLE_DEMO_SEED", "false")
	t.Setenv("ENABLE_DEV_ROUTES", "false")
	t.Setenv("STORAGE_PROVIDER", "cos")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://admin.example.com")
	t.Setenv("AUTH_SESSION_MODE", "secure_session")
	t.Setenv("AUTH_SECURE_COOKIE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AppEnv != EnvProduction {
		t.Fatalf("got env %q", cfg.AppEnv)
	}
}

func TestAllowsLocalStorage(t *testing.T) {
	t.Parallel()
	if !AllowsLocalStorage(EnvDevelopment) {
		t.Fatal("dev should allow local storage")
	}
	if AllowsLocalStorage(EnvProduction) {
		t.Fatal("production must not allow local storage by policy")
	}
}

func TestProductionDangerousRoutesAllowed(t *testing.T) {
	t.Parallel()
	prod := &Config{AppEnv: EnvProduction}
	if prod.ProductionDangerousRoutesAllowed() {
		t.Fatal("production must block dangerous routes")
	}
	dev := &Config{AppEnv: EnvDevelopment}
	if !dev.ProductionDangerousRoutesAllowed() {
		t.Fatal("development should allow dev routes by default")
	}
}

func init() {
	_ = os.Unsetenv("APP_ENV")
}

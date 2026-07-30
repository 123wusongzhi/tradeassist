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

func productionP6BackupRelease() (BackupConfig, ReleaseConfig) {
	return BackupConfig{
			Enabled:               true,
			Mode:                  "object_storage",
			StorageProvider:       "s3",
			EncryptionEnabled:     true,
			RetentionDaily:        14,
			RetentionWeekly:       8,
			RetentionMonthly:      12,
			CommandTimeoutSeconds: 900,
		}, ReleaseConfig{
			Strategy:             "blue_green",
			RequirePreBackup:     true,
			HealthTimeoutSeconds: 120,
			KeepCount:            5,
		}
}

func productionP7() P7Config {
	return P7Config{
		PaginationDefaultLimit:     50,
		PaginationMaxLimit:         200,
		PaginationMaxOffset:        10000,
		PaginationCursorSigningKey: strings.Repeat("c", 48),
		DBMaxOpenConnections:       100,
		DBMaxIdleConnections:       10,
		DBConnMaxLifetimeSeconds:   3600,
		DBConnMaxIdleTimeSeconds:   900,
		DBQueryTimeoutMs:           5000,
		DBTransactionTimeoutMs:     10000,
		WorkerConcurrencyDefault:   2,
		WorkerQueueCapacity:        1000,
		WorkerMaxInflight:          100,
		WorkerPrefetch:             10,
		WorkerShutdownTimeoutSecs:  60,
		RateLimitEnabled:           true,
		RateLimitMode:              "local",
		RateLimitRedisPrefix:       "trademind:ratelimit",
		RateLimitFailMode:          "closed",
		RateLimitLocalFallback:     true,
		RateLimitPolicyVersion:     "p7-default-v1",
		CacheEnabled:               true,
		CacheDefaultTTLSeconds:     300,
		CacheMaxEntries:            10000,
		CacheSingleflightEnabled:   true,
		ExportBatchSize:            500,
		ExportMaxRows:              100000,
		ExportMaxBytes:             104857600,
		ExportMaxConcurrent:        2,
		PprofInternalOnly:          true,
	}
}

func TestValidate_productionRequiresStrongJWT(t *testing.T) {
	t.Parallel()
	backupCfg, releaseCfg := productionP6BackupRelease()
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
		Backup:                 backupCfg,
		Release:                releaseCfg,
		P7:                     productionP7(),
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

func TestValidateP9InventorySyncSafetyRejectsDangerousEnv(t *testing.T) {
	cases := []struct {
		key   string
		value string
	}{
		{key: "REAL_DOUYIN_ENABLED", value: "true"},
		{key: "REAL_PLATFORM_READ", value: "true"},
		{key: "INVENTORY_MUTATION_ENABLED", value: "true"},
		{key: "AUTO_INVENTORY_SYNC", value: "true"},
		{key: "INVENTORY_SYNC_PROVIDER_MODE", value: "production"},
		{key: "INVENTORY_SYNC_ACCESS_TOKEN", value: "secret-token"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			cfg := &Config{AppEnv: EnvDevelopment, DB: DBConfig{Driver: "postgres", User: "u", Name: "db"}}
			t.Setenv(tc.key, tc.value)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), ErrCodeP9ProductionCapabilityForbidden) {
				t.Fatalf("expected P9 safety rejection, got %v", err)
			}
		})
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
	t.Setenv("BACKUP_ENABLED", "true")
	t.Setenv("BACKUP_MODE", "object_storage")
	t.Setenv("BACKUP_STORAGE_PROVIDER", "s3")
	t.Setenv("BACKUP_ENCRYPTION_ENABLED", "true")
	t.Setenv("BACKUP_RETENTION_DAILY", "14")
	t.Setenv("BACKUP_RETENTION_WEEKLY", "8")
	t.Setenv("BACKUP_RETENTION_MONTHLY", "12")
	t.Setenv("RELEASE_REQUIRE_PRE_BACKUP", "true")
	t.Setenv("PAGINATION_CURSOR_SIGNING_KEY", strings.Repeat("c", 48))

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

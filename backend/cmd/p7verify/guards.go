package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/config"
)

func envBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func validateGuardsFromEnv() error {
	appEnv := config.NormalizeEnv(strings.TrimSpace(os.Getenv("APP_ENV")))
	if config.IsProduction(appEnv) {
		return fmt.Errorf("production verification is forbidden")
	}
	if appEnv != config.EnvPerformance {
		return fmt.Errorf("APP_ENV must be performance")
	}
	driver := strings.ToLower(strings.TrimSpace(firstEnv("DB_DRIVER", "postgres")))
	if driver != "postgres" {
		return fmt.Errorf("P7-C4 verification requires PostgreSQL")
	}
	host := strings.ToLower(strings.TrimSpace(firstEnv("DB_HOST", "127.0.0.1")))
	if host != "localhost" && host != "127.0.0.1" && !strings.HasPrefix(host, "/") {
		return fmt.Errorf("DB host must be localhost, 127.0.0.1, or a unix socket path, got %q", host)
	}
	if !envBool("PERFORMANCE_TEST_MODE") || !envBool("ALLOW_PERFORMANCE_DATASET") {
		return fmt.Errorf("PERFORMANCE_TEST_MODE and ALLOW_PERFORMANCE_DATASET must both be true")
	}
	if strings.ToLower(strings.TrimSpace(firstEnv("EXTERNAL_PROVIDER_MODE", "real"))) != "mock" {
		return fmt.Errorf("EXTERNAL_PROVIDER_MODE must be mock")
	}
	name := strings.TrimSpace(firstEnv("DB_NAME", ""))
	if !strings.HasPrefix(name, "trademind_p7c4_") {
		return fmt.Errorf("DB_NAME must start with trademind_p7c4_")
	}
	return nil
}

func firstEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func validateGuards(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if config.IsProduction(cfg.AppEnv) {
		return fmt.Errorf("production verification is forbidden")
	}
	if cfg.AppEnv != config.EnvPerformance {
		return fmt.Errorf("APP_ENV must be performance")
	}
	if cfg.DB.Driver != "postgres" {
		return fmt.Errorf("P7-C4 verification requires PostgreSQL")
	}
	host := strings.ToLower(strings.TrimSpace(cfg.DB.Host))
	if host != "localhost" && host != "127.0.0.1" && !strings.HasPrefix(host, "/") {
		return fmt.Errorf("DB host must be localhost, 127.0.0.1, or a unix socket path, got %q", cfg.DB.Host)
	}
	if !cfg.P7.PerformanceTestMode || !cfg.P7.AllowPerformanceDataset {
		return fmt.Errorf("PERFORMANCE_TEST_MODE and ALLOW_PERFORMANCE_DATASET must both be true")
	}
	if cfg.P7.ExternalProviderMode != "mock" {
		return fmt.Errorf("EXTERNAL_PROVIDER_MODE must be mock")
	}
	name := strings.TrimSpace(cfg.DB.Name)
	if !strings.HasPrefix(name, "trademind_p7c4_") {
		return fmt.Errorf("DB_NAME must start with trademind_p7c4_")
	}
	return nil
}

func guardList() []string {
	return []string{
		"no production datasets",
		"requires APP_ENV=performance",
		"requires PERFORMANCE_TEST_MODE=true",
		"requires ALLOW_PERFORMANCE_DATASET=true",
		"requires EXTERNAL_PROVIDER_MODE=mock",
		"requires PostgreSQL database name prefix trademind_p7c4_",
		"requires DB host localhost or 127.0.0.1",
	}
}

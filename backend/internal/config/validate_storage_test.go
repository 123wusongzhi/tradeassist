package config

import (
	"strings"
	"testing"
)

func TestValidate_storageProductionLocalFails(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AppEnv:                 EnvProduction,
		StorageProvider:        "local",
		JWTSecret:              strings.Repeat("a", 48),
		MasterKey:              strings.Repeat("b", 64),
		APIPublicURL:           "https://api.example.com",
		AdminPublicURL:         "https://admin.example.com",
		BootstrapAdminPassword: "StrongPass!2026",
		CORSAllowedOrigins:     []string{"https://admin.example.com"},
		DB:                     DBConfig{Driver: "postgres", User: "u", Name: "db"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected storage validation failure")
	}
	if !strings.Contains(err.Error(), ErrCodeStorageProviderInvalid) {
		t.Fatalf("expected %s, got %v", ErrCodeStorageProviderInvalid, err)
	}
}

func TestValidate_storageStagingLocalFails(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AppEnv:             EnvStaging,
		StorageProvider:    "local",
		CORSAllowedOrigins: []string{"https://admin.staging.example.com"},
		DB:                 DBConfig{Driver: "postgres", User: "u", Name: "db"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected staging local storage failure")
	}
	if !strings.Contains(err.Error(), ErrCodeStorageProviderInvalid) {
		t.Fatalf("expected %s, got %v", ErrCodeStorageProviderInvalid, err)
	}
}

func TestValidate_storageDemoLocalPasses(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AppEnv:          EnvDemo,
		StorageProvider: "local",
		DB:              DBConfig{Driver: "postgres", User: "u", Name: "db"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("demo+local should pass: %v", err)
	}
}

func TestValidate_storageDevelopmentLocalPasses(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AppEnv:          EnvDevelopment,
		StorageProvider: "local",
		DB:              DBConfig{Driver: "postgres", User: "u", Name: "db"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development+local should pass: %v", err)
	}
}

func TestValidate_storageProductionCOSPassesBase(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AppEnv:                 EnvProduction,
		StorageProvider:        "cos",
		JWTSecret:              strings.Repeat("a", 48),
		MasterKey:              strings.Repeat("b", 64),
		APIPublicURL:           "https://api.example.com",
		AdminPublicURL:         "https://admin.example.com",
		BootstrapAdminPassword: "StrongPass!2026",
		CORSAllowedOrigins:     []string{"https://admin.example.com"},
		Auth:                   productionP4Auth(),
		Observability:          ValidProductionObservability(),
		DB:                     DBConfig{Driver: "postgres", User: "u", Name: "db"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("production+cos should pass base validate: %v", err)
	}
}

func TestValidate_corsProductionWildcardRejected(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AppEnv:                 EnvProduction,
		StorageProvider:        "cos",
		CORSAllowedOrigins:     []string{"*"},
		CORSAllowCredentials:   true,
		JWTSecret:              strings.Repeat("a", 48),
		MasterKey:              strings.Repeat("b", 64),
		APIPublicURL:           "https://api.example.com",
		AdminPublicURL:         "https://admin.example.com",
		BootstrapAdminPassword: "StrongPass!2026",
		DB:                     DBConfig{Driver: "postgres", User: "u", Name: "db"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected cors wildcard failure")
	}
}

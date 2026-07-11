package config

import "testing"

func TestResolveRequestTenantID_developmentAllowsLegacyZero(t *testing.T) {
	t.Parallel()
	cfg := &Config{AppEnv: EnvDevelopment}
	tid, src, err := cfg.ResolveRequestTenantID(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tid != 0 || src != "legacy_dev_zero" {
		t.Fatalf("got tid=%d src=%q", tid, src)
	}
}

func TestResolveRequestTenantID_developmentFallback(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AppEnv: EnvDevelopment,
		Tenant: TenantConfig{
			EnableDevDefaultTenant: true,
			DevDefaultTenantID:     7,
		},
	}
	tid, src, err := cfg.ResolveRequestTenantID(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tid != 7 || src != "dev_tenant_fallback" {
		t.Fatalf("got tid=%d src=%q", tid, src)
	}
}

func TestResolveRequestTenantID_productionRejectsZero(t *testing.T) {
	t.Parallel()
	cfg := &Config{AppEnv: EnvProduction}
	_, _, err := cfg.ResolveRequestTenantID(0)
	if err == nil {
		t.Fatal("expected production error for tenant 0")
	}
}

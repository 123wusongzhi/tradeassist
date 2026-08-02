package collect

import "testing"

func TestProviderProfileKeyScopesNonLegacyTenants(t *testing.T) {
	if got := providerProfileKey(0, PinduoduoProfileKey); got != PinduoduoProfileKey {
		t.Fatalf("tenant 0 profile = %q", got)
	}
	if got := providerProfileKey(-1, PinduoduoProfileKey); got == PinduoduoProfileKey {
		t.Fatalf("invalid negative tenant must not share the system profile: %q", got)
	}
	if got := providerProfileKey(42, PinduoduoProfileKey); got != "tenant_42_pinduoduo" {
		t.Fatalf("pinduoduo profile = %q", got)
	}
	if got := providerProfileKey(42, TaobaoTmallProfileKey); got != "tenant_42_taobao_tmall" {
		t.Fatalf("taobao profile = %q", got)
	}
	if providerProfileKey(7, PinduoduoProfileKey) == providerProfileKey(8, PinduoduoProfileKey) {
		t.Fatal("distinct tenants must not share a provider profile key")
	}
}

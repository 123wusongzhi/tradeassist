package config

import "testing"

func TestParseTrustedProxies(t *testing.T) {
	got, err := parseTrustedProxies("127.0.0.1, 10.0.0.0/8,2001:db8::1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %v", got)
	}
}

func TestParseTrustedProxiesRejectsInvalidAndBroadCIDR(t *testing.T) {
	for _, raw := range []string{"not-an-ip", "0.0.0.0/0", "::/0"} {
		if _, err := parseTrustedProxies(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}

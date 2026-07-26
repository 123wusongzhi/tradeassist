package storagepublic

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestValidatePublicBase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		raw   string
		env   string
		valid bool
	}{
		{"empty", "", "production", false},
		{"relative", "/static", "production", false},
		{"localhost", "http://localhost/x", "production", false},
		{"127", "https://127.0.0.1/x", "production", false},
		{"private v4", "https://10.0.0.1/x", "production", false},
		{"file", "file:///tmp/x", "production", false},
		{"http staging fail", "http://cdn.example.com", "staging", false},
		{"https ok", "https://cdn.example.com", "production", true},
		{"http dev ok", "http://cdn.example.com", "development", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ValidatePublicBase(tc.raw, tc.env)
			if got.Valid != tc.valid {
				t.Fatalf("ValidatePublicBase(%q, %q) valid=%v checks=%+v", tc.raw, tc.env, got.Valid, got.Checks)
			}
		})
	}
}

func TestValidatePublicBase_rejectsCredentials(t *testing.T) {
	t.Parallel()
	got := ValidatePublicBase("https://user:pass@cdn.example.com", "production")
	if got.Valid {
		t.Fatal("expected credentials rejection")
	}
}

func TestPublicCheckObjectKey_prefix(t *testing.T) {
	t.Parallel()
	key := PublicCheckObjectKey("2026/01/02", "abc")
	if !strings.HasPrefix(key, "system-tests/storage-public-check/") {
		t.Fatalf("unexpected key: %s", key)
	}
}

func TestValidatePublicBase_privateIPv6(t *testing.T) {
	t.Parallel()
	if ValidatePublicBase("https://[::1]/x", "production").Valid {
		t.Fatal("expected ::1 rejection")
	}
	if ip := net.ParseIP("fc00::1"); ip != nil {
		_ = ip
	}
}

func TestValidatePublicBaseError(t *testing.T) {
	t.Parallel()
	if ValidatePublicBaseError("", "production") == nil {
		t.Fatal("expected error")
	}
	if ValidatePublicBaseError("https://cdn.example.com", "production") != nil {
		t.Fatal("expected nil for valid base")
	}
}

func TestVerifyPublicURL_privateAndRelative_extended(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"https://192.168.1.1/x.png",
		"https://172.16.0.1/x.png",
		"https://169.254.169.254/x.png",
	} {
		res := VerifyPublicURL(context.Background(), raw)
		if res.OK {
			t.Fatalf("expected failure for %q", raw)
		}
	}
}

package p7diag

import (
	"strings"
	"testing"
)

func TestNormalizeSQLStripsParameters(t *testing.T) {
	raw := `SELECT * FROM admin_users WHERE email = 'user@example.com' AND id = 42`
	norm := NormalizeSQL(raw)
	if strings.Contains(norm, "user@example.com") || strings.Contains(norm, "42") {
		t.Fatalf("normalized SQL leaked parameters: %q", norm)
	}
	if !strings.Contains(norm, "?") {
		t.Fatalf("expected placeholders in %q", norm)
	}
}

func TestFingerprintIsDeterministicAndLowCardinality(t *testing.T) {
	a := FingerprintFromParts("auth", "auth.account_lookup", "select", "admin_users", false)
	b := FingerprintFromParts("auth", "auth.account_lookup", "select", "admin_users", false)
	if a.QueryFingerprint == "" || a.QueryFingerprint != b.QueryFingerprint {
		t.Fatalf("fingerprint not deterministic: %#v %#v", a, b)
	}
	if len(a.QueryFingerprint) != fingerprintPrefixLen {
		t.Fatalf("unexpected fingerprint length: %d", len(a.QueryFingerprint))
	}
	c := FingerprintFromParts("webhook", "webhook.event_insert", "upsert", "webhook_events", false)
	if c.QueryFingerprint == a.QueryFingerprint {
		t.Fatal("different operations must not share fingerprint")
	}
}

func TestUnknownSQLDoesNotEmitRawParameters(t *testing.T) {
	fp := FingerprintFromNormalizedSQL("auth", "auth.account_lookup", "select", "admin_users", false, "SELECT * FROM t WHERE email='secret@x.com'")
	raw := fp.QueryFingerprint + fp.Operation + fp.Module
	if strings.Contains(raw, "secret@x.com") || strings.Contains(strings.ToLower(raw), "select * from") {
		t.Fatalf("fingerprint payload leaked SQL/params: %#v", fp)
	}
}

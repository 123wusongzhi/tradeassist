package security

import "testing"

func TestOriginAllowedRequiresExactOrigin(t *testing.T) {
	allowed := []string{"https://admin.example.com"}
	if originAllowed("https://admin.example.com.attacker.tld", allowed) {
		t.Fatal("suffix spoofed origin must be rejected")
	}
	if !originAllowed("https://admin.example.com", allowed) {
		t.Fatal("configured origin must be allowed")
	}
}

func TestOriginAllowedNormalizesDefaultPort(t *testing.T) {
	allowed := []string{"https://admin.example.com:443"}
	if !originAllowed("https://admin.example.com", allowed) {
		t.Fatal("default HTTPS port must compare equal to explicit :443")
	}
	if !refererAllowed("https://admin.example.com/dashboard?tab=home", allowed) {
		t.Fatal("referer must compare by origin only")
	}
}

func TestOriginAllowedRejectsUserInfoAndPaths(t *testing.T) {
	allowed := []string{"https://admin.example.com"}
	if originAllowed("https://admin.example.com@attacker.tld", allowed) {
		t.Fatal("userinfo origin must be rejected")
	}
	if originAllowed("https://admin.example.com/path", allowed) {
		t.Fatal("origin header containing a path must be rejected")
	}
}

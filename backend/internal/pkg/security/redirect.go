package security

import (
	"fmt"
	"net/url"
	"strings"
)

// SafeRedirect validates post-login or OAuth redirect targets.
func SafeRedirect(raw string, allowedHosts []string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/", nil
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
		return "", fmt.Errorf("unsafe redirect scheme")
	}
	if strings.HasPrefix(raw, "//") {
		return "", fmt.Errorf("protocol-relative redirect forbidden")
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		if strings.Contains(raw, "..") {
			return "", fmt.Errorf("path traversal in redirect")
		}
		return raw, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported redirect scheme")
	}
	host := strings.ToLower(u.Hostname())
	for _, a := range allowedHosts {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		if host == a || strings.HasSuffix(host, "."+a) {
			return raw, nil
		}
	}
	return "", fmt.Errorf("redirect host not allowed")
}

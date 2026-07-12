package logging

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/safefields"
)

var sensitiveKeys = []string{
	"authorization", "cookie", "set-cookie", "x-api-key", "x-signature",
	"password", "token", "secret", "api_key", "phone", "email", "address",
	"oauth_code", "signed_url", "access_token", "refresh_token", "app_secret",
	"webhook_secret", "full_phone", "full_email", "full_address", "full_customer_message",
}

// SanitizeLogFields recursively redacts sensitive keys from log field maps.
func SanitizeLogFields(fields map[string]any) map[string]any {
	if fields == nil {
		return nil
	}
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		if isSensitiveLogKey(k) {
			out[k] = "****"
			continue
		}
		out[k] = safefields.RedactValue(v, sensitiveKeys...)
	}
	return out
}

// SanitizeFieldValue redacts a single field value.
func SanitizeFieldValue(key string, value any) any {
	if isSensitiveLogKey(key) {
		return "****"
	}
	return safefields.RedactValue(value, sensitiveKeys...)
}

func isSensitiveLogKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	for _, s := range sensitiveKeys {
		if k == s || strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// TruncateString limits string field length for logs.
func TruncateString(s string, max int) string {
	if max <= 0 {
		max = 2048
	}
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

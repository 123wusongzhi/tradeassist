package security

import (
	"strings"
	"unicode/utf8"
)

// MaskPhone masks a phone number: 138****5678
func MaskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	runes := []rune(phone)
	if len(runes) <= 4 {
		return "****"
	}
	if len(runes) <= 7 {
		return string(runes[:2]) + "****" + string(runes[len(runes)-2:])
	}
	return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
}

// MaskEmail masks email: e***@example.com
func MaskEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	at := strings.Index(email, "@")
	if at <= 0 {
		return "****"
	}
	local := email[:at]
	domain := email[at:]
	if utf8.RuneCountInString(local) <= 1 {
		return "*" + domain
	}
	runes := []rune(local)
	return string(runes[:1]) + "***" + domain
}

// MaskName masks personal names: 李**
func MaskName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	runes := []rune(name)
	if len(runes) <= 1 {
		return "*"
	}
	return string(runes[:1]) + strings.Repeat("*", len(runes)-1)
}

// MaskAddress returns province/city level summary when possible.
func MaskAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if utf8.RuneCountInString(addr) <= 6 {
		return addr[:min(3, len(addr))] + "****"
	}
	runes := []rune(addr)
	if len(runes) > 12 {
		return string(runes[:6]) + "****"
	}
	return string(runes[:3]) + "****"
}

// MaskIP keeps network prefix summary.
func MaskIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	if strings.Contains(ip, ":") {
		parts := strings.Split(ip, ":")
		if len(parts) >= 2 {
			return parts[0] + ":" + parts[1] + ":***"
		}
	}
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".***.***"
	}
	return "***"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

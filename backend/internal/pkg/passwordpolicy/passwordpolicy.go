// Package passwordpolicy provides the shared password rules for account creation.
package passwordpolicy

import "strings"

const defaultMinimumLength = 8

var commonWeakPasswords = map[string]struct{}{
	"password":  {},
	"12345678":  {},
	"admin123":  {},
	"changeme":  {},
	"trademind": {},
	"admin@123": {},
	"test1234":  {},
	"qwerty123": {},
	"11111111":  {},
}

// IsWeak reports whether password violates the common password policy.
// A non-positive minimum uses the secure default of eight characters.
func IsWeak(password string, minimumLength int) bool {
	return IsWeakWithForbidden(password, minimumLength)
}

// IsWeakWithForbidden applies the shared policy and rejects any exact
// environment-specific secret that must never be reused as an account
// password (for example the production bootstrap administrator password).
func IsWeakWithForbidden(password string, minimumLength int, forbidden ...string) bool {
	if minimumLength <= 0 {
		minimumLength = defaultMinimumLength
	}
	if len(password) < minimumLength {
		return true
	}
	_, weak := commonWeakPasswords[strings.ToLower(strings.TrimSpace(password))]
	if weak {
		return true
	}
	for _, candidate := range forbidden {
		if candidate != "" && password == candidate {
			return true
		}
	}
	return false
}

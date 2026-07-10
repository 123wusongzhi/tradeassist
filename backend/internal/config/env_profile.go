package config

import "strings"

// Known environment profiles (APP_ENV).
const (
	EnvDevelopment = "development"
	EnvDemo        = "demo"
	EnvTest        = "test"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// NormalizeEnv lowercases and trims APP_ENV; empty becomes development.
func NormalizeEnv(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return EnvDevelopment
	}
	return v
}

// IsProduction reports whether the profile requires production hardening.
func IsProduction(env string) bool {
	return NormalizeEnv(env) == EnvProduction
}

// IsStagingOrProduction reports profiles that require HTTPS public URLs.
func IsStagingOrProduction(env string) bool {
	switch NormalizeEnv(env) {
	case EnvStaging, EnvProduction:
		return true
	default:
		return false
	}
}

// AllowsLocalStorage reports whether local storage provider is permitted.
func AllowsLocalStorage(env string) bool {
	switch NormalizeEnv(env) {
	case EnvDevelopment, EnvDemo, EnvTest:
		return true
	default:
		return false
	}
}

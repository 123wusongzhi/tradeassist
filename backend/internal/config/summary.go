package config

import (
	"fmt"
	"strings"
)

// Summary is a redacted startup snapshot safe for logs.
type Summary struct {
	AppEnv               string `json:"appEnv"`
	AppName              string `json:"appName"`
	AppVersion           string `json:"appVersion"`
	HTTPAddr             string `json:"httpAddr"`
	AdminPublicURL       string `json:"adminPublicUrl,omitempty"`
	APIPublicURL         string `json:"apiPublicUrl,omitempty"`
	DBDriver             string `json:"dbDriver"`
	DBHost               string `json:"dbHost"`
	DBPort               int    `json:"dbPort"`
	DBName               string `json:"dbName"`
	DBUser               string `json:"dbUser"`
	RedisAddr            string `json:"redisAddr"`
	JWTSecretConfigured  bool   `json:"jwtSecretConfigured"`
	MasterKeyConfigured  bool   `json:"masterKeyConfigured"`
	EnableSwagger        bool   `json:"enableSwagger"`
	EnableDevRoutes      bool   `json:"enableDevRoutes"`
	EnableDemoSeed       bool   `json:"enableDemoSeed"`
	EnableDebugEndpoints bool   `json:"enableDebugEndpoints"`
	LogLevel             string `json:"logLevel"`
	UploadMaxMB          int    `json:"uploadMaxMb"`
}

// RedactedSummary builds a log-safe config overview.
func (c *Config) RedactedSummary() Summary {
	if c == nil {
		return Summary{}
	}
	return Summary{
		AppEnv:               c.AppEnv,
		AppName:              c.AppName,
		AppVersion:           c.AppVersion,
		HTTPAddr:             c.HTTPAddr,
		AdminPublicURL:       redactURLCredentials(c.AdminPublicURL),
		APIPublicURL:         redactURLCredentials(c.APIPublicURL),
		DBDriver:             c.DB.Driver,
		DBHost:               c.DB.Host,
		DBPort:               c.DB.Port,
		DBName:               c.DB.Name,
		DBUser:               c.DB.User,
		RedisAddr:            c.Redis.Addr,
		JWTSecretConfigured:  strings.TrimSpace(c.JWTSecret) != "" && c.JWTSecret != defaultJWTSecret,
		MasterKeyConfigured:  strings.TrimSpace(c.MasterKey) != "",
		EnableSwagger:        c.EnableSwagger,
		EnableDevRoutes:      c.EnableDevRoutes,
		EnableDemoSeed:       c.EnableDemoSeed,
		EnableDebugEndpoints: c.EnableDebugEndpoints,
		LogLevel:             c.LogLevel,
		UploadMaxMB:          c.UploadMaxMB,
	}
}

func redactURLCredentials(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "@") {
		return "[redacted-url-with-credentials]"
	}
	return raw
}

// String returns a one-line redacted summary for slog.
func (s Summary) String() string {
	return fmt.Sprintf("env=%s addr=%s db=%s@%s:%d/%s redis=%s jwt=%t masterKey=%t devRoutes=%t demoSeed=%t",
		s.AppEnv, s.HTTPAddr, s.DBUser, s.DBHost, s.DBPort, s.DBName, s.RedisAddr,
		s.JWTSecretConfigured, s.MasterKeyConfigured, s.EnableDevRoutes, s.EnableDemoSeed)
}

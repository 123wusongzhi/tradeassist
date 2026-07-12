package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// ObservabilityMode values.
const (
	ObsModeDisabled   = "disabled"
	ObsModeLocal      = "local"
	ObsModePrometheus = "prometheus"
	ObsModeOTel       = "otel"
	ObsModeHybrid     = "hybrid"
)

// ObservabilityConfig holds P5 observability settings.
type ObservabilityConfig struct {
	Enabled                  bool
	Mode                     string
	Environment              string
	LogFormat                string
	LogLevel                 string
	LogIncludeSource         bool
	LogMaxFieldLength        int
	MetricsEnabled           bool
	MetricsPath              string
	MetricsInternalOnly      bool
	TracingEnabled           bool
	OTELServiceName          string
	OTELServiceVersion       string
	OTELExporterOTLPEndpoint string
	OTELExporterOTLPProtocol string
	OTELTraceSampleRatio     float64
	OTELExportTimeoutSeconds int
	AlertingEnabled          bool
	AlertDefaultCooldownSecs int
	AlertRecoveryEnabled     bool
	DBSlowQueryThresholdMS   int
	DBTraceEnabled           bool
}

// ValidProductionObservability returns production-safe observability defaults for tests.
func ValidProductionObservability() ObservabilityConfig {
	return ObservabilityConfig{
		Enabled:              true,
		Mode:                 ObsModeHybrid,
		LogFormat:            "json",
		LogLevel:             "info",
		MetricsEnabled:       true,
		MetricsInternalOnly:  true,
		TracingEnabled:       false,
		OTELTraceSampleRatio: 0.1,
		AlertingEnabled:      true,
	}
}

// LoadObservabilityConfig reads observability env vars.
func LoadObservabilityConfig(appEnv string, appName, appVersion string) ObservabilityConfig {
	env := firstNonEmpty(strings.TrimSpace(os.Getenv("OBSERVABILITY_ENVIRONMENT")), appEnv)
	mode := strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("OBSERVABILITY_MODE"), defaultObsMode(appEnv))))
	enabled := envBool(os.Getenv("OBSERVABILITY_ENABLED"), appEnv != EnvDevelopment || mode != ObsModeDisabled)
	logFormat := strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("LOG_FORMAT"), defaultLogFormat(appEnv))))
	logLevel := firstNonEmpty(os.Getenv("LOG_LEVEL"), defaultLogLevel(appEnv))
	metricsEnabled := envBool(os.Getenv("METRICS_ENABLED"), mode == ObsModePrometheus || mode == ObsModeHybrid || mode == ObsModeLocal)
	tracingEnabled := envBool(os.Getenv("TRACING_ENABLED"), mode == ObsModeOTel || mode == ObsModeHybrid)
	alertingEnabled := envBool(os.Getenv("ALERTING_ENABLED"), true)
	sampleRatio := envFloat(os.Getenv("OTEL_TRACE_SAMPLE_RATIO"), defaultTraceSampleRatio(appEnv))
	cfg := ObservabilityConfig{
		Enabled:                  enabled,
		Mode:                     mode,
		Environment:              env,
		LogFormat:                logFormat,
		LogLevel:                 logLevel,
		LogIncludeSource:         envBool(os.Getenv("LOG_INCLUDE_SOURCE"), false),
		LogMaxFieldLength:        atoiOrDefault(os.Getenv("LOG_MAX_FIELD_LENGTH"), 2048),
		MetricsEnabled:           metricsEnabled,
		MetricsPath:              firstNonEmpty(os.Getenv("METRICS_PATH"), "/internal/metrics"),
		MetricsInternalOnly:      envBool(os.Getenv("METRICS_INTERNAL_ONLY"), appEnv == EnvProduction || appEnv == EnvStaging),
		TracingEnabled:           tracingEnabled,
		OTELServiceName:          firstNonEmpty(os.Getenv("OTEL_SERVICE_NAME"), firstNonEmpty(appName, "trademind-api")),
		OTELServiceVersion:       firstNonEmpty(os.Getenv("OTEL_SERVICE_VERSION"), appVersion),
		OTELExporterOTLPEndpoint: strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
		OTELExporterOTLPProtocol: firstNonEmpty(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"), "http/protobuf"),
		OTELTraceSampleRatio:     sampleRatio,
		OTELExportTimeoutSeconds: atoiOrDefault(os.Getenv("OTEL_EXPORT_TIMEOUT_SECONDS"), 10),
		AlertingEnabled:          alertingEnabled,
		AlertDefaultCooldownSecs: atoiOrDefault(os.Getenv("ALERT_DEFAULT_COOLDOWN_SECONDS"), 300),
		AlertRecoveryEnabled:     envBool(os.Getenv("ALERT_RECOVERY_ENABLED"), true),
		DBSlowQueryThresholdMS:   atoiOrDefault(os.Getenv("DB_SLOW_QUERY_THRESHOLD_MS"), 500),
		DBTraceEnabled:           envBool(os.Getenv("DB_TRACE_ENABLED"), tracingEnabled),
	}
	return cfg
}

func defaultObsMode(appEnv string) string {
	if IsProduction(appEnv) || appEnv == EnvStaging {
		return ObsModeHybrid
	}
	return ObsModeLocal
}

func defaultLogFormat(appEnv string) string {
	if IsProduction(appEnv) || appEnv == EnvStaging {
		return "json"
	}
	return "console"
}

func defaultTraceSampleRatio(appEnv string) float64 {
	if IsProduction(appEnv) {
		return 0.1
	}
	if appEnv == EnvStaging {
		return 0.25
	}
	return 0.0
}

func envFloat(raw string, def float64) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	v, err := parseFloat(raw)
	if err != nil {
		return def
	}
	return v
}

func parseFloat(s string) (float64, error) {
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}

// ValidateObservability enforces production observability rules.
func (c *Config) ValidateObservability() error {
	if c == nil {
		return nil
	}
	obs := c.Observability
	if !IsProduction(c.AppEnv) {
		return nil
	}
	if !obs.Enabled || obs.Mode == ObsModeDisabled {
		return fmt.Errorf("OBSERVABILITY_MODE=disabled is forbidden in production")
	}
	if obs.LogFormat == "console" || obs.LogFormat == "text" {
		return fmt.Errorf("LOG_FORMAT=console is forbidden in production")
	}
	if strings.EqualFold(obs.LogLevel, "debug") {
		return fmt.Errorf("LOG_LEVEL=debug is forbidden in production unless explicitly approved")
	}
	if !obs.MetricsEnabled {
		return fmt.Errorf("METRICS_ENABLED must be true in production")
	}
	if !obs.MetricsInternalOnly {
		return fmt.Errorf("METRICS_INTERNAL_ONLY=false is forbidden in production")
	}
	if obs.OTELTraceSampleRatio > 0.5 {
		return fmt.Errorf("OTEL_TRACE_SAMPLE_RATIO exceeds production safe upper bound")
	}
	return nil
}

// ObservabilityExportTimeout returns OTLP export timeout.
func (o ObservabilityConfig) ExportTimeout() time.Duration {
	if o.OTELExportTimeoutSeconds <= 0 {
		return 10 * time.Second
	}
	return time.Duration(o.OTELExportTimeoutSeconds) * time.Second
}

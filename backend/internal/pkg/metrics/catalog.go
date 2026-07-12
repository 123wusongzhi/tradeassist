package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var defaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120}

// Catalog holds pre-registered application metrics.
type Catalog struct {
	reg *Registry

	HTTPRequestsTotal         *prometheus.CounterVec
	HTTPRequestDuration       *prometheus.HistogramVec
	HTTPRequestsInFlight      prometheus.Gauge
	HTTPPanicsTotal           prometheus.Counter
	ProviderRequestsTotal     *prometheus.CounterVec
	ProviderRequestDuration   *prometheus.HistogramVec
	ProviderTimeoutsTotal     *prometheus.CounterVec
	ProviderRateLimitedTotal  *prometheus.CounterVec
	TasksCompletedTotal       *prometheus.CounterVec
	TasksFailedTotal          *prometheus.CounterVec
	TasksRetriedTotal         *prometheus.CounterVec
	TasksDeadLetterTotal      *prometheus.CounterVec
	TaskDurationSeconds       *prometheus.HistogramVec
	WebhookRequestsTotal      *prometheus.CounterVec
	OrderSyncRunsTotal        *prometheus.CounterVec
	InventoryAdjustmentsTotal *prometheus.CounterVec
	AITextRequestsTotal       *prometheus.CounterVec
	AIImageRequestsTotal      *prometheus.CounterVec
	AIImageProviderTimeouts   *prometheus.CounterVec
	FileScanTasksTotal        *prometheus.CounterVec
	SecretRotationJobsTotal   *prometheus.CounterVec
	AuthLoginAttemptsTotal    *prometheus.CounterVec
	AuthRefreshReuseTotal     prometheus.Counter
	SecurityEventsTotal       *prometheus.CounterVec
	TelemetryExportFailures   prometheus.Counter
	TelemetryDroppedItems     prometheus.Counter
	SLOComplianceRatio        *prometheus.GaugeVec

	once sync.Once
	err  error
}

// RegisterCatalog registers all standard metrics on the registry.
func RegisterCatalog(reg *Registry) (*Catalog, error) {
	c := &Catalog{reg: reg}
	c.once.Do(func() {
		c.err = c.registerAll()
	})
	return c, c.err
}

func (c *Catalog) registerAll() error {
	var err error
	c.HTTPRequestsTotal, err = c.reg.Counter(
		"http_server_requests_total", "HTTP server requests",
		"method", "route_template", "status_class", "result")
	if err != nil {
		return err
	}
	c.HTTPRequestDuration, err = c.reg.Histogram(
		"http_server_request_duration_seconds", "HTTP request duration",
		defaultBuckets, "method", "route_template", "status_class", "result")
	if err != nil {
		return err
	}
	c.HTTPRequestsInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_server_requests_in_flight",
		Help: "In-flight HTTP requests",
	})
	c.reg.prom.MustRegister(c.HTTPRequestsInFlight)
	c.HTTPPanicsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "http_server_panics_total",
		Help: "HTTP handler panics",
	})
	c.reg.prom.MustRegister(c.HTTPPanicsTotal)

	c.ProviderRequestsTotal, err = c.reg.Counter(
		"provider_requests_total", "Provider HTTP requests",
		"provider", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.ProviderRequestDuration, err = c.reg.Histogram(
		"provider_request_duration_seconds", "Provider request duration",
		defaultBuckets, "provider", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.ProviderTimeoutsTotal, err = c.reg.Counter(
		"provider_request_timeouts_total", "Provider timeouts",
		"provider", "operation", "error_class")
	if err != nil {
		return err
	}
	c.ProviderRateLimitedTotal, err = c.reg.Counter(
		"provider_rate_limited_total", "Provider rate limits",
		"provider", "operation")
	if err != nil {
		return err
	}

	c.TasksCompletedTotal, err = c.reg.Counter(
		"tasks_completed_total", "Tasks completed", "task_type", "result", "error_class")
	if err != nil {
		return err
	}
	c.TasksFailedTotal, err = c.reg.Counter(
		"tasks_failed_total", "Tasks failed", "task_type", "result", "error_class")
	if err != nil {
		return err
	}
	c.TasksRetriedTotal, err = c.reg.Counter(
		"tasks_retried_total", "Tasks retried", "task_type", "result", "error_class")
	if err != nil {
		return err
	}
	c.TasksDeadLetterTotal, err = c.reg.Counter(
		"tasks_dead_letter_total", "Tasks dead lettered", "task_type", "result", "error_class")
	if err != nil {
		return err
	}
	c.TaskDurationSeconds, err = c.reg.Histogram(
		"task_duration_seconds", "Task processing duration",
		defaultBuckets, "task_type", "result", "error_class")
	if err != nil {
		return err
	}

	c.WebhookRequestsTotal, err = c.reg.Counter(
		"webhook_requests_total", "Webhook requests",
		"platform", "event_group", "result", "error_class")
	if err != nil {
		return err
	}
	c.OrderSyncRunsTotal, err = c.reg.Counter(
		"order_sync_runs_total", "Order sync runs",
		"platform", "source", "result")
	if err != nil {
		return err
	}
	c.InventoryAdjustmentsTotal, err = c.reg.Counter(
		"inventory_adjustments_total", "Inventory adjustments",
		"platform", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.AITextRequestsTotal, err = c.reg.Counter(
		"ai_text_requests_total", "AI text requests",
		"provider", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.AIImageRequestsTotal, err = c.reg.Counter(
		"ai_image_requests_total", "AI image requests",
		"provider", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.AIImageProviderTimeouts, err = c.reg.Counter(
		"ai_image_provider_timeouts_total", "AI image provider timeouts (P5-OBS-001)",
		"provider", "operation", "error_class")
	if err != nil {
		return err
	}
	c.FileScanTasksTotal, err = c.reg.Counter(
		"file_scan_tasks_total", "File scan tasks",
		"scanner", "result", "mime_group")
	if err != nil {
		return err
	}
	c.SecretRotationJobsTotal, err = c.reg.Counter(
		"secret_rotation_jobs_total", "Secret rotation jobs",
		"target", "result", "status")
	if err != nil {
		return err
	}
	c.AuthLoginAttemptsTotal, err = c.reg.Counter(
		"auth_login_attempts_total", "Auth login attempts",
		"result", "reason", "auth_mode")
	if err != nil {
		return err
	}
	c.AuthRefreshReuseTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "auth_refresh_reuse_detected_total",
		Help: "Refresh token reuse detected",
	})
	c.reg.prom.MustRegister(c.AuthRefreshReuseTotal)
	c.SecurityEventsTotal, err = c.reg.Counter(
		"security_events_total", "Security events",
		"event_type", "result", "severity", "module")
	if err != nil {
		return err
	}
	c.TelemetryExportFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_export_failures_total",
		Help: "Telemetry export failures",
	})
	c.TelemetryDroppedItems = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_dropped_items_total",
		Help: "Telemetry dropped items",
	})
	c.reg.prom.MustRegister(c.TelemetryExportFailures, c.TelemetryDroppedItems)
	c.SLOComplianceRatio, err = c.reg.Gauge(
		"slo_compliance_ratio", "SLO compliance ratio", "slo_id", "window")
	return err
}

// ObserveHTTP records HTTP server metrics.
func (c *Catalog) ObserveHTTP(method, route string, status int, result string, dur time.Duration) {
	if c == nil || c.HTTPRequestsTotal == nil {
		return
	}
	sc := StatusClass(status)
	res := NormalizeResult(result)
	c.HTTPRequestsTotal.WithLabelValues(method, route, sc, res).Inc()
	c.HTTPRequestDuration.WithLabelValues(method, route, sc, res).Observe(dur.Seconds())
}

// ObserveProvider records provider call metrics.
func (c *Catalog) ObserveProvider(provider, operation, result, errorClass string, dur time.Duration, timeout bool) {
	if c == nil {
		return
	}
	res := NormalizeResult(result)
	ec := NormalizeResult(errorClass)
	if c.ProviderRequestsTotal != nil {
		c.ProviderRequestsTotal.WithLabelValues(provider, operation, res, ec).Inc()
		c.ProviderRequestDuration.WithLabelValues(provider, operation, res, ec).Observe(dur.Seconds())
	}
	if timeout && c.ProviderTimeoutsTotal != nil {
		c.ProviderTimeoutsTotal.WithLabelValues(provider, operation, ec).Inc()
	}
}

// ObserveTask records task worker metrics.
func (c *Catalog) ObserveTask(taskType, result, errorClass string, dur time.Duration) {
	if c == nil {
		return
	}
	res := NormalizeResult(result)
	ec := NormalizeResult(errorClass)
	switch res {
	case "success":
		c.TasksCompletedTotal.WithLabelValues(taskType, res, ec).Inc()
	case "failure", "timeout":
		c.TasksFailedTotal.WithLabelValues(taskType, res, ec).Inc()
	default:
		c.TasksCompletedTotal.WithLabelValues(taskType, res, ec).Inc()
	}
	if dur > 0 && c.TaskDurationSeconds != nil {
		c.TaskDurationSeconds.WithLabelValues(taskType, res, ec).Observe(dur.Seconds())
	}
}

// ObserveAIImageProviderTimeout records P5-OBS-001 metric.
func (c *Catalog) ObserveAIImageProviderTimeout(provider, operation, errorClass string) {
	if c == nil || c.AIImageProviderTimeouts == nil {
		return
	}
	c.AIImageProviderTimeouts.WithLabelValues(provider, operation, NormalizeResult(errorClass)).Inc()
	if c.AIImageRequestsTotal != nil {
		c.AIImageRequestsTotal.WithLabelValues(provider, operation, "timeout", "provider_timeout").Inc()
	}
}

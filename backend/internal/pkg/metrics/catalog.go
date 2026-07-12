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
	ProviderRetriesTotal      *prometheus.CounterVec
	ProviderTimeoutsTotal     *prometheus.CounterVec
	ProviderRateLimitedTotal  *prometheus.CounterVec
	ProviderUnknownResults    *prometheus.CounterVec
	ProviderContractMismatch  *prometheus.CounterVec
	ProviderCircuitState      *prometheus.GaugeVec
	ProviderCircuitChanges    *prometheus.CounterVec
	TasksCreatedTotal         *prometheus.CounterVec
	TasksClaimedTotal         *prometheus.CounterVec
	TasksCompletedTotal       *prometheus.CounterVec
	TasksFailedTotal          *prometheus.CounterVec
	TasksRetriedTotal         *prometheus.CounterVec
	TasksDeadLetterTotal      *prometheus.CounterVec
	TasksManualReviewTotal    *prometheus.CounterVec
	TasksInProgress           *prometheus.GaugeVec
	TaskDurationSeconds       *prometheus.HistogramVec
	TaskQueueAgeSeconds       *prometheus.HistogramVec
	TaskLeaseLostTotal        *prometheus.CounterVec
	TaskHeartbeatMissedTotal  *prometheus.CounterVec
	TaskReaperRecoveredTotal  *prometheus.CounterVec
	TaskUnknownResultTotal    *prometheus.CounterVec
	WebhookRequestsTotal      *prometheus.CounterVec
	WebhookSignatureFailures  *prometheus.CounterVec
	WebhookReplayRejected     *prometheus.CounterVec
	WebhookPayloadRejected    *prometheus.CounterVec
	WebhookEventsPersisted    *prometheus.CounterVec
	WebhookEventsProcessed    *prometheus.CounterVec
	WebhookProcessingDuration *prometheus.HistogramVec
	WebhookProcessingLag      *prometheus.HistogramVec
	WebhookUnknownEvents      *prometheus.CounterVec
	WebhookShopResolutionFail *prometheus.CounterVec
	WebhookTenantMismatch     *prometheus.CounterVec
	WebhookDuplicateEvents    *prometheus.CounterVec
	OrderSyncRunsTotal        *prometheus.CounterVec
	OrderSyncFailuresTotal    *prometheus.CounterVec
	InventoryAdjustmentsTotal *prometheus.CounterVec
	InventoryUnknownResults   *prometheus.CounterVec
	AITextRequestsTotal       *prometheus.CounterVec
	AITextProviderTimeouts    *prometheus.CounterVec
	AITextEnvironmentBlocked  *prometheus.CounterVec
	AIImageRequestsTotal      *prometheus.CounterVec
	AIImageProviderTimeouts   *prometheus.CounterVec
	AIImageEnvironmentBlocked *prometheus.CounterVec
	AIImageTaskStageDuration  *prometheus.HistogramVec
	AIImageTaskStuckTotal     *prometheus.CounterVec
	FileScanTasksTotal        *prometheus.CounterVec
	FileScanDurationSeconds   *prometheus.HistogramVec
	FileScanQueueAgeSeconds   *prometheus.HistogramVec
	FileScanFailuresTotal     *prometheus.CounterVec
	FileScanStuckTotal        *prometheus.CounterVec
	SecretRotationJobsTotal   *prometheus.CounterVec
	SecretRotationFailures    *prometheus.CounterVec
	AuthLoginAttemptsTotal    *prometheus.CounterVec
	AuthRefreshReuseTotal     prometheus.Counter
	TenantAccessDeniedTotal   *prometheus.CounterVec
	AuditChainMismatchTotal   prometheus.Counter
	SecurityEventsTotal       *prometheus.CounterVec
	TelemetryExportFailures   prometheus.Counter
	TelemetryDroppedItems     prometheus.Counter
	TelemetryExportSuccess    prometheus.Counter
	TelemetryQueueDepth       prometheus.Gauge
	SLOComplianceRatio        *prometheus.GaugeVec
	SLOErrorBudgetRemaining   *prometheus.GaugeVec
	SLOBurnRate               *prometheus.GaugeVec
	DBConnectionsOpen         *prometheus.GaugeVec
	DBConnectionsInUse        *prometheus.GaugeVec
	DBConnectionsIdle         *prometheus.GaugeVec
	DBMaxOpenConnections      *prometheus.GaugeVec
	DBConnectionWaitCount     *prometheus.CounterVec
	DBConnectionWaitDuration  *prometheus.CounterVec
	DBQueryDuration           *prometheus.HistogramVec
	DBQueryErrors             *prometheus.CounterVec
	DBTransactionDuration     *prometheus.HistogramVec
	DBTransactionRollbacks    *prometheus.CounterVec

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
	c.ProviderRetriesTotal, err = c.reg.Counter(
		"provider_request_retries_total", "Provider physical retry attempts",
		"provider", "operation", "result", "error_class")
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
	c.ProviderUnknownResults, err = c.reg.Counter(
		"provider_unknown_results_total", "Provider write requests with unknown result",
		"provider", "operation", "error_class")
	if err != nil {
		return err
	}
	c.ProviderContractMismatch, err = c.reg.Counter(
		"provider_contract_mismatches_total", "Provider contract mismatches",
		"provider", "operation", "error_class")
	if err != nil {
		return err
	}
	c.ProviderCircuitState, err = c.reg.Gauge(
		"provider_circuit_breaker_state", "Provider circuit breaker state",
		"provider", "operation", "state")
	if err != nil {
		return err
	}
	c.ProviderCircuitChanges, err = c.reg.Counter(
		"provider_circuit_breaker_transitions_total", "Provider circuit breaker state transitions",
		"provider", "operation", "from_state", "to_state")
	if err != nil {
		return err
	}

	c.TasksCreatedTotal, err = c.reg.Counter(
		"tasks_created_total", "Tasks created", "task_type", "result", "error_class")
	if err != nil {
		return err
	}
	c.TasksClaimedTotal, err = c.reg.Counter(
		"tasks_claimed_total", "Tasks claimed", "task_type", "result", "error_class")
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
	c.TasksManualReviewTotal, err = c.reg.Counter(
		"tasks_manual_review_total", "Tasks moved to manual review", "task_type", "result", "error_class")
	if err != nil {
		return err
	}
	c.TasksInProgress, err = c.reg.Gauge(
		"tasks_in_progress", "Tasks currently in progress", "task_type")
	if err != nil {
		return err
	}
	c.TaskDurationSeconds, err = c.reg.Histogram(
		"task_duration_seconds", "Task processing duration",
		defaultBuckets, "task_type", "result", "error_class")
	if err != nil {
		return err
	}
	c.TaskQueueAgeSeconds, err = c.reg.Histogram(
		"task_queue_age_seconds", "Task queue age on claim",
		defaultBuckets, "task_type")
	if err != nil {
		return err
	}
	c.TaskLeaseLostTotal, err = c.reg.Counter(
		"task_lease_lost_total", "Task leases lost", "task_type", "result", "error_class")
	if err != nil {
		return err
	}
	c.TaskHeartbeatMissedTotal, err = c.reg.Counter(
		"task_heartbeat_missed_total", "Task heartbeats missed", "task_type", "result", "error_class")
	if err != nil {
		return err
	}
	c.TaskReaperRecoveredTotal, err = c.reg.Counter(
		"task_reaper_recovered_total", "Tasks recovered by reaper", "task_type", "result", "error_class")
	if err != nil {
		return err
	}
	c.TaskUnknownResultTotal, err = c.reg.Counter(
		"task_unknown_result_total", "Tasks ending with unknown result", "task_type", "result", "error_class")
	if err != nil {
		return err
	}

	c.WebhookRequestsTotal, err = c.reg.Counter(
		"webhook_requests_total", "Webhook requests",
		"platform", "event_group", "result", "error_class")
	if err != nil {
		return err
	}
	c.WebhookSignatureFailures, err = c.reg.Counter("webhook_signature_failures_total", "Webhook signature failures", "platform", "event_group", "error_class")
	if err != nil {
		return err
	}
	c.WebhookReplayRejected, err = c.reg.Counter("webhook_replay_rejected_total", "Webhook replay rejections", "platform", "event_group", "error_class")
	if err != nil {
		return err
	}
	c.WebhookPayloadRejected, err = c.reg.Counter("webhook_payload_rejected_total", "Webhook payload rejections", "platform", "event_group", "error_class")
	if err != nil {
		return err
	}
	c.WebhookEventsPersisted, err = c.reg.Counter("webhook_events_persisted_total", "Webhook events persisted", "platform", "event_group", "result")
	if err != nil {
		return err
	}
	c.WebhookEventsProcessed, err = c.reg.Counter("webhook_events_processed_total", "Webhook events processed", "platform", "event_group", "result", "error_class")
	if err != nil {
		return err
	}
	c.WebhookProcessingDuration, err = c.reg.Histogram("webhook_processing_duration_seconds", "Webhook worker processing duration", defaultBuckets, "platform", "event_group", "result", "error_class")
	if err != nil {
		return err
	}
	c.WebhookProcessingLag, err = c.reg.Histogram("webhook_processing_lag_seconds", "Webhook processing lag", defaultBuckets, "platform", "event_group")
	if err != nil {
		return err
	}
	c.WebhookUnknownEvents, err = c.reg.Counter("webhook_unknown_events_total", "Webhook unknown events", "platform", "event_group")
	if err != nil {
		return err
	}
	c.WebhookShopResolutionFail, err = c.reg.Counter("webhook_shop_resolution_failures_total", "Webhook shop resolution failures", "platform", "event_group", "error_class")
	if err != nil {
		return err
	}
	c.WebhookTenantMismatch, err = c.reg.Counter("webhook_tenant_mismatch_total", "Webhook tenant mismatches", "platform", "event_group")
	if err != nil {
		return err
	}
	c.WebhookDuplicateEvents, err = c.reg.Counter("webhook_duplicate_events_total", "Webhook duplicate events", "platform", "event_group")
	if err != nil {
		return err
	}
	c.OrderSyncRunsTotal, err = c.reg.Counter(
		"order_sync_runs_total", "Order sync runs",
		"platform", "source", "result")
	if err != nil {
		return err
	}
	c.OrderSyncFailuresTotal, err = c.reg.Counter(
		"order_sync_failures_total", "Order sync failures",
		"platform", "source", "error_class")
	if err != nil {
		return err
	}
	c.InventoryAdjustmentsTotal, err = c.reg.Counter(
		"inventory_adjustments_total", "Inventory adjustments",
		"platform", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.InventoryUnknownResults, err = c.reg.Counter(
		"inventory_unknown_results_total", "Inventory write requests with unknown result",
		"platform", "operation", "error_class")
	if err != nil {
		return err
	}
	c.AITextRequestsTotal, err = c.reg.Counter(
		"ai_text_requests_total", "AI text requests",
		"provider", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.AITextProviderTimeouts, err = c.reg.Counter(
		"ai_text_provider_timeouts_total", "AI text provider timeouts",
		"provider", "operation", "error_class")
	if err != nil {
		return err
	}
	c.AITextEnvironmentBlocked, err = c.reg.Counter(
		"ai_text_environment_blocked_total", "AI text environment blocked",
		"provider", "operation", "error_class")
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
	c.AIImageEnvironmentBlocked, err = c.reg.Counter(
		"ai_image_environment_blocked_total", "AI image environment blocked",
		"provider", "operation", "error_class")
	if err != nil {
		return err
	}
	c.AIImageTaskStageDuration, err = c.reg.Histogram(
		"ai_image_task_stage_duration_seconds", "AI image task stage duration",
		defaultBuckets, "stage", "result")
	if err != nil {
		return err
	}
	c.AIImageTaskStuckTotal, err = c.reg.Counter(
		"ai_image_task_stuck_total", "AI image stuck tasks", "stage", "result")
	if err != nil {
		return err
	}
	c.FileScanTasksTotal, err = c.reg.Counter(
		"file_scan_tasks_total", "File scan tasks",
		"scanner", "result", "mime_group")
	if err != nil {
		return err
	}
	c.FileScanDurationSeconds, err = c.reg.Histogram("file_scan_duration_seconds", "File scan duration", defaultBuckets, "scanner", "result", "mime_group")
	if err != nil {
		return err
	}
	c.FileScanQueueAgeSeconds, err = c.reg.Histogram("file_scan_queue_age_seconds", "File scan queue age", defaultBuckets, "scanner", "mime_group")
	if err != nil {
		return err
	}
	c.FileScanFailuresTotal, err = c.reg.Counter("file_scan_failures_total", "File scan failures", "scanner", "result", "mime_group")
	if err != nil {
		return err
	}
	c.FileScanStuckTotal, err = c.reg.Counter("file_scan_stuck_total", "File scan stuck tasks", "scanner", "mime_group")
	if err != nil {
		return err
	}
	c.SecretRotationJobsTotal, err = c.reg.Counter(
		"secret_rotation_jobs_total", "Secret rotation jobs",
		"target", "result", "status")
	if err != nil {
		return err
	}
	c.SecretRotationFailures, err = c.reg.Counter("secret_rotation_failures_total", "Secret rotation failures", "target", "result", "status")
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
	c.AuditChainMismatchTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "audit_chain_mismatch_total",
		Help: "Audit chain mismatch events",
	})
	c.reg.prom.MustRegister(c.AuthRefreshReuseTotal, c.AuditChainMismatchTotal)
	c.TenantAccessDeniedTotal, err = c.reg.Counter(
		"tenant_access_denied_total", "Tenant access denied events",
		"module", "result", "severity")
	if err != nil {
		return err
	}
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
	c.TelemetryExportSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_export_success_total",
		Help: "Telemetry export successes",
	})
	c.TelemetryQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "telemetry_queue_depth",
		Help: "Telemetry export queue depth",
	})
	c.reg.prom.MustRegister(c.TelemetryExportFailures, c.TelemetryDroppedItems, c.TelemetryExportSuccess, c.TelemetryQueueDepth)
	c.SLOComplianceRatio, err = c.reg.Gauge(
		"slo_compliance_ratio", "SLO compliance ratio", "slo_id", "window")
	if err != nil {
		return err
	}
	c.SLOErrorBudgetRemaining, err = c.reg.Gauge(
		"slo_error_budget_remaining_ratio", "SLO error budget remaining ratio", "slo_id", "window")
	if err != nil {
		return err
	}
	c.SLOBurnRate, err = c.reg.Gauge(
		"slo_burn_rate", "SLO burn rate", "slo_id", "window")
	if err != nil {
		return err
	}
	c.DBConnectionsOpen, err = c.reg.Gauge("db_connections_open", "Open database connections", "db_role")
	if err != nil {
		return err
	}
	c.DBConnectionsInUse, err = c.reg.Gauge("db_connections_in_use", "Database connections in use", "db_role")
	if err != nil {
		return err
	}
	c.DBConnectionsIdle, err = c.reg.Gauge("db_connections_idle", "Idle database connections", "db_role")
	if err != nil {
		return err
	}
	c.DBMaxOpenConnections, err = c.reg.Gauge("db_max_open_connections", "Max open database connections", "db_role")
	if err != nil {
		return err
	}
	c.DBConnectionWaitCount, err = c.reg.Counter("db_connection_wait_count_total", "Database connection wait count", "db_role")
	if err != nil {
		return err
	}
	c.DBConnectionWaitDuration, err = c.reg.Counter("db_connection_wait_duration_seconds", "Database connection wait duration", "db_role")
	if err != nil {
		return err
	}
	c.DBQueryDuration, err = c.reg.Histogram("db_query_duration_seconds", "Database query duration", defaultBuckets, "operation", "table_group", "result", "driver")
	if err != nil {
		return err
	}
	c.DBQueryErrors, err = c.reg.Counter("db_query_errors_total", "Database query errors", "operation", "table_group", "result", "driver")
	if err != nil {
		return err
	}
	c.DBTransactionDuration, err = c.reg.Histogram("db_transaction_duration_seconds", "Database transaction duration", defaultBuckets, "operation", "table_group", "result", "driver")
	if err != nil {
		return err
	}
	c.DBTransactionRollbacks, err = c.reg.Counter("db_transaction_rollbacks_total", "Database transaction rollbacks", "operation", "table_group", "result", "driver")
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
	if res == "rate_limited" && c.ProviderRateLimitedTotal != nil {
		c.ProviderRateLimitedTotal.WithLabelValues(provider, operation).Inc()
	}
	if res == "unknown" && c.ProviderUnknownResults != nil {
		c.ProviderUnknownResults.WithLabelValues(provider, operation, ec).Inc()
	}
}

// ObserveProviderRetry records one physical provider retry attempt.
func (c *Catalog) ObserveProviderRetry(provider, operation, result, errorClass string) {
	if c == nil || c.ProviderRetriesTotal == nil {
		return
	}
	c.ProviderRetriesTotal.WithLabelValues(provider, operation, NormalizeResult(result), NormalizeResult(errorClass)).Inc()
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

// ObserveTaskLifecycle records non-terminal task lifecycle events.
func (c *Catalog) ObserveTaskLifecycle(taskType, event, result, errorClass string, queueAge time.Duration) {
	if c == nil {
		return
	}
	res := NormalizeResult(result)
	ec := NormalizeResult(errorClass)
	switch event {
	case "created":
		c.TasksCreatedTotal.WithLabelValues(taskType, res, ec).Inc()
	case "claimed":
		c.TasksClaimedTotal.WithLabelValues(taskType, res, ec).Inc()
		if c.TasksInProgress != nil {
			c.TasksInProgress.WithLabelValues(taskType).Inc()
		}
		if queueAge > 0 && c.TaskQueueAgeSeconds != nil {
			c.TaskQueueAgeSeconds.WithLabelValues(taskType).Observe(queueAge.Seconds())
		}
	case "manual_review":
		c.TasksManualReviewTotal.WithLabelValues(taskType, res, ec).Inc()
	case "lease_lost":
		c.TaskLeaseLostTotal.WithLabelValues(taskType, res, ec).Inc()
	case "heartbeat_missed":
		c.TaskHeartbeatMissedTotal.WithLabelValues(taskType, res, ec).Inc()
	case "reaper_recovered":
		c.TaskReaperRecoveredTotal.WithLabelValues(taskType, res, ec).Inc()
	case "unknown_result":
		c.TaskUnknownResultTotal.WithLabelValues(taskType, res, ec).Inc()
	}
}

// ObserveWebhookProcessed records webhook worker processing metrics.
func (c *Catalog) ObserveWebhookProcessed(platform, eventGroup, result, errorClass string, processing, lag time.Duration) {
	if c == nil || c.WebhookEventsProcessed == nil {
		return
	}
	res := NormalizeResult(result)
	ec := NormalizeResult(errorClass)
	c.WebhookEventsProcessed.WithLabelValues(platform, eventGroup, res, ec).Inc()
	if processing > 0 && c.WebhookProcessingDuration != nil {
		c.WebhookProcessingDuration.WithLabelValues(platform, eventGroup, res, ec).Observe(processing.Seconds())
	}
	if lag > 0 && c.WebhookProcessingLag != nil {
		c.WebhookProcessingLag.WithLabelValues(platform, eventGroup).Observe(lag.Seconds())
	}
}

// ObserveSLO records SLO derived gauges.
func (c *Catalog) ObserveSLO(sloID, window string, compliance, budgetRemaining, burnRate float64) {
	if c == nil {
		return
	}
	if c.SLOComplianceRatio != nil {
		c.SLOComplianceRatio.WithLabelValues(sloID, window).Set(compliance)
	}
	if c.SLOErrorBudgetRemaining != nil {
		c.SLOErrorBudgetRemaining.WithLabelValues(sloID, window).Set(budgetRemaining)
	}
	if c.SLOBurnRate != nil {
		c.SLOBurnRate.WithLabelValues(sloID, window).Set(burnRate)
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

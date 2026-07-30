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

	HTTPRequestsTotal          *prometheus.CounterVec
	HTTPRequestDuration        *prometheus.HistogramVec
	HTTPRequestsInFlight       prometheus.Gauge
	HTTPPanicsTotal            prometheus.Counter
	ProviderRequestsTotal      *prometheus.CounterVec
	ProviderRequestDuration    *prometheus.HistogramVec
	ProviderRetriesTotal       *prometheus.CounterVec
	ProviderTimeoutsTotal      *prometheus.CounterVec
	ProviderRateLimitedTotal   *prometheus.CounterVec
	ProviderUnknownResults     *prometheus.CounterVec
	ProviderContractMismatch   *prometheus.CounterVec
	ProviderCircuitState       *prometheus.GaugeVec
	ProviderCircuitChanges     *prometheus.CounterVec
	TasksCreatedTotal          *prometheus.CounterVec
	TasksClaimedTotal          *prometheus.CounterVec
	TasksCompletedTotal        *prometheus.CounterVec
	TasksFailedTotal           *prometheus.CounterVec
	TasksRetriedTotal          *prometheus.CounterVec
	TasksDeadLetterTotal       *prometheus.CounterVec
	TasksManualReviewTotal     *prometheus.CounterVec
	TasksInProgress            *prometheus.GaugeVec
	TaskDurationSeconds        *prometheus.HistogramVec
	TaskQueueAgeSeconds        *prometheus.HistogramVec
	TaskLeaseLostTotal         *prometheus.CounterVec
	TaskHeartbeatMissedTotal   *prometheus.CounterVec
	TaskReaperRecoveredTotal   *prometheus.CounterVec
	TaskUnknownResultTotal     *prometheus.CounterVec
	WebhookRequestsTotal       *prometheus.CounterVec
	WebhookSignatureFailures   *prometheus.CounterVec
	WebhookReplayRejected      *prometheus.CounterVec
	WebhookPayloadRejected     *prometheus.CounterVec
	WebhookEventsPersisted     *prometheus.CounterVec
	WebhookEventsProcessed     *prometheus.CounterVec
	WebhookProcessingDuration  *prometheus.HistogramVec
	WebhookProcessingLag       *prometheus.HistogramVec
	WebhookUnknownEvents       *prometheus.CounterVec
	WebhookShopResolutionFail  *prometheus.CounterVec
	WebhookTenantMismatch      *prometheus.CounterVec
	WebhookDuplicateEvents     *prometheus.CounterVec
	OrderSyncRunsTotal         *prometheus.CounterVec
	OrderSyncOrdersReceived    *prometheus.CounterVec
	OrderSyncOrdersCreated     *prometheus.CounterVec
	OrderSyncOrdersUpdated     *prometheus.CounterVec
	OrderSyncStaleUpdates      *prometheus.CounterVec
	OrderSyncDuplicates        *prometheus.CounterVec
	OrderSyncFailuresTotal     *prometheus.CounterVec
	OrderSyncDuration          *prometheus.HistogramVec
	OrderSyncCursorLag         *prometheus.HistogramVec
	OrderSyncLastSuccess       *prometheus.GaugeVec
	InventoryAdjustmentsTotal  *prometheus.CounterVec
	InventoryDeductionsTotal   *prometheus.CounterVec
	InventoryCompensations     *prometheus.CounterVec
	InventoryPushTotal         *prometheus.CounterVec
	InventoryPushFailures      *prometheus.CounterVec
	InventoryUnknownResults    *prometheus.CounterVec
	InventoryVersionConflicts  *prometheus.CounterVec
	InventoryNegativePrevent   *prometheus.CounterVec
	InventorySyncDuration      *prometheus.HistogramVec
	AITextRequestsTotal        *prometheus.CounterVec
	AITextRequestDuration      *prometheus.HistogramVec
	AITextProviderTimeouts     *prometheus.CounterVec
	AITextProviderFailures     *prometheus.CounterVec
	AITextEnvironmentBlocked   *prometheus.CounterVec
	AITextBatchesTotal         *prometheus.CounterVec
	AITextBatchDuration        *prometheus.HistogramVec
	AITextApplyTotal           *prometheus.CounterVec
	AITextApplyConflicts       *prometheus.CounterVec
	AITextReconciliation       *prometheus.CounterVec
	AIImageRequestsTotal       *prometheus.CounterVec
	AIImageRequestDuration     *prometheus.HistogramVec
	AIImageProviderTimeouts    *prometheus.CounterVec
	AIImageProviderFailures    *prometheus.CounterVec
	AIImageEnvironmentBlocked  *prometheus.CounterVec
	AIImageBatchesTotal        *prometheus.CounterVec
	AIImageBatchDuration       *prometheus.HistogramVec
	AIImageTaskStageDuration   *prometheus.HistogramVec
	AIImageTaskStuckTotal      *prometheus.CounterVec
	AIImageAssetsCreated       *prometheus.CounterVec
	AIImageScanWaitSeconds     *prometheus.HistogramVec
	AIImageApplyTotal          *prometheus.CounterVec
	AIImageReconciliation      *prometheus.CounterVec
	FileScanTasksTotal         *prometheus.CounterVec
	FileScanDurationSeconds    *prometheus.HistogramVec
	FileScanResultsTotal       *prometheus.CounterVec
	FileScanQueueAgeSeconds    *prometheus.HistogramVec
	FileScanFailuresTotal      *prometheus.CounterVec
	FileQuarantinedTotal       *prometheus.CounterVec
	FileRejectedTotal          *prometheus.CounterVec
	FileScanStuckTotal         *prometheus.CounterVec
	FileAssetsByStatus         *prometheus.GaugeVec
	SecretRotationJobsTotal    *prometheus.CounterVec
	SecretRotationFailures     *prometheus.CounterVec
	SecretRotationUnknown      *prometheus.CounterVec
	SecretRotationOldKeyRefs   *prometheus.CounterVec
	AuthorizationDeniedTotal   *prometheus.CounterVec
	AuthLoginAttemptsTotal     *prometheus.CounterVec
	AuthLoginFailuresTotal     *prometheus.CounterVec
	AuthLoginRateLimited       *prometheus.CounterVec
	AuthAccountLocked          *prometheus.CounterVec
	AuthSessionsActive         *prometheus.GaugeVec
	AuthSessionsRevoked        *prometheus.CounterVec
	AuthRefreshTotal           *prometheus.CounterVec
	AuthRefreshReuseTotal      prometheus.Counter
	AuthTokenValidationFail    *prometheus.CounterVec
	AuthReauthRequired         *prometheus.CounterVec
	TenantAccessDeniedTotal    *prometheus.CounterVec
	ShopAccessDeniedTotal      *prometheus.CounterVec
	IDORAttemptsTotal          *prometheus.CounterVec
	SystemContextDenied        *prometheus.CounterVec
	CSRFRejectedTotal          *prometheus.CounterVec
	OriginRejectedTotal        *prometheus.CounterVec
	OpenRedirectRejected       *prometheus.CounterVec
	AuditChainMismatchTotal    prometheus.Counter
	SecurityEventsTotal        *prometheus.CounterVec
	TelemetryExportFailures    prometheus.Counter
	TelemetryDroppedItems      prometheus.Counter
	TelemetryExportSuccess     prometheus.Counter
	TelemetryQueueDepth        prometheus.Gauge
	SLOComplianceRatio         *prometheus.GaugeVec
	SLOErrorBudgetRemaining    *prometheus.GaugeVec
	SLOBurnRate                *prometheus.GaugeVec
	DBConnectionsOpen          *prometheus.GaugeVec
	DBConnectionsInUse         *prometheus.GaugeVec
	DBConnectionsIdle          *prometheus.GaugeVec
	DBMaxOpenConnections       *prometheus.GaugeVec
	DBConnectionWaitCount      *prometheus.CounterVec
	DBConnectionWaitDuration   *prometheus.CounterVec
	DBQueryDuration            *prometheus.HistogramVec
	DBQueryErrors              *prometheus.CounterVec
	DBTransactionDuration      *prometheus.HistogramVec
	DBTransactionRollbacks     *prometheus.CounterVec
	BackupJobsTotal            *prometheus.CounterVec
	BackupJobDuration          *prometheus.HistogramVec
	BackupBytesTotal           *prometheus.CounterVec
	BackupFailuresTotal        *prometheus.CounterVec
	BackupVerificationTotal    *prometheus.CounterVec
	BackupVerificationDuration *prometheus.HistogramVec
	BackupLastSuccess          *prometheus.GaugeVec
	BackupAgeSeconds           *prometheus.GaugeVec
	BackupUploadFailures       *prometheus.CounterVec
	BackupRetentionDeletions   *prometheus.CounterVec
	RestoreJobsTotal           *prometheus.CounterVec
	RestoreJobDuration         *prometheus.HistogramVec
	RestoreFailuresTotal       *prometheus.CounterVec
	RestoreValidationTotal     *prometheus.CounterVec
	RestoreLastSuccess         *prometheus.GaugeVec
	ReleaseRunsTotal           *prometheus.CounterVec
	ReleaseStepDuration        *prometheus.HistogramVec
	ReleaseFailuresTotal       *prometheus.CounterVec
	ReleaseRollbacksTotal      *prometheus.CounterVec
	ReleaseLastSuccess         *prometheus.GaugeVec
	ReleaseCurrentVersion      *prometheus.GaugeVec
	ReleaseHealthFailures      *prometheus.CounterVec
	DRDrillsTotal              *prometheus.CounterVec
	DRDrillDuration            *prometheus.HistogramVec
	DRDrillFailures            *prometheus.CounterVec
	DRLastSuccess              *prometheus.GaugeVec

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
	c.OrderSyncOrdersReceived, err = c.reg.Counter("order_sync_orders_received_total", "Order sync orders received", "platform", "source", "result")
	if err != nil {
		return err
	}
	c.OrderSyncOrdersCreated, err = c.reg.Counter("order_sync_orders_created_total", "Order sync orders created", "platform", "source", "result")
	if err != nil {
		return err
	}
	c.OrderSyncOrdersUpdated, err = c.reg.Counter("order_sync_orders_updated_total", "Order sync orders updated", "platform", "source", "result")
	if err != nil {
		return err
	}
	c.OrderSyncStaleUpdates, err = c.reg.Counter("order_sync_stale_updates_total", "Order sync stale updates", "platform", "source")
	if err != nil {
		return err
	}
	c.OrderSyncDuplicates, err = c.reg.Counter("order_sync_duplicates_total", "Order sync duplicate events", "platform", "source")
	if err != nil {
		return err
	}
	c.OrderSyncFailuresTotal, err = c.reg.Counter(
		"order_sync_failures_total", "Order sync failures",
		"platform", "source", "error_class")
	if err != nil {
		return err
	}
	c.OrderSyncDuration, err = c.reg.Histogram("order_sync_duration_seconds", "Order sync duration", defaultBuckets, "platform", "source", "result")
	if err != nil {
		return err
	}
	c.OrderSyncCursorLag, err = c.reg.Histogram("order_sync_cursor_lag_seconds", "Order sync cursor lag", defaultBuckets, "platform", "source")
	if err != nil {
		return err
	}
	c.OrderSyncLastSuccess, err = c.reg.Gauge("order_sync_last_success_timestamp", "Order sync last success unix timestamp", "platform", "source")
	if err != nil {
		return err
	}
	c.InventoryAdjustmentsTotal, err = c.reg.Counter(
		"inventory_adjustments_total", "Inventory adjustments",
		"platform", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.InventoryDeductionsTotal, err = c.reg.Counter("inventory_deductions_total", "Inventory deductions", "platform", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.InventoryCompensations, err = c.reg.Counter("inventory_compensations_total", "Inventory compensations", "platform", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.InventoryPushTotal, err = c.reg.Counter("inventory_push_total", "Inventory provider pushes", "platform", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.InventoryPushFailures, err = c.reg.Counter("inventory_push_failures_total", "Inventory provider push failures", "platform", "operation", "error_class")
	if err != nil {
		return err
	}
	c.InventoryUnknownResults, err = c.reg.Counter(
		"inventory_unknown_results_total", "Inventory write requests with unknown result",
		"platform", "operation", "error_class")
	if err != nil {
		return err
	}
	c.InventoryVersionConflicts, err = c.reg.Counter("inventory_version_conflicts_total", "Inventory version conflicts", "platform", "operation")
	if err != nil {
		return err
	}
	c.InventoryNegativePrevent, err = c.reg.Counter("inventory_negative_prevented_total", "Inventory negative stock prevented", "platform", "operation")
	if err != nil {
		return err
	}
	c.InventorySyncDuration, err = c.reg.Histogram("inventory_sync_duration_seconds", "Inventory sync duration", defaultBuckets, "platform", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.AITextRequestsTotal, err = c.reg.Counter(
		"ai_text_requests_total", "AI text requests",
		"provider", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.AITextRequestDuration, err = c.reg.Histogram("ai_text_request_duration_seconds", "AI text request duration", defaultBuckets, "provider", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.AITextProviderTimeouts, err = c.reg.Counter(
		"ai_text_provider_timeouts_total", "AI text provider timeouts",
		"provider", "operation", "error_class")
	if err != nil {
		return err
	}
	c.AITextProviderFailures, err = c.reg.Counter("ai_text_provider_failures_total", "AI text provider failures", "provider", "operation", "error_class")
	if err != nil {
		return err
	}
	c.AITextEnvironmentBlocked, err = c.reg.Counter(
		"ai_text_environment_blocked_total", "AI text environment blocked",
		"provider", "operation", "error_class")
	if err != nil {
		return err
	}
	c.AITextBatchesTotal, err = c.reg.Counter("ai_text_batches_total", "AI text batches", "operation", "result")
	if err != nil {
		return err
	}
	c.AITextBatchDuration, err = c.reg.Histogram("ai_text_batch_duration_seconds", "AI text batch duration", defaultBuckets, "operation", "result")
	if err != nil {
		return err
	}
	c.AITextApplyTotal, err = c.reg.Counter("ai_text_apply_total", "AI text apply operations", "operation", "result")
	if err != nil {
		return err
	}
	c.AITextApplyConflicts, err = c.reg.Counter("ai_text_apply_conflicts_total", "AI text apply conflicts", "operation", "result")
	if err != nil {
		return err
	}
	c.AITextReconciliation, err = c.reg.Counter("ai_text_reconciliation_total", "AI text reconciliation", "operation", "result")
	if err != nil {
		return err
	}
	c.AIImageRequestsTotal, err = c.reg.Counter(
		"ai_image_requests_total", "AI image requests",
		"provider", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.AIImageRequestDuration, err = c.reg.Histogram("ai_image_request_duration_seconds", "AI image request duration", defaultBuckets, "provider", "operation", "result", "error_class")
	if err != nil {
		return err
	}
	c.AIImageProviderTimeouts, err = c.reg.Counter(
		"ai_image_provider_timeouts_total", "AI image provider timeouts (P5-OBS-001)",
		"provider", "operation", "error_class")
	if err != nil {
		return err
	}
	c.AIImageProviderFailures, err = c.reg.Counter("ai_image_provider_failures_total", "AI image provider failures", "provider", "operation", "error_class")
	if err != nil {
		return err
	}
	c.AIImageEnvironmentBlocked, err = c.reg.Counter(
		"ai_image_environment_blocked_total", "AI image environment blocked",
		"provider", "operation", "error_class")
	if err != nil {
		return err
	}
	c.AIImageBatchesTotal, err = c.reg.Counter("ai_image_batches_total", "AI image batches", "operation", "result")
	if err != nil {
		return err
	}
	c.AIImageBatchDuration, err = c.reg.Histogram("ai_image_batch_duration_seconds", "AI image batch duration", defaultBuckets, "operation", "result")
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
	c.AIImageAssetsCreated, err = c.reg.Counter("ai_image_assets_created_total", "AI image assets created", "provider", "operation", "result")
	if err != nil {
		return err
	}
	c.AIImageScanWaitSeconds, err = c.reg.Histogram("ai_image_scan_wait_seconds", "AI image scan wait duration", defaultBuckets, "provider", "operation")
	if err != nil {
		return err
	}
	c.AIImageApplyTotal, err = c.reg.Counter("ai_image_apply_total", "AI image apply operations", "operation", "result")
	if err != nil {
		return err
	}
	c.AIImageReconciliation, err = c.reg.Counter("ai_image_reconciliation_total", "AI image reconciliation", "operation", "result")
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
	c.FileScanResultsTotal, err = c.reg.Counter("file_scan_results_total", "File scan results", "scanner", "result", "mime_group")
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
	c.FileQuarantinedTotal, err = c.reg.Counter("file_quarantined_total", "Files quarantined", "scanner", "mime_group")
	if err != nil {
		return err
	}
	c.FileRejectedTotal, err = c.reg.Counter("file_rejected_total", "Files rejected", "scanner", "mime_group")
	if err != nil {
		return err
	}
	c.FileScanStuckTotal, err = c.reg.Counter("file_scan_stuck_total", "File scan stuck tasks", "scanner", "mime_group")
	if err != nil {
		return err
	}
	c.FileAssetsByStatus, err = c.reg.Gauge("file_assets_by_security_status", "File assets by security status", "security_status", "mime_group")
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
	c.SecretRotationUnknown, err = c.reg.Counter("secret_rotation_unknown_formats_total", "Secret rotation unknown formats", "target", "result", "status")
	if err != nil {
		return err
	}
	c.SecretRotationOldKeyRefs, err = c.reg.Counter("secret_rotation_old_key_references", "Secret rotation old key references", "target", "result", "status")
	if err != nil {
		return err
	}
	c.AuthorizationDeniedTotal, err = c.reg.Counter("authorization_denied_total", "Authorization denied events", "module", "result", "severity")
	if err != nil {
		return err
	}
	c.AuthLoginAttemptsTotal, err = c.reg.Counter(
		"auth_login_attempts_total", "Auth login attempts",
		"result", "reason", "auth_mode")
	if err != nil {
		return err
	}
	c.AuthLoginFailuresTotal, err = c.reg.Counter("auth_login_failures_total", "Auth login failures", "reason", "auth_mode")
	if err != nil {
		return err
	}
	c.AuthLoginRateLimited, err = c.reg.Counter("auth_login_rate_limited_total", "Auth login rate limited", "reason", "auth_mode")
	if err != nil {
		return err
	}
	c.AuthAccountLocked, err = c.reg.Counter("auth_account_locked_total", "Auth account locked", "reason", "auth_mode")
	if err != nil {
		return err
	}
	c.AuthSessionsActive, err = c.reg.Gauge("auth_sessions_active", "Active auth sessions", "auth_mode")
	if err != nil {
		return err
	}
	c.AuthSessionsRevoked, err = c.reg.Counter("auth_sessions_revoked_total", "Auth sessions revoked", "reason", "auth_mode")
	if err != nil {
		return err
	}
	c.AuthRefreshTotal, err = c.reg.Counter("auth_refresh_total", "Auth refresh attempts", "result", "reason", "auth_mode")
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
	c.AuthTokenValidationFail, err = c.reg.Counter("auth_token_validation_failures_total", "Auth token validation failures", "reason", "auth_mode")
	if err != nil {
		return err
	}
	c.AuthReauthRequired, err = c.reg.Counter("auth_reauthentication_required_total", "Auth reauthentication required", "reason", "auth_mode")
	if err != nil {
		return err
	}
	c.TenantAccessDeniedTotal, err = c.reg.Counter(
		"tenant_access_denied_total", "Tenant access denied events",
		"module", "result", "severity")
	if err != nil {
		return err
	}
	c.ShopAccessDeniedTotal, err = c.reg.Counter("shop_access_denied_total", "Shop access denied events", "module", "result", "severity")
	if err != nil {
		return err
	}
	c.IDORAttemptsTotal, err = c.reg.Counter("idor_attempts_total", "IDOR attempts", "module", "result", "severity")
	if err != nil {
		return err
	}
	c.SystemContextDenied, err = c.reg.Counter("system_context_denied_total", "System context denied events", "module", "result", "severity")
	if err != nil {
		return err
	}
	c.CSRFRejectedTotal, err = c.reg.Counter("csrf_rejected_total", "CSRF rejected events", "module", "result", "severity")
	if err != nil {
		return err
	}
	c.OriginRejectedTotal, err = c.reg.Counter("origin_rejected_total", "Origin rejected events", "module", "result", "severity")
	if err != nil {
		return err
	}
	c.OpenRedirectRejected, err = c.reg.Counter("open_redirect_rejected_total", "Open redirect rejected events", "module", "result", "severity")
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
	if err != nil {
		return err
	}
	c.BackupJobsTotal, err = c.reg.Counter("backup_jobs_total", "Backup jobs", "backup_type", "result", "environment_class")
	if err != nil {
		return err
	}
	c.BackupJobDuration, err = c.reg.Histogram("backup_job_duration_seconds", "Backup job duration", defaultBuckets, "backup_type", "result", "environment_class")
	if err != nil {
		return err
	}
	c.BackupBytesTotal, err = c.reg.Counter("backup_bytes_total", "Backup bytes written", "backup_type", "result", "environment_class")
	if err != nil {
		return err
	}
	c.BackupFailuresTotal, err = c.reg.Counter("backup_failures_total", "Backup failures", "backup_type", "result", "environment_class")
	if err != nil {
		return err
	}
	c.BackupVerificationTotal, err = c.reg.Counter("backup_verification_total", "Backup verifications", "backup_type", "result", "environment_class")
	if err != nil {
		return err
	}
	c.BackupVerificationDuration, err = c.reg.Histogram("backup_verification_duration_seconds", "Backup verification duration", defaultBuckets, "backup_type", "result", "environment_class")
	if err != nil {
		return err
	}
	c.BackupLastSuccess, err = c.reg.Gauge("backup_last_success_timestamp", "Last successful backup timestamp", "backup_type", "environment_class")
	if err != nil {
		return err
	}
	c.BackupAgeSeconds, err = c.reg.Gauge("backup_age_seconds", "Backup age seconds", "backup_type", "environment_class")
	if err != nil {
		return err
	}
	c.BackupUploadFailures, err = c.reg.Counter("backup_storage_upload_failures_total", "Backup storage upload failures", "backup_type", "result", "environment_class")
	if err != nil {
		return err
	}
	c.BackupRetentionDeletions, err = c.reg.Counter("backup_retention_deletions_total", "Backup retention deletions", "backup_type", "result", "environment_class")
	if err != nil {
		return err
	}
	c.RestoreJobsTotal, err = c.reg.Counter("restore_jobs_total", "Restore jobs", "result", "environment_class")
	if err != nil {
		return err
	}
	c.RestoreJobDuration, err = c.reg.Histogram("restore_job_duration_seconds", "Restore job duration", defaultBuckets, "result", "environment_class")
	if err != nil {
		return err
	}
	c.RestoreFailuresTotal, err = c.reg.Counter("restore_failures_total", "Restore failures", "result", "environment_class")
	if err != nil {
		return err
	}
	c.RestoreValidationTotal, err = c.reg.Counter("restore_validation_total", "Restore validations", "result", "environment_class")
	if err != nil {
		return err
	}
	c.RestoreLastSuccess, err = c.reg.Gauge("restore_last_success_timestamp", "Last successful restore timestamp", "environment_class")
	if err != nil {
		return err
	}
	c.ReleaseRunsTotal, err = c.reg.Counter("release_runs_total", "Release runs", "strategy", "result", "environment_class")
	if err != nil {
		return err
	}
	c.ReleaseStepDuration, err = c.reg.Histogram("release_step_duration_seconds", "Release step duration", defaultBuckets, "step", "strategy", "result", "environment_class")
	if err != nil {
		return err
	}
	c.ReleaseFailuresTotal, err = c.reg.Counter("release_failures_total", "Release failures", "strategy", "result", "environment_class")
	if err != nil {
		return err
	}
	c.ReleaseRollbacksTotal, err = c.reg.Counter("release_rollbacks_total", "Release rollbacks", "strategy", "result", "environment_class")
	if err != nil {
		return err
	}
	c.ReleaseLastSuccess, err = c.reg.Gauge("release_last_success_timestamp", "Last successful release timestamp", "strategy", "environment_class")
	if err != nil {
		return err
	}
	c.ReleaseCurrentVersion, err = c.reg.Gauge("release_current_version_info", "Current release version info", "strategy", "environment_class")
	if err != nil {
		return err
	}
	c.ReleaseHealthFailures, err = c.reg.Counter("release_health_check_failures_total", "Release health check failures", "strategy", "result", "environment_class")
	if err != nil {
		return err
	}
	c.DRDrillsTotal, err = c.reg.Counter("dr_drills_total", "DR drills", "result", "environment_class")
	if err != nil {
		return err
	}
	c.DRDrillDuration, err = c.reg.Histogram("dr_drill_duration_seconds", "DR drill duration", defaultBuckets, "result", "environment_class")
	if err != nil {
		return err
	}
	c.DRDrillFailures, err = c.reg.Counter("dr_drill_failures_total", "DR drill failures", "result", "environment_class")
	if err != nil {
		return err
	}
	c.DRLastSuccess, err = c.reg.Gauge("dr_last_success_timestamp", "Last successful DR drill timestamp", "environment_class")
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

// ObserveWebhook records webhook receive, validation, persistence and worker outcomes.
func (c *Catalog) ObserveWebhook(platform, eventGroup, event, result, errorClass string, dur time.Duration) {
	if c == nil {
		return
	}
	res := NormalizeResult(result)
	ec := NormalizeResult(errorClass)
	switch event {
	case "request":
		if c.WebhookRequestsTotal != nil {
			c.WebhookRequestsTotal.WithLabelValues(platform, eventGroup, res, ec).Inc()
		}
	case "signature_failure":
		if c.WebhookSignatureFailures != nil {
			c.WebhookSignatureFailures.WithLabelValues(platform, eventGroup, ec).Inc()
		}
	case "replay_rejected":
		if c.WebhookReplayRejected != nil {
			c.WebhookReplayRejected.WithLabelValues(platform, eventGroup, ec).Inc()
		}
	case "payload_rejected":
		if c.WebhookPayloadRejected != nil {
			c.WebhookPayloadRejected.WithLabelValues(platform, eventGroup, ec).Inc()
		}
	case "persisted":
		if c.WebhookEventsPersisted != nil {
			c.WebhookEventsPersisted.WithLabelValues(platform, eventGroup, res).Inc()
		}
	case "unknown":
		if c.WebhookUnknownEvents != nil {
			c.WebhookUnknownEvents.WithLabelValues(platform, eventGroup).Inc()
		}
	case "shop_resolution_failure":
		if c.WebhookShopResolutionFail != nil {
			c.WebhookShopResolutionFail.WithLabelValues(platform, eventGroup, ec).Inc()
		}
	case "tenant_mismatch":
		if c.WebhookTenantMismatch != nil {
			c.WebhookTenantMismatch.WithLabelValues(platform, eventGroup).Inc()
		}
	case "duplicate":
		if c.WebhookDuplicateEvents != nil {
			c.WebhookDuplicateEvents.WithLabelValues(platform, eventGroup).Inc()
		}
	case "processed":
		c.ObserveWebhookProcessed(platform, eventGroup, res, ec, dur, 0)
	}
}

// ObserveOrder records order sync counters and durations.
func (c *Catalog) ObserveOrder(platform, source, event, result, errorClass string, count int, dur time.Duration, cursorLag time.Duration) {
	if c == nil {
		return
	}
	if count <= 0 {
		count = 1
	}
	res := NormalizeResult(result)
	ec := NormalizeResult(errorClass)
	add := float64(count)
	switch event {
	case "run":
		if c.OrderSyncRunsTotal != nil {
			c.OrderSyncRunsTotal.WithLabelValues(platform, source, res).Add(add)
		}
		if c.OrderSyncDuration != nil && dur > 0 {
			c.OrderSyncDuration.WithLabelValues(platform, source, res).Observe(dur.Seconds())
		}
		if res == "success" && c.OrderSyncLastSuccess != nil {
			c.OrderSyncLastSuccess.WithLabelValues(platform, source).Set(float64(time.Now().UTC().Unix()))
		}
	case "received":
		if c.OrderSyncOrdersReceived != nil {
			c.OrderSyncOrdersReceived.WithLabelValues(platform, source, res).Add(add)
		}
	case "created":
		if c.OrderSyncOrdersCreated != nil {
			c.OrderSyncOrdersCreated.WithLabelValues(platform, source, res).Add(add)
		}
	case "updated":
		if c.OrderSyncOrdersUpdated != nil {
			c.OrderSyncOrdersUpdated.WithLabelValues(platform, source, res).Add(add)
		}
	case "stale":
		if c.OrderSyncStaleUpdates != nil {
			c.OrderSyncStaleUpdates.WithLabelValues(platform, source).Add(add)
		}
	case "duplicate":
		if c.OrderSyncDuplicates != nil {
			c.OrderSyncDuplicates.WithLabelValues(platform, source).Add(add)
		}
	case "failure":
		if c.OrderSyncFailuresTotal != nil {
			c.OrderSyncFailuresTotal.WithLabelValues(platform, source, ec).Add(add)
		}
	}
	if cursorLag > 0 && c.OrderSyncCursorLag != nil {
		c.OrderSyncCursorLag.WithLabelValues(platform, source).Observe(cursorLag.Seconds())
	}
}

// ObserveInventory records local and provider inventory operations.
func (c *Catalog) ObserveInventory(platform, operation, event, result, errorClass string, count int, dur time.Duration) {
	if c == nil {
		return
	}
	if count <= 0 {
		count = 1
	}
	res := NormalizeResult(result)
	ec := NormalizeResult(errorClass)
	add := float64(count)
	switch event {
	case "adjust":
		if c.InventoryAdjustmentsTotal != nil {
			c.InventoryAdjustmentsTotal.WithLabelValues(platform, operation, res, ec).Add(add)
		}
	case "deduct":
		if c.InventoryDeductionsTotal != nil {
			c.InventoryDeductionsTotal.WithLabelValues(platform, operation, res, ec).Add(add)
		}
	case "compensate":
		if c.InventoryCompensations != nil {
			c.InventoryCompensations.WithLabelValues(platform, operation, res, ec).Add(add)
		}
	case "push":
		if c.InventoryPushTotal != nil {
			c.InventoryPushTotal.WithLabelValues(platform, operation, res, ec).Add(add)
		}
	case "push_failure":
		if c.InventoryPushFailures != nil {
			c.InventoryPushFailures.WithLabelValues(platform, operation, ec).Add(add)
		}
	case "unknown_result":
		if c.InventoryUnknownResults != nil {
			c.InventoryUnknownResults.WithLabelValues(platform, operation, ec).Add(add)
		}
	case "version_conflict":
		if c.InventoryVersionConflicts != nil {
			c.InventoryVersionConflicts.WithLabelValues(platform, operation).Add(add)
		}
	case "negative_prevented":
		if c.InventoryNegativePrevent != nil {
			c.InventoryNegativePrevent.WithLabelValues(platform, operation).Add(add)
		}
	}
	if dur > 0 && c.InventorySyncDuration != nil {
		c.InventorySyncDuration.WithLabelValues(platform, operation, res, ec).Observe(dur.Seconds())
	}
}

// ObserveAIText records AI text generation, batch, apply and reconciliation outcomes.
func (c *Catalog) ObserveAIText(provider, operation, event, result, errorClass string, dur time.Duration) {
	if c == nil {
		return
	}
	res := NormalizeResult(result)
	ec := NormalizeResult(errorClass)
	switch event {
	case "request":
		if c.AITextRequestsTotal != nil {
			c.AITextRequestsTotal.WithLabelValues(provider, operation, res, ec).Inc()
		}
		if c.AITextRequestDuration != nil && dur > 0 {
			c.AITextRequestDuration.WithLabelValues(provider, operation, res, ec).Observe(dur.Seconds())
		}
	case "timeout":
		if c.AITextProviderTimeouts != nil {
			c.AITextProviderTimeouts.WithLabelValues(provider, operation, ec).Inc()
		}
	case "failure":
		if c.AITextProviderFailures != nil {
			c.AITextProviderFailures.WithLabelValues(provider, operation, ec).Inc()
		}
	case "environment_blocked":
		if c.AITextEnvironmentBlocked != nil {
			c.AITextEnvironmentBlocked.WithLabelValues(provider, operation, ec).Inc()
		}
	case "batch":
		if c.AITextBatchesTotal != nil {
			c.AITextBatchesTotal.WithLabelValues(operation, res).Inc()
		}
		if c.AITextBatchDuration != nil && dur > 0 {
			c.AITextBatchDuration.WithLabelValues(operation, res).Observe(dur.Seconds())
		}
	case "apply":
		if c.AITextApplyTotal != nil {
			c.AITextApplyTotal.WithLabelValues(operation, res).Inc()
		}
	case "apply_conflict":
		if c.AITextApplyConflicts != nil {
			c.AITextApplyConflicts.WithLabelValues(operation, res).Inc()
		}
	case "reconcile":
		if c.AITextReconciliation != nil {
			c.AITextReconciliation.WithLabelValues(operation, res).Inc()
		}
	}
}

// ObserveAIImage records AI image provider, batch, stage, asset and apply outcomes.
func (c *Catalog) ObserveAIImage(provider, operation, event, result, errorClass string, dur time.Duration) {
	if c == nil {
		return
	}
	res := NormalizeResult(result)
	ec := NormalizeResult(errorClass)
	switch event {
	case "request":
		if c.AIImageRequestsTotal != nil {
			c.AIImageRequestsTotal.WithLabelValues(provider, operation, res, ec).Inc()
		}
		if c.AIImageRequestDuration != nil && dur > 0 {
			c.AIImageRequestDuration.WithLabelValues(provider, operation, res, ec).Observe(dur.Seconds())
		}
	case "timeout":
		c.ObserveAIImageProviderTimeout(provider, operation, ec)
	case "failure":
		if c.AIImageProviderFailures != nil {
			c.AIImageProviderFailures.WithLabelValues(provider, operation, ec).Inc()
		}
	case "environment_blocked":
		if c.AIImageEnvironmentBlocked != nil {
			c.AIImageEnvironmentBlocked.WithLabelValues(provider, operation, ec).Inc()
		}
	case "batch":
		if c.AIImageBatchesTotal != nil {
			c.AIImageBatchesTotal.WithLabelValues(operation, res).Inc()
		}
		if c.AIImageBatchDuration != nil && dur > 0 {
			c.AIImageBatchDuration.WithLabelValues(operation, res).Observe(dur.Seconds())
		}
	case "stage":
		if c.AIImageTaskStageDuration != nil && dur > 0 {
			c.AIImageTaskStageDuration.WithLabelValues(operation, res).Observe(dur.Seconds())
		}
	case "stuck":
		if c.AIImageTaskStuckTotal != nil {
			c.AIImageTaskStuckTotal.WithLabelValues(operation, res).Inc()
		}
	case "asset_created":
		if c.AIImageAssetsCreated != nil {
			c.AIImageAssetsCreated.WithLabelValues(provider, operation, res).Inc()
		}
	case "scan_wait":
		if c.AIImageScanWaitSeconds != nil && dur > 0 {
			c.AIImageScanWaitSeconds.WithLabelValues(provider, operation).Observe(dur.Seconds())
		}
	case "apply":
		if c.AIImageApplyTotal != nil {
			c.AIImageApplyTotal.WithLabelValues(operation, res).Inc()
		}
	case "reconcile":
		if c.AIImageReconciliation != nil {
			c.AIImageReconciliation.WithLabelValues(operation, res).Inc()
		}
	}
}

// ObserveFileScan records file security scan lifecycle outcomes.
func (c *Catalog) ObserveFileScan(scanner, event, result, mimeGroup string, dur time.Duration) {
	if c == nil {
		return
	}
	res := NormalizeResult(result)
	if mimeGroup == "" {
		mimeGroup = "unknown"
	}
	switch event {
	case "enqueue", "claim":
		if c.FileScanTasksTotal != nil {
			c.FileScanTasksTotal.WithLabelValues(scanner, event, mimeGroup).Inc()
		}
	case "result":
		if c.FileScanResultsTotal != nil {
			c.FileScanResultsTotal.WithLabelValues(scanner, res, mimeGroup).Inc()
		}
		if c.FileScanDurationSeconds != nil && dur > 0 {
			c.FileScanDurationSeconds.WithLabelValues(scanner, res, mimeGroup).Observe(dur.Seconds())
		}
		if res == "failure" && c.FileScanFailuresTotal != nil {
			c.FileScanFailuresTotal.WithLabelValues(scanner, res, mimeGroup).Inc()
		}
		if res == "quarantined" && c.FileQuarantinedTotal != nil {
			c.FileQuarantinedTotal.WithLabelValues(scanner, mimeGroup).Inc()
		}
		if res == "rejected" && c.FileRejectedTotal != nil {
			c.FileRejectedTotal.WithLabelValues(scanner, mimeGroup).Inc()
		}
	case "queue_age":
		if c.FileScanQueueAgeSeconds != nil && dur > 0 {
			c.FileScanQueueAgeSeconds.WithLabelValues(scanner, mimeGroup).Observe(dur.Seconds())
		}
	case "stuck":
		if c.FileScanStuckTotal != nil {
			c.FileScanStuckTotal.WithLabelValues(scanner, mimeGroup).Inc()
		}
	}
	if c.FileAssetsByStatus != nil && res != "" {
		c.FileAssetsByStatus.WithLabelValues(res, mimeGroup).Inc()
	}
}

// ObserveSecurity records security denial and integrity events.
func (c *Catalog) ObserveSecurity(module, event, result, severity string) {
	if c == nil {
		return
	}
	res := NormalizeResult(result)
	if severity == "" {
		severity = "warning"
	}
	if c.SecurityEventsTotal != nil {
		c.SecurityEventsTotal.WithLabelValues(event, res, severity, module).Inc()
	}
	switch event {
	case "authorization_denied":
		if c.AuthorizationDeniedTotal != nil {
			c.AuthorizationDeniedTotal.WithLabelValues(module, res, severity).Inc()
		}
	case "tenant_access_denied":
		if c.TenantAccessDeniedTotal != nil {
			c.TenantAccessDeniedTotal.WithLabelValues(module, res, severity).Inc()
		}
	case "shop_access_denied":
		if c.ShopAccessDeniedTotal != nil {
			c.ShopAccessDeniedTotal.WithLabelValues(module, res, severity).Inc()
		}
	case "idor_attempt":
		if c.IDORAttemptsTotal != nil {
			c.IDORAttemptsTotal.WithLabelValues(module, res, severity).Inc()
		}
	case "system_context_denied":
		if c.SystemContextDenied != nil {
			c.SystemContextDenied.WithLabelValues(module, res, severity).Inc()
		}
	case "csrf_rejected":
		if c.CSRFRejectedTotal != nil {
			c.CSRFRejectedTotal.WithLabelValues(module, res, severity).Inc()
		}
	case "origin_rejected":
		if c.OriginRejectedTotal != nil {
			c.OriginRejectedTotal.WithLabelValues(module, res, severity).Inc()
		}
	case "open_redirect_rejected":
		if c.OpenRedirectRejected != nil {
			c.OpenRedirectRejected.WithLabelValues(module, res, severity).Inc()
		}
	case "audit_chain_mismatch":
		if c.AuditChainMismatchTotal != nil {
			c.AuditChainMismatchTotal.Inc()
		}
	}
}

// ObserveAuth records login, session, refresh and token validation events.
func (c *Catalog) ObserveAuth(event, result, reason, authMode string) {
	if c == nil {
		return
	}
	res := NormalizeResult(result)
	if reason == "" {
		reason = "unknown"
	}
	if authMode == "" {
		authMode = "password"
	}
	switch event {
	case "login_attempt", "login_success":
		if c.AuthLoginAttemptsTotal != nil {
			c.AuthLoginAttemptsTotal.WithLabelValues(res, reason, authMode).Inc()
		}
	case "login_failure":
		if c.AuthLoginFailuresTotal != nil {
			c.AuthLoginFailuresTotal.WithLabelValues(reason, authMode).Inc()
		}
	case "rate_limited":
		if c.AuthLoginRateLimited != nil {
			c.AuthLoginRateLimited.WithLabelValues(reason, authMode).Inc()
		}
	case "account_locked":
		if c.AuthAccountLocked != nil {
			c.AuthAccountLocked.WithLabelValues(reason, authMode).Inc()
		}
	case "session_created":
		if c.AuthSessionsActive != nil {
			c.AuthSessionsActive.WithLabelValues(authMode).Inc()
		}
	case "session_revoked":
		if c.AuthSessionsRevoked != nil {
			c.AuthSessionsRevoked.WithLabelValues(reason, authMode).Inc()
		}
		if c.AuthSessionsActive != nil {
			c.AuthSessionsActive.WithLabelValues(authMode).Dec()
		}
	case "refresh":
		if c.AuthRefreshTotal != nil {
			c.AuthRefreshTotal.WithLabelValues(res, reason, authMode).Inc()
		}
	case "refresh_reuse":
		if c.AuthRefreshReuseTotal != nil {
			c.AuthRefreshReuseTotal.Inc()
		}
	case "token_validation_failure":
		if c.AuthTokenValidationFail != nil {
			c.AuthTokenValidationFail.WithLabelValues(reason, authMode).Inc()
		}
	case "reauthentication_required":
		if c.AuthReauthRequired != nil {
			c.AuthReauthRequired.WithLabelValues(reason, authMode).Inc()
		}
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

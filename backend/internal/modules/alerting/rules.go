package alerting

import (
	"context"

	"gorm.io/gorm"
)

// DefaultRules returns built-in alert rules.
func DefaultRules() []AlertRule {
	return []AlertRule{
		{ID: "audit_chain_mismatch", Name: "Audit chain mismatch", Metric: "audit_chain_mismatch_total", Condition: ">", Threshold: 0, Severity: SeverityCritical, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/AUDIT_CHAIN_MISMATCH.md", ChannelGroup: "security"},
		{ID: "auth_refresh_reuse", Name: "Refresh token reuse", Metric: "auth_refresh_reuse_detected_total", Condition: ">", Threshold: 0, Severity: SeverityCritical, CooldownSeconds: 60, Enabled: true, RunbookURL: "docs/runbooks/AUTH_REFRESH_REUSE.md", ChannelGroup: "security"},
		{ID: "http_5xx_elevated", Name: "HTTP 5xx elevated", Metric: "http_server_requests_total", Condition: "rate", Threshold: 0.05, Severity: SeverityWarning, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/HTTP_5XX_SPIKE.md", ChannelGroup: "ops"},
		{ID: "provider_timeout_elevated", Name: "Provider timeout elevated", Metric: "provider_request_timeouts_total", Condition: "rate", Threshold: 0.1, Severity: SeverityWarning, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/PROVIDER_TIMEOUT_SPIKE.md", ChannelGroup: "ops"},
		{ID: "ai_image_provider_timeout", Name: "AI image provider timeout", Metric: "ai_image_provider_timeouts_total", Condition: ">", Threshold: 0, Severity: SeverityWarning, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/AI_IMAGE_PROVIDER_TIMEOUT.md", ChannelGroup: "ops"},
		{ID: "task_dead_letter_spike", Name: "Task dead letter spike", Metric: "tasks_dead_letter_total", Condition: ">", Threshold: 5, Severity: SeverityCritical, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/TASK_DEAD_LETTER_SPIKE.md", ChannelGroup: "ops"},
		{ID: "task_queue_backlog", Name: "Task queue age elevated", Metric: "task_queue_age_seconds", Condition: ">", Threshold: 300, Severity: SeverityWarning, CooldownSeconds: 600, Enabled: true, RunbookURL: "docs/runbooks/TASK_QUEUE_BACKLOG.md", ChannelGroup: "ops"},
		{ID: "webhook_lag", Name: "Webhook processing lag", Metric: "webhook_processing_lag_seconds", Condition: ">", Threshold: 120, Severity: SeverityWarning, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/WEBHOOK_LAG.md", ChannelGroup: "ops"},
		{ID: "order_sync_lag", Name: "Order sync cursor lag", Metric: "order_sync_cursor_lag_seconds", Condition: ">", Threshold: 600, Severity: SeverityWarning, CooldownSeconds: 600, Enabled: true, RunbookURL: "docs/runbooks/ORDER_SYNC_LAG.md", ChannelGroup: "ops"},
		{ID: "file_scan_backlog", Name: "File scan backlog", Metric: "file_scan_queue_age_seconds", Condition: ">", Threshold: 300, Severity: SeverityWarning, CooldownSeconds: 600, Enabled: true, RunbookURL: "docs/runbooks/FILE_SCAN_BACKLOG.md", ChannelGroup: "security"},
		{ID: "secret_rotation_failed", Name: "Secret rotation failed", Metric: "secret_rotation_failures_total", Condition: ">", Threshold: 0, Severity: SeverityWarning, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/SECRET_ROTATION_FAILED.md", ChannelGroup: "security"},
		{ID: "tenant_access_denial_spike", Name: "Tenant access denial spike", Metric: "tenant_access_denied_total", Condition: "rate", Threshold: 0.2, Severity: SeverityWarning, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/TENANT_ACCESS_DENIAL_SPIKE.md", ChannelGroup: "security"},
		{ID: "backup_consecutive_failures", Name: "Backup consecutive failures", Metric: "backup_failures_total", Condition: ">", Threshold: 0, Severity: SeverityCritical, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/BACKUP_FAILED.md", ChannelGroup: "ops"},
		{ID: "backup_too_old", Name: "Backup too old", Metric: "backup_age_seconds", Condition: ">", Threshold: 108000, Severity: SeverityCritical, CooldownSeconds: 600, Enabled: true, RunbookURL: "docs/runbooks/BACKUP_TOO_OLD.md", ChannelGroup: "ops"},
		{ID: "backup_verification_failed", Name: "Backup verification failed", Metric: "backup_verification_total", Condition: ">", Threshold: 0, Severity: SeverityCritical, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/BACKUP_VERIFICATION_FAILED.md", ChannelGroup: "ops"},
		{ID: "restore_validation_failed", Name: "Restore validation failed", Metric: "restore_validation_total", Condition: ">", Threshold: 0, Severity: SeverityCritical, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/RESTORE_VALIDATION_FAILED.md", ChannelGroup: "ops"},
		{ID: "release_migration_failed", Name: "Release migration failed", Metric: "release_failures_total", Condition: ">", Threshold: 0, Severity: SeverityCritical, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/MIGRATION_FAILED.md", ChannelGroup: "ops"},
		{ID: "release_health_failed", Name: "Release health check failed", Metric: "release_health_check_failures_total", Condition: ">", Threshold: 0, Severity: SeverityCritical, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/DEPLOYMENT_HEALTH_FAILED.md", ChannelGroup: "ops"},
		{ID: "automatic_rollback_failed", Name: "Automatic rollback failed", Metric: "release_rollbacks_total", Condition: ">", Threshold: 0, Severity: SeverityCritical, CooldownSeconds: 300, Enabled: true, RunbookURL: "docs/runbooks/AUTOMATIC_ROLLBACK_FAILED.md", ChannelGroup: "ops"},
		{ID: "wal_archive_interrupted", Name: "WAL archive interrupted", Metric: "dr_drill_failures_total", Condition: ">", Threshold: 0, Severity: SeverityCritical, CooldownSeconds: 600, Enabled: true, RunbookURL: "docs/runbooks/WAL_ARCHIVE_INTERRUPTED.md", ChannelGroup: "ops"},
	}
}

// EnsureDefaultRules seeds default rules idempotently.
func EnsureDefaultRules(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	for _, rule := range DefaultRules() {
		var count int64
		if err := db.WithContext(ctx).Model(&AlertRule{}).Where("id = ?", rule.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.WithContext(ctx).Create(&rule).Error; err != nil {
			return err
		}
	}
	return nil
}

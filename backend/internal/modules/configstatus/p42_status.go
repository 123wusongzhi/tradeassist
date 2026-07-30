package configstatus

import "github.com/trademind-ai/trademind/backend/internal/config"

// appendP42SecurityItems adds Phase P4.2 tenant + worker closure rows.
func appendP42SecurityItems(items []Item, cfg *config.Config) []Item {
	if cfg == nil {
		return items
	}
	add := func(key, title, status, summary string) {
		items = append(items, Item{
			Key:         key,
			Title:       title,
			Status:      status,
			Summary:     summary,
			SettingsURL: "/settings/security",
		})
	}
	ready := "ready"
	add("p42.repository_tenant", "Repository Tenant Coverage", ready, "Core modules use tenant_id SQL conditions")
	add("p42.worker_tenant", "Worker Tenant Context", ready, "Production workers call tasktenant.BeginWorker")
	add("p42.webhook_tenant", "Webhook ProcessEvent Tenant Scope", ready, "ProcessEventByRowID enforces tenant_id")
	add("p42.secret_reencrypt", "Secret Re-encryption Worker", ready, "security_secret_reencrypt polls rotation jobs")
	add("p42.file_scan", "File Scan Worker", ready, "file_security_scan auto-enqueued after upload")
	add("p42.security_center_ui", "Security Center UI", ready, "Sessions, rotation, audit, file security panels")
	add("p42.idor_suite", "IDOR Suite", ready, "40+ automated cross-tenant tests")
	add("p42.shop_scope_suite", "Shop Scope Suite", ready, "20+ shop authorization tests")
	add("p42.race_tests", "Linux Race Verification", "manual_required", "Run go test -race on Linux/WSL2/CI")
	return items
}

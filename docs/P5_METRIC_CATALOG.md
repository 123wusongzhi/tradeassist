# P5 Metric Catalog

核心指标见 backend/internal/pkg/metrics/catalog.go：http_server_*, provider_*, tasks_*, webhook_*, order_sync_*, inventory_*, ai_text_*, ai_image_*（含 ai_image_provider_timeouts_total P5-OBS-001）, file_scan_*, secret_rotation_*, auth_*, security_*, db_*, telemetry_export_*, telemetry_dropped_items_total, telemetry_queue_depth, slo_compliance_ratio, slo_error_budget_remaining_ratio, slo_burn_rate。

P5.1 要求区分 `registered` 与 `instrumented`：本目录记录指标存在，不代表所有业务模块已经接入真实调用点。真实接线状态以 `scripts/p5-1-observability-closure-check.mjs` 与 `P5_1_BUSINESS_INSTRUMENTATION.md` 为准。

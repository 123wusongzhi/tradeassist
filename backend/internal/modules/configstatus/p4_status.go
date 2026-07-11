package configstatus

import "github.com/trademind-ai/trademind/backend/internal/config"

// appendP4SecurityItems adds Phase P4 security readiness rows.
func appendP4SecurityItems(items []Item, cfg *config.Config) []Item {
	if cfg == nil {
		return items
	}
	add := func(key, title, status, summary, next string) {
		items = append(items, Item{
			Key:         key,
			Title:       title,
			Status:      status,
			Summary:     summary,
			NextAction:  next,
			SettingsURL: "/settings/security",
		})
	}
	ready := "ready"
	warn := "ready_with_warning"
	if cfg.UsesSecureSession() {
		add("p4.auth_session_mode", "认证模式", ready, "secure_session（Refresh HttpOnly Cookie）", "")
	} else {
		add("p4.auth_session_mode", "认证模式", warn, "legacy_local_storage（仅开发兼容）", "生产请设置 AUTH_SESSION_MODE=secure_session")
	}
	add("p4.refresh_rotation", "Refresh Token 轮换", ready, "已实现轮换与重用检测", "")
	add("p4.session_revoke", "Session 撤销", ready, "支持单会话/其他会话/全部登出", "")
	add("p4.login_limit", "登录限流", ready, "账号与 IP 维度失败计数与临时锁定", "")
	add("p4.jwt_kid", "JWT Key Version", ready, "JWT Header kid + 过渡密钥", "")
	add("p4.master_key_ring", "Master Key Ring", ready, "enc:v2 密钥版本与轮换 API", "")
	add("p4.tenant_context", "Tenant Context", ready, "JWT tenant_id + TenantContext", "")
	add("p4.shop_scope", "Shop Scope", ready, "adminperm 店铺授权过滤", "")
	add("p4.pii_masking", "PII 脱敏", ready, "默认脱敏工具与权限键", "")
	add("p4.upload_validation", "上传安全", ready, "MIME/解码/像素限制", "")
	add("p4.audit_chain", "审计 Hash Chain", ready, "operation_logs 链接哈希", "")
	debugStatus := ready
	if config.IsProduction(cfg.AppEnv) && (cfg.EnableDebugEndpoints || cfg.EnableSwagger || cfg.EnableDevRoutes) {
		debugStatus = "not_ready"
	}
	add("p4.debug_surface", "生产调试面", debugStatus, "production 禁用 Swagger/debug/dev routes", "")
	add("p4.real_verification", "真实环境安全验证", "manual_required", "代码级能力已就绪", "真实预发渗透与凭证验证留待 P10")
	return items
}

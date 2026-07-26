package configstatus

import (
	"context"

	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

func (s *Service) p31SummaryItem(_ context.Context) Item {
	return Item{
		Key:         "p31_closure_summary",
		Title:       "P3.1 抖店收口状态",
		SettingsURL: "/settings/config-status",
		ImpactScope: "Production Capability Development In Progress — Real Credential Verification Deferred",
		Status:      StatusCodeReady,
		Summary:     "Phase P3 Fully Closed · 订单 Webhook 已接入 · 契约门控能力已隔离 · 非 Production Ready",
		NextAction:  "真实凭证 E2E 与最终验收留待冻结后统一执行",
	}
}

func (s *Service) p31OrderWebhookItem(_ context.Context) Item {
	return Item{
		Key:         "p31_douyin_order_webhook",
		Title:       "订单 Webhook Handler",
		SettingsURL: "/settings/config-status",
		Status:      StatusCodeReady,
		Summary:     "OrderEventHandler 已注入 · Webhook 与轮询共用 UpsertPlatformOrder · 乱序/重复保护已实现",
	}
}

func (s *Service) p31AIReconcileItem(_ context.Context) Item {
	return Item{
		Key:         "p31_ai_apply_reconcile",
		Title:       "AI apply reconciliation",
		SettingsURL: "/settings/config-status",
		Status:      StatusCodeReady,
		Summary:     "文案/图片 apply 提交间隙幂等恢复已实现",
	}
}

func (s *Service) p31DraftReconcileItem(_ context.Context) Item {
	return Item{
		Key:         "p31_douyin_draft_reconcile",
		Title:       "平台草稿 reconciliation",
		SettingsURL: "/settings/config-status",
		Status:      StatusCodeReady,
		Summary:     "平台草稿创建前回查 + 未知结果恢复已实现",
	}
}

func (s *Service) p31TokenRecoveryItem(_ context.Context) Item {
	return Item{
		Key:         "p31_douyin_token_recovery",
		Title:       "Token refresh recovery",
		SettingsURL: "/settings/platforms?platform=douyin_shop",
		Status:      StatusCodeReady,
		Summary:     "TokenVersion DB 持久化 + singleflight 已实现；等待真实凭证",
	}
}

func (s *Service) p31DouyinBrandItem(_ context.Context) Item {
	return Item{
		Key:         "p31_douyin_brand",
		Title:       "抖店品牌 API",
		SettingsURL: "/settings/platforms?platform=douyin_shop",
		ImpactScope: "blocked_by_contract_verification",
		Status:      StatusAwaitingCredential,
		Summary:     "品牌接口仍等待平台契约确认；支持手工品牌 ID 输入",
	}
}

func (s *Service) p31WebhookSignatureItem(ctx context.Context) Item {
	it := Item{
		Key:         "p31_douyin_webhook_signature",
		Title:       "Webhook 签名契约版本",
		SettingsURL: "/settings/platforms?platform=douyin_shop",
	}
	appEnv := ""
	if s != nil && s.Config != nil {
		appEnv = s.Config.AppEnv
	}
	st := platformdouyin.NewDefaultContractGate(appEnv).Status(platformdouyin.CapDouyinWebhookSignatureV1)
	it.Summary = "douyin_webhook_signature_v1 · " + st.Status + " · " + st.Message
	if st.Status == platformdouyin.ContractStatusFixtureVerified {
		it.Status = StatusAwaitingCredential
	} else {
		it.Status = StatusCodeReady
	}
	return it
}

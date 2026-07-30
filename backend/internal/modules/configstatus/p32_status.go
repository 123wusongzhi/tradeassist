package configstatus

import (
	"context"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

func (s *Service) p32WebhookResolverItem(ctx context.Context) Item {
	it := Item{
		Key:         "p32_webhook_multishop_resolver",
		Title:       "Webhook 多店铺解析器",
		SettingsURL: "/settings/platforms?platform=douyin_shop",
		Status:      StatusCodeReady,
		Summary:     "WebhookShopResolver 已接入；按已验签 payload/header 中的平台店铺 ID 与 appKey 解析店铺绑定",
	}
	if s == nil || s.DB == nil {
		return it
	}
	var active int64
	_ = s.DB.WithContext(ctx).Model(&shop.Shop{}).
		Where("platform IN ? AND status = ? AND auth_status IN ? AND external_shop_id <> ''",
			[]string{"douyin_shop", "douyin"}, shop.StatusActive, []string{shop.AuthAuthorized, shop.AuthNeedCheck}).
		Count(&active).Error
	it.Summary = fmt.Sprintf("%s；当前有效抖店绑定 %d 个", it.Summary, active)
	return it
}

func (s *Service) p32WebhookAppBindingItem(_ context.Context) Item {
	return Item{
		Key:         "p32_webhook_app_secret_binding",
		Title:       "Webhook App / Secret 绑定",
		SettingsURL: "/settings/platforms?platform=douyin_shop",
		Status:      StatusCodeReady,
		Summary:     "Resolver 仅接收 bindingId/appKey 线索，不传递 Secret 明文；绑定不一致会拒绝处理",
	}
}

func (s *Service) p32WebhookTenantIsolationItem(_ context.Context) Item {
	return Item{
		Key:         "p32_webhook_tenant_isolation",
		Title:       "Webhook Tenant 隔离",
		SettingsURL: "/settings/config-status",
		Status:      StatusCodeReady,
		Summary:     "webhook_events 保存 tenantId/internalShopId/platformShopId；订单 Upsert 使用 resolver 输出的 tenant scope",
	}
}

func (s *Service) p32WebhookProductionFallbackItem(_ context.Context) Item {
	it := Item{
		Key:         "p32_webhook_production_fallback_disabled",
		Title:       "Production fallback 禁用",
		SettingsURL: "/settings/config-status",
		Status:      StatusCodeReady,
		Summary:     "staging/production 禁止 DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID 和 demo fallback",
	}
	if s != nil && s.Config != nil && config.IsStagingOrProduction(s.Config.AppEnv) {
		if strings.TrimSpace(s.Config.DouyinWebhookTestShopBindingID) != "" || s.Config.EnableDouyinWebhookDemoFallback {
			it.Status = StatusConfigError
			it.Summary = "当前环境启用了 Webhook fallback，配置校验应 fail-fast"
		}
	}
	return it
}

func (s *Service) p32WebhookConcurrencyItem(_ context.Context) Item {
	return Item{
		Key:         "p32_webhook_multishop_concurrency",
		Title:       "多店铺并发验证",
		SettingsURL: "/settings/config-status",
		Status:      StatusCodeReady,
		Summary:     "Webhook 幂等键与事件唯一键包含 tenant/platformShop 维度；worker 按事件行 ID 处理，避免同 eventId 跨店铺串扰",
	}
}

func (s *Service) p32RaceVerificationItem(_ context.Context) Item {
	return Item{
		Key:         "p32_linux_race_verification",
		Title:       "Linux Race 验证",
		SettingsURL: "/settings/config-status",
		Status:      StatusCodeReady,
		Summary:     "P3.2 race 已在 WSL2 Ubuntu 22.04 通过；见 docs/P3_2_RACE_TEST_REPORT.md；不等于真实抖店 E2E 或 Production Ready",
	}
}

package configstatus

import (
	"context"
	"strings"
)

// P3 Douyin adapter capability items.
// These items report code-implementation status, NOT E2E verified status.
// Real credential verification is deliberately deferred.

const (
	p3SettingsURL   = "/settings/platforms?platform=douyin_shop"
	p3ImpactScope   = "抖店 P3 适配器：代码已实现，等待真实凭证 E2E 验证"
	StatusCodeReady = "代码已实现"
)

func (s *Service) p3DouyinOAuthItem(ctx context.Context) Item {
	it := Item{
		Key:         "p3_douyin_oauth",
		Title:       "抖店 OAuth / 授权状态",
		SettingsURL: p3SettingsURL,
		ImpactScope: p3ImpactScope,
		NextAction:  "配置 App Key / App Secret / ServiceID 并完成店铺 OAuth 授权",
	}
	plain, err := s.readDouyinSettings(ctx)
	if err != nil {
		it.Status = StatusConfigError
		it.Summary = "读取抖店配置失败"
		return it
	}
	if strings.TrimSpace(plain["service_id"]) == "" {
		it.Status = StatusAwaitingCredential
		it.Summary = "service_id 未配置 — OAuth 授权 URL 无法生成"
		return it
	}
	if strings.TrimSpace(plain["redirect_uri"]) == "" {
		it.Status = StatusAwaitingCredential
		it.Summary = "redirect_uri 未配置"
		return it
	}
	it.Status = StatusCodeReady
	it.Summary = "OAuth 流程代码已实现（Redis state + DB state），等待真实凭证"
	return it
}

func (s *Service) p3DouyinTokenItem(_ context.Context) Item {
	return Item{
		Key:         "p3_douyin_token",
		Title:       "抖店 Token 刷新 / 版本锁",
		SettingsURL: p3SettingsURL,
		ImpactScope: p3ImpactScope,
		Status:      StatusCodeReady,
		Summary:     "EnsureFreshAccessSingleflight + TokenVersion 冲突检测已实现；等待真实凭证",
	}
}

func (s *Service) p3DouyinCatalogItem(_ context.Context) Item {
	return Item{
		Key:         "p3_douyin_catalog",
		Title:       "抖店类目同步",
		SettingsURL: p3SettingsURL,
		ImpactScope: p3ImpactScope,
		Status:      StatusCodeReady,
		Summary:     "category.list + attributes API 代码已实现；等待真实凭证验证类目树",
	}
}

func (s *Service) p3DouyinImageItem(_ context.Context) Item {
	return Item{
		Key:         "p3_douyin_image",
		Title:       "抖店图片上传 / 幂等缓存",
		SettingsURL: p3SettingsURL,
		ImpactScope: p3ImpactScope,
		Status:      StatusCodeReady,
		Summary:     "图片上传 + DouyinImageAsset 内容哈希去重已实现；等待真实凭证",
	}
}

func (s *Service) p3DouyinDraftItem(_ context.Context) Item {
	return Item{
		Key:         "p3_douyin_draft",
		Title:       "抖店商品草稿创建",
		SettingsURL: p3SettingsURL,
		ImpactScope: p3ImpactScope,
		Status:      StatusCodeReady,
		Summary:     "product.addV2 (commit=false) + 幂等防重 + 未知结果恢复已实现；不自动上架",
	}
}

func (s *Service) p3DouyinOrderItem(_ context.Context) Item {
	return Item{
		Key:         "p3_douyin_order",
		Title:       "抖店订单同步 / 详情",
		SettingsURL: p3SettingsURL,
		ImpactScope: p3ImpactScope,
		Status:      StatusCodeReady,
		Summary:     "order.searchList + order.orderDetail 已实现；等待真实凭证",
	}
}

func (s *Service) p3DouyinInventoryItem(_ context.Context) Item {
	return Item{
		Key:         "p3_douyin_inventory",
		Title:       "抖店库存查询 / 同步",
		SettingsURL: p3SettingsURL,
		ImpactScope: p3ImpactScope,
		Status:      StatusCodeReady,
		Summary:     "sku.syncStock (写) + product.detail SKU stock (读) 已实现；等待真实凭证",
	}
}

func (s *Service) p3DouyinCustomerItem(_ context.Context) Item {
	return Item{
		Key:         "p3_douyin_customer",
		Title:       "抖店客服消息",
		SettingsURL: p3SettingsURL,
		ImpactScope: "blocked_by_contract_verification — IM 接口合同核查后方可启用",
		Status:      StatusAwaitingCredential,
		Summary:     "客服消息 API 已阻断（blocked_by_contract_verification）；接口形状待合同确认",
		NextAction:  "通过抖店开放平台合同申请 IM 接口权限后再启用",
	}
}

func (s *Service) p3DouyinWebhookItem(ctx context.Context) Item {
	it := Item{
		Key:         "p3_douyin_webhook",
		Title:       "抖店 Webhook 签名验证",
		SettingsURL: p3SettingsURL,
		ImpactScope: p3ImpactScope,
	}
	plain, err := s.readDouyinSettings(ctx)
	if err != nil {
		it.Status = StatusConfigError
		it.Summary = "读取抖店配置失败"
		return it
	}
	if strings.TrimSpace(plain["app_secret"]) == "" {
		it.Status = StatusAwaitingCredential
		it.Summary = "app_secret 未配置 — Webhook 签名验证器已注册但处于 blocked_by_config 状态"
		return it
	}
	it.Status = StatusCodeReady
	it.Summary = "SHA1(appSecret + rawBody) 验证器已注册；事件路由已实现"
	return it
}

func (s *Service) readDouyinSettings(ctx context.Context) (map[string]string, error) {
	if s == nil || s.Settings == nil {
		return nil, nil
	}
	return s.Settings.PlainByGroup(ctx, 0, "platform_douyin_shop")
}

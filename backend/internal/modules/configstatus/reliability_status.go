package configstatus

import (
	"context"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/config"
)

func (s *Service) reliabilityFoundationItem(ctx context.Context) Item {
	it := Item{
		Key:         "reliability_foundation",
		Title:       "P2 可靠性基础",
		SettingsURL: "/settings/config-status",
	}
	env := config.EnvDevelopment
	if s.Config != nil {
		env = config.NormalizeEnv(s.Config.AppEnv)
	}
	var parts []string
	parts = append(parts, "幂等服务: ready")
	parts = append(parts, "任务租约: ready")
	parts = append(parts, "迁移锁: ready")
	if config.IsStagingOrProduction(env) && s.Config != nil && len(s.Config.CORSAllowedOrigins) == 0 {
		it.Status = StatusAbnormal
		it.Summary = "CORS 未配置"
		it.NextAction = "设置 CORS_ALLOWED_ORIGINS"
		return it
	}
	parts = append(parts, "CORS: configured")
	if s.Config != nil && !s.Config.AllowsLocalStorageProvider() {
		it.Status = StatusAbnormal
		it.Summary = strings.Join(parts, "；") + "；Storage fail-fast 已触发"
		it.NextAction = "切换 STORAGE_PROVIDER 为 cos/oss/s3/r2"
		return it
	}
	it.Status = StatusConfigured
	it.Summary = strings.Join(parts, "；")
	it.ImpactScope = "订单/库存/客服/刊登/AI 幂等与任务恢复"
	return it
}

func (s *Service) idempotencyServiceItem(ctx context.Context) Item {
	return Item{
		Key:     "idempotency_service",
		Title:   "幂等服务",
		Status:  StatusConfigured,
		Summary: fmt.Sprintf("统一 idempotency_records 表；scope+key 唯一约束；租约 %s", "2m"),
	}
}

func (s *Service) providerHealthItem(ctx context.Context) Item {
	it := Item{
		Key:         "provider_health",
		Title:       "Provider Health",
		SettingsURL: "/settings/config-status",
	}
	it.Status = StatusConfigured
	it.Summary = "缓存健康检查；失败不影响 liveness"
	return it
}

func (s *Service) circuitBreakerItem(ctx context.Context) Item {
	return Item{
		Key:     "circuit_breaker",
		Title:   "熔断状态",
		Status:  StatusConfigured,
		Summary: "Provider 维度 closed/open/half_open；配置状态中心展示",
	}
}

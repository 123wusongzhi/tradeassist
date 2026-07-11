package configstatus

import (
	"context"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/config"
	storagepub "github.com/trademind-ai/trademind/backend/internal/pkg/storagepublic"
)

// ProjectPhase describes release posture for the config status center header.
type ProjectPhase struct {
	Phase                    string   `json:"phase"`
	StatusLines              []string `json:"statusLines"`
	FinalAcceptance          string   `json:"finalAcceptance"`
	ProductionReady          bool     `json:"productionReady"`
	TagDeferred              bool     `json:"tagDeferred"`
	GrayReleaseAllowed       bool     `json:"grayReleaseAllowed"`
	InfrastructureFoundation bool     `json:"infrastructureFoundationReady"`
}

func defaultProjectPhase() ProjectPhase {
	return ProjectPhase{
		Phase: "Production Capability Development In Progress",
		StatusLines: []string{
			"Production Capability Development In Progress",
			"Core Reliability Foundation Ready",
			"Critical Domain Idempotency Integrated",
			"AI Result Application Idempotency Ready",
			"Webhook Receiver Foundation Ready",
			"All Production Workers Lease-Protected",
			"Infrastructure Foundation Ready",
			"MVP Demo Ready",
			"Tag deferred",
			"非 Production Ready",
			"抖店 Production Adapter Implemented",
			"Real Credential Verification Deferred",
			"只创建平台草稿 / 不自动上架",
			"代码实现完成不等于真实 E2E",
		},
		FinalAcceptance:          "Phase P10",
		ProductionReady:          false,
		TagDeferred:              true,
		GrayReleaseAllowed:       false,
		InfrastructureFoundation: true,
	}
}

func (s *Service) environmentItem(ctx context.Context) Item {
	it := Item{
		Key:    "environment",
		Title:  "运行环境",
		Status: StatusConfigured,
	}
	env := config.EnvDevelopment
	if s.Config != nil && strings.TrimSpace(s.Config.AppEnv) != "" {
		env = config.NormalizeEnv(s.Config.AppEnv)
	}
	it.Summary = fmt.Sprintf("APP_ENV=%s", env)
	if config.IsProduction(env) {
		it.ImpactScope = "生产环境：危险调试功能必须关闭"
		it.NextAction = "确认 ENABLE_DEMO_SEED / ENABLE_DEV_ROUTES 均为 false"
	} else {
		it.NextAction = "staging/production 部署前切换 APP_ENV 并填写公网 URL"
	}
	return it
}

func (s *Service) productionSafetyItem(ctx context.Context) Item {
	it := Item{
		Key:         "production_safety",
		Title:       "生产危险功能",
		SettingsURL: "/settings/config-status",
	}
	if s.Config == nil {
		it.Status = StatusConfigError
		return it
	}
	env := config.NormalizeEnv(s.Config.AppEnv)
	if !config.IsProduction(env) {
		it.Status = StatusConfigured
		it.Summary = fmt.Sprintf("非 production（%s）；Demo Seed=%t DevRoutes=%t", env, s.Config.EnableDemoSeed, s.Config.EnableDevRoutes)
		return it
	}
	var issues []string
	if s.Config.EnableDemoSeed {
		issues = append(issues, "Demo Seed 仍启用")
	}
	if s.Config.EnableDevRoutes {
		issues = append(issues, "Dev 路由仍启用")
	}
	if s.Config.EnableDebugEndpoints {
		issues = append(issues, "调试接口仍启用")
	}
	if s.Config.EnableSwagger {
		issues = append(issues, "Swagger 仍启用")
	}
	if len(issues) > 0 {
		it.Status = StatusAbnormal
		it.Summary = strings.Join(issues, "；")
		it.NextAction = "立即关闭并重启 API"
		return it
	}
	it.Status = StatusConfigured
	it.Summary = "危险调试功能已关闭"
	return it
}

func (s *Service) storageProductionItem(ctx context.Context) Item {
	it := Item{
		Key:         "storage_production",
		Title:       "Storage 生产边界",
		SettingsURL: "/settings/storage",
	}
	st, err := s.Settings.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		it.Status = StatusConfigError
		return it
	}
	kind := strings.TrimSpace(st["kind"])
	if kind == "" {
		kind = "local"
	}
	env := config.EnvDevelopment
	if s.Config != nil {
		env = config.NormalizeEnv(s.Config.AppEnv)
	}
	if kind == "local" && !config.AllowsLocalStorage(env) {
		it.Status = StatusAbnormal
		it.Summary = "production/staging 禁止使用 local Storage"
		it.NextAction = "切换为 COS/OSS/S3 等对象存储"
		return it
	}
	pub := storagepub.ResolvePublicBase(st)
	v := storagepub.ValidatePublicBase(pub, env)
	if !v.Valid && config.IsStagingOrProduction(env) {
		it.Status = StatusAwaitingPublicURL
		it.Summary = "public_base 未通过生产校验"
		it.NextAction = "配置 HTTPS 公网域名并执行公网访问测试"
		return it
	}
	it.Status = StatusConfigured
	it.Summary = fmt.Sprintf("Provider=%s；public_base 结构校验通过", kind)
	return it
}

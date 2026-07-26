package configstatus

import "context"

const (
	StatusIntegrated          = "已接入"
	StatusImplemented         = "已实现"
	StatusAwaitingPlatform    = "等待真实平台适配"
	StatusAwaitingFinalAccept = "等待最终环境验收"
)

func (s *Service) p22AITextApplyItem(ctx context.Context) Item {
	return Item{
		Key:         "p22_ai_text_apply_idempotency",
		Title:       "AI 文案 apply 幂等",
		Status:      StatusIntegrated,
		Summary:     "ai-text-apply 业务键 + 目标版本冲突保护 + Acquire/Complete",
		ImpactScope: "商品文案应用并发与重放",
		SettingsURL: "/settings/config-status",
	}
}

func (s *Service) p22AITextUndoItem(ctx context.Context) Item {
	return Item{
		Key:         "p22_ai_text_undo_idempotency",
		Title:       "AI 文案 undo 幂等",
		Status:      StatusIntegrated,
		Summary:     "ai-text-undo 业务键；基于 apply 记录补偿撤销",
		ImpactScope: "文案撤销并发与版本冲突",
		SettingsURL: "/settings/config-status",
	}
}

func (s *Service) p22AIImageApplyItem(ctx context.Context) Item {
	return Item{
		Key:         "p22_ai_image_apply_idempotency",
		Title:       "AI 图片 apply 幂等",
		Status:      StatusIntegrated,
		Summary:     "ai-image-apply 业务键含 slot；目标版本冲突保护",
		ImpactScope: "商品图片槽位应用",
		SettingsURL: "/settings/config-status",
	}
}

func (s *Service) p22AIImageUndoItem(ctx context.Context) Item {
	return Item{
		Key:         "p22_ai_image_undo_idempotency",
		Title:       "AI 图片 undo 幂等",
		Status:      StatusIntegrated,
		Summary:     "ai-image-undo 业务键；恢复原图片引用",
		ImpactScope: "图片撤销并发与版本冲突",
		SettingsURL: "/settings/config-status",
	}
}

func (s *Service) p22WebhookHTTPItem(ctx context.Context) Item {
	return Item{
		Key:         "p22_webhook_http_receiver",
		Title:       "Webhook HTTP 接收",
		Status:      StatusImplemented,
		Summary:     "POST /api/v1/webhooks/:platform/:eventType；快速 ACK + DB 持久化",
		ImpactScope: "平台事件入站",
		NextAction:  StatusAwaitingPlatform,
		SettingsURL: "/settings/config-status",
	}
}

func (s *Service) p22WebhookSignatureItem(ctx context.Context) Item {
	return Item{
		Key:         "p22_webhook_signature",
		Title:       "Webhook 签名校验",
		Status:      StatusImplemented,
		Summary:     "SignatureVerifier 注册表；production 禁止 bypass；测试 verifier 仅 dev/test",
		ImpactScope: "入站请求真实性",
		NextAction:  StatusAwaitingPlatform,
		SettingsURL: "/settings/config-status",
	}
}

func (s *Service) p22WebhookReplayItem(ctx context.Context) Item {
	return Item{
		Key:         "p22_webhook_replay",
		Title:       "Webhook 重放保护",
		Status:      StatusImplemented,
		Summary:     "时间戳窗口 + eventId/payload hash 去重 + 幂等记录",
		ImpactScope: "重复投递与时钟偏移",
		SettingsURL: "/settings/config-status",
	}
}

func (s *Service) p22CollectLeaseItem(ctx context.Context) Item {
	return Item{
		Key:         "p22_collect_worker_lease",
		Title:       "Collect Worker 租约",
		Status:      StatusIntegrated,
		Summary:     "tasklease TryClaimPendingOrRetrying + 续租 + 提交前 ValidateLease",
		ImpactScope: "采集任务旧 Worker 写回防护",
		SettingsURL: "/settings/config-status",
	}
}

func (s *Service) p22ImageTaskLeaseItem(ctx context.Context) Item {
	return Item{
		Key:         "p22_imagetask_worker_lease",
		Title:       "ImageTask Worker 租约",
		Status:      StatusIntegrated,
		Summary:     "tasklease 领取/续租/提交前校验；execution_id + lease_version",
		ImpactScope: "图片任务旧 Worker 写回防护",
		SettingsURL: "/settings/config-status",
	}
}

func (s *Service) p22CustomerSyncLeaseItem(ctx context.Context) Item {
	return Item{
		Key:         "p22_customersync_worker_lease",
		Title:       "CustomerSync Worker 租约",
		Status:      StatusIntegrated,
		Summary:     "tasklease TryClaim + finishCustomerSyncTask",
		ImpactScope: "客服同步旧 Worker 写回防护",
		SettingsURL: "/settings/config-status",
	}
}

func (s *Service) p22AllWorkersLeaseItem(ctx context.Context) Item {
	return Item{
		Key:         "p22_all_workers_lease_coverage",
		Title:       "全部生产 Worker 租约覆盖率",
		Status:      StatusIntegrated,
		Summary:     "ordersync / inventory / productpublish / collect / imagetask / customersync 均已接入任务行级租约",
		ImpactScope: "生产异步任务可靠性",
		NextAction:  StatusAwaitingFinalAccept,
		SettingsURL: "/settings/config-status",
	}
}

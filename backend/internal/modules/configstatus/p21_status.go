package configstatus

import "context"

func (s *Service) p21DomainIdempotencyItem(ctx context.Context) Item {
	return Item{
		Key:         "p21_domain_idempotency",
		Title:       "P2.1 关键写路径幂等接入",
		Status:      StatusConfigured,
		Summary:     "关键生产写路径已接入统一幂等服务（订单同步/导入、库存扣减/推送、刊登、客服、AI 批次）",
		ImpactScope: "并发重复请求与失败重试",
		SettingsURL: "/settings/config-status",
	}
}

func (s *Service) p21OrderSyncIdempotencyItem(ctx context.Context) Item {
	return Item{Key: "p21_order_sync_idempotency", Title: "订单同步幂等", Status: StatusConfigured, Summary: "order-sync-job 业务键 + idempotency.Service Acquire/Complete"}
}

func (s *Service) p21OrderImportIdempotencyItem(ctx context.Context) Item {
	return Item{Key: "p21_order_import_idempotency", Title: "订单导入幂等", Status: StatusConfigured, Summary: "order-import 键 + 平台订单唯一约束 + 陈旧更新忽略"}
}

func (s *Service) p21InventoryDeductItem(ctx context.Context) Item {
	return Item{Key: "p21_inventory_deduct_idempotency", Title: "库存扣减幂等", Status: StatusConfigured, Summary: "inventory-deduct 键 + business_event_key 唯一约束"}
}

func (s *Service) p21InventoryPushItem(ctx context.Context) Item {
	return Item{Key: "p21_inventory_push_idempotency", Title: "库存推送幂等", Status: StatusConfigured, Summary: "inventory-push 键 + 同版本任务去重"}
}

func (s *Service) p21PublishIdempotencyItem(ctx context.Context) Item {
	return Item{Key: "p21_publish_idempotency", Title: "刊登任务幂等", Status: StatusConfigured, Summary: "publish-batch / publish-enqueue 键"}
}

func (s *Service) p21CustomerSendItem(ctx context.Context) Item {
	return Item{Key: "p21_customer_send_idempotency", Title: "客服发送幂等", Status: StatusConfigured, Summary: "customer-send 键；平台发送前 Acquire"}
}

func (s *Service) p21AIBatchItem(ctx context.Context) Item {
	return Item{Key: "p21_ai_batch_idempotency", Title: "AI 批次幂等", Status: StatusConfigured, Summary: "ai-text-batch / ai-image-batch 键"}
}

func (s *Service) p21TaskLeaseItem(ctx context.Context) Item {
	return Item{
		Key:     "p21_task_lease",
		Title:   "任务行级心跳与租约",
		Status:  StatusConfigured,
		Summary: "heartbeat_at / execution_id / lease_version；续租与提交前校验",
	}
}

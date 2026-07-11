package idempotency

// Business scopes for unified idempotency records.
const (
	ScopeOrderSync      = "order_sync"
	ScopeOrderImport    = "order_import"
	ScopeInventory      = "inventory"
	ScopeInventoryPush  = "inventory_push"
	ScopePublish        = "publish"
	ScopeCustomerSend   = "customer_send"
	ScopeAIText         = "ai_text"
	ScopeAIImage        = "ai_image"
	ScopeWebhook        = "webhook"
	ScopeProductAIApply = "product_ai_apply"
	ScopeDouyinImage    = "douyin_image"
)

# 抖店契约能力门控 (ContractCapabilityGate)

实现：`backend/internal/providers/platform/douyinshop/contract_gate.go`

## 状态模型

| 状态 | 含义 |
|------|------|
| verified | 真实环境或明确契约确认 |
| fixture_verified | 仅 Fixture/Transport 测试 |
| blocked_by_contract_verification | 路径/Scope/字段未确认 |
| unsupported | 当前版本不支持 |
| disabled | 功能开关关闭 |

## 能力键

- `douyin_im_conversation_list` / `douyin_im_message_list` / `douyin_im_send` → blocked
- `douyin_brand_list` → blocked
- `douyin_webhook_signature_v1` → fixture_verified（production 拒绝未 verified）
- `douyin_order_webhook_events` → fixture_verified
- `douyin_inventory_query` / `douyin_product_draft_create` → fixture_verified

阻塞错误：`DOUYIN_CONTRACT_VERIFICATION_REQUIRED`

# 抖店客服消息适配器设计

## 当前状态

**blocked_by_contract_verification** — 抖店 IM 消息接口需通过合同申请后方可使用。

## 已实现内容

- `CustomerCapability` 接口（`douyinshop/customer.go`）
- `PullMessages` / `SendMessage`：返回 `CodeDouyinContractMismatch`
- `CustomerMessageEnvelope` DTO（用于 fixture 驱动的解析测试）
- `ParseCustomerMessageEnvelope`：synthetic 测试用，不适用于真实 API

## 接口形状（待合同确认）

以下字段名为推测，**不可用于真实 API 调用**：

```json
{
  "messages": [
    {
      "message_id": "...",
      "conversation_id": "...",
      "content": "...",
      "content_type": "text",
      "sender_type": "buyer"
    }
  ],
  "next_token": "..."
}
```

## 启用条件

1. 通过抖店开放平台申请 IM 接口权限
2. 在 settings `platform_douyin_shop.customer_message_api_enabled = true`
3. 配置消息拉取/推送接口路径
4. 移除 `contractMismatchError` 拦截

## MVP 约束

客服消息必须人工确认后才能发送，不自动外发（规则来自 `.cursorrules`）。

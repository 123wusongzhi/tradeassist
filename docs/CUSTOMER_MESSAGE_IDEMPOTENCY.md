# 客服消息幂等设计（P2）

> 人工确认后的平台外发消息必须 **至多成功一次**；客户端重试或网络超时不得产生 duplicate 平台消息记录。

## `clientMessageId` 约束

表 `customer_messages`：

| 字段 | 说明 |
| --- | --- |
| `client_message_id` | 客户端生成的稳定 ID（建议 UUID） |
| `conversation_id` | 会话 UUID |

PostgreSQL 部分唯一索引（P2 迁移）：

```sql
CREATE UNIQUE INDEX ux_customer_msg_client_id
 ON customer_messages (conversation_id, client_message_id)
 WHERE client_message_id IS NOT NULL AND client_message_id <> '';
```

同一会话内相同 `clientMessageId` 只能持久化一条消息行。

## API 行为

`POST /api/v1/customer/conversations/:id/send-platform-message`

请求体（节选）：

```json
{
  "reply": "您好，已为您查询物流…",
  "suggestionId": "…",
  "idempotencyKey": "optional-legacy",
  "clientMessageId": "client-uuid-001"
}
```

处理顺序（`customerchat/send_platform.go`）：

1. 校验 `reply` 非空。
2. `clientMessageId` 为空时 **回退** 为 `idempotencyKey`（兼容旧客户端）。
3. 若 `(conversation_id, client_message_id)` 已存在 → **直接返回已有行**，不再次调用平台 API。
4. 否则调用 `platform.CustomerMessage.SendMessage`，成功后事务写入消息并更新会话 `last_message_at`。

## 与统一幂等键的关系

推荐客户端始终传 `clientMessageId`；服务端幂等键模式：

```text
idempotency.CustomerSend(conversationId, clientMessageId)
→ customer-send:{conversationId}:{clientMessageId}
```

未来可接入 `idempotency.Service.Acquire` 覆盖「平台 API 已成功但 DB 写入失败」的窄窗口；当前以 DB 唯一索引 + 发送前查询为主路径。

## 平台侧幂等

Provider 请求携带：

- TikTok：`idempotency_key` body 字段
- Shopee / Lazada / Amazon：本地 `idempotencyLocal` 元数据或模板级键

平台键与 `clientMessageId` 应对齐或派生，避免平台重复发信。

## 失败场景

| 情况 | 行为 |
| --- | --- |
| 平台 API 失败 | 不写 `customer_messages`；记 `customer_failure_events` |
| DB 唯一冲突（并发双写） | 一方成功，另一方应读回已有行（需客户端重试 GET 或重复 POST） |
| 无 `clientMessageId` | 仅 `idempotencyKey` 时仍受唯一约束保护（若非空） |

## 前端与验收

发送前 `crypto.randomUUID()` 生成 ID 并固定到本次点击；重试复用同一 ID。相同 `clientMessageId` 二次 POST 返回同一消息行。

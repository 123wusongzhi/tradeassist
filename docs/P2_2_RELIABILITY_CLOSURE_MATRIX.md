# P2.2 Reliability Closure Matrix

> Phase: P2.2 — AI apply 幂等、Webhook HTTP 接收、全 Worker 租约统一  
> Audit date: 2026-07-11  
> Status: **Phase P2.2 Completed** · **Core Reliability Foundation Ready** · **非 Production Ready** · **Final Acceptance Deferred**

## Closure status (implemented)

| 能力 | 真实入口 | 事务 / 持久化边界 | 幂等方式 | 业务唯一键 | 租约方式 | 重复请求行为 | 旧 Worker 写回 | 状态 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| AI 文案 apply | `POST .../ai-text/items/:id/apply` → `applyOneItem` | product 事务 + item 状态 | `idempotency.Service` ScopeAIText + frozen version | `ai-text-apply:{batch}:{item}:{product}:{version}` | 幂等租约 | 重放 AlreadySucceeded；并发 InProgress | 生成写回 `WHERE status=running` | **implemented** |
| AI 文案 undo | `POST .../batches/:id/undo-applied` | product.UndoAIContent + item | `ai-text-undo:{applicationId}:{version}` | application_id | 幂等租约 | 重放 undone | — | **implemented** |
| AI 图片 apply | `POST .../ai-images/items/:id/apply` | 图片事务 / application / item | ScopeAIImage + slot + frozen ImageUpdatedAt | `ai-image-apply:{batch}:{item}:{product}:{version}:{slot}` | 幂等租约 | 重放 / InProgress | `WHERE status=running` | **implemented** |
| AI 图片 undo | `UndoApplied` → `undoOneApplication` | 恢复图片；set_main 恢复 previousBestMain | `ai-image-undo:{applicationId}:{version}` | application_id | 幂等租约 | 重放 undone | — | **implemented** |
| Webhook 接收 | `POST /api/v1/webhooks/:platform/:eventType` → Ingest + DB poll Worker | Acquire + ON CONFLICT；持久化后 ACK | `webhook:{platform}:{eventId}`；process=`webhook-process:…` | platform+event_id | 幂等租约 | duplicate ACK | noop 未知平台 | **implemented**（业务适配后置） |
| Collect Worker | `collect.RunCollectJob` | Claim → Collector → ImportDraft → finish | 任务创建侧；草稿导入侧 | task.id | **tasklease** TryClaimPendingOrRetrying | 租约过期可重领 | finish 守卫 execution_id | **implemented** |
| ImageTask Worker | `imagetask.ProcessQueuedTask` | Claim → Provider → persist | 任务级租约 | task.id | **tasklease** | 同 collect | finish 守卫 | **implemented** |
| CustomerSync Worker | `customersync.ProcessQueuedTask` | Claim → Pull → Sync | client_message_id + 租约 | task.id | **tasklease** | 重复创建多任务仍靠业务键 | finish 守卫 | **implemented** |
| OrderSync / Inventory / Publish | 既有 worker | finish* + lease | 已接入 | 业务键 | **tasklease** | 幂等重放 | finish ValidateLease / WHERE | **implemented** |

## Target state checklist

| 能力 | 目标 | 验收 |
| --- | --- | --- |
| AI text apply/undo | `idempotency.Service` + 目标版本冲突 | **已完成**：`AI_TEXT_*_VERSION_CONFLICT`；并发单测 |
| AI image apply/undo | 同上 + slot | **已完成**：`AI_IMAGE_*_VERSION_CONFLICT`；并发单测 |
| Webhook HTTP | 公开 POST + 签名/时间戳/重放/体限制/快 ACK/异步 | **已完成**（noop 处理；真实平台适配后置） |
| Collect / Image / CustomerSync | 统一 tasklease | **已完成**：stale worker 单测 |
| P2.1 warnings | 清零（webhook 路由、AI apply） | 由 `p2-1` / `p2-2` 扫描验证 |
| Docs + scan script | 设计文档 + `p2-2-reliability-closure-check.mjs` | **已完成**（本阶段文档收口） |

## Out of scope

最终人工验收、真实预发、抖店真实 E2E、生产灰度、打 tag、Production Ready、真实平台 Webhook 业务适配、`-race` CI 全量结果（见 [`P2_2_RACE_TEST_REPORT.md`](P2_2_RACE_TEST_REPORT.md) placeholder）。

## Design docs

- [`AI_RESULT_APPLY_IDEMPOTENCY.md`](AI_RESULT_APPLY_IDEMPOTENCY.md)
- [`AI_RESULT_UNDO_DESIGN.md`](AI_RESULT_UNDO_DESIGN.md)
- [`WEBHOOK_HTTP_RECEIVER_DESIGN.md`](WEBHOOK_HTTP_RECEIVER_DESIGN.md)
- [`WEBHOOK_SIGNATURE_AND_REPLAY_PROTECTION.md`](WEBHOOK_SIGNATURE_AND_REPLAY_PROTECTION.md)
- [`P2_2_WORKER_LEASE_ADOPTION_MATRIX.md`](P2_2_WORKER_LEASE_ADOPTION_MATRIX.md)
- [`P2_2_RACE_TEST_REPORT.md`](P2_2_RACE_TEST_REPORT.md)

# P2.1 Domain Idempotency Scan Report

Generated: 2026-07-13T06:15:22.508Z

**Overall:** passed (77 passed, 0 warnings, 0 failed)

> Phase P2.1 validates unified `idempotency.Service` on critical write paths and `tasklease` heartbeat/execution identity on async workers. This scan is static; it does not imply Production Ready or full acceptance green.

## Domain paths

| ID | Path | Source file |
| --- | --- | --- |
| path-order-sync | order sync job create | `backend/internal/modules/ordersync/idempotency_create.go` |
| path-order-import | order import/upsert | `backend/internal/modules/order/idempotency_import.go` |
| path-inventory-deduct | inventory deduct | `backend/internal/modules/inventory/idempotency_deduct.go` |
| path-inventory-push | inventory push | `backend/internal/modules/inventory/idempotency_push.go` |
| path-publish-batch | publish batch / enqueue | `backend/internal/modules/productpublish/idempotency_batch.go` |
| path-customer-send | customer message send | `backend/internal/modules/customerchat/send_platform.go` |
| path-ai-text-batch | AI text batch create | `backend/internal/modules/aiproducttext/service.go` |
| path-ai-image-batch | AI image batch create | `backend/internal/modules/aiproductimage/service.go` |
| path-webhook | webhook ingest | `backend/internal/modules/webhook/service.go` |

## Checks

| ID | Status | Message |
| --- | --- | --- |
| idem-module-service | passed | idempotency.Service exists |
| idem-module-scopes | passed | idempotency business scopes |
| idem-module-keys | passed | idempotency key builders |
| idem-module-decision | passed | idempotency Classify |
| idem-module-execute | passed | idempotency Execute helper |
| idem-model-unique | passed | idempotency unique constraint |
| router-idempotency-svc | passed | router creates shared idempotency.Service |
| router-wire-ordersync | passed | router wires idempotency to ordersync |
| router-wire-order | passed | router wires idempotency to order import |
| router-wire-inventory | passed | router wires idempotency to inventory |
| router-wire-publish | passed | router wires idempotency to productpublish |
| router-wire-customer | passed | router wires idempotency to customerchat |
| router-wire-ai-text | passed | router wires idempotency to aiproducttext |
| router-wire-ai-image | passed | router wires idempotency to aiproductimage |
| router-wire-webhook | passed | webhook handler wired in router |
| path-order-sync-file | passed | order sync job create exists |
| path-order-sync-acquire | passed | order sync job create Acquire via ScopeOrderSync |
| path-order-sync-complete | passed | order sync job create Complete/Fail lifecycle |
| path-order-sync-key | passed | order sync job create uses key pattern |
| path-order-import-file | passed | order import/upsert exists |
| path-order-import-acquire | passed | order import/upsert Acquire via ScopeOrderImport |
| path-order-import-complete | passed | order import/upsert Complete/Fail lifecycle |
| path-order-import-key | passed | order import/upsert uses key pattern |
| path-inventory-deduct-file | passed | inventory deduct exists |
| path-inventory-deduct-acquire | passed | inventory deduct Acquire via ScopeInventory |
| path-inventory-deduct-complete | passed | inventory deduct Complete/Fail lifecycle |
| path-inventory-deduct-key | passed | inventory deduct uses key pattern |
| path-inventory-push-file | passed | inventory push exists |
| path-inventory-push-acquire | passed | inventory push Acquire via ScopeInventoryPush |
| path-inventory-push-complete | passed | inventory push Complete/Fail lifecycle |
| path-inventory-push-key | passed | inventory push uses key pattern |
| path-publish-batch-file | passed | publish batch / enqueue exists |
| path-publish-batch-acquire | passed | publish batch / enqueue Acquire via ScopePublish |
| path-publish-batch-complete | passed | publish batch / enqueue Complete/Fail lifecycle |
| path-publish-batch-key | passed | publish batch / enqueue uses key pattern |
| path-customer-send-file | passed | customer message send exists |
| path-customer-send-acquire | passed | customer message send Acquire via ScopeCustomerSend |
| path-customer-send-complete | passed | customer message send Complete/Fail lifecycle |
| path-customer-send-key | passed | customer message send uses key pattern |
| path-ai-text-batch-file | passed | AI text batch create exists |
| path-ai-text-batch-acquire | passed | AI text batch create Acquire via ScopeAIText |
| path-ai-text-batch-complete | passed | AI text batch create Complete/Fail lifecycle |
| path-ai-text-batch-key | passed | AI text batch create uses key pattern |
| path-ai-image-batch-file | passed | AI image batch create exists |
| path-ai-image-batch-acquire | passed | AI image batch create Acquire via ScopeAIImage |
| path-ai-image-batch-complete | passed | AI image batch create Complete/Fail lifecycle |
| path-ai-image-batch-key | passed | AI image batch create uses key pattern |
| path-webhook-file | passed | webhook ingest exists |
| path-webhook-acquire | passed | webhook ingest Acquire via ScopeWebhook |
| path-webhook-complete | passed | webhook ingest Complete/Fail lifecycle |
| path-webhook-key | passed | webhook ingest uses key pattern |
| path-publish-enqueue | passed | publish enqueue idempotency |
| tasklease-pkg | passed | tasklease package exists |
| tasklease-try-claim | passed | tasklease TryClaim fields |
| tasklease-renew | passed | tasklease renewal API |
| tasklease-takeover | passed | tasklease stale takeover |
| tasklease-ordersync | passed | order sync worker lease |
| tasklease-inventory | passed | inventory sync worker lease |
| tasklease-publish | passed | product publish worker lease |
| task-model-ordersync | passed | order_sync_tasks model lease fields |
| task-model-inventory | passed | inventory_sync_tasks model lease fields |
| task-model-publish | passed | product_publish_tasks model lease fields |
| migrate-p21 | passed | P2.1 migration |
| migrate-p21-collect_tasks | passed | P2.1 migration adds heartbeat_at to collect_tasks |
| migrate-p21-image_tasks | passed | P2.1 migration adds heartbeat_at to image_tasks |
| migrate-p21-customer_message_sync_tasks | passed | P2.1 migration adds heartbeat_at to customer_message_sync_tasks |
| inventory-event-key-index | passed | inventory business_event_key partial unique index |
| path-ai-text-apply | passed | AI text apply idempotency referenced |
| path-ai-image-apply | passed | AI image apply idempotency referenced |
| doc-IDEMPOTENCY_DESIGN.md | passed | docs/IDEMPOTENCY_DESIGN.md exists |
| doc-P2_1_IDEMPOTENCY_ADOPTION_MATRIX.md | passed | docs/P2_1_IDEMPOTENCY_ADOPTION_MATRIX.md exists |
| doc-DOMAIN_IDEMPOTENCY_INTEGRATION.md | passed | docs/DOMAIN_IDEMPOTENCY_INTEGRATION.md exists |
| doc-TASK_LEASE_AND_HEARTBEAT_DESIGN.md | passed | docs/TASK_LEASE_AND_HEARTBEAT_DESIGN.md exists |
| doc-STALE_WORKER_PROTECTION.md | passed | docs/STALE_WORKER_PROTECTION.md exists |
| doc-CONCURRENT_WRITE_SAFETY.md | passed | docs/CONCURRENT_WRITE_SAFETY.md exists |
| idem-concurrency-test | passed | idempotency concurrency test exists |
| tasklease-test | passed | tasklease unit test exists |

## Run

```bash
node scripts/p2-1-domain-idempotency-check.mjs
```

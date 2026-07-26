# P2.2 Reliability Closure Scan Report

Generated: 2026-07-13T09:30:18.192Z

**Overall:** passed (59 passed, 0 warnings, 0 failed)

> Phase P2.2 validates AI apply/undo idempotency, Webhook HTTP receiver (signature / replay / ACK / async worker), and tasklease adoption on six workers. This scan is **static**; it does **not** imply Production Ready, gray release, or real platform E2E. Race results: see [`P2_2_RACE_TEST_REPORT.md`](P2_2_RACE_TEST_REPORT.md) (WSL2 Linux race passed 2026-07-11).

## Sections

| Section | Status |
| --- | --- |
| aiApply | passed |
| webhook | passed |
| workers | passed |
| race | passed |
| docs | passed |

## Checks

| ID | Status | Message |
| --- | --- | --- |
| ai-text-idempotency-apply | passed | AI text idempotency_apply.go exists |
| ai-text-apply-service | passed | AI text apply uses idempotency.Service (AITextApply + Acquire) |
| ai-text-undo | passed | AI text undo uses AITextUndo |
| ai-text-version-conflict | passed | AI text version conflict codes |
| ai-image-idempotency-apply | passed | AI image idempotency_apply.go exists |
| ai-image-apply-service | passed | AI image apply uses AIImageApply + Acquire |
| ai-image-undo | passed | AI image undo uses AIImageUndo |
| ai-image-version-conflict | passed | AI image version conflict codes |
| keys-apply-builders | passed | idempotency key builders for apply/undo/webhook-process |
| webhook-register-public | passed | Webhook RegisterPublic route |
| router-webhook-register | passed | router.go wires webhook.RegisterPublic |
| webhook-body-limit | passed | Webhook body limit (MaxBytesReader / maxPayload) |
| webhook-max-body-config | passed | WEBHOOK_MAX_BODY_KB / MaxPayloadBytes config |
| webhook-signature-verifier | passed | SignatureVerifier exists |
| webhook-clock-skew | passed | MaxClockSkew / timestamp validation |
| webhook-replay | passed | Replay protection (duplicate / OnConflict) |
| webhook-fast-ack | passed | Fast ACK (accepted / Ingest) |
| webhook-async-process | passed | Async ProcessEvent / ProcessQueuedEvents |
| webhook-start-worker | passed | Webhook StartWorker |
| webhook-bypass-forbidden | passed | Production signature bypass forbidden |
| webhook-sensitive-logging | passed | Webhook modules avoid logging raw secrets; truncateSummary present |
| worker-collect-claim | passed | collect uses tasklease claim |
| worker-collect-validate-finish | passed | collect ValidateLease + finishCollectTask |
| worker-collect-renewal | passed | collect heartbeat renewal / execution_id |
| worker-collect-stale-test | passed | collect lease_stale_worker_test.go exists |
| worker-imagetask-claim | passed | imagetask uses tasklease claim |
| worker-imagetask-validate-finish | passed | imagetask ValidateLease + finishImageTask |
| worker-imagetask-renewal | passed | imagetask heartbeat renewal / execution_id |
| worker-imagetask-stale-test | passed | imagetask lease_stale_worker_test.go exists |
| worker-customersync-claim | passed | customersync uses tasklease claim |
| worker-customersync-validate-finish | passed | customersync ValidateLease + finishCustomerSyncTask |
| worker-customersync-renewal | passed | customersync heartbeat renewal / execution_id |
| worker-customersync-stale-test | passed | customersync lease_stale_worker_test.go exists |
| worker-ordersync-claim | passed | ordersync uses tasklease claim |
| worker-ordersync-validate-finish | passed | ordersync ValidateLease + finishOrderSyncTask |
| worker-ordersync-renewal | passed | ordersync heartbeat renewal / execution_id |
| worker-inventory-claim | passed | inventory uses tasklease claim |
| worker-inventory-validate-finish | passed | inventory ValidateLease + finishInventorySyncTask |
| worker-inventory-renewal | passed | inventory heartbeat renewal / execution_id |
| worker-productpublish-claim | passed | productpublish uses tasklease claim |
| worker-productpublish-validate-finish | passed | productpublish ValidateLease + finishProductPublishTask |
| worker-productpublish-renewal | passed | productpublish heartbeat renewal / execution_id |
| stale-collect | passed | lease_stale_worker_test.go exists |
| stale-imagetask | passed | lease_stale_worker_test.go exists |
| stale-customersync | passed | lease_stale_worker_test.go exists |
| test-ai-text-apply | passed | AI text apply_idempotency_test.go exists |
| test-ai-image-apply | passed | AI image apply_idempotency_test.go exists |
| test-webhook | passed | Webhook handler_test.go exists |
| doc-race-report | passed | P2_2_RACE_TEST_REPORT.md exists |
| doc-AI_RESULT_APPLY_IDEMPOTENCY.md | passed | docs/AI_RESULT_APPLY_IDEMPOTENCY.md exists |
| doc-AI_RESULT_UNDO_DESIGN.md | passed | docs/AI_RESULT_UNDO_DESIGN.md exists |
| doc-WEBHOOK_HTTP_RECEIVER_DESIGN.md | passed | docs/WEBHOOK_HTTP_RECEIVER_DESIGN.md exists |
| doc-WEBHOOK_SIGNATURE_AND_REPLAY_PROTECTION.md | passed | docs/WEBHOOK_SIGNATURE_AND_REPLAY_PROTECTION.md exists |
| doc-P2_2_WORKER_LEASE_ADOPTION_MATRIX.md | passed | docs/P2_2_WORKER_LEASE_ADOPTION_MATRIX.md exists |
| doc-P2_2_RACE_TEST_REPORT.md | passed | docs/P2_2_RACE_TEST_REPORT.md exists |
| doc-P2_2_RELIABILITY_CLOSURE_MATRIX.md | passed | docs/P2_2_RELIABILITY_CLOSURE_MATRIX.md exists |
| p21-warning-webhook-router | passed | P2.1 former warning cleared: webhook route in router |
| p21-warning-ai-text-apply | passed | P2.1 former warning cleared: AI text apply idempotency |
| p21-warning-ai-image-apply | passed | P2.1 former warning cleared: AI image apply idempotency |

## Run

```bash
node scripts/p2-2-reliability-closure-check.mjs
node scripts/p2-1-domain-idempotency-check.mjs
```

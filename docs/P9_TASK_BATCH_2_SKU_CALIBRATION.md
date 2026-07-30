# P9 Task Batch 2 SKU Calibration and Manual Binding

Status: **completed locally; PostgreSQL integration deferred to P9 final development closure**

```text
batchId=P9-TASK-BATCH-2
phase=P9
phaseStatus=In Progress
changesCommitted=false
stagedFileCount=0
workingTreeDirty=true
productionReady=false
postgresIntegrationStatus=not_run
postgresIntegrationPassed=false
postgresIntegrationDeferredTo=P9_Final_Development_Closure
p9FinalClosureBlocker=true
```

## Scope

Completed Batch 2 tasks:

- `P9-601` SKU Identifier Normalization
- `P9-602` Exact Identifier Matching
- `P9-603` Candidate Scoring and Explainability
- `P9-604` Calibration Threshold Policy
- `P9-605` Manual Binding Fallback Service
- `P9-606` Calibration Service Verification

Implemented only SKU calibration and manual binding domain services under `backend/internal/modules/inventorysyncp9`. Batch 2 reuses the Batch 1 domain and persistence contracts and adds deterministic service logic plus immutable manual decision history.

## Implementation Evidence

- Normalization version: `sku-normalization-v1`
- Calibration version: `1`
- Threshold policy version: `calibration-threshold-policy-v1`
- Match strategies: `exact_barcode`, `exact_sku_code`, `normalized_barcode`, `normalized_sku_code`
- Candidate ordering: confidence descending, match strategy priority ascending, local SKU ID ascending
- Score range: basis points `0..10000`
- Manual binding authorization: required; missing authorizer denies by default
- Manual binding decisions: immutable `p9_manual_binding_decisions` history rows
- Idempotency: request and decision payload fingerprints reject conflicting replay
- Optimistic concurrency: expected revision required for confirm/reject
- Transactions: calibration persistence and manual confirm/reject are domain transactions

## Validation Evidence

```text
gofmtTouchedGoFiles=passed
inventorysyncp9ModuleTests=passed
inventorysyncp9RaceTests=passed
p9EntryFixture=passed
p9PlanFixture=passed
p9Batch1DomainPersistenceFixture=passed
p9Batch2SKUCalibrationFixture=passed
gitDiffCheck=passed
TEST_DATABASE_URL=unset
postgresIntegrationStatus=not_run
postgresIntegrationPassed=false
postgresIntegrationDeferredTo=P9_Final_Development_Closure
p9FinalClosureBlocker=true
```

PostgreSQL 未验证不阻止本地 Batch 2 开发，但在 P9 最终关闭前必须完成。此项不得延期到 P10，因为这是开发数据库兼容性验证，不是生产验收。

## Boundary

```text
syncOrchestratorImplemented=false
syncWorkerImplemented=false
cronImplemented=false
tickerImplemented=false
queueConsumerImplemented=false
apiImplemented=false
httpHandlerImplemented=false
ginRouterImplemented=false
restApiImplemented=false
adminUiImplemented=false
frontendApiClientImplemented=false
realDouyinProviderImplemented=false
oauthImplemented=false
realCredentialsEnabled=false
realPlatformNetworkEnabled=false
realPlatformReadEnabled=false
realPlatformWriteEnabled=false
realInventoryReadEnabled=false
realInventoryWriteEnabled=false
aiMatchingImplemented=false
vectorMatchingImplemented=false
llmMatchingImplemented=false
automaticPublishEnabled=false
automaticListingEnabled=false
p10BoundaryPreserved=true
productionReady=false
p9Complete=false
```

Batch 2 does not authorize P9-701 through P9-1105, sync orchestration, workers, API, Admin UI, real Douyin/OAuth/credentials, real platform network, real inventory reads/writes, release, tag, P9 completion, or Production Ready.

## PostgreSQL Revalidation

The original Batch evidence remains unchanged: PostgreSQL was not run because `TEST_DATABASE_URL` was unset. The Batch was revalidated later against an isolated PostgreSQL test database.

```text
initialPostgresVerification.status=not_run
initialPostgresVerification.reason=TEST_DATABASE_URL_not_set
postgresRevalidation.status=passed
postgresRevalidation.evidencePath=docs/P9_POSTGRESQL_INTEGRATION_CLOSURE.md
postgresRevalidation.gateReportPath=docs/P9_POSTGRESQL_INTEGRATION_CLOSURE_GATE.md
postgresRevalidation.verifiedAt=2026-07-30T07:47:32.008Z
postgresRevalidation.runId=p9pg-20260730074632-3b1bbb38
postgresRevalidation.runtimeSummarySha256=bf2427b1a6e2961b19298afbf1d77784ef12333cbe465a410d96a441d77f23f6
currentPostgresIntegrationStatus=passed
```

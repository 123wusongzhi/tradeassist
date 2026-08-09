# P9 Task Batch 1 Domain Persistence

Status: **completed locally; verified with known planning gate boundary**

```text
batchId=P9-TASK-BATCH-1
phase=P9
phaseStatus=In Progress
baseCheckpoint=1ac652ac4797eee636bf615f1e9ed272f2b82f84
changesCommitted=false
checkpointCreated=false
stagedFileCount=0
workingTreeDirty=true
productionReady=false
```

## Scope

Completed Batch 1 tasks:

- `P9-501` Inventory Sync Run Model
- `P9-502` Inventory Snapshot Model
- `P9-503` SKU Binding Model
- `P9-504` SKU Binding Calibration Model
- `P9-505` Manual Binding Fallback Model
- `P9-506` Migration, Repository and Persistence Verification

Implemented only the Batch 1 domain and persistence foundation under `backend/internal/modules/inventorysyncp9` and registered its migration through `backend/internal/database/migrate.go`.

Not implemented in this batch:

- Calibration service or matching algorithm service
- Automatic confirmation threshold
- Inventory sync orchestrator
- Background worker, cron, ticker, or queue consumer
- HTTP handler, Gin route, REST API, Admin UI, or frontend API client
- Real Douyin provider, OAuth client, real access token, real credential access, real platform network, real inventory read, or real inventory write

## Conventions

| Item | Decision |
| --- | --- |
| Domain model convention | GORM module models under `backend/internal/modules/inventorysyncp9` |
| Repository convention | Module-local GORM repositories with stable sentinel domain errors |
| Migration convention | `AutoMigrate` plus explicit P9 indexes, Postgres constraints, and SQLite/Postgres immutable triggers |
| ID convention | Shared UUID v4 hard-delete base (`model.HardDeleteBase`) |
| Tenant isolation convention | Every repository method requires `tenant_id`; cross-model references verify tenant/shop/local SKU ownership before writes |
| JSON convention | `gorm.io/datatypes.JSON` with safe JSON normalization and sensitive-key rejection |
| Error convention | Stable domain errors without SQLSTATE, SQL, connection string, or database stack leakage |
| Test database convention | Real GORM SQLite database integration tests using temporary files; no SQL mock layer |

## Schema Evidence

### `p9_inventory_sync_runs`

Purpose: tenant-scoped persistence model for an inventory sync run status record.

Key fields and constraints:

- `tenant_id`, `shop_connection_id`, `platform`, `provider_mode`, `status`
- safe cursor/checkpoint/error metadata JSON fields
- `snapshot_count`, `calibration_count`, `manual_request_count`
- `request_id`, `idempotency_key_hash`, `input_fingerprint`, `revision`
- `started_at`, `finished_at`
- unique tenant/id index
- partial unique tenant/idempotency index for non-empty idempotency hashes
- Postgres checks for status, provider mode, non-negative counts, hash format, and time ordering

### `p9_inventory_snapshot_items`

Purpose: immutable historical inventory observation rows scoped to tenant and run.

Key fields and constraints:

- `tenant_id`, `inventory_sync_run_id`, `shop_connection_id`, `platform`
- remote product/SKU identifiers, safe display fields, and integer quantities
- `observed_at`, `source_updated_at`, `payload_hash`, `safe_metadata`
- unique `(tenant_id, inventory_sync_run_id, external_sku_id)`
- Postgres quantity/hash checks
- GORM hooks plus SQLite/Postgres triggers reject update/delete
- full raw provider responses are not stored

Quantity relationship contract:

```text
quantityRelationshipContract=provider_defined_safe_observation
availableQuantity.nonNegative=true
reservedQuantity.nonNegative=true
totalQuantity.nonNegative=true
rawProviderPayloadStored=false
```

### `p9_sku_bindings`

Purpose: revisioned remote SKU to local SKU binding records.

Key fields and constraints:

- `tenant_id`, `shop_connection_id`, `platform`
- remote product/SKU identifiers
- `local_product_id`, `local_sku_id`
- `binding_source`, `binding_status`, `confidence`, `calibration_version`, `revision`
- confirmer metadata
- partial unique current confirmed binding per `(tenant_id, shop_connection_id, external_sku_id)`
- no global one-to-one restriction on local SKU

### `p9_sku_binding_calibrations`

Purpose: immutable candidate evidence for SKU binding calibration.

Key fields and constraints:

- `tenant_id`, `inventory_sync_run_id`, `inventory_snapshot_item_id`, `external_sku_id`
- candidate local product/SKU IDs
- `match_strategy`, `confidence`, `score_breakdown`, `reason_codes`, `calibration_version`, `status`, `input_fingerprint`
- unique `(tenant_id, inventory_sync_run_id, inventory_snapshot_item_id, candidate_local_sku_id, calibration_version)`
- confidence basis points constrained to `0..10000`
- GORM hooks plus SQLite/Postgres triggers reject update/delete
- stores candidate evidence only; no matching algorithm or auto-confirm policy is implemented

### `p9_manual_binding_requests`

Purpose: pending/resolved manual fallback persistence row.

Key fields and constraints:

- `tenant_id`, `inventory_sync_run_id`, `inventory_snapshot_item_id`, `shop_connection_id`, `external_sku_id`
- `status`, `reason_code`, `candidate_count`
- suggested/selected local SKU references
- assignment/resolver metadata
- `request_id`, `idempotency_key_hash`, `input_fingerprint`, `revision`
- unique `(tenant_id, request_id)`
- partial unique pending request per `(tenant_id, shop_connection_id, external_sku_id)`
- partial unique tenant/idempotency index for non-empty idempotency hashes

## Repository Methods

Inventory Sync Run:

- `Create`
- `GetByID`
- `GetByIdempotency`
- `UpdateStatusWithRevision`

Inventory Snapshot:

- `CreateBatch`
- `ListByRun`
- `GetByRunAndExternalSKU`
- `CountByRun`

SKU Binding:

- `CreateProposed`
- `GetCurrentConfirmed`
- `ListByExternalSKU`
- `ListByLocalSKU`
- `TransitionWithRevision`

SKU Binding Calibration:

- `CreateBatch`
- `ListBySnapshot`
- `ListByRun`
- `GetBestCandidate`

Manual Binding Request:

- `Create`
- `GetByID`
- `GetPendingByExternalSKU`
- `ListPending`
- `ResolveWithRevision`

## Validation Evidence

```text
tenantIsolationPassed=true
idempotencyTestsPassed=true
optimisticConcurrencyTestsPassed=true
immutableHistoryTestsPassed=true
batchAtomicityTestsPassed=true
repositoryTestsPassed=true
migrationTestsPassed=true
sqliteIntegrationTestsPassed=true
raceStatus=passed
dataRaces=0
backendTestSuite=passed
p9Batch1ScopeGate=passed
p9Batch1DomainPersistenceGate=passed
architectureBoundaryCheck=passed_new_violations_0
sensitiveDiffScan=passed_findings_0
contractTests=passed
touchedGoFilesGofmt=passed
postgresIntegrationStatus=not_executed_test_database_url_not_set
p9EntryGate=blocked_discovery_head_match_expected_planning_gate_boundary
p9PlanGate=failed_discovery_head_match_expected_planning_gate_boundary
qualityBackend=failed_existing_full_repo_gofmt_baseline_456_files_touched_go_files_passed
architectureAffected=failed_architecture_test_invalid_token_existing_vitest_import_issue
fullVerificationStatus=completed_with_known_planning_gate_boundary
```

Covered local SQLite-backed tests:

- sync run idempotency replay and payload conflict
- sync run revision increment, stale revision conflict, and terminal-to-running rejection
- snapshot duplicate external SKU rejection
- snapshot tenant isolation and immutable update/delete rejection
- binding proposed-to-confirmed transition, revision conflict, and current confirmed uniqueness
- calibration batch insert, duplicate candidate rejection, immutable update/delete rejection, and atomic rollback
- manual binding idempotency replay, payload conflict, pending uniqueness, and concurrent resolution single-winner behavior
- scope protection against API/Admin/worker/provider/credential implementation

## Stable Error Boundary

Stable domain errors implemented for Batch 1:

- `validation_error`
- `not_found`
- `tenant_mismatch`
- `revision_conflict`
- `state_conflict`
- `duplicate_external_sku`
- `binding_conflict`
- `binding_not_confirmed`
- `manual_binding_already_pending`
- `manual_binding_already_resolved`
- `idempotency_payload_conflict`
- `immutable_record`

Repository methods normalize database failures to stable errors and do not expose SQLSTATE, SQL text, connection strings, or database stack traces.

## Boundary

```text
calibrationServiceImplemented=false
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
shopAuthTokenCredentialFieldsRead=false
rawProviderResponseStored=false
automaticPublishEnabled=false
automaticListingEnabled=false
productionReady=false
p10BoundaryPreserved=true
```

## Next Batch Boundary

`P9-601` through `P9-606` remain **notStarted**. Batch 1 completion does not mean P9 is complete, production-ready, real Douyin sync capable, or authorized for APIs/Admin UI/workers/services.

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

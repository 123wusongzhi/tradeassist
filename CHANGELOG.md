# Changelog

All notable changes to TradeMind are documented here.

This project follows a lightweight changelog format before the first stable release. Keep entries short, grouped by date or version, and link large changes back to the relevant PR when available.

## Unreleased

### Phase P7-C2 - Runtime evidence and Linux race closure attempt (2026-07-13)

- Added P7-C2 isolated runtime environment start/stop, Medium dataset resume drill, WSL/Linux race, capability evidence, pagination runtime, query-plan, N+1, and final closure gate scripts.
- Executed Medium Dataset Resume Drill against isolated WSL PostgreSQL 14: `plannedRows=1,900,150`, interrupted at `3,150` rows, resumed to `actualRows=1,900,150`, `duplicateRows=0`, idempotency rerun inserted `0` rows, fingerprint stable.
- Executed WSL2 Linux race matrix: baseline `go mod verify`, `go test ./...`, `go build ./cmd/server/... ./cmd/p7load`, plus 11/11 mapped race package groups passed with `dataRaces=0` and `deadlocks=0`.
- Normalized mandatory capability evidence from 33 partial items to 11 remaining partial/blocking items, and updated P7-C closure reports to reject stale or shallow evidence.
- Status: Phase P7-C2 remains incomplete; six business keyset cursor pagination runtime checks fail, Query Plan and N+1 are blocked on pagination evidence, provider concurrency limit/adaptive slowdown/permission cache invalidation still lack code evidence; no load/soak/P8/tag and not Production Ready.

### Phase P7-C - Capability closure preflight (2026-07-13)

- Added signed cursor encoding for P7 pagination and a production guard requiring `PAGINATION_CURSOR_SIGNING_KEY`.
- Added `backend/internal/pkg/cache` with bounded TTL entries, invalidation, negative cache, tenant-scoped keys and singleflight loading.
- Added guarded `p7load` interruption flags for dataset resume drills: `--fail-after-batches` and `--stop-after-rows`.
- Added P7-C resume, pagination runtime, query-plan, N+1, Linux race and closure gate scripts plus generated incomplete/blocking evidence reports.
- Status: Phase P7-C remains incomplete; 33 mandatory partial items remain in the source audit, Medium runtime verification and Linux race still need execution in the isolated environment; no tag and not Production Ready.

### Phase P7-V - Medium dataset and closure gate hardening (2026-07-13)

- Reworked `p7load` from a plan-only recorder into a guarded PostgreSQL dataset loader with batch writes, idempotent reruns, run-scoped cleanup, real row counts and deterministic dataset fingerprints.
- Added P7-V scripts for isolated performance environment startup/cleanup, capability audit, k6 load harness, soak harness, WSL/Linux race matrix and final closure gating.
- Executed an isolated local PostgreSQL Medium dataset run: `plannedRows=1,900,150`, `insertedRows=1,900,150`, `actualRows=1,900,150`, `failedRows=0`; idempotency rerun passed with unchanged fingerprint.
- Fixed a real migration/model issue exposed by the Medium run by pinning `product_sku_id` / `external_sku_id` GORM column names on order and inventory models.
- Status: Phase P7-V remains incomplete; capability audit has partial items, k6 is unavailable, Soak is not executed for 30 minutes, regression baseline/current comparison is incomplete, and Linux Race is blocked by missing `internal/pkg/cache` coverage; no tag and not Production Ready.

### Phase P7 - Performance and capacity foundation started (2026-07-13)

- Added P7 config and production guards for performance-test mode, datasets, pagination, DB pool, worker capacity, rate limits, cache, export and pprof.
- Wired DB pool settings, bounded pagination helpers, local token-bucket HTTP rate limiting, P7 performance/capacity tables and guarded dataset-plan tooling.
- Added P7 static capacity and regression gate scripts, P7 docs, dashboards, runbooks and env/API documentation.
- Status: Phase P7 closure verification incomplete; real medium dataset, load test, soak test, Linux race, distributed limiter and management UI remain pending; no tag and not Production Ready.

### Phase P6-VR - Linux race remediation and P6 closure (2026-07-13)

- Installed and verified WSL2 Ubuntu Go `go1.25.12` for the repository `go 1.25.0` requirement, with CGO and GCC available.
- Hardened `scripts/p6-v-linux-race.mjs` to distinguish environment blockers from test/build/race failures and to record real command exit codes, stdout/stderr tails, run ID, baseline, per-package race results, and combined race matrix.
- Hardened `scripts/p6-v-final-closure-gate.mjs` to validate the Linux race report fields instead of accepting a shallow status.
- Executed WSL2 Linux baseline and race matrix: `go mod verify`, `go test ./...`, `go build ./cmd/server/... ./cmd/p6drill`, 9 race package groups, and combined race matrix all passed with `dataRaces=0`.
- Status: Phase P6 fully closed; real production backup, restore, PITR, release, telemetry, and Douyin credential verification remain deferred; no tag and not Production Ready.

### Phase P6-V - Isolated restore and release rollback drills (2026-07-13)

- Added `p6drill` CLI plus P6-V restore, release rollback, race, and final gate scripts.
- Executed isolated PostgreSQL restore drill with real `pg_dump`, encrypted artifact verification, `pg_restore --list`, `pg_restore`, integrity comparison, audit chain validation, and six negative restore safety tests.
- Executed isolated release rollback drill through the existing release service and confirmed database auto-restore/down migration boundaries.
- Fixed fresh PostgreSQL migration ordering for Douyin 10.2 indexes and stabilized operation log hash-chain timestamps for restore verification.
- Follow-up P6-VR remediated the WSL Go 1.18.1 environment blocker and passed Linux race verification.

### Phase P6 - Backup, restore, release rollback and DR foundation (2026-07-13)

- Added P6 backend modules for backup, restore safety gates, release state machine, application rollback boundary, and DR drill status.
- Added chunked AES-GCM backup encryption, checksum helpers, safe PostgreSQL command builders, PITR target validation, WAL continuity checks, and release manifest helpers.
- Added P6 environment variables, RBAC permissions, Admin ops pages, API docs, dashboards, runbooks, and static gate script.
- Follow-up P6-V and P6-VR closure evidence passed; real production verification remains deferred.

### Phase P5-V - Standard OTLP closure gate (2026-07-13)

- Replaced the tracing custom HTTP span JSON payload with a standard OTLP/HTTP JSON TraceService export request.
- Added strict Mock Collector protocol tests, retry/failure classification coverage, golden OTLP fixture, and P5-V final observability gate script.
- Added P5-V audit, dependency matrix, protocol implementation, protocol test, race, frontend/collector, demo acceptance, and final observability reports.
- Status remains closure-incomplete until Linux race, frontend/collector builds, two demo acceptance runs, and all P1-P5-V scans pass.

### Phase P5.2 - Business instrumentation observability (2026-07-12)

- Wired real metrics from HTTP client, webhook, order sync, inventory, AI text, AI image, file scan, security, and auth services into the shared metrics catalog.
- Added P5.2 observability unit tests, static scan, smoke script, metric/dashboard validation docs, and final report.
- Status remains closure-incomplete for standard OTLP HTTP exporter, Linux race verification, demo acceptance, and real telemetry/alert-channel verification.

### Phase P4-R - Demo regression stabilization (2026-07-12)

- Added isolated Go test runner for `demo:auto-acceptance` so backend `.env` / process env does not pollute unit tests.
- Added P4-R seed result semantics, demo dataset/template version markers, production seed guards, and verifier scripts.
- Added AI image trial timeout classification with last completed stage and sanitized reason codes.
- Added P4-R closure scan and documentation. Run 2 / Run 3 remain pending until executed against a running local demo backend.

### Phase P3.2 - Multi-shop Douyin webhook routing (2026-07-11)

- Added `WebhookShopResolver` for Douyin webhook tenant/shop/app/binding resolution before ingest and order processing.
- Scoped webhook persistence and idempotency by `platform + tenant_id + platform_shop_id + event_id`; async worker processes concrete event rows by ID.
- Removed legacy implicit single-authorized-shop fallback from Douyin order webhooks; staging/production reject webhook fallback env vars.
- Added config-status and task-center categories for shop resolution, tenant mismatch, app/binding mismatch, expired/revoked authorization, and race verification status.
- Added P3.2 static scan and docs. Linux WSL2 race verification passed; real Douyin credential E2E remains deferred.

### Phase P3.1 — Douyin closure (2026-07-11)

- Order webhook handler wired to unified `UpsertPlatformOrder` (shared with polling sync).
- Order stale protection: `platform_revision` / `platform_updated_at` + lifecycle validator.
- `ContractCapabilityGate` (IM, brand, webhook signature v1); production rejects fixture-only signature.
- AI text/image apply idempotency reconciliation; `token_version` incremented on OAuth refresh persist.
- `scripts/p3-1-douyin-closure-check.mjs`; P3.1 docs. Real credential E2E still deferred.

### Phase P3 — 抖店 Production Adapter (2026-07-11)

- **DouyinProvider facade**: thin interface wrapping all Client capabilities (Auth/Shop/Catalog/Image/Product/Order/Inventory/Customer).
- **Error classification**: `ErrorClass` enum (auth_error / rate_limited / timeout / unknown_result / contract_mismatch); `UnknownResult`, `SafeRetry`, `ManualReviewRequired`, `RetryAfter` fields.
- **Token version locking**: `TokenVersion` on `ShopAuthToken`; stale-write rejection (`DOUYIN_TOKEN_VERSION_CONFLICT`); singleflight dedup.
- **Order detail**: `GetOrderDetail` via `order.orderDetail`; maps to `PlatformOrder` via existing helpers.
- **Inventory query**: `GetSKUStockFromDetail` reusing `product.detail` (no undocumented stock API used).
- **Customer messaging**: `blocked_by_contract_verification` gated; DTO + fixture helpers ready.
- **Webhook**: `DouyinSignatureVerifier` (SHA1), `DouyinVerifier` adapter, `HandleDouyinPlatformEvent` dispatcher; Douyin + Jinritemai envelope parsing; unknown tag safe ACK.
- **OAuth state DB**: `DouyinOAuthState` model; one-time consume; redirect_uri allowlist.
- **Image idempotency**: `DouyinImageAsset` cache by `(shop_id, content_hash)`; unknown_result on upload timeout.
- **Product draft idempotency**: key `douyin-product-draft-create:{shopId}:{draftId}:{version}`; `tryRecoverDouyinDraftFromPlatform` on unknown_result.
- **AI apply idempotency (P2-DEBT-001)**: `applyAIContent` wrapped with `idempotency.Acquire/Complete/Fail`.
- **DouyinSyncCursor**: order sync cursor model with version-monotonic upsert.
- **configstatus**: fixed `platform_douyinshop` → `platform_douyin_shop`; 9 P3 Douyin capability items added.
- **Migrations**: `migrate_p3_douyin.go` (AutoMigrate new models + ALTER TABLE column additions).
- **Failure classifier**: 16 `douyin_*` task type strings added.
- **Tests**: all unit tests pass (webhook sig, token version conflict, order detail parse, customer blocked, facade compile, transport 429).
- **Docs**: 12 design docs + audit matrix + adapter report.
- Status: **抖店 Production Adapter Implemented** · **Real Credential Verification Deferred** · **只创建平台草稿 / 不自动上架** · **代码实现完成不等于真实 E2E** · **非 Production Ready**.

### Phase P2.2 — Reliability closure (2026-07-11)

- AI text/image apply + undo via shared `idempotency.Service` (`AITextApply`/`AITextUndo`/`AIImageApply`/`AIImageUndo`); version conflict codes; concurrency unit tests.
- Public Webhook HTTP receiver: `POST /api/v1/webhooks/:platform/:eventType` (signature, clock skew, body limit, fast ACK, DB-poll async worker); production signature bypass forbidden.
- tasklease adoption on collect / imagetask / customersync (plus existing ordersync / inventory / productpublish); stale worker finish guards.
- Docs + scan: `scripts/p2-2-reliability-closure-check.mjs` → `docs/P2_2_RELIABILITY_CLOSURE_REPORT.md`.
- Status: **Phase P2.2 Completed** · **Core Reliability Foundation Ready** · **非 Production Ready** · Final Acceptance Deferred · no Production Ready / gray / Douyin E2E claim.

### Phase P1 — Production infrastructure (2026-07-10)

- Multi-environment config profiles, production fail-fast validation, redacted startup summary.
- Health probes: `GET /health/live`, `GET /health/ready`.
- Storage public_base validation, settings storage public-check API alias, upload image decode hardening.
- Deploy assets: `deploy/nginx`, `deploy/systemd`, `deploy/scripts`, env profile examples.
- Config status center: environment, production safety, storage production boundary, project phase banner.
- P1 static scan: `scripts/p1-production-config-check.mjs`.

### Release Status

- **Production Capability Development In Progress**; F9 retained as historical Demo baseline; **Final Acceptance Deferred to Phase P10**.
- MVP Demo Ready; **Tag deferred**; **非 Production Ready**; gray release not allowed.
- Douyin Shop remains Release Candidate until real read/write E2E (P3/P10).
- Phase H1.1 adds URL-based state restoration for key operation workbenches.
- Phase H1.2 extends URL state to orders, product drafts, inventory, and customer service list pages (filters, pagination, drawers, legacy deep links).
- Phase H1.2.1 browser spot-check on H1.1 + H1.2 pages; fixes Dashboard `productSource` split, mount URL hydration, drawer reset, and drafts source sync.
- Phase H1.3 AI image warning codes (explainable/locatable/recoverable), batch overview, failure-task categories, config-status linkage, Douyin/Storage precheck banners; no real Douyin E2E.
- Phase H1.4 order/exception URL state (`status`, `fulfillmentStatus`, `severity`, date range via `start`/`end`), keyword UX (max 80, sensitive hint, clear syncs URL); browser/responsive spot-check passed_with_warning.
- Phase H1.5.1 live Chrome browser sign-off (13/13 core cases, back/forward/refresh), real 1366×768 and 1024×768 screenshots, AI image baseline **stable_range_14_to_15_of_16** (14/16 this run); ProTable URL hydration fixes.
- Phase H1.5 secondary list URL state (publish batches, collect, order/customer sync, AI text/image batches), browser back/forward sign-off; 1366/1024 responsive passed / passed_with_warning.

### Added

- H1.5.1 reports: `docs/H1_5_LIVE_BROWSER_ACCEPTANCE.md`, `docs/H1_5_AI_IMAGE_BASELINE_CONFIRMATION.md`, `scripts/h1-5-live-browser-acceptance.mjs`, `scripts/h1-5-live-browser-acceptance-check.mjs`.
- H1.5 reports: `docs/H1_5_SECONDARY_URL_BROWSER_CHECK.md`, `docs/h1-5-secondary-url-browser-check.json`, `scripts/h1-5-secondary-url-browser-check.mjs`.
- H1.5.1: ProTable `params` URL seeding on orders, exceptions, task center, drafts, conversations; AI workbench compare-before-write URL sync.
- H1.4 frontend: `keywordSafety.ts`, `KeywordSafetyHint.tsx`, `useKeywordSearchField.ts`.
- H1.3 guides: `docs/AI_IMAGE_WARNING_RECOVERY_GUIDE.md`, `docs/DOUYIN_E2E_PRECHECK_GUIDE.md`, `docs/STORAGE_PUBLIC_URL_GUIDE.md`.
- H1.3 backend `aiproductimage/warning_codes.go`; frontend `aiImageWarnings.ts`, platform precheck banners.
- H1.2.1 reports: `docs/H1_2_URL_STATE_BROWSER_CHECK.md`, `docs/h1-2-url-state-browser-check.json`, `scripts/h1-2-url-state-browser-check.mjs`.
- H1.2 URL query state for `/orders/list`, `/orders/exceptions`, `/product/drafts`, `/inventory`, `/inventory/alerts`, `/inventory/sync-tasks`, `/customer/hub`, `/customer/conversations`.

- Full-project Demo dataset, RBAC demo accounts, route/API smoke scripts, and `pnpm demo:auto-acceptance`.
- AI product operations workbench, batch AI text/image review flows, multi-product publishing drafts, and full-chain dashboard coverage.
- Order, inventory, customer service, RBAC, configuration status, and failure task center completion for the MVP demo loop.

### Initial Foundation

- Open-source governance configuration: CODEOWNERS, Dependabot, PR labeler, and Docker Compose config CI.
- AI coding support docs: module map, environment variable reference, API contract, Provider template, and task checklist.

### Changed

- GitHub Actions can be triggered manually with `workflow_dispatch`.

## v0.1.0

### Added

- Initial TradeMind monorepo foundation.
- Go backend, React admin, Node collector, PostgreSQL, Redis, and Docker Compose development infrastructure.
- Open-source documentation foundation: README, contribution guide, issue templates, PR template, security policy, code of conduct, sponsor page, and Apache-2.0 license.

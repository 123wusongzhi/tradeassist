# Changelog

All notable changes to TradeMind are documented here.

This project follows a lightweight changelog format before the first stable release. Keep entries short, grouped by date or version, and link large changes back to the relevant PR when available.

## Unreleased

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

# Changelog

All notable changes to TradeMind are documented here.

This project follows a lightweight changelog format before the first stable release. Keep entries short, grouped by date or version, and link large changes back to the relevant PR when available.

## Unreleased

### Release Status

- Phase F9 final acceptance completed: MVP Demo Ready, P0/P1 cleared, and the demo tag is deferred for Post-F9 enhancement.
- Production Ready and gray release remain blocked by environment, public storage, and real Douyin credentials.
- Douyin Shop remains Release Candidate until real read/write E2E and gray observation complete.
- Phase H1.1 adds URL-based state restoration for key operation workbenches.
- Phase H1.2 extends URL state to orders, product drafts, inventory, and customer service list pages (filters, pagination, drawers, legacy deep links).
- Phase H1.2.1 browser spot-check on H1.1 + H1.2 pages; fixes Dashboard `productSource` split, mount URL hydration, drawer reset, and drafts source sync.
- Phase H1.3 AI image warning codes (explainable/locatable/recoverable), batch overview, failure-task categories, config-status linkage, Douyin/Storage precheck banners; no real Douyin E2E.
- Phase H1.4 order/exception URL state (`status`, `fulfillmentStatus`, `severity`, date range via `start`/`end`), keyword UX (max 80, sensitive hint, clear syncs URL); browser/responsive spot-check passed_with_warning.

### Added

- H1.4 reports: `docs/H1_4_URL_KEYWORD_RESPONSIVE_CHECK.md`, `docs/h1-4-url-keyword-responsive-check.json`, `scripts/h1-4-url-keyword-responsive-check.mjs`.
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

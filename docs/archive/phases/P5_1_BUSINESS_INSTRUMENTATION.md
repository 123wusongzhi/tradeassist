# P5.1 Business Instrumentation

Current status: incomplete.

The metric catalog includes P5.1 metric names for providers, tasks, webhooks, orders, inventory, AI text, AI image, file scanning, secret rotation, auth/security, DB, telemetry, and SLO. However, real module calls are not yet fully wired across all required business paths.

Known incomplete paths:

- `backend/internal/pkg/httpclient`
- `backend/internal/modules/webhook`
- `backend/internal/modules/ordersync`
- `backend/internal/modules/inventory`
- `backend/internal/modules/aiproducttext`
- `backend/internal/modules/aiproductimage`
- `backend/internal/modules/files`
- `backend/internal/modules/securitymod`
- `backend/internal/modules/auth`

Do not mark `Business Instrumentation Ready` until `scripts/p5-1-observability-closure-check.mjs` reports `failed=0` and module-level tests execute real service/worker methods.

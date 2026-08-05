---
doc_type: runbook
audience: maintainer
status: current
owner: maintainers
source_of_truth:
  - package.json
  - scripts/
review_cycle_days: 90
---

# Performance harness environment variables

Performance / load harness variables are **not** part of ordinary local development. Prefer stable app variables in `docs/reference/configuration/environment.md`.

Typical harness variables (names come from scripts under `scripts/`):

| Variable | Notes |
| --- | --- |
| `P7_V2_API_HOST` | Loopback API host for harness only |
| `P7_V2_API_PORT` | Harness API port |
| `P7_BASE_URL` | Must match host/port above |
| `P7_DIAGNOSTICS_*` | Local diagnostics for harness runs |

Do not point harness variables at production endpoints. Evidence outputs belong under `artifacts/` or CI artifacts, not current product docs.

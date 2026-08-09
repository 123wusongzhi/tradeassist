# Phase P4-R Permission Seed Root Cause

## Finding

`seed-demo-permissions.ps1` created or reused demo users but did not expose a versioned permission template or structured result. Existing demo roles/users could be confused with a failed seed when wrapped by the acceptance orchestrator.

## Fix

- Added `DEMO_PERMISSION_TEMPLATE_VERSION=p4-r-v1`.
- Added structured JSON result and counters.
- Added production/staging guard: `DEMO_SEED_FORBIDDEN_IN_PRODUCTION`.
- Kept standard demo roles scoped to explicit demo users and explicit first shop grant.
- Existing demo users are treated as unchanged, not failed.

## Exit Codes

| Exit | Meaning |
| --- | --- |
| 0 | latest template present or safely updated |
| 1 | code_failed |
| 2 | environment_blocked |
| 3 | validation_conflict |
| 4 | manual_action_required |

## Boundary

The seed does not reset custom roles and does not grant all shops to operator by default.

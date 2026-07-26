# P7-V2-R3B Fast-Close R2 Truth Audit

Status: **passed — the repaired harness is eligible to begin Recovery5.**

## Evidence examined

- Current scripts under `scripts/p7-v2-*`, the R3B manifest, k6 load scripts, and P7-V2 gate fixtures.
- Fixture commands executed on 2026-07-15: `test:p7-v2-gates`, `test:p7-v2-regression`, `test:p7-v2-load-profile-fingerprint`, `test:p7-v2-process-identity`, `test:p7-v2-current-independence`, `test:p7-v2-artifact-freeze`, `test:p7-v2-port-owner`, and `test:p7-v2-fast-close`. All passed, but their coverage does not prove the complete formal harness.

## Findings

| Check | Result | Current-tree evidence |
| --- | --- | --- |
| Fast-close orchestration | Passed | The orchestrator supports fail-fast, `--resume-from`, `--stop-after`, and a non-mutating `--dry-run`; generated IDs are Recovery5 IDs. |
| p99 emission | Passed | `tests/load/p7v2-baseline.js` explicitly requests `p(99)`. |
| Missing p99 semantics | Passed | Missing metric values remain absent; the regression parser classifies missing p99 as `summary_stat_missing`, null/non-finite values as invalid, and preserves zero as an explicit value. |
| Load-profile fingerprint | Passed | Formal load reports now derive the canonical V2 fingerprint from stages, scenarios, request/credential mix, and the load-script hash. |
| Independent scenario metrics | Passed | Task List and Webhook Ingestion have distinct steady metrics; negative auth/security scenarios are split and tag stable identifiers. |
| Steady/sample gates | Passed | Formal scenario reports use steady counters and enforce 100 samples. |
| Cooldown evidence | Passed | Runtime probes collect health, PostgreSQL, RSS, and goroutine evidence; unavailable evidence fails explicitly and topology exceptions use `not_applicable`. |
| Canonical demo manifest | Passed | Demo preflight, demo runs, and final gates read the R3B canonical manifest; preflight also consumes the V2 comparability and regression reports. |
| Artifact freezing | Passed | Atomic copy, hash verification, and Recovery5 run-ID guards are enforced. |

## Recovery4 preservation

The Recovery4 raw artifacts, manifests, hashes, and failed comparability evidence remain untouched. They must remain marked `validForRegression=false`, `superseded=true`, with reason `load_profile_fingerprint_semantics_changed_after_execution`; they cannot be used for Recovery5 regression.

## Re-verification

All existing P7-V2 fixtures plus the new cooldown and canonical-manifest fixtures passed. `pnpm p7-v2:r3b:fast-close -- --dry-run` confirmed the required fail-fast order without starting k6 or deleting a database. Workspace and backend verification also passed.

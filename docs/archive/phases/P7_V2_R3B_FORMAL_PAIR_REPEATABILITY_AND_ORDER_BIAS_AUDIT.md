# P7-V2-R3B Formal Pair Repeatability And Order Bias Audit

Status: **incomplete**

- Diagnostic only: `true`
- Formal: `false`
- Valid for closure: `false`
- Failed baseline run: `p7v2-baseline-r3b-recovery6-20260718001301`
- Failed current run: `p7v2-current-r3b-recovery6-20260718001301`
- Failed runtime freeze: `2af8b39b123a132b56b983ab4de0d4181771b794d817ff1f406b512fd927217d`
- Formal rerun started: `false`

## Failure State

The failed Recovery6 formal pair remains valid for historical audit and invalid for closure. The formal regression failed three material relative-regression metrics:

| Scenario | Stat | Baseline | Current | Relative Delta | Verdict |
| --- | ---: | ---: | ---: | ---: | --- |
| Webhook Ingestion | p95 | 16.44 ms | 26.94 ms | +63.83% | failed_material_regression |
| Webhook Ingestion | p99 | 27.43 ms | 71.29 ms | +159.87% | failed_material_regression |
| Auth Invalid Login | p99 | 31.99 ms | 39.86 ms | +24.60% | failed_material_regression |

## Process Identity Probe V2

The process identity tooling is repaired at fixture level:

- `processIdentityProbeVersion=2`
- Linux/WSL probe method: `linux_procfs`
- Windows WSL probe method: `windows_wsl_exe`
- Linux path does not spawn `wsl.exe`
- External `wsl.exe` shims are rejected with a non-zero semantic failure
- PID reuse changes block kill identity matching

## Repeatability Matrix

The bounded diagnostic matrix has not been executed yet.

Required order:

```text
B-C-C-B
```

Required run labels:

```text
B1
C1
C2
B2
```

Current status:

- Run count: `0`
- Input sequence hash match: `false`
- Branch mix fingerprint match: `false`
- Binary provenance passed: `false`
- Baseline self variance calculated: `false`
- Current self variance calculated: `false`
- Cross-version variance calculated: `false`
- Root cause classified: `false`

Primary root cause is therefore not classified yet:

```text
not_classified_bccb_repeatability_matrix_not_executed
```

## Guardrails

No threshold, materiality, SLO, VU, stage, duration, dataset, request mix, or business-code change is recorded in this audit report. No new formal pair, soak, demo, stability, race, cleanup, final gate, tag, release, production resource, real Provider, real Douyin call, auto-listing, or Production Ready declaration was started by this audit.

Next minimum action:

```text
Complete read-only binary/input binding audit, then run exactly four diagnostic runs in B-C-C-B order with identical medium dataset, load profile, seeds, request sequence, and branch mix.
```

# P7-V2-R3B-REBASELINE2 Baseline Report

Status: **blocked**

- Run ID: `p7v2-baseline-r3b-recovery2-20260715-022554`
- Command: `pnpm p7-v2:env:start -- --run-id p7v2-baseline-r3b-recovery2-20260715-022554`
- Exit code: `1`
- Blocker: port 8080 remained occupied before API start.

No Dataset, k6 load, raw Artifact freeze, Registry update, Current, Comparability, or Regression V2 was executed. The explicitly created isolated database was dropped; the pre-existing unknown port owner was not stopped.

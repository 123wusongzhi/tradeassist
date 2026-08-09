# P7-V2-R3B-REBASELINE2 Final Report

Status: **Incomplete**

## Blocker

- Step: Formal Baseline environment start
- Run ID: `p7v2-baseline-r3b-recovery2-20260715-022554`
- Command: `pnpm p7-v2:env:start -- --run-id p7v2-baseline-r3b-recovery2-20260715-022554`
- Exit code: `1`
- Reason: port `8080` remained occupied before API start.

## Evidence

- Runtime Source Tree Hash: `089b708a0f8a84273a95de07f3dee541bd790b465c89b39c6cf2ce1f1f8aa98a`
- Load Scripts Hash: `a8ea65b2d995566e09c80a9aa236c4c5ba19bf66fd6c7c2d37aaa6304ecef39c`
- Metric Semantics Hash: `91ff96bf17153a63bfc96a0bdde75fefcf9a6bf096cd3bccf36cd9874c0b6f68`
- Dataset Fingerprint: unavailable; Dataset was not executed.
- SLO Fingerprint: `7045791c5d4478ca41227917c350dac19e252fabda52dc9d29492fd306638c40`

No raw Artifact was created, so no Baseline Registry update, Current, Comparability, or Regression V2 was permitted. The exact isolated database created for this attempt was dropped. The unknown process occupying port `8080` was not stopped.

## Required repair

Identify and stop or relocate the port `8080` owner, then use new Baseline and Current Run IDs to restart the complete chain. The old recovery Baseline remains preserved and must not be reused.

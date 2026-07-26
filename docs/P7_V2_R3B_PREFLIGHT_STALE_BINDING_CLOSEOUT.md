# P7-V2-R3B Preflight Stale Binding Closeout

Status: **blocked**

- Classification: `recovery6_preflight_stale_plan_and_runtime_freeze_binding_defect`
- Failed command: `pnpm p7-v2:r3b:preflight -- --recovery6`
- Failed preflight exit code: `0`
- Failed preflight status: `passed`
- Semantic gate passed: `false`
- Failed preflight resolved baseline: `p7v2-baseline-r3b-recovery6-20260715165422`
- Expected baseline: `p7v2-baseline-r3b-recovery6-20260716082252`
- Failed preflight runtime freeze created: `false`
- Actual runtime freeze ID: `7ede969be242469202e7273d669f6ad7a23a03195aad79d96d278c4f9b2f4b31`
- Actual runtime freeze superseded: `true`
- Supersede reason: `preflight_fresh_plan_binding_tooling_changed`

Fresh Run IDs were not consumed:

- Baseline: `p7v2-baseline-r3b-recovery6-20260716082252`
- Current: `p7v2-current-r3b-recovery6-20260716082252`
- Soak: `p7v2-soak-r3b-recovery6-20260716082252`
- Demo 1: `p7v2-demo1-r3b-recovery6-20260716082252`
- Demo 2: `p7v2-demo2-r3b-recovery6-20260716082252`

No formal environment, dataset, k6 run, raw artifact, or frozen artifact was created from these Run IDs.

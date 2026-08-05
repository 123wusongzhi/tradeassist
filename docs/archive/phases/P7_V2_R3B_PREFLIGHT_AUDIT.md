# P7-V2-R3B Preflight Audit

Status: **failed**

| Check | Result |
| --- | --- |
| Baseline Run ID | `p7v2-baseline-r3a-20260714225500` |
| Baseline status / immutable / valid for regression | passed / true / true |
| Baseline requests / scenario coverage | 29,475 / true |
| Expected raw SHA-256 | `1c324a90bcd53dc370620b97a9bd6e5d144b47439000e1fdbe4795f05954bf9e` |
| Frozen raw artifact available | false |
| Runtime raw artifact available | false |
| Baseline artifact hash verified | false |
| Frozen runtime source tree hash | `b7fe3eb3698ba3f40ae74878f7c666afbcdbd6fccf27220813bca2aa738cc282` |
| Current runtime source tree hash | `93c8687dbdd9b4e215b2bebc9075570e525d8e96cc7699f9f8ea97a5aea49bf9` |
| Runtime source tree match | false |
| k6 | `v0.57.0` |
| Current execution allowed | false |

## Blocking evidence

- `docs/baselines/frozen/p7v2-baseline-r3a-20260714225500/manifest.json` references `artifacts/p7-v2/baseline/p7v2-baseline-r3a-20260714225500/baseline.summary.json`.
- Neither the frozen evidence directory nor the referenced runtime artifact path contains `baseline.summary.json`; the required SHA-256 cannot be recomputed.
- The runtime source tree no longer matches the frozen baseline fingerprint.

No Current, Regression, Soak, or Demo command was started. This preserves the failed preflight evidence and prevents an invalid baseline comparison.

## Required remediation

Restore the exact immutable raw baseline artifact from trustworthy retained evidence. If the runtime source change is intentional, establish and freeze a new baseline before executing the R3B validation chain.

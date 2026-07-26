# P7-V2-R3 Preflight Audit

Status: **passed**

| Field | Value |
| --- | --- |
| Baseline run ID | p7v2-baseline-20260714181000 |
| Historical baseline comparable | false |
| k6 | k6 v0.57.0 (go1.25.12, linux/amd64) |
| Runtime source hash | c87d513fc04171f8bd073197dd81150b00abaa80d23877986bdc71448b4897a7 |
| Dataset fingerprint | 0dd02fbd730cba2e8503690ccaf7a1104f88d35d77cd94a7bd8016efd0a340be |

## Issues
- historical baseline lacks immutable fingerprints: trackedDiffHash, untrackedRuntimeManifestHash, runtimeSourceTreeHash, apiSourceHash, loadScriptHash, sloFingerprint, routeCredentialMatrixFingerprint
- historical baseline does not prove non-zero k6 traffic

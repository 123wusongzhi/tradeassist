# P7-V2-R3B-LPC-R3 Stage Source Audit

Status: **passed**

- Failed Recovery5 run: `p7v2-baseline-r3b-recovery5-20260715091700`
- Failed field: `stages[0].targetVUs`
- Raw source: `scripts/p7-v2-load.mjs` built a profile with `configuredVUs`, but no explicit `stages`.
- Expansion source: `scripts/p7-v2-load-profile-fingerprint.mjs` generated the warmup stage and read `profile.targetVUs`, which was absent.
- No credentials, tokens, DSNs, cookies, headers, or PII are recorded.

The machine-readable stage evidence is in `docs/p7-v2-r3b-lpc-r3-stage-source-audit.json`.

# P5-V Development Acceptance Run 2

Phase: P5-V
Status: passed_with_blocked

## Command

```text
pnpm demo:auto-acceptance
```

## Environment

- API: `http://127.0.0.1:8080`
- Started: `2026-07-13T04:53:28.3915703Z`
- Finished: `2026-07-13T05:05:22.8419963Z`
- Raw report: `docs/demo-auto-acceptance.run2.json`

## Summary

- total: 21
- passed: 19
- warning: 1
- blocked: 1
- deferred: 0
- failed: 0
- codeFailed: 0
- nonAiFailed: 0

## Classification

- `ai-text-trial-run`: blocked, `environment_blocked`, external provider only.
- `ai-image-trial-run`: warning, `provider_timeout`, external provider only.
- Seed idempotency, smoke paths, builds, and code checks remained stable on the second run.

Run 2 confirmed repeatability with `failed=0`, `codeFailed=0`, and `nonAiFailed=0`. Real AI provider completion remains outside the code-level closure gate.

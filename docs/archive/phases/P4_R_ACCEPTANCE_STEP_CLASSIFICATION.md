# Phase P4-R Acceptance Step Classification

## Result Model

```json
{
  "runId": "",
  "startedAt": "",
  "finishedAt": "",
  "summary": {
    "total": 0,
    "passed": 0,
    "warning": 0,
    "blocked": 0,
    "deferred": 0,
    "failed": 0,
    "codeFailed": 0,
    "nonAiFailed": 0
  },
  "steps": []
}
```

## Categories

| Category | Examples | Failed Means |
| --- | --- | --- |
| `code_check` | go test, doc links, static scans | code failure |
| `build` | backend/admin/collector build | code failure |
| `seed` | demo data, demo permissions | code failure unless environment blocked |
| `smoke` | internal route/API smoke | code failure |
| `external_provider` | AI text/image provider trials | blocked/warning when backed by provider evidence |
| `real_credential` | real Douyin credential E2E | deferred in P4-R |

## Rules

- Non-AI code/build/seed/smoke failures count as `nonAiFailed`.
- Provider missing credentials count as `blocked`, not `failed`.
- Real credential and real environment security checks remain `deferred`.
- A timeout requires evidence before being classified.

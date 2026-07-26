# Production Configuration Design (Phase P1)

## Goals

- Multi-profile env loading without leaking dev defaults into production.
- Fail-fast on insecure or missing production configuration.
- Redacted startup summary in logs.

## Load priority

```text
code safe defaults → profile env file → system environment → APP_ENV_FILE override
```

## Profiles

| APP_ENV | Local storage | Demo seed | Dev routes |
| --- | --- | --- | --- |
| development | yes | default on | default on |
| demo | yes | default on | off |
| test | yes | off | off |
| staging | no | off | off |
| production | no | off | off |

## Fail-fast (production)

Blocking: missing DB, weak JWT, missing APP_MASTER_KEY, missing public URLs, demo seed / dev routes enabled.

Degraded (warn in config status): AI/OCR/抖店 credentials, storage public_base not E2E tested.

## Implementation

- `backend/internal/config/config.go` — env fields
- `backend/internal/config/validate.go` — production gates
- `backend/internal/config/summary.go` — log redaction

See also: [ENVIRONMENT_PROFILE_GUIDE.md](ENVIRONMENT_PROFILE_GUIDE.md), [SECRET_MANAGEMENT_GUIDE.md](SECRET_MANAGEMENT_GUIDE.md).

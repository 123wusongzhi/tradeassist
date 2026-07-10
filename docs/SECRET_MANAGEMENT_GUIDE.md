# Secret Management Guide (Phase P1)

## Categories

Database password, Redis password, JWT secret, APP_MASTER_KEY, AI keys, Storage secrets, 抖店 App Secret, OAuth tokens.

## Rules

1. API never returns full secret values — masked or configured/not-configured only.
2. Logs and operation logs must not contain secrets or full connection strings.
3. Startup logs use `config.RedactedSummary()` only.
4. Production requires `APP_MASTER_KEY` for settings encryption.

## Storage of runtime secrets

- Env bootstrap: JWT, DB, Redis, APP_MASTER_KEY.
- Business secrets: encrypted in `settings` table via AES-GCM.

Field-level DB encryption deepens in Phase P4; interface reserved via `encrypt.Service`.

# Environment Profile Guide

## Supported profiles

`development` · `demo` · `test` · `staging` · `production`

## Templates (committed)

- `.env.example` — local default
- `.env.development.example`
- `.env.demo.example`
- `.env.staging.example`
- `.env.production.example`

## Server deployment

1. Copy `.env.production.example` → `/etc/trademind/trademind.env` (mode `600`).
2. Set `APP_ENV=production` in systemd `EnvironmentFile`.
3. Do **not** commit `.env.production` (gitignored).

## Production dotenv rule

When `APP_ENV=production`, the API loads only `.env.production` or `APP_ENV_FILE` — never `.env.local` or development `.env`.

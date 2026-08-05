# P4 Production Debug Surface

Debug endpoints, dev routes, and configuration flags that must be disabled in production.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Overview

Production deployments must minimize attacker-visible surface: API documentation, demo seeders, debug handlers, and verbose health details. Phase P4 tracks these via config flags and `configstatus/p4_status.go`.

---

## Debug Flags

| Flag | Env Var | Default (production) | Purpose |
| --- | --- | --- | --- |
| Swagger UI | `ENABLE_SWAGGER` | `false` | OpenAPI browser |
| Dev routes | `ENABLE_DEV_ROUTES` | `false` | Internal dev-only HTTP routes |
| Debug endpoints | `ENABLE_DEBUG_ENDPOINTS` | `false` | Verbose/debug API |
| Demo seed | `ENABLE_DEMO_SEED` | `false` | Demo data seeder API |

Loader: `backend/internal/config/config.go`.

### Production Validation

`config.validate()` fails startup in production when insecure combinations detected (see `validate.go`):

- Dev routes + demo seed together
- Flags enabled when `APP_ENV=production` (warnings/errors per rule)

P4 config status item `p4.debug_surface` marks **not_ready** if any debug flag true in production.

Source: `backend/internal/modules/configstatus/p4_status.go`.

---

## Security Overview Exposure

`GET /api/v1/security/overview` (requires `settings.manage`) returns:

```json
{
  "productionDebugSurface": true|false,
  "authSessionMode": "...",
  "jwtActiveKeyId": "...",
  ...
}
```

Computed as:

```go
cfg.EnableDebugEndpoints || cfg.EnableSwagger || cfg.EnableDevRoutes
```

Does **not** expose secrets — key IDs and TTLs only.

Source: `backend/internal/modules/auth/sessions_handler.go` (`SecurityOverview`).

---

## Demo Seed Routes

Registered only when:

```go
dep.Config.EnableDemoSeed && !config.IsProduction(dep.Config.AppEnv)
```

Module: `demoseed.Register(authed, demoSeedH)`.

Never mount in production builds.

---

## Health Endpoint

| Route | Auth | Notes |
| --- | --- | --- |
| `GET /health` | Public | Basic liveness |
| `GET /api/v1/health` | Public | DB + Redis status |

Health may include environment hints when demo/dev flags set — review `backend/internal/health/health.go`.

---

## Static & Public Routes

| Route | Risk |
| --- | --- |
| `GET /static/*` | File exposure — ensure non-sensitive content |
| `POST /api/v1/webhooks/*` | Expected public; signature required |
| `GET /api/v1/shops/oauth/*/callback` | OAuth callbacks — state parameter validation |
| `POST /api/v1/auth/login` | Public — rate limited |

---

## Collector Alias Routes

`/api/collector/*` duplicates collect browser login helpers with Bearer auth — not a debug surface but increases route count. Same auth middleware as admin API.

---

## Gin Mode

Production should set:

```text
GIN_MODE=release
APP_ENV=production
```

Disables Gin debug route registration and verbose error pages.

---

## Security Headers (Related)

Even with debug off, baseline headers apply:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Strict-Transport-Security` (production)
- `Content-Security-Policy` baseline

Source: `backend/internal/pkg/security/headers.go`, mounted in `cmd/server/main.go`.

---

## Pre-Production Checklist

| Check | Command / Method |
| --- | --- |
| Debug flags false | Env audit |
| `AUTH_SESSION_MODE=secure_session` | Env |
| `AUTH_SECURE_COOKIE=true` | Env |
| Swagger 404 | `curl /swagger/index.html` |
| Demo seed 404 | `curl /api/v1/demo-seed/...` |
| Config status P4 green | Admin config-status page |
| `productionDebugSurface: false` | Security overview API |

---

## CI / Config Status Integration

`configstatus.appendP4SecurityItems` exposes rows in admin UI:

- `p4.debug_surface` — production debug flag audit
- `p4.real_verification` — manual_required for pen test

Navigate: `/settings/security` (SettingsURL in config status items).

---

## Known Gaps

- No centralized route manifest diff in CI
- Swagger mount path depends on registration code path — verify per release
- Error responses may leak internal messages in non-production env

---

## Deferred Verification

- [ ] Production deployment audit script
- [ ] External port scan of staging/prod
- [ ] Verify 404 on all dev routes post-deploy

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

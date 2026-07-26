# Nginx Production Guide

Templates:

- `deploy/nginx/trademind.conf` — split admin + API hostnames
- `deploy/nginx/trademind-staging.conf` — single staging host

## Routing

| Path | Target |
| --- | --- |
| `/` | Admin `dist` + `try_files … /index.html` |
| `/api/` | Backend upstream |
| `/static/` | Backend (local storage) |
| `/health`, `/health/*` | Backend probes |

## Cache

- Hashed assets: long cache
- `index.html`: no long-term cache

## Security headers

See template: `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`.

## Upload limit

`client_max_body_size 20m` (align with `UPLOAD_MAX_MB`).

See [PRODUCTION_DOMAIN_HTTPS_GUIDE.md](PRODUCTION_DOMAIN_HTTPS_GUIDE.md) for TLS.

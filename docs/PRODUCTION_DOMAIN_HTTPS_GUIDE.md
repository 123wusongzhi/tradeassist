# Production Domain & HTTPS Guide

## Recommended layout

| Role | Example |
| --- | --- |
| Admin | `https://admin.example.com` |
| API | `https://api.example.com` or same host `/api/` |

Set `ADMIN_PUBLIC_URL` and `API_PUBLIC_URL` in production env.

## DNS

Point A/AAAA records to Nginx host.

## TLS

Use Certbot with templates in `deploy/nginx/trademind.conf`:

```bash
certbot certonly --nginx -d admin.example.com -d api.example.com
```

Enable auto-renewal via systemd timer.

## CORS

Configure allowed origins at reverse proxy or future middleware using `ADMIN_PUBLIC_URL`.

## Cookies

Production: `Secure`, `SameSite=Lax` or `Strict` when serving Admin over HTTPS.

## Verification

- `curl -sf https://api.example.com/health/live`
- `curl -sf https://api.example.com/health/ready`
- Run storage public check from Admin after bucket/CDN setup.

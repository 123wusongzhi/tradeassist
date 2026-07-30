# Storage Production Design

## Providers

| Kind | Use |
| --- | --- |
| local | development / demo only |
| cos / oss / s3 / r2 / minio | staging / production |

Factory: `backend/internal/providers/storage/factory.go`

## public_base

Must pass `storagepublic.ValidatePublicBase` for staging/production (HTTPS, no private hosts).

## E2E test

`POST /api/v1/storage/test-public-access`  
`POST /api/v1/settings/storage/public-check` (alias)

Probe prefix: `system-tests/storage-public-check/`

## Production boundary

Config status item `storage_production` blocks local provider in staging/production.

See [STORAGE_PUBLIC_CHECK_GUIDE.md](STORAGE_PUBLIC_CHECK_GUIDE.md).
